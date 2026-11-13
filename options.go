package signal

import (
	"errors"
	"time"

	"go.uber.org/dig"
)

// In aliases [dig.In] for constructor parameter structs.
type In = dig.In

// Out aliases [dig.Out] for constructor result structs.
type Out = dig.Out

// Lifecycle registers startup and shutdown hooks during application construction.
// Append adds hooks in registration order and is not concurrent-safe. Configure
// all hooks before App.Run or App.Serve; the application owns their execution.
type Lifecycle interface {
	Append(hook Hook)
}

// Shutdowner requests application-local shutdown. The first request wins,
// including requests made before Serve begins waiting. Zero means success.
type Shutdowner interface {
	Shutdown(code int)
}

// ErrInvalidTimeout reports a non-positive startup or shutdown timeout.
var ErrInvalidTimeout = errors.New("signal: timeout must be positive")

// Option configures an application during NewApp. Options can be reused by
// independent applications; they must not be changed during construction.
type Option func(*options)

// Constructor registers factories with Dig. Factories can return multiple
// values and an optional error; outputs are constructed at most once per app.
func Constructor(constructors ...any) Option {
	return func(o *options) {
		o.constructors = append(o.constructors, constructors...)
	}
}

// Decorate registers decorators applied after all constructors are registered
// and before any setup functions run. It wraps [dig.Container.Decorate].
func Decorate(decorators ...any) Option {
	return func(o *options) {
		o.decorators = append(o.decorators, decorators...)
	}
}

// Register adds setup functions whose parameters are resolved by Dig. They run
// in registration order during NewApp, before lifecycle start hooks execute.
func Register(functions ...any) Option {
	return func(o *options) {
		o.functions = append(o.functions, functions...)
	}
}

// Module combines options without introducing a separate dependency scope.
func Module(opts ...Option) Option {
	return func(o *options) {
		for _, opt := range opts {
			opt(o)
		}
	}
}

// StartTimeout bounds startup in App.Run and App.Serve. The default is 15
// seconds. NewApp rejects non-positive timeouts.
func StartTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.startTimeout = timeout
	}
}

// StopTimeout bounds cleanup and startup rollback. The default is 15 seconds.
// NewApp rejects non-positive timeouts.
func StopTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.stopTimeout = timeout
	}
}

// RootCause wraps [dig.RootCause] for reporting construction errors.
func RootCause(err error) error {
	return dig.RootCause(err)
}

type options struct {
	constructors []any
	decorators   []any
	functions    []any
	startTimeout time.Duration
	stopTimeout  time.Duration
}
