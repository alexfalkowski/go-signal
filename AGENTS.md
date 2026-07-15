# AGENTS.md

## Shared guidance

Use `bin/AGENTS.md` for shared skills and cross-repository defaults.

## Repo

- Module: `github.com/alexfalkowski/go-signal`.
- Read current toolchain details from repo files such as `go.mod`,
  `.circleci/config.yml`, and `bin/`; do not duplicate them here.
- Purpose: small library for startup/shutdown hooks around OS signals.
- Main files: `signal.go`, `run_test.go`, `serve_test.go`, `signal_test.go`,
  `internal/test/`, `cmd/main.go`, `README.md`, `.circleci/config.yml`.
- Public surface includes lifecycle helpers (`Register`, `Run`, `Serve`,
  `Shutdown`, `Terminate`, `Go`, `Timer`), lifecycle constructors/defaults, hooks,
  the `Signal` alias with its exported signal vars (`Interrupt`, `Termination`,
  `Hangup`), `Raise` (sends a signal to the current process), and sentinel
  helpers/errors.
- `cmd/main.go` is a manual testing script for `make run`, not production
  surface. Do not require `cmd/main_test.go` or raise missing command coverage
  for `cmd/main.go` during `$test-gaps` reviews.

## Commands

Initialize shared tooling first when needed:

Use `make submodule` once the shared `bin` checkout is present; see
`bin/AGENTS.md` for fresh-clone bootstrap details.

Common targets: `make dep`, `make lint`, `make sec`, `make specs`,
`make coverage`, `make run param=start|timer|terminate`.

CI order: `make source-key`, `make clean`, `make dep`, `make clean`,
`make lint`, `make sec`, `make specs`, `make fuzzes`, `make coverage`,
`make benchmarks`, `make codecov-upload`.

## Behavior

- The package stores a process-wide default `*Lifecycle` in
  `sync.Pointer[Lifecycle]`; `init()` sets it to `NewDefaultLifecycle()` with a
  30-second stop timeout.
- `Hook` callbacks are optional; `Start`, `Tick`, and `Stop` treat nil
  callbacks as no-ops.
- `Lifecycle.Register` is setup-time only, before `Run` or `Serve`; it is not
  concurrent-safe.
- `Run` starts all hooks in registration order, joins startup errors, rolls back
  only successfully started hooks on startup failure, and always runs stop hooks
  after successful startup.
- `Run`, `Serve`, and `Timer` use fresh timeout-bound background contexts for
  rollback/shutdown stop hooks. Returning `context.Cause(ctx)` from an expired
  stop context should match `signal.ErrTimeout`.
- `Serve` is the final process-lifetime blocking call: it always resets and
  owns `SIGINT` and `SIGTERM`, plus any additional `signal.Signal` values
  passed as variadic arguments (`signal.Interrupt`, `signal.Termination`,
  `signal.Hangup` are the exported names; `Signal` aliases `os.Signal`); it
  does not restore prior signal handlers after returning, and shutdown can come
  from parent cancellation, an OS signal in the active set, or
  `signal.Shutdown()` or `signal.Terminate(err)`.
- `Shutdown` sends `os.Interrupt` to the current process via `Raise`.
- `Terminate` records a shutdown cause and sends `os.Interrupt` to the current
  process; `Serve` returns the cause joined with stop-hook errors.
- `Raise(sig)` sends `sig` to the current process (`os.FindProcess(os.Getpid())`
  + `process.Signal(sig)`); `Shutdown` uses it internally, and tests use it
  directly to simulate arbitrary signals.
- `Terminated(err)` marks an error with `ErrTerminated`; `IsTerminated` checks
  it; `Go` calls `Terminate(err)` when it sees one.
- `Timer` runs `hook.Start` once, ticks at the interval, stops on parent
  cancellation or hook error, and returns `ErrInvalidInterval` for
  `interval <= 0`.

## Tests

- Tests use external package `signal_test` and commonly pass `t.Context()`.
- Do not add tests for `cmd/main.go`; it is only a manual testing script.
- Several `Serve` and `Timer` tests intentionally unblock via
  `signal.Shutdown()` after `time.Sleep(time.Second)`; they are
  timing-sensitive by design.
- `TestHTTPServe` binds to `127.0.0.1:0`.
