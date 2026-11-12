package signal

import "github.com/alexfalkowski/go-signal/v2/internal/lifecycle"

// Handler is the callback signature shared by application hooks and Timer.
// Callbacks should observe their supplied context's cancellation and deadline.
type Handler = lifecycle.Handler

// Hook exposes the callbacks used by application startup and cleanup.
// Callbacks are optional; nil callbacks are no-ops. A callback panic is recovered
// as an error marked with ErrRecovered. App executes OnStart and OnStop.
type Hook = lifecycle.Hook

// ErrStarted reports an attempt to start an application that is already active.
var ErrStarted = lifecycle.ErrStarted

// ErrTimeout marks an expired application startup or cleanup deadline. It also
// matches context.DeadlineExceeded through errors.Is.
var ErrTimeout = lifecycle.ErrTimeout

// ErrRecovered marks an error produced by recovering a callback panic.
var ErrRecovered = lifecycle.ErrRecovered
