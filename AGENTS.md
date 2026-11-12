# AGENTS.md

## Shared guidance

Use `bin/AGENTS.md` for shared skills and cross-repository defaults.

## Repo

- Module: `github.com/alexfalkowski/go-signal/v2`.
- Read current toolchain details from repo files such as `go.mod`,
  `.circleci/config.yml`, and `bin/`; do not duplicate them here.
- Purpose: small library for startup/shutdown hooks around OS signals.
- Main files: `signal.go`, `app.go`, `options.go`, `hook.go`, `timer.go`, `app_test.go`, `timer_test.go`,
  `internal/lifecycle/`, `signaltest/`,
  `cmd/main.go`, `README.md`, `.circleci/config.yml`.
- The root `signal` package owns `App`, construction options, `Hook`, `Handler`,
  `Lifecycle`, `Shutdowner`, timeout/panic errors, and OS signal selectors.
- Lifecycle execution is internal; root signal owns OS notification.
  The public API has no process-wide default lifecycle or public `di` and
  `lifecycle` packages. `App` exposes only `Run` and `Serve`; `signaltest`
  wraps the internal engine for component tests.
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

- `NewApp` registers Dig constructors and decorators, then runs setup functions.
  Construction panics become errors. Resource acquisition belongs in start hooks.
- `Lifecycle.Append` is setup-time only and is not concurrent-safe. Configure
  hooks before calling `App.Run` or `App.Serve`.
- Hooks have optional callbacks; nil callbacks are no-ops. Hook panics become
  errors matching `signal.ErrRecovered`.
- Each application owns its lifecycle and programmatic shutdown request.
  It supports one execution; construct a fresh app for each command or retry.
  Startup stops at the first error/cancellation and rolls back successfully
  started hooks. Cleanup runs in reverse order and joins errors.
- Default startup and stop deadlines are 15 seconds. An available result takes
  precedence over waiting-context cancellation. Waiting cannot interrupt a Go
  callback; late startup completion still receives cleanup after a canceled wait.
- Internal cleanup detaches cancellation so hooks can run with a fresh
  deadline. Timeout errors match `signal.ErrTimeout` and `context.DeadlineExceeded`.
- `App.Serve` listens for SIGINT/SIGTERM and additional signals through
  `os/signal.NotifyContext` and unregisters its notifications on return.
- `Shutdowner.Shutdown(code)` is application-local and sends no OS signal. The
  first code wins, including a buffered request when cancellation is also ready.
  A non-zero code returns `ExitError`, joined with startup or cleanup errors.
- The caller owns process exit. `App.Run` and `App.Serve` consume shutdown
  requests internally; `App.Serve` additionally handles OS signals.
- `Timer(ctx, interval, tick)` blocks and calls a non-nil handler sequentially,
  starting after one positive interval. Cancellation between callbacks returns
  nil; callback errors take precedence and panics match `ErrRecovered`.
  Non-positive intervals return `ErrInvalidInterval`. Lifecycle hooks own worker
  cancellation and waiting through `go-sync.Go` and its task.

## Tests

- Tests use external packages such as `signal_test` and commonly pass `t.Context()`.
- Do not add tests for `cmd/main.go`; it is only a manual testing script.
- Signal tests run serially because notification changes process-wide handlers.
- HTTP lifecycle tests bind to `127.0.0.1:0`.
