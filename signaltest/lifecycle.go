package signaltest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal/v2/internal/lifecycle"
)

// NewLifecycle wraps the application's hook engine for component tests.
// It accepts tests and benchmarks. Register RequireStop with tb.Cleanup before
// starting so cleanup also runs after the test context is canceled.
func NewLifecycle(tb testing.TB) *Lifecycle {
	tb.Helper()

	return &Lifecycle{Lifecycle: lifecycle.New(), tb: tb, timeout: 30 * time.Second}
}

// Lifecycle registers and executes component hooks using the internal engine.
// Register hooks before startup. Use Start and Stop directly to assert errors.
// Service execution belongs to signal.App.Run and signal.App.Serve.
type Lifecycle struct {
	*lifecycle.Lifecycle

	tb      testing.TB
	timeout time.Duration
}

// Start runs component hooks and rolls back successfully started hooks on
// failure with a fresh 30-second deadline, joining startup and cleanup errors.
// Hooks must honor their contexts; a blocked callback cannot be interrupted.
func (l *Lifecycle) Start(ctx context.Context) error {
	err := l.Lifecycle.Start(ctx)
	if err == nil || errors.Is(err, lifecycle.ErrStarted) {
		return err
	}

	stopCtx, cancel := context.WithTimeoutCause(context.WithoutCancel(ctx), l.timeout, lifecycle.ErrTimeout)
	defer cancel()

	return errors.Join(err, l.Stop(stopCtx))
}

// RequireStart starts component hooks with a 30-second test-context deadline.
// Hooks must honor their context; a blocked callback cannot be interrupted.
func (l *Lifecycle) RequireStart() {
	l.tb.Helper()

	ctx, cancel := context.WithTimeout(l.tb.Context(), l.timeout)
	defer cancel()

	if err := l.Start(ctx); err != nil {
		l.tb.Fatalf("lifecycle start failed: %v", err)
	}
}

// RequireStop stops component hooks with a fresh 30-second cleanup deadline.
func (l *Lifecycle) RequireStop() {
	l.tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()

	if err := l.Stop(ctx); err != nil {
		l.tb.Fatalf("lifecycle stop failed: %v", err)
	}
}
