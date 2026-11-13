package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/alexfalkowski/go-sync"
)

// ErrStarted reports an attempt to start a lifecycle that is already running.
var ErrStarted = errors.New("signal: lifecycle already started")

// ErrTimeout is the timeout cause used by application lifecycle contexts.
//
// It wraps [sync.ErrTimeout] from github.com/alexfalkowski/go-sync, so
// [errors.Is] also matches [context.DeadlineExceeded].
var ErrTimeout = fmt.Errorf("signal: %w", sync.ErrTimeout)

// New returns an empty lifecycle.
func New() *Lifecycle {
	return &Lifecycle{hooks: make([]Hook, 0)}
}

// Lifecycle executes the application's registered startup and cleanup hooks.
// Configure hooks before Start; Start and Stop serialize transitions.
type Lifecycle struct {
	hooks   []Hook
	started []Hook
	phase   sync.Mutex
}

// Append adds a hook to this lifecycle.
//
// Append is not designed to be called concurrently. Configure hooks during
// setup, typically in main, before calling [Lifecycle.Start].
func (l *Lifecycle) Append(h Hook) {
	l.hooks = append(l.hooks, h)
}

// Start runs hooks in registration order, stopping at the first error or
// cancellation. Successfully started hooks are retained for Stop, including
// after a startup failure. Callers own rollback and its deadline. A running
// lifecycle returns [ErrStarted].
//
// Start and Stop serialize lifecycle transitions. Hooks must honor their
// contexts; these methods do not interrupt a blocked callback. Cancellation is
// checked before startup and between hooks. Once every hook has succeeded,
// startup is complete even if cancellation arrives as the last hook returns.
func (l *Lifecycle) Start(ctx context.Context) error {
	l.phase.Lock()
	defer l.phase.Unlock()

	if l.started != nil {
		return ErrStarted
	}

	started, err := l.start(ctx)
	l.started = started
	return err
}

// Stop releases successfully started hooks in reverse order, joining errors
// while attempting every stop hook. It uses the supplied context, so callers
// should provide a fresh shutdown deadline. Stopping an inactive lifecycle is
// a no-op. Each lifecycle belongs to one application execution.
func (l *Lifecycle) Stop(ctx context.Context) error {
	l.phase.Lock()
	defer l.phase.Unlock()

	started := l.started
	l.started = nil

	return l.stop(ctx, started)
}

func (l *Lifecycle) start(ctx context.Context) ([]Hook, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}

	started := make([]Hook, 0, len(l.hooks))

	for _, hook := range l.hooks {
		if err := context.Cause(ctx); err != nil {
			return started, err
		}

		if err := hook.Start(ctx); err != nil {
			return started, err
		}

		started = append(started, hook)
	}

	return started, nil
}

func (l *Lifecycle) stop(ctx context.Context, hooks []Hook) error {
	errs := make([]error, 0)

	for _, hook := range slices.Backward(hooks) {
		if err := hook.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
