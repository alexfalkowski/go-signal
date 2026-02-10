// Package signal coordinates application start/stop hooks around OS signals.
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

// Timer runs hook.Start once, then calls hook.Tick at the given interval until ctx is done.
// When ctx is done, it calls hook.Stop with a fresh context bounded by timeout.
// The overall execution is bounded by timeout via [Go].
//
// The interval must be greater than zero.
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

// ErrTerminated marks an error as requesting shutdown.
var ErrTerminated = errors.New("signal: terminated")

// Terminated wraps err so that [IsTerminated] reports true.
func Terminated(err error) error {
	return fmt.Errorf("%w: %w", err, ErrTerminated)
}

// IsTerminated reports whether err is marked with [ErrTerminated].
func IsTerminated(err error) bool {
	return errors.Is(err, ErrTerminated)
}

// Go waits for handler to complete or for timeout to elapse.
//
// If handler returns an error marked with [ErrTerminated], Go triggers a shutdown by
// calling [Shutdown].
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

// Handler is a function invoked by hooks and lifecycle methods.
type Handler func(context.Context) error

// Hook defines optional lifecycle callbacks.
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

// NewLifeCycle returns a new [Lifecycle] configured with the given stop timeout.
//
// The stop timeout is used by [Lifecycle.Serve] when running stop hooks.
func NewLifeCycle(timeout time.Duration) *Lifecycle {
	return &Lifecycle{hooks: make([]Hook, 0), timeout: timeout}
}

// Lifecycle manages registered hooks.
type Lifecycle struct {
	hooks   []Hook
	timeout time.Duration
}

// Register adds a hook to this lifecycle.
//
// Note: Lifecycle is not designed to be used concurrently. Register during setup (typically in
// main), before calling Run or Serve.
func (l *Lifecycle) Register(h Hook) {
	l.hooks = append(l.hooks, h)
}

// Run runs start hooks, then h, then stop hooks.
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
// so other packages' signal handlers for these signals will not run while Serve is active.
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

// Shutdown sends an os.Interrupt signal to the current process.
//
// This is primarily intended to unblock [Lifecycle.Serve].
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
