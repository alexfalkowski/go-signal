package signal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/alexfalkowski/go-sync"
)

// Timer runs hook.Start once, then calls hook.Tick at the given interval until
// ctx is done.
//
// With a valid interval, if ctx is already done on entry or timeout is not
// positive, Timer returns nil without starting the timer worker.
//
// If hook.Start fails, or if ctx is canceled or a timer hook returns an
// error, the timer worker calls hook.Stop with a fresh background context
// bounded by timeout. If that stop context expires and the stop hook returns
// [context.Cause], the returned error matches [ErrTimeout]. Nil hook callbacks
// are treated as no-ops.
//
// Timer executes its work through [Go], so a [Terminated] error still triggers
// package-level [Terminate]. Package-level termination targets [Default]; if
// callers run a custom lifecycle directly with [Lifecycle.Serve], background
// work should call [Lifecycle.Terminate] on that lifecycle or install it with
// [SetDefault]. Because [Go] is a best-effort waiting helper, Timer may return
// before the timer worker has run hook.Stop when ctx is canceled or timeout
// elapses first, and late non-terminated hook errors are not returned to the
// caller. The interval must be greater than zero or Timer returns
// [ErrInvalidInterval].
func Timer(ctx context.Context, timeout, interval time.Duration, hook Hook) error {
	if interval <= 0 {
		return fmt.Errorf("%w: %s", ErrInvalidInterval, interval)
	}

	return Go(ctx, timeout, func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if err := hook.Start(ctx); err != nil {
			return errors.Join(err, stopHook(timeout, hook))
		}

		for {
			select {
			case <-ctx.Done():
				return stopHook(timeout, hook)
			case <-ticker.C:
				if err := hook.Tick(ctx); err != nil {
					return errors.Join(err, stopHook(timeout, hook))
				}
			}
		}
	})
}

// ErrInvalidInterval reports that [Timer] was called with an interval less than
// or equal to zero.
var ErrInvalidInterval = errors.New("signal: invalid interval")

// ErrTimeout is the timeout cause used by derived stop contexts in this package.
//
// It wraps [sync.ErrTimeout] from github.com/alexfalkowski/go-sync, so
// [errors.Is] also matches [context.DeadlineExceeded].
var ErrTimeout = fmt.Errorf("signal: %w", sync.ErrTimeout)

// ErrTerminated marks an error as requesting process shutdown.
//
// Use [Terminated] to wrap an application error with this sentinel so that
// [IsTerminated] reports true and [Go] can trigger [Terminate].
var ErrTerminated = errors.New("signal: terminated")

// Terminated wraps err so that [IsTerminated] reports true.
//
// This is typically used by background work started with [Go] to signal that a
// concurrently running [Serve] loop should exit. If err is nil, Terminated
// returns [ErrTerminated].
func Terminated(err error) error {
	if err == nil {
		return ErrTerminated
	}

	return fmt.Errorf("%w: %w", err, ErrTerminated)
}

// IsTerminated reports whether err is marked with [ErrTerminated].
func IsTerminated(err error) bool {
	return errors.Is(err, ErrTerminated)
}

// Go runs handler with ctx and waits up to timeout for it to complete.
//
// Go is a best-effort waiting helper. If timeout elapses or ctx is done first,
// Go returns nil immediately while handler may continue running in the
// background.
//
// If ctx is already done on entry or timeout is not positive, Go returns nil
// without invoking handler.
//
// If handler is nil, Go returns [sync.ErrNoOnRunProvided].
//
// If handler returns an error marked with [ErrTerminated], Go triggers
// package-level [Terminate] before returning the error. Package-level
// termination targets [Default]; Go does not infer a custom [Lifecycle] from
// ctx. If that terminated error arrives after the waiting window has elapsed,
// Terminate is still triggered from the background goroutine, but Go has already
// returned nil. Other late errors are not returned to the caller.
func Go(ctx context.Context, timeout time.Duration, handler Handler) error {
	return sync.Wait(ctx, timeout, sync.Hook{
		OnRun: sync.Handler(handler),
		OnError: func(_ context.Context, err error) error {
			if IsTerminated(err) {
				_ = Terminate(err)
			}

			return err
		},
	})
}

// Handler is the function signature used by hooks and lifecycle methods.
//
// The supplied context is owned by the caller and should be observed for
// cancellation and deadlines.
type Handler func(context.Context) error

// Hook groups optional lifecycle callbacks.
//
// Each callback is optional. When invoked through [Hook.Start], [Hook.Tick], or
// [Hook.Stop], a nil callback is treated as a no-op.
type Hook struct {
	// OnStart runs during the lifecycle start phase before [Run] executes its
	// handler or [Serve] begins waiting for shutdown.
	OnStart Handler
	// OnTick runs on each interval when the hook is used with [Timer].
	OnTick Handler
	// OnStop runs during the lifecycle stop phase and should release resources
	// using the provided shutdown context.
	OnStop Handler
}

// Start calls [Hook.OnStart] if it is set, otherwise it returns nil.
func (h Hook) Start(ctx context.Context) error {
	if h.OnStart == nil {
		return nil
	}
	return h.OnStart(ctx)
}

// Tick calls [Hook.OnTick] if it is set, otherwise it returns nil.
func (h Hook) Tick(ctx context.Context) error {
	if h.OnTick == nil {
		return nil
	}
	return h.OnTick(ctx)
}

// Stop calls [Hook.OnStop] if it is set, otherwise it returns nil.
func (h Hook) Stop(ctx context.Context) error {
	if h.OnStop == nil {
		return nil
	}
	return h.OnStop(ctx)
}

var defaultLifecycle sync.Pointer[Lifecycle]

func init() {
	defaultLifecycle.Store(NewDefaultLifecycle())
}

// Default returns the process-wide default [Lifecycle].
//
// The default lifecycle is initialized during package init with
// [NewDefaultLifecycle].
func Default() *Lifecycle {
	return defaultLifecycle.Load()
}

// SetDefault replaces the process-wide default [Lifecycle].
//
// Callers typically use this in tests or when they want package-level helpers
// such as [Register], [Run], and [Serve] to target a custom lifecycle. If l is
// nil, SetDefault restores a fresh lifecycle from [NewDefaultLifecycle].
func SetDefault(l *Lifecycle) {
	if l == nil {
		l = NewDefaultLifecycle()
	}

	defaultLifecycle.Store(l)
}

// Register adds h to the default [Lifecycle].
//
// Register during setup, typically in main, before calling [Run] or [Serve].
// Registration is not designed to be used concurrently.
func Register(h Hook) {
	Default().Register(h)
}

// Run calls [Lifecycle.Run] on the default [Lifecycle].
func Run(ctx context.Context, h Handler) error {
	return Default().Run(ctx, h)
}

// Serve calls [Lifecycle.Serve] on the default [Lifecycle].
func Serve(ctx context.Context) error {
	return Default().Serve(ctx)
}

// Shutdown sends an [os.Interrupt] signal to the current process through the
// default [Lifecycle].
func Shutdown() error {
	return Default().Shutdown()
}

// Terminate records err as the default [Lifecycle]'s shutdown cause and sends an
// [os.Interrupt] signal to the current process.
//
// Terminate is useful when background work should stop a running [Serve] loop
// and have Serve return the terminating cause. A nil err records
// [ErrTerminated]. A non-nil err is marked with [ErrTerminated] unless it is
// already marked.
func Terminate(err error) error {
	return Default().Terminate(err)
}

// NewDefaultLifecycle returns a new empty [Lifecycle] with the package's
// default 30-second stop timeout.
func NewDefaultLifecycle() *Lifecycle {
	return NewLifeCycle(30 * time.Second)
}

// NewLifeCycle returns a new empty [Lifecycle] configured with the given stop
// timeout.
//
// The stop timeout is used by [Lifecycle.Run] and [Lifecycle.Serve] when
// running stop hooks during rollback and shutdown. It is passed directly to
// [context.WithTimeoutCause], so a non-positive timeout creates an
// already-expired stop context. Use a positive duration when stop hooks need
// time to clean up.
func NewLifeCycle(timeout time.Duration) *Lifecycle {
	return &Lifecycle{hooks: make([]Hook, 0), timeout: timeout}
}

// Lifecycle manages a set of registered hooks.
//
// A lifecycle is usually configured during application setup by calling
// [Lifecycle.Register], then executed through [Lifecycle.Run] or
// [Lifecycle.Serve].
//
// Use [NewLifeCycle] or [NewDefaultLifecycle] to construct a lifecycle. The zero
// value has a zero stop timeout, so stop contexts are already expired.
type Lifecycle struct {
	cause   error
	hooks   []Hook
	timeout time.Duration
	mu      sync.Mutex
}

// Register adds a hook to this lifecycle.
//
// Note: Lifecycle is not designed to be used concurrently. Register during
// setup, typically in main, before calling [Lifecycle.Run] or [Lifecycle.Serve].
func (l *Lifecycle) Register(h Hook) {
	l.hooks = append(l.hooks, h)
}

// Run executes the lifecycle against ctx.
//
// Run calls each registered start hook in registration order. If any start hook
// fails, Run still attempts the remaining start hooks, then rolls back by
// calling stop hooks for the hooks that started successfully in reverse
// registration order with a fresh background context bounded by the lifecycle
// timeout. If startup succeeds, it calls h, then calls each registered stop
// hook in reverse registration order with the same kind of fresh shutdown
// context. If a stop hook returns [context.Cause] after that context expires,
// the returned error matches [ErrTimeout].
//
// Run requires a non-nil handler; passing nil panics when Run invokes it. It
// does not recover panics from start hooks, the handler, or stop hooks; stop
// hooks run after the handler returns.
//
// Startup, handler, and stop-hook errors are combined with [errors.Join].
func (l *Lifecycle) Run(ctx context.Context, h Handler) error {
	started, err := l.start(ctx)
	if err != nil {
		stopCtx, cancel := l.stopContext()
		defer cancel()

		return errors.Join(err, l.stop(stopCtx, started))
	}

	handlerErr := h(ctx)

	stopCtx, cancel := l.stopContext()
	defer cancel()

	return errors.Join(handlerErr, l.stop(stopCtx, l.hooks))
}

// Serve runs the lifecycle until shutdown is requested.
//
// Serve resets any existing SIGINT and SIGTERM handlers, registers its own
// notification context, runs all start hooks with that context, then blocks
// until the notification context is done. If startup fails, Serve still
// attempts the remaining start hooks, then rolls back successfully started hooks
// in reverse registration order with a fresh background context bounded by the
// lifecycle timeout. Shutdown can happen because the parent ctx is canceled,
// because the process receives SIGINT or SIGTERM, because [Shutdown] delivers
// an interrupt to the current process, or because [Terminate] records a cause
// and delivers an interrupt.
//
// After shutdown is requested, Serve runs stop hooks in reverse registration
// order with a fresh background context bounded by the lifecycle timeout
// configured by [NewLifeCycle]. If a stop hook returns [context.Cause] after
// that context expires, the returned error matches [ErrTimeout].
//
// Normal shutdown from parent cancellation, SIGINT, SIGTERM, or [Shutdown]
// returns nil unless startup, rollback, or stop hooks return errors.
// Shutdown from [Terminate] returns the terminating cause joined with any
// stop-hook errors.
// Package-level [Terminate] records causes on [Default]. When using a custom
// lifecycle through this method directly, background work that needs to stop this
// specific lifecycle should call [Lifecycle.Terminate] on the same receiver, or
// the lifecycle should be installed with [SetDefault] before package-level
// helpers such as [Go] or [Timer] are used.
//
// Note: Serve is intended to be used as the final process-lifetime blocking
// call. It takes ownership of SIGINT and SIGTERM, does not restore prior signal
// handlers after returning, and callers should normally return from main after
// Serve returns.
//
// Because Serve resets and re-registers SIGINT and SIGTERM handling during
// startup, there is a narrow handoff window in which an arriving signal may
// need to be sent again.
func (l *Lifecycle) Serve(ctx context.Context) error {
	signals := []os.Signal{os.Interrupt, syscall.SIGTERM}

	// Reset and ignore prior handlers so Serve only captures signals delivered after it starts.
	signal.Reset(signals...)
	signal.Ignore(signals...)

	notifyCtx, stop := signal.NotifyContext(ctx, signals...)
	defer stop()
	l.setTerminationCause(nil)

	started, err := l.start(notifyCtx)
	if err != nil {
		stopCtx, cancel := l.stopContext()
		defer cancel()

		return errors.Join(err, l.stop(stopCtx, started))
	}

	<-notifyCtx.Done()
	stop()

	stopCtx, cancel := l.stopContext()
	defer cancel()

	return errors.Join(l.terminationCause(), l.stop(stopCtx, l.hooks))
}

// Shutdown sends an [os.Interrupt] signal to the current process.
//
// This is primarily intended to unblock [Lifecycle.Serve] programmatically, for
// example from a background goroutine or from tests.
func (l *Lifecycle) Shutdown() error {
	process, _ := os.FindProcess(os.Getpid())
	return process.Signal(os.Interrupt)
}

// Terminate records err as this lifecycle's shutdown cause and sends an
// [os.Interrupt] signal to the current process.
//
// Terminate differs from [Lifecycle.Shutdown] by preserving a non-nil cause for
// [Lifecycle.Serve] to return after stop hooks run. A nil err records
// [ErrTerminated]. A non-nil err is marked with [ErrTerminated] unless it is
// already marked.
func (l *Lifecycle) Terminate(err error) error {
	l.setTerminationCause(terminationError(err))
	return l.Shutdown()
}

func (l *Lifecycle) start(ctx context.Context) ([]Hook, error) {
	started := make([]Hook, 0, len(l.hooks))
	errs := make([]error, 0)

	for _, hook := range l.hooks {
		if err := hook.Start(ctx); err != nil {
			errs = append(errs, err)
			continue
		}

		started = append(started, hook)
	}

	return started, errors.Join(errs...)
}

func (l *Lifecycle) setTerminationCause(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cause = err
}

func (l *Lifecycle) terminationCause() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.cause
}

func (l *Lifecycle) stopContext() (context.Context, context.CancelFunc) {
	return context.WithTimeoutCause(context.Background(), l.timeout, ErrTimeout)
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

func stopHook(timeout time.Duration, hook Hook) error {
	stopCtx, cancel := context.WithTimeoutCause(context.Background(), timeout, ErrTimeout)
	defer cancel()

	return hook.Stop(stopCtx)
}

func terminationError(err error) error {
	if err == nil {
		return ErrTerminated
	}
	if IsTerminated(err) {
		return err
	}
	return Terminated(err)
}
