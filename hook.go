package signal

import "context"

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
