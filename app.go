package signal

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"time"

	"github.com/alexfalkowski/go-signal/v2/internal/lifecycle"
	"github.com/alexfalkowski/go-sync"
	"go.uber.org/dig"
)

// NewApp registers constructors and decorators, then runs setup functions. It
// returns initialization errors directly. Dig recovers panics in constructors,
// decorators, and setup functions. Resource acquisition belongs in start hooks
// so initialization failures do not leave running resources behind.
func NewApp(opts ...Option) (*App, error) {
	o := options{startTimeout: 15 * time.Second, stopTimeout: 15 * time.Second}
	for _, opt := range opts {
		opt(&o)
	}

	if min(o.startTimeout, o.stopTimeout) <= 0 {
		return nil, ErrInvalidTimeout
	}

	shutdown := make(chan int, 1)
	app := &App{
		lifecycle:    lifecycle.New(),
		shutdown:     shutdown,
		startTimeout: o.startTimeout,
		stopTimeout:  o.stopTimeout,
	}

	container := dig.New(dig.RecoverFromPanics())
	err := container.Provide(func() (Lifecycle, Shutdowner) {
		return app.lifecycle, &shutdowner{requests: shutdown}
	})
	if err != nil {
		return nil, err
	}

	for _, constructor := range o.constructors {
		if err := container.Provide(constructor); err != nil {
			return nil, err
		}
	}

	for _, decorator := range o.decorators {
		if err := container.Decorate(decorator); err != nil {
			return nil, err
		}
	}

	for _, function := range o.functions {
		if err := container.Invoke(function); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// App owns a constructed application's lifecycle, deadlines, and shutdown
// request for one execution. Build a fresh App for each command or retry; reuse
// after startup failure or shutdown is unsupported. Shutdown is safe to request
// concurrently through the injected Shutdowner; register hooks before Run or Serve.
type App struct {
	lifecycle    *lifecycle.Lifecycle
	shutdown     <-chan int
	startTimeout time.Duration
	stopTimeout  time.Duration
}

// Run starts and immediately stops a short-lived command. Command work belongs
// in start hooks. A non-zero Shutdown request is returned as an [ExitError],
// joined with any startup or cleanup errors.
func (a *App) Run(ctx context.Context) error {
	if err := a.start(ctx); err != nil {
		return err
	}

	var code int
	select {
	case code = <-a.shutdown:
	default:
	}

	return a.finish(ctx, code)
}

// Serve starts the application, waits for parent cancellation, SIGINT, SIGTERM,
// any additional signal, or Shutdown, then stops with a fresh shutdown deadline.
// It unregisters its signal notifications on return. Normal cancellation
// returns nil unless startup or cleanup fails.
// An already-buffered Shutdown request preserves its exit code, including when
// cancellation or another error ends startup.
func (a *App) Serve(ctx context.Context, signals ...Signal) error {
	signals = append([]Signal{Interrupt, Termination}, signals...)

	ctx, stop := signal.NotifyContext(ctx, signals...)
	defer stop()

	if err := a.start(ctx); err != nil {
		return err
	}

	var code int
	select {
	case code = <-a.shutdown:
	case <-ctx.Done():
		select {
		case code = <-a.shutdown:
		default:
		}
	}

	return a.finish(ctx, code)
}

// start bounds startup waiting separately from rollback. A callback can outlive
// cancellation; its successfully started hooks still need cleanup afterward.
func (a *App) start(ctx context.Context) error {
	startCtx, cancel := context.WithTimeoutCause(ctx, a.startTimeout, ErrTimeout)

	// Publish the startup result separately so Await only reports wait cancellation.
	// A successful await synchronizes access to startupErr with task completion.
	var startupErr error
	task := sync.Go(startCtx, func(ctx context.Context) error {
		startupErr = a.lifecycle.Start(ctx)
		return nil
	})
	err := task.Await(startCtx)
	cancel()

	switch {
	case err != nil:
		a.stopAfterStart(context.WithoutCancel(startCtx), task)
	case startupErr != nil:
		err = errors.Join(startupErr, a.stop(ctx))
	default:
		return nil
	}

	select {
	case code := <-a.shutdown:
		if code != 0 {
			err = errors.Join(err, &ExitError{Code: code})
		}
	default:
	}

	return err
}

// stop gives cleanup a fresh deadline even after the execution is canceled.
func (a *App) stop(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeoutCause(context.WithoutCancel(ctx), a.stopTimeout, ErrTimeout)
	defer cancel()

	task := sync.Go(stopCtx, a.lifecycle.Stop)
	return task.Await(stopCtx)
}

func (a *App) stopAfterStart(ctx context.Context, startup *sync.Task) {
	// A canceled wait can leave startup running. Roll back its completed hooks
	// after it returns, regardless of whether the remaining startup succeeds.
	sync.Go(ctx, func(ctx context.Context) error {
		<-startup.Done()

		return a.stop(ctx)
	})
}

func (a *App) finish(ctx context.Context, code int) error {
	err := a.stop(ctx)

	select {
	case code = <-a.shutdown:
	default:
	}

	if code != 0 {
		err = errors.Join(err, &ExitError{Code: code})
	}

	return err
}

type shutdowner struct {
	requests chan<- int
	once     sync.Once
}

// Shutdown records the first exit code without sending an OS signal.
func (s *shutdowner) Shutdown(code int) {
	s.once.Do(func() {
		s.requests <- code
	})
}

// ExitError reports a non-zero requested exit code. It can be found through
// joined startup or cleanup errors with [errors.As]. Callers own process termination.
type ExitError struct {
	// Code is the requested process exit code, preserved without normalization.
	Code int
}

// Error implements [error].
func (e *ExitError) Error() string {
	return fmt.Sprintf("signal: shutdown with exit code %d", e.Code)
}
