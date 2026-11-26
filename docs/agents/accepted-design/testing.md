# Accepted design for testing

Part of the [accepted-design index](../accepted-design.md).

## Application and component tests

Use external test packages such as `signal_test` and pass `t.Context()` where
appropriate. Use `App.Run` and `App.Serve` for application behavior and
`signaltest.NewLifecycle` for component hooks. The helper wraps the same internal
hook engine and lets component tests drive startup and cleanup directly.

Run OS signal tests serially because notifications change process-wide handlers.
Bind HTTP lifecycle tests to `127.0.0.1:0` so the OS selects an available port.

[Application tests](../../../app_test.go) · [Component helper tests](../../../signaltest/lifecycle_test.go)

## Manual application

`cmd/main.go` is a manual testing script exercised through `make run` with
`param=start`, `param=timer`, or `param=terminate`. Scope automated coverage
reviews to the library's supported entrypoints and treat the script as a manual
example.

[Manual application](../../../cmd/main.go) · [Run target](../../../Makefile)
