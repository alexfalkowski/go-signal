// Package signal constructs applications and coordinates their startup and shutdown.
//
// NewApp registers constructors and decorators with Dig, then executes setup
// functions supplied through Register. Setup functions receive Lifecycle to
// register Hook callbacks. Resource acquisition belongs in OnStart, and OnStop
// releases resources in reverse startup order.
//
// App.Run starts and stops a short-lived command; App.Serve waits for
// cancellation, an OS signal, or an application-local shutdown request.
// Startup stops at the first error or cancellation and rolls back successfully
// started hooks. Callbacks must honor cancellation and deadlines. Each App owns
// one execution; construct a fresh App for each command or retry.
//
// Applications own their lifecycle and shutdown requests. The first requested
// exit code wins. A non-zero code becomes an ExitError, joined with startup or
// cleanup errors; callers own process exit. Hook panics are recovered as ErrRecovered.
//
// App.Serve listens for OS signals and unregisters its notifications on return.
// The signaltest package provides component helpers using the same internal hook engine.
// Timer runs periodic callbacks; lifecycle hooks own cancellation and waiting
// when it is launched as a background worker.
package signal
