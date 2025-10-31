package signal

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Go waits for the handler to complete or timeout.
func Go(ctx context.Context, timeout time.Duration, handler Handler) error {
	ch := make(chan error, 1)

	go func() {
		ch <- handler(ctx)
	}()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return nil
	}
}

// Handler used for hook.
type Handler func(context.Context) error

// Hook for a lifecycle.
type Hook struct {
	OnStart Handler
	OnStop  Handler
}

// Start safely runs the OnStart.
func (h *Hook) Start(ctx context.Context) error {
	if h == nil || h.OnStart == nil {
		return nil
	}

	return h.OnStart(ctx)
}

// Stop safely runs the OnStop.
func (h *Hook) Stop(ctx context.Context) error {
	if h == nil || h.OnStop == nil {
		return nil
	}

	return h.OnStop(ctx)
}

// NewLifeCycle handles hooks.
func NewLifeCycle(timeout time.Duration) *Lifecycle {
	return &Lifecycle{hooks: make([]*Hook, 0), timeout: timeout}
}

// Lifecycle of hooks.
type Lifecycle struct {
	hooks   []*Hook
	timeout time.Duration
}

// Register a hook.
func (l *Lifecycle) Register(h *Hook) {
	l.hooks = append(l.hooks, h)
}

// Client will start run the handler and stop.
func (l *Lifecycle) Client(ctx context.Context, h Handler) error {
	if err := l.start(ctx); err != nil {
		return err
	}

	if err := h(ctx); err != nil {
		return err
	}

	if err := l.stop(ctx); err != nil {
		return err
	}

	return nil
}

// Server will run start, wait for signal and stop.
func (l *Lifecycle) Server(ctx context.Context) error {
	notifyCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := l.start(notifyCtx); err != nil {
		return err
	}

	<-notifyCtx.Done()
	stop()

	stopCtx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	if err := l.stop(stopCtx); err != nil {
		return err
	}

	return nil
}

// Terminate the lifecycle.
func (l *Lifecycle) Terminate() error {
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
