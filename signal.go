package signal

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

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
func NewLifeCycle() *Lifecycle {
	return &Lifecycle{hooks: make([]*Hook, 0)}
}

// Lifecycle of hooks.
type Lifecycle struct {
	hooks []*Hook
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
	startCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := l.start(startCtx); err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	cancel()

	if err := l.stop(ctx); err != nil {
		return err
	}

	return nil
}

// Terminate the lifecycle.
func (l *Lifecycle) Terminate() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

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
