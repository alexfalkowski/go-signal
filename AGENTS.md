# AGENTS.md

## Overview

- Module: `github.com/alexfalkowski/go-signal`
- Purpose: small Go library for coordinating application start/stop hooks around OS signals
- Go version: `1.26.0`

Primary public API lives in `signal.go`:

- `signal.Register(Hook)`
- `signal.Run(ctx, handler)`
- `signal.Serve(ctx)`
- `signal.Shutdown()`
- `signal.Go(ctx, timeout, handler)`
- `signal.Timer(ctx, timeout, interval, hook)`
- `signal.ErrTimeout`
- `signal.NewLifeCycle(timeout)`
- `signal.SetDefault(lifecycle)` and `signal.Default()`

## Layout

- `signal.go`: library implementation
- `internal/test/`: shared test helpers for startup rollback scenarios
- `cmd/main.go`: runnable example used by `make run`
- `run_test.go`: tests for `Run`
- `serve_test.go`: tests for `Serve` and `Timer`
- `signal_test.go`: integration-style tests
- `README.md`: user-facing package documentation
- `.circleci/config.yml`: CI workflow
- `bin/`: git submodule with shared Make tooling

## Tooling

The top-level `Makefile` includes Makefiles from the `bin/` submodule, so most
`make` targets depend on `bin/` being initialized:

```sh
git submodule sync
git submodule update --init
```

The submodule URL is SSH-based: `git@github.com:alexfalkowski/bin.git`.

Formatting defaults from `.editorconfig`:

- LF line endings
- tabs for `*.go`
- tabs for `Makefile`

## Key commands

Run the example:

```sh
make run param=start
make run param=timer
make run param=terminate
```

Dependency maintenance:

```sh
make dep
make clean
```

Tests:

```sh
go test ./...
make specs
```

Lint and security:

```sh
make lint
make fix-lint
make sec
```

Coverage:

```sh
make coverage
```

## CI

CircleCI runs the main build job in this order:

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

`make source-key` writes `.source-key`. Test reports and coverage artifacts are
stored under `test/reports/`.

## Behavior notes

### Lifecycle model

- The package keeps a process-wide default lifecycle in `sync.Pointer[Lifecycle]`.
- The default lifecycle is initialized in `init()` with a 30 second stop timeout.
- Tests often replace it with `signal.SetDefault(signal.NewLifeCycle(...))`.

### Hooks

- `Hook` callbacks are optional: `OnStart`, `OnTick`, and `OnStop`
- `Hook.Start`, `Hook.Tick`, and `Hook.Stop` return `nil` when the callback is unset

### Registration

- `Lifecycle.Register` appends to an internal slice
- registration is not designed to be concurrent-safe
- register hooks during setup, before calling `Run` or `Serve`

### Run semantics

- `Lifecycle.Run` runs start hooks in registration order
- it attempts all start hooks and collects startup errors with `errors.Join`
- if startup fails, it rolls back by running stop hooks only for successfully started hooks using the caller context
- if all start hooks succeed, it runs the supplied handler
- after successful startup, stop hooks run even if the handler returns an error
- if a stop hook returns `context.Cause(ctx)` after the lifecycle stop context expires, the returned error matches `signal.ErrTimeout`
- stop collects all hook errors with `errors.Join`

### Serve semantics

- `Lifecycle.Serve` resets and ignores existing `SIGINT` and `SIGTERM` handlers
- it attempts all start hooks and collects startup errors with `errors.Join`
- if startup fails, it rolls back successfully started hooks with a fresh background context bounded by the lifecycle timeout and returns without entering the wait loop
- it creates a `signal.NotifyContext` and blocks until shutdown is requested
- shutdown can come from parent context cancellation, an OS signal, or `signal.Shutdown()`
- stop hooks run with a fresh background context bounded by the lifecycle timeout
- if a stop hook returns `context.Cause(ctx)` after that stop context expires, the returned error matches `signal.ErrTimeout`
- while `Serve` is active, other handlers for `SIGINT` and `SIGTERM` will not run

### Shutdown and termination

- `Lifecycle.Shutdown` sends `os.Interrupt` to the current process
- `signal.Terminated(err)` marks an error with `ErrTerminated`
- `signal.IsTerminated(err)` checks that marker via `errors.Is`
- `signal.Go` triggers `signal.Shutdown()` when it sees a terminated error

### Timer

- `signal.Timer` runs `hook.Start` once, then `hook.Tick` at the requested interval
- when the parent context ends, `Timer` runs `hook.Stop` with a fresh timeout-bound context
- if that timeout-bound stop context expires and the hook returns `context.Cause(ctx)`, the returned error matches `signal.ErrTimeout`
- `interval <= 0` returns `ErrInvalidInterval`
- `Timer` executes through `signal.Go`, so terminated errors still request shutdown

## Testing notes

- tests use `package signal_test`
- many `Serve` and `Timer` tests unblock `signal.Serve(...)` by calling `signal.Shutdown()` from a goroutine after `time.Sleep(time.Second)`
- these tests are intentionally timing-sensitive
- `TestHTTPServe` binds to `127.0.0.1:0` instead of a fixed port
- tests commonly pass `t.Context()` into library calls
