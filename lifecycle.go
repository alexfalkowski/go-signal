package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// NewLifeCycle handles hooks.
func NewLifeCycle(opts ...Option) *Lifecycle {
	os := applyOptions(opts...)
	return &Lifecycle{hooks: make([]*Hook, 0), timeout: os.timeout}
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

// Start will run all the hooks start.
func (l *Lifecycle) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(len(l.hooks))

	for _, hook := range l.hooks {
		group.Go(func() error {
			return hook.Start(ctx)
		})
	}

	return group.Wait()
}

// Stop will run all the hooks stop.
func (l *Lifecycle) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(len(l.hooks))

	for _, hook := range l.hooks {
		group.Go(func() error {
			return hook.Stop(ctx)
		})
	}

	return group.Wait()
}

// Client will start run the handler and stop.
func (l *Lifecycle) Client(ctx context.Context, h Handler) error {
	if err := l.Start(ctx); err != nil {
		return err
	}

	if err := h(ctx); err != nil {
		return err
	}

	if err := l.Stop(ctx); err != nil {
		return err
	}

	return nil
}

// Server will run start, wait for signal and stop.
func (l *Lifecycle) Server(ctx context.Context) error {
	startCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := l.Start(startCtx); err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	cancel()

	if err := l.Stop(ctx); err != nil {
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
