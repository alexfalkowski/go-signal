# AGENTS.md

## Repository overview

This repository is a small Go library (`module github.com/alexfalkowski/go-signal`) for coordinating application start/stop hooks around OS signals.

Primary public entry points live in `signal.go`:

- `signal.Register(Hook)` (`signal.go:122-125`)
- `signal.Run(ctx, handler)` (`signal.go:127-130`)
- `signal.Serve(ctx)` (`signal.go:132-135`)
- `signal.Shutdown()` (`signal.go:137-140`)
- `signal.Go(ctx, timeout, handler)` (`signal.go:55-67`)
- `signal.Timer(ctx, timeout, interval, hook)` (`signal.go:16-40`)

Go version: `go 1.25.0` (`go.mod:3`).

## Directory layout

- `signal.go`: library implementation.
- `cmd/main.go`: example program (used by `make run`).
- `*_test.go`: package tests (use `package signal_test`).
  - `run_test.go`: tests for `Run`.
  - `serve_test.go`: tests for `Serve` and `Timer`.
  - `signal_test.go`: integration-style tests (HTTP server + exec command).
- `.circleci/config.yml`: CI pipeline invoking Make targets.
- `test/reports/`: CI artifacts (JUnit XML, coverage outputs).
- `bin/`: git submodule providing shared build tooling and Makefile includes.

## Tooling and prerequisites

### Git submodule (`bin/`)

The top-level `Makefile` includes Makefiles from the `bin/` submodule (`Makefile:1-2`). Most `make` targets depend on scripts in `bin/`, so CI/local usage typically requires the submodule.

Initialize the submodule:

```sh
git submodule sync
git submodule update --init
```

Note: the submodule URL is configured as SSH (`.gitmodules:1-3`): `git@github.com:alexfalkowski/bin.git`.

### Editor/formatting

`.editorconfig` enforces (`.editorconfig:1-16`):

- LF line endings.
- Tabs for `*.go` files.
- Tabs for `Makefile` recipes.

GolangCI-Lint and formatters are configured via `.golangci.yml`.

## Essential commands

### Run the example

The repository `Makefile` defines a single direct target:

```sh
make run param=start
make run param=timer
make run param=terminate
```

This runs: `go run cmd/main.go $(param)` (`Makefile:4-6`).

### Dependencies (vendoring)

CI and some Make targets run tests with `-mod vendor`, so vendoring matters.

```sh
make dep    # go mod download && go mod tidy && go mod vendor (bin/build/make/go.mak:9-26)
make clean  # runs bin/build/go/clean (bin/build/make/go.mak:36-38)
```

### Tests

Fast local run (module mode):

```sh
go test ./...
```

CI-style run (race + coverage + junit report; uses vendored deps):

```sh
make specs
```

`make specs` runs `gotestsum` and writes JUnit + coverage into `test/reports/` (`bin/build/make/go.mak:62-64`).

### Lint

```sh
make lint
make fix-lint
```

- `make lint` runs `bin/build/go/fa` (field alignment) and golangci-lint (`bin/build/make/go.mak:39-53`).
- `make fix-lint` runs the corresponding `-fix` variants when possible (`bin/build/make/go.mak:42-55`).

### Security

```sh
make sec
```

Runs `govulncheck -show verbose -test ./...` (`bin/build/make/go.mak:95-98`).

### Coverage

```sh
make coverage
```

Produces HTML and function coverage outputs in `test/reports/` (`bin/build/make/go.mak:76-86`).

## CI

CircleCI runs the following (see `.circleci/config.yml:19-55`):

```sh
make source-key
make clean
make dep
make lint
make sec
make specs
make coverage
make codecov-upload
```

`make source-key` comes from `bin/build/make/git.mak` and writes `.source-key` (`bin/build/make/git.mak:175-177`).

## Code patterns and conventions

### Default lifecycle (global)

- The package stores a default lifecycle in `atomic.Pointer[Lifecycle]` (`signal.go:106-120`).
- It is initialized in `init()` to a 30s timeout (`signal.go:108-110`).
- Tests often override it via `signal.SetDefault(signal.NewLifeCycle(...))`.

### Hooks are nil-safe

`Hook` handlers are optional (`OnStart`, `OnTick`, `OnStop`), and `Hook.Start/Tick/Stop` return `nil` when the corresponding handler is unset (`signal.go:72-104`).

### Lifecycle registration is not concurrent-safe

`Lifecycle.Register` appends to an internal slice and is not designed for concurrent use; it should be called during setup before `Run`/`Serve` (`signal.go:153-159`).

### Serve owns SIGINT/SIGTERM while running

`Lifecycle.Serve`:

- Resets and ignores existing SIGINT/SIGTERM handlers (`signal.go:180-185`).
- Creates a `signal.NotifyContext` and blocks until it is cancelled (`signal.go:186-199`).

This means other packages’ signal handlers for these signals will not run while `Serve` is active.

### Termination semantics

Termination is modeled by wrapping an error with `ErrTerminated`:

- `signal.Terminated(err)` wraps an error (`signal.go:45-48`).
- `signal.IsTerminated(err)` detects it via `errors.Is` (`signal.go:50-53`).

`signal.Go` delegates to `github.com/alexfalkowski/go-sync` and triggers `signal.Shutdown()` when it sees a terminated error (`signal.go:55-67`).

### Stop error aggregation

Stopping hooks accumulates errors and returns them via `errors.Join` (`signal.go:217-224`). Start stops on first error (`signal.go:208-215`).

## Testing approach and gotchas

- Tests are in `package signal_test` (black-box style).
- Many `Serve`/`Timer` tests call `signal.Shutdown()` from a goroutine after `time.Sleep(time.Second)` to unblock `signal.Serve(...)` (e.g., `serve_test.go:19-25`). These are timing-sensitive by design.
- `TestHTTPServe` binds to an ephemeral loopback port (`127.0.0.1:0`) rather than a fixed port (`signal_test.go:22-35`).
- Tests often pass `t.Context()` (Go 1.20+ API) into the library calls (e.g., `signal_test.go:47-48`).
