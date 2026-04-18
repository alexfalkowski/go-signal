# AGENTS.md

## Shared skill

Use the shared `coding-standards` skill from `./bin/skills/coding-standards`
for general coding, review, testing, documentation, and PR conventions. This
file only captures repo-specific guidance.

## Repo

- Module: `github.com/alexfalkowski/go-signal`
- Purpose: Go library for coordinating startup and shutdown hooks around OS
  signals
- Go version: `1.26.0`
- Public API: `Register`, `Run`, `Serve`, `Shutdown`, `Go`, `Timer`,
  `ErrTimeout`, `NewDefaultLifecycle`, `NewLifeCycle`, `SetDefault`, `Default`

## Layout

- `signal.go`: library implementation
- `run_test.go`: `Run` coverage
- `serve_test.go`: `Serve` and `Timer` coverage
- `signal_test.go`: integration-style tests
- `internal/test/`: shared rollback test helpers
- `cmd/main.go`: runnable example for `make run`
- `README.md`: user-facing docs
- `.circleci/config.yml`: CI source of truth
- `bin/`: shared make tooling submodule

## Commands

Most `make` targets depend on `bin/` being initialized:

```sh
git submodule sync
git submodule update --init
```

Primary commands:

```sh
make dep
make lint
make sec
make specs
make coverage
make run param=start
make run param=timer
make run param=terminate
```

CI build order:

```sh
make source-key
make clean
make dep
make clean
make lint
make sec
make specs
make coverage
make codecov-upload
```

## Repo-specific behavior

- The package keeps a process-wide default lifecycle in
  `sync.Pointer[Lifecycle]`.
- The default lifecycle is initialized in `init()` with
  `signal.NewDefaultLifecycle()`, which uses a 30 second stop timeout.
- `Hook` callbacks are optional. `Hook.Start`, `Hook.Tick`, and `Hook.Stop`
  treat nil callbacks as no-ops.
- `Lifecycle.Register` is not concurrent-safe. Register hooks during setup,
  before `Run` or `Serve`.
- `Lifecycle.Run` starts hooks in registration order, attempts every start hook,
  joins startup errors, rolls back only successfully started hooks on startup
  failure, and always runs stop hooks after successful startup.
- `Lifecycle.Run` rollback and shutdown hooks use fresh timeout-bound stop
  contexts. Returning `context.Cause(ctx)` from an expired stop context should
  match `signal.ErrTimeout`.
- `Lifecycle.Serve` resets and takes ownership of `SIGINT` and `SIGTERM` while
  active.
- `Lifecycle.Serve` startup failure still attempts remaining start hooks, then
  rolls back successfully started hooks with a fresh timeout-bound background
  context.
- `Lifecycle.Serve` shutdown can come from parent context cancellation, an OS
  signal, or `signal.Shutdown()`.
- `Lifecycle.Shutdown` sends `os.Interrupt` to the current process.
- `signal.Terminated(err)` marks an error with `ErrTerminated`,
  `signal.IsTerminated(err)` checks that marker, and `signal.Go` triggers
  `signal.Shutdown()` when it sees a terminated error.
- `signal.Timer` runs `hook.Start` once, then `hook.Tick` on each interval, and
  runs `hook.Stop` with a fresh timeout-bound context when the parent context
  ends or a timer hook returns an error.
- `signal.Timer` returns `ErrInvalidInterval` for `interval <= 0`.

## Testing notes

- Tests use `package signal_test`.
- Many `Serve` and `Timer` tests intentionally unblock with
  `signal.Shutdown()` after `time.Sleep(time.Second)`.
- Those tests are timing-sensitive by design.
- `TestHTTPServe` binds to `127.0.0.1:0`.
- Tests commonly pass `t.Context()` into library calls.
