package lifecycle

import (
	"context"
)

// Handler is the function signature used by hooks and lifecycle methods.
//
// The supplied context is owned by the caller and should be observed for
// cancellation and deadlines.
type Handler func(context.Context) error

// Hook groups optional lifecycle callbacks.
//
// Each callback is optional. When invoked through [Hook.Start] or
// [Hook.Stop], a nil callback is treated as a no-op.
type Hook struct {
	// OnStart acquires resources during the lifecycle start phase.
	OnStart Handler
	// OnStop runs during the lifecycle stop phase and should release resources
	// using the provided shutdown context.
	OnStop Handler
}

// Start calls [Hook.OnStart] if it is set, otherwise it returns nil.
//
// A panic from OnStart is recovered and returned as an error marked with
// [ErrRecovered].
func (h Hook) Start(ctx context.Context) (err error) {
	if h.OnStart == nil {
		return nil
	}

	defer func() {
		if v := recover(); v != nil {
			err = RecoveredError(v)
		}
	}()

	return h.OnStart(ctx)
}

// Stop calls [Hook.OnStop] if it is set, otherwise it returns nil.
//
// A panic from OnStop is recovered and returned as an error marked with
// [ErrRecovered].
func (h Hook) Stop(ctx context.Context) (err error) {
	if h.OnStop == nil {
		return nil
	}

	defer func() {
		if v := recover(); v != nil {
			err = RecoveredError(v)
		}
	}()

	return h.OnStop(ctx)
}
