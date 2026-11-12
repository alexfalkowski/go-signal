// Package lifecycle executes application startup and cleanup hooks.
//
// Start runs hooks in registration order, stops at the first error or
// cancellation, and retains successfully started hooks for rollback by the
// caller. Stop releases resources in reverse order and joins cleanup errors.
// Callers supply contexts and own startup and cleanup deadlines.
// Hook panics become errors marked with ErrRecovered. Phase transitions are
// serialized; registration is setup-time only.
package lifecycle
