# AGENTS.md

## Repository overview

This is a small Go library (`module github.com/alexfalkowski/go-signal`) for coordinating application start/stop hooks around OS signals.

Key public entry points (see `signal.go`):
- `signal.Register(Hook)`
- `signal.Run(ctx, handler)`
- `signal.Serve(ctx)`
- `signal.Shutdown()`
- `signal.Go(ctx, timeout, handler)` and `signal.Timer(...)`

Go version is declared in `go.mod` (`go 1.25.0`).

## Layout

- `signal.go`: main library implementation.
- `cmd/main.go`: example program used by `make run`.
- `*_test.go`: unit/integration-style tests.
- `test/reports/`: CI artifacts (JUnit XML, coverage outputs).
- `bin/`: git submodule containing shared build tooling and Makefile includes.
- `.circleci/config.yml`: CI pipeline; mirrors the Make targets used locally.

## Tooling and prerequisites

### Git submodule

The top-level `Makefile` includes makefiles from the `bin/` submodule:

- `Makefile:1-2` includes `bin/build/make/go.mak` and `bin/build/make/git.mak`.

If `bin/` is not initialized, most `make` targets will fail.

Initialize the submodule:

```sh
git submodule sync
git submodule update --init
```

### Formatting / editor settings

`.editorconfig` enforces:
- LF line endings.
- Tabs for `*.go` files.
- Tabs for `Makefile` recipes.

## Essential commands

### Build / run

Run the example program (delegates to `go run cmd/main.go $(param)`):

```sh
make run param=start
make run param=timer
make run param=terminate
```

### Dependencies

This repo uses vendoring via Make targets from `bin/build/make/go.mak`:

```sh
make dep    # go mod download, tidy, vendor
make clean  # uses bin/build/go/clean
```

### Tests

Fast local test run:

```sh
go test ./...
```

CI-style test run (race + coverage + junit report, uses vendored deps):

```sh
make specs
```

Notes (observed from `bin/build/make/go.mak:62-64`):
- Uses `gotestsum --junitfile test/reports/specs.xml -- -vet=off -race -mod vendor ...`.
- Produces `test/reports/profile.cov` and then post-processes coverage for reports.

### Lint

```sh
make lint
```

This runs:
- Field alignment check: `bin/build/go/fa`
- GolangCI-Lint wrapper: `bin/build/go/lint run --timeout 5m`

Linter configuration is in `.golangci.yml`.

### Security

```sh
make sec
```

This runs `govulncheck -show verbose -test ./...` (see `bin/build/make/go.mak:96-98`).

## Testing gotchas

- `TestHTTPServe` binds to `:8080` (see `signal_test.go:16-50`). If that port is already in use on your machine, `go test ./...` / `make specs` will fail with `bind: address already in use`.
- Several tests call `signal.Shutdown()` in a goroutine after `time.Sleep(time.Second)` to unblock `signal.Serve(...)` (see `serve_test.go` and `signal_test.go`). These are timing-sensitive by design.

## Code patterns and conventions

### Public API pattern

- The package exposes a default lifecycle stored in an `atomic.Pointer` (`signal.go:106-120`).
- The default lifecycle is initialized in `init()` with a 30s timeout (`signal.go:108-110`).
- Tests frequently override defaults using `signal.SetDefault(signal.NewLifeCycle(...))`.

### Hook lifecycle

- `Hook` is a struct of optional handlers (`OnStart`, `OnTick`, `OnStop`). Methods `Start/Tick/Stop` are nil-safe.
- `Lifecycle.Serve` resets/ignores signals first and then uses `signal.NotifyContext` (`signal.go:173-193`).

### Error semantics

- Termination is signaled by wrapping errors with `signal.Terminated(err)` and detected via `signal.IsTerminated(err)`.
- `signal.Go` delegates to `github.com/alexfalkowski/go-sync` and triggers `Shutdown()` when it sees a terminated error (`signal.go:55-67`).
- When stopping hooks, errors are accumulated and returned via `errors.Join` (`signal.go:210-218`).

## CI mapping

CircleCI runs (in order) roughly:

```sh
make clean
make dep
make lint
make sec
make specs
make coverage
make codecov-upload
```

See `.circleci/config.yml:19-55`.
