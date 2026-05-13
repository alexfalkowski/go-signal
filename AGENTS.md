# AGENTS.md

## Shared skills

This repository uses the shared skills from `bin/skills/`. Read
`bin/AGENTS.md` for the canonical shared skill list and use the smallest
matching skill for the task.

## Repo

- Module: `github.com/alexfalkowski/go-signal`.
- Read current toolchain details from repo files such as `go.mod`,
  `.circleci/config.yml`, and `bin/`; do not duplicate them here.
- Purpose: small library for startup/shutdown hooks around OS signals.
- Main files: `signal.go`, `run_test.go`, `serve_test.go`, `signal_test.go`,
  `internal/test/`, `cmd/main.go`, `README.md`, `.circleci/config.yml`.
- Public surface includes lifecycle helpers (`Register`, `Run`, `Serve`,
  `Shutdown`, `Go`, `Timer`), lifecycle constructors/defaults, hooks, and
  sentinel helpers/errors.

## Commands

Initialize shared tooling first when needed:

```sh
git submodule sync
git submodule update --init
```

Common targets: `make dep`, `make lint`, `make sec`, `make specs`,
`make coverage`, `make run param=start|timer|terminate`.

CI order: `make source-key`, `make clean`, `make dep`, `make clean`,
`make lint`, `make sec`, `make specs`, `make coverage`,
`make codecov-upload`.

## Behavior

- The package stores a process-wide default `*Lifecycle` in
  `sync.Pointer[Lifecycle]`; `init()` sets it to `NewDefaultLifecycle()` with a
  30 second stop timeout.
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
- `Serve` is a process-lifetime blocking call: it resets and owns `SIGINT` and
  `SIGTERM` while active, and shutdown can come from parent cancellation, an OS
  signal, or `signal.Shutdown()`.
- `Shutdown` sends `os.Interrupt` to the current process.
- `Terminated(err)` marks an error with `ErrTerminated`; `IsTerminated` checks
  it; `Go` calls `Shutdown()` when it sees one.
- `Timer` runs `hook.Start` once, ticks at the interval, stops on parent
  cancellation or hook error, and returns `ErrInvalidInterval` for
  `interval <= 0`.

## Tests

- Tests use external package `signal_test` and commonly pass `t.Context()`.
- Several `Serve` and `Timer` tests intentionally unblock via
  `signal.Shutdown()` after `time.Sleep(time.Second)`; they are
  timing-sensitive by design.
- `TestHTTPServe` binds to `127.0.0.1:0`.
