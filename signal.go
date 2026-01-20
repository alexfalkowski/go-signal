package signal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/alexfalkowski/go-sync"
)

// Timer will call Go with the given timeout which creates a timer to run at an interval.
func Timer(ctx context.Context, timeout, interval time.Duration, hook Hook) error {
	return Go(ctx, timeout, func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if err := hook.Start(ctx); err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				stopCtx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				return hook.Stop(stopCtx)
			case <-ticker.C:
				if err := hook.Tick(ctx); err != nil {
					return err
				}
			}
		}
	})
}

// ErrTerminated is returned when we need to terminate the program.
var ErrTerminated = errors.New("signal: terminated")

// Terminated wraps the given error with ErrTerminated.
func Terminated(err error) error {
	return fmt.Errorf("%w: %w", err, ErrTerminated)
}

// IsTerminated checks if the given error is ErrTerminated.
func IsTerminated(err error) bool {
	return errors.Is(err, ErrTerminated)
}

// Go waits for the handler to complete or timeout.
func Go(ctx context.Context, timeout time.Duration, handler Handler) error {
	return sync.Wait(ctx, timeout, sync.Hook{
		OnRun: sync.Handler(handler),
		OnError: func(_ context.Context, err error) error {
			if IsTerminated(err) {
				_ = Shutdown()
			}

			return err
		},
	})
}

// Handler used for hook.
type Handler func(context.Context) error

// Hook for operations.
type Hook struct {
	OnStart Handler
	OnTick  Handler
	OnStop  Handler
}

// Start safely runs the OnStart.
func (h Hook) Start(ctx context.Context) error {
	if h.OnStart == nil {
		return nil
	}

	return h.OnStart(ctx)
}

// Tick safely runs the OnTick.
func (h Hook) Tick(ctx context.Context) error {
	if h.OnTick == nil {
		return nil
	}

	return h.OnTick(ctx)
}

// Stop safely runs the OnStop.
func (h Hook) Stop(ctx context.Context) error {
	if h.OnStop == nil {
		return nil
	}

	return h.OnStop(ctx)
}

var defaultLifecycle atomic.Pointer[Lifecycle]

func init() {
	defaultLifecycle.Store(NewLifeCycle(30 * time.Second))
}

// Default returns the default [Lifecycle].
func Default() *Lifecycle {
	return defaultLifecycle.Load()
}

// SetDefault makes l the default [Lifecycle].
func SetDefault(l *Lifecycle) {
	defaultLifecycle.Store(l)
}

// Register with the default [Lifecycle].
func Register(h Hook) {
	Default().Register(h)
}

// Run with the default [Lifecycle].
func Run(ctx context.Context, h Handler) error {
	return Default().Run(ctx, h)
}

// Serve with the default [Lifecycle].
func Serve(ctx context.Context) error {
	return Default().Serve(ctx)
}

// Shutdown with the default [Lifecycle].
func Shutdown() error {
	return Default().Shutdown()
}

// NewLifeCycle handles hooks.
func NewLifeCycle(timeout time.Duration) *Lifecycle {
	return &Lifecycle{hooks: make([]Hook, 0), timeout: timeout}
}

// Lifecycle of hooks.
type Lifecycle struct {
	hooks   []Hook
	timeout time.Duration
}

// Register a hook.
func (l *Lifecycle) Register(h Hook) {
	l.hooks = append(l.hooks, h)
}

// Run will start run the handler and stop.
func (l *Lifecycle) Run(ctx context.Context, h Handler) error {
	if err := l.start(ctx); err != nil {
		return err
	}

	if err := h(ctx); err != nil {
		return err
	}

	return l.stop(ctx)
}

// Serve runs start hooks, waits for SIGINT/SIGTERM, then runs stop hooks.
//
// Note: Serve takes ownership of SIGINT/SIGTERM for the entire process. It resets any
// prior handlers and ignores these signals before registering its own notification,
// so other packages’ signal handlers for these signals will not run while Serve is active.
func (l *Lifecycle) Serve(ctx context.Context) error {
	signals := []os.Signal{os.Interrupt, syscall.SIGTERM}

	// Reset and ignore signals that were set before, to only capture the ones set after Serve is called.
	signal.Reset(signals...)
	signal.Ignore(signals...)

	notifyCtx, stop := signal.NotifyContext(ctx, signals...)
	defer stop()

	if err := l.start(notifyCtx); err != nil {
		return err
	}

	<-notifyCtx.Done()
	stop()

	stopCtx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()

	return l.stop(stopCtx)
}

// Shutdown the lifecycle.
func (l *Lifecycle) Shutdown() error {
	process, _ := os.FindProcess(os.Getpid())
	return process.Signal(os.Interrupt)
}

func (l *Lifecycle) start(ctx context.Context) error {
	for _, hook := range l.hooks {
		if err := hook.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) stop(ctx context.Context) error {
	errs := make([]error, 0)
	for _, hook := range l.hooks {
		if err := hook.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
