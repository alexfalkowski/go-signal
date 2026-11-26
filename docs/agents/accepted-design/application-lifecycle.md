# Accepted design for the application lifecycle

Part of the [accepted-design index](../accepted-design.md).

## Application boundary

The root `signal` package owns `App`, its construction options, `Hook`,
`Handler`, `Lifecycle`, `Shutdowner`, timeout and panic errors, and OS signal
selectors. The lifecycle engine lives in `internal/lifecycle`. V2 retired the
process-wide lifecycle and the public `di` and `lifecycle` packages so each
application owns its hooks and shutdown state.

The public execution entrypoints are `App.Run` and `App.Serve`. `Run` starts
and immediately stops a short-lived command. Command work belongs in start
hooks. `Serve` starts a service and waits for cancellation, an OS signal, or a
programmatic shutdown request before cleanup. Each `App` supports one
execution. Construct a fresh application for each command or retry.

[Application entrypoints](../../../app.go) · [V2 migration](../../../README.md#-v2-migration)

## Construction and registration

`NewApp` registers Dig constructors and decorators before it runs setup
functions. Dig recovers panics in constructors, decorators, and setup functions
as initialization errors. Acquire resources in start hooks so construction
failures leave no running workers or open resources. Register all hooks with
`Lifecycle.Append` during setup, before calling `Run` or `Serve`. `Append`
requires serial use.

[Construction and execution](../../../app.go) · [Registration options](../../../options.go)

## Hook execution and deadlines

Hook callbacks are optional. Nil callbacks are no-ops, and recovered hook
panics match `signal.ErrRecovered`. Startup runs hooks in registration order
and stops at the first error or cancellation. It checks cancellation before
startup and between hooks. Rollback covers successfully started hooks. Cleanup
runs in reverse order and joins errors while attempting each stop hook.

Startup and cleanup each default to 15 seconds. Cleanup detaches parent
cancellation and gets a fresh stop deadline, including during rollback.
Timeout errors match both `signal.ErrTimeout` and `context.DeadlineExceeded`.
An available result takes precedence over cancellation of the wait. Deadlines
bound waiting, and callbacks must honor their contexts to finish promptly. If
startup completes after its wait is canceled, cleanup runs after startup returns.

[Hook engine](../../../internal/lifecycle/lifecycle.go) · [Hook callbacks](../../../internal/lifecycle/hook.go) · [Application tests](../../../app_test.go)

## Shutdown ownership

`App.Serve` owns signal notification through `os/signal.NotifyContext`. It
listens for SIGINT, SIGTERM, and caller-supplied signals, and unregisters its
notifications when it returns.

`Shutdowner.Shutdown(code)` records an application-local request without
sending an OS signal. The first code wins, including zero. `Run` and `Serve`
preserve a buffered request when cancellation is also ready. When they wait
for cleanup, they also preserve a first request made during that wait. A
non-zero code becomes an `*ExitError`, joined with any startup or cleanup
errors returned to the caller. The caller owns process exit.

[Shutdown handling](../../../app.go) · [Shutdown tests](../../../app_test.go)
