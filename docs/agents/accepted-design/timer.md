# Accepted design for Timer

Part of the [accepted-design index](../accepted-design.md).

## Callback scheduling and cancellation

`Timer(ctx, interval, tick)` blocks and calls a non-nil handler sequentially,
starting after one positive interval. An interval of zero or less returns
`ErrInvalidInterval`. Cancellation observed between callbacks returns `nil`.
A running callback must observe its context. Its error takes precedence over
cancellation, and a recovered panic matches `ErrRecovered`.

[Timer implementation](../../../timer.go) · [Executable example](../../../example_test.go)

## Background worker ownership

To run a timer in the background, launch it with `go-sync.Go` from `OnStart`
using a worker-owned cancelable context. The startup context ends when startup
finishes. `OnStop` cancels the worker and awaits its task with the stop context.
The owning component controls the task's lifetime.

[Timer tests](../../../timer_test.go) · [Manual worker](../../../cmd/main.go)
