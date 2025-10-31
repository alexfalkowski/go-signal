package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

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
	group := &errgroup.Group{}
	group.SetLimit(len(l.hooks))

	for _, hook := range l.hooks {
		group.Go(func() error {
			return hook.Start(ctx)
		})
	}

	return group.Wait()
}

func (l *Lifecycle) stop(ctx context.Context) error {
	group := &errgroup.Group{}
	group.SetLimit(len(l.hooks))

	for _, hook := range l.hooks {
		group.Go(func() error {
			return hook.Stop(ctx)
		})
	}

	return group.Wait()
}
