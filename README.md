![Gopher](assets/gopher.png)
[![CircleCI](https://circleci.com/gh/alexfalkowski/go-signal.svg?style=shield)](https://circleci.com/gh/alexfalkowski/go-signal)
[![codecov](https://codecov.io/gh/alexfalkowski/go-signal/graph/badge.svg?token=Q7B3VZYL9K)](https://codecov.io/gh/alexfalkowski/go-signal)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexfalkowski/go-signal)](https://goreportcard.com/report/github.com/alexfalkowski/go-signal)
[![Go Reference](https://pkg.go.dev/badge/github.com/alexfalkowski/go-signal.svg)](https://pkg.go.dev/github.com/alexfalkowski/go-signal)
[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)

# 📡 go-signal

A small Go library for coordinating application startup and shutdown hooks
around OS signals.

The package centers on a `Lifecycle` that runs hooks in three phases:

- start: call each registered `OnStart` in registration order
- run: execute your handler with `Run` or wait for shutdown with `Serve`
- stop: call each registered `OnStop` in reverse registration order

The package-level helpers operate on a process-wide default lifecycle initialized
with `NewDefaultLifecycle()`, which uses a 30-second stop timeout.

`signal.ErrTimeout` is the package-owned timeout cause used for lifecycle stop
contexts and timer stop hooks. It wraps `github.com/alexfalkowski/go-sync`'s
`sync.ErrTimeout`, which in turn wraps `context.DeadlineExceeded`.

## 📦 Install

Use the Go toolchain version declared in [go.mod](go.mod).

```sh
go get github.com/alexfalkowski/go-signal
```

## 🧪 Development

### Benchmarks

```sh
make benchmarks
make lifecycle-benchmarks
make benchtime=10x benchmark
```

### Fuzz smoke tests

```sh
make fuzz-smoke
make lifecycle-fuzz
make timer-fuzz
make terminated-fuzz
make package=. name=FuzzLifecycleRunHookMatrix fuzztime=10s fuzz
```

## 🧭 Core API

### 🧱 NewDefaultLifecycle

Use `NewDefaultLifecycle` when you want the package's standard lifecycle
configuration without relying on the package-level singleton.

### 🧩 Register

Use `Register` during application setup to add hooks to the default lifecycle.
Hook callbacks are optional, and nil callbacks are treated as no-ops.

> [!IMPORTANT]
> Register hooks during single-threaded setup before calling `Run` or `Serve`;
> registration is not concurrent-safe.

```go
signal.Register(signal.Hook{
    OnStart: func(context.Context) error {
        // Acquire resources.
        return nil
    },
    OnStop: func(context.Context) error {
        // Release resources.
        return nil
    },
})
```

### ▶️ Run

`Run` executes all start hooks, then your handler, then all stop hooks.

- `Run` attempts every start hook, even if an earlier one fails.
- If startup fails, `Run` rolls back by calling stop hooks for the hooks that
  started successfully in reverse registration order.
- `Run` executes rollback and stop hooks with a fresh background context bounded
  by the lifecycle timeout.
- If a rollback or stop hook returns `context.Cause(ctx)` after that stop
  context expires, the returned error matches `signal.ErrTimeout`.
- After successful startup, `Run` always runs stop hooks when the handler
  returns, even if it returns an error, in reverse registration order.
- Startup, handler, and stop-hook errors are combined with `errors.Join`.
- `Run` does not recover panics from hooks or the handler, so panic recovery
  should live in application code when cleanup must be guaranteed.

```go
import (
    "context"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(context.Context) error {
        // Start dependencies.
        return nil
    },
    OnStop: func(context.Context) error {
        // Stop dependencies.
        return nil
    },
})

err := signal.Run(context.Background(), func(context.Context) error {
    // Run application code.
    return nil
})
```

### 🛎️ Serve

`Serve` is the long-running variant for processes that should stay alive until
shutdown is requested.

- It runs start hooks first.
- If startup fails, it still attempts the remaining start hooks and then rolls
  back successfully started hooks in reverse registration order.
- It waits for `SIGINT` or `SIGTERM`, or for the parent context to be canceled.
- It then runs stop hooks with a fresh background context bounded by the
  lifecycle timeout in reverse registration order.
- If a stop hook returns `context.Cause(ctx)` after that stop context expires,
  the returned error matches `signal.ErrTimeout`.
- Normal shutdown from parent cancellation, `SIGINT`, `SIGTERM`, or `Shutdown()`
  returns nil unless startup, rollback, or stop hooks return errors.
- Shutdown from `Terminate(err)` returns the terminating cause, joined with any
  stop-hook errors.
- During signal takeover, there is a narrow startup handoff window where an
  incoming `SIGINT` or `SIGTERM` may need to be sent again.
- If background work should stop the process after `Go`'s wait window has
  elapsed, wrap its error with `signal.Terminated(err)` so `Go` can request
  shutdown from the background goroutine.

> [!NOTE]
> `Serve` is intended to be used as the final process-lifetime blocking call.
> It takes ownership of `SIGINT` and `SIGTERM`, does not restore prior signal
> handlers after returning, and callers should normally return from `main` after
> `Serve` returns.

```go
import (
    "context"
    "errors"
    "net"
    "net/http"
    "time"

    "github.com/alexfalkowski/go-signal"
)

srv := &http.Server{ReadHeaderTimeout: time.Minute}

signal.Register(signal.Hook{
    OnStart: func(ctx context.Context) error {
        ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:8080")
        if err != nil {
            return err
        }

        return signal.Go(ctx, time.Second, func(context.Context) error {
            if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
                return signal.Terminated(err)
            }

            return nil
        })
    },
    OnStop: func(ctx context.Context) error {
        return srv.Shutdown(ctx)
    },
})

err := signal.Serve(context.Background())
```

### 🛑 Shutdown

`Shutdown` sends `os.Interrupt` to the current process through the default
lifecycle. It is mainly useful for programmatic shutdown, such as from tests or
from a background goroutine that wants to stop a running `Serve` loop.

### 🧯 Terminate

`Terminate` records a shutdown cause on the default lifecycle, then sends
`os.Interrupt` to the current process. Use it when background work should stop a
running `Serve` loop and have `Serve` return the terminating cause after stop
hooks run.

Passing `nil` records `ErrTerminated`. Passing a non-nil error marks it with
`ErrTerminated` unless it is already marked.

## 🧰 Background helpers

### 🚀 Go

`Go` runs a handler with a timeout-aware wait. It is useful for long-running
background work that should share a lifecycle context.

- `Go` only reports errors observed before its timeout or parent context
  cancellation.
- If the parent context is already done or the timeout is not positive, `Go`
  returns nil without starting the handler.
- If the worker keeps running after that waiting window, later non-terminated
  errors are not returned to the caller.
- If the handler returns an error marked with `signal.Terminated(err)`, `Go`
  triggers `Terminate(err)` before returning the error.
- If that terminated error happens after the waiting window has elapsed,
  `Terminate(err)` is still triggered, but `Go` has already returned `nil`.

```go
import (
    "context"
    "time"

    "github.com/alexfalkowski/go-signal"
)

err := signal.Go(context.Background(), 5*time.Second, func(context.Context) error {
    // Run background work.
    return nil
})
```

### ⏱️ Timer

`Timer` is a convenience helper for periodic work:

- call `hook.OnStart` once
- call `hook.OnTick` on each interval
- if `hook.OnStart` fails, or when the parent context is canceled or a timer
  hook returns an error, the timer worker calls `hook.OnStop` with a fresh
  background context bounded by the supplied timeout
- if that stop context expires and the hook returns `context.Cause(ctx)`, the
  returned error matches `signal.ErrTimeout`

Because `Timer` executes through `Go`, it uses best-effort waiting semantics:
when the parent context is canceled or the wait timeout elapses first, `Timer`
may return before the timer worker has run `hook.OnStop`, and late
non-terminated hook errors are not returned to the caller. With a valid
interval, if the parent context is already done or the timeout is not positive,
the timer worker does not start.

The interval must be greater than zero or `Timer` returns `ErrInvalidInterval`.

```go
import (
    "context"
    "time"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(ctx context.Context) error {
        return signal.Timer(ctx, 5*time.Second, time.Minute, signal.Hook{
            OnStart: func(context.Context) error {
                // Initialize the periodic worker.
                return nil
            },
            OnTick: func(context.Context) error {
                // Perform one scheduled iteration.
                return nil
            },
            OnStop: func(context.Context) error {
                // Flush or clean up.
                return nil
            },
        })
    },
})
```

## 🛠️ Custom lifecycle

Use `NewLifeCycle` when you need a custom stop timeout. Use
`NewDefaultLifecycle` when you want the same 30-second timeout as the
package-level default, but on your own lifecycle instance.

Use a positive custom stop timeout. A zero or negative timeout creates an
already-expired stop context, so stop hooks that observe the context may return
`signal.ErrTimeout` immediately.

Use `SetDefault` when package-level helpers such as `Register`, `Run`, and
`Serve` should target a custom lifecycle. Passing `nil` to `SetDefault` restores
a fresh default lifecycle. `Default` returns the current package-level
lifecycle.

```go
import (
    "context"
    "time"

    "github.com/alexfalkowski/go-signal"
)

lc := signal.NewLifeCycle(10 * time.Second)

lc.Register(signal.Hook{
    OnStart: func(context.Context) error { return nil },
    OnStop:  func(context.Context) error { return nil },
})

err := lc.Run(context.Background(), func(context.Context) error {
    return nil
})
```

To use that lifecycle through the package-level helpers:

```go
signal.SetDefault(lc)
defer signal.SetDefault(nil)

err := signal.Run(context.Background(), func(context.Context) error {
    return nil
})
```

## 🧪 Example

See [cmd/main.go](cmd/main.go) for a runnable example covering `Serve`, `Go`,
`Timer`, and termination-triggered shutdown. It is a manual `make run` example,
not an installed or production CLI.

Initialize the shared `bin` tooling first when running Make targets from a
fresh clone. The submodule uses GitHub SSH, so SSH access must be configured
before this step:

```sh
git submodule sync
git submodule update --init
```

```sh
make run param=terminate
```

Use `param=start` or `param=timer` for examples that run until interrupted. Use
`param=terminate` for the example that triggers shutdown from a terminated
background worker.
