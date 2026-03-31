![Gopher](assets/gopher.png)
[![CircleCI](https://circleci.com/gh/alexfalkowski/go-signal.svg?style=shield)](https://circleci.com/gh/alexfalkowski/go-signal)
[![codecov](https://codecov.io/gh/alexfalkowski/go-signal/graph/badge.svg?token=Q7B3VZYL9K)](https://codecov.io/gh/alexfalkowski/go-signal)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexfalkowski/go-signal)](https://goreportcard.com/report/github.com/alexfalkowski/go-signal)
[![Go Reference](https://pkg.go.dev/badge/github.com/alexfalkowski/go-signal.svg)](https://pkg.go.dev/github.com/alexfalkowski/go-signal)
[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)

# go-signal

A small Go library for coordinating application startup and shutdown hooks
around OS signals.

The package centers on a `Lifecycle` that runs hooks in three phases:

- start: call each registered `OnStart`
- run: execute your handler with `Run` or wait for shutdown with `Serve`
- stop: call each registered `OnStop`

The package-level helpers operate on a process-wide default lifecycle initialized
with a 30 second stop timeout.

## Install

```sh
go get github.com/alexfalkowski/go-signal
```

## Core API

### Register

Use `Register` during application setup to add hooks to the default lifecycle.
Hook callbacks are optional, and nil callbacks are treated as no-ops.

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

### Run

`Run` executes all start hooks, then your handler, then all stop hooks.

- `Run` attempts every start hook, even if an earlier one fails.
- If startup fails, `Run` rolls back by calling stop hooks for the hooks that
  started successfully.
- `Run` executes rollback and stop hooks with a fresh background context bounded
  by the lifecycle timeout.
- After successful startup, `Run` always runs stop hooks, even if the handler
  fails.
- Startup, handler, and stop-hook errors are combined with `errors.Join`.

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

### Serve

`Serve` is the long-running variant for processes that should stay alive until
shutdown is requested.

- It runs start hooks first.
- If startup fails, it still attempts the remaining start hooks and then rolls
  back successfully started hooks.
- It waits for `SIGINT` or `SIGTERM`, or for the parent context to be canceled.
- It then runs stop hooks with a fresh background context bounded by the
  lifecycle timeout.
- During signal takeover, there is a narrow startup handoff window where an
  incoming `SIGINT` or `SIGTERM` may need to be sent again.

While `Serve` is active it takes ownership of `SIGINT` and `SIGTERM`, so other
signal handlers for those signals will not run during that time.

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
                return err
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

### Shutdown

`Shutdown` sends `os.Interrupt` to the current process through the default
lifecycle. It is mainly useful for programmatic shutdown, such as from tests or
from a background goroutine that wants to stop a running `Serve` loop.

## Background helpers

### Go

`Go` runs a handler with a timeout-aware wait. It is useful for long-running
background work that should share a lifecycle context.

- `Go` only reports errors observed before its timeout or parent context
  cancellation.
- If the worker keeps running after that waiting window, later non-terminated
  errors are not returned to the caller.
- If the handler returns an error marked with `signal.Terminated(err)`, `Go`
  triggers `Shutdown()` before returning the error.
- If that terminated error happens after the waiting window has elapsed,
  `Shutdown()` is still triggered, but `Go` has already returned `nil`.

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

### Timer

`Timer` is a convenience helper for periodic work:

- call `hook.OnStart` once
- call `hook.OnTick` on each interval
- when the parent context is canceled or a timer hook returns an error, call
  `hook.OnStop` with a fresh background context bounded by the supplied timeout

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

## Custom lifecycle

Use `NewLifeCycle` when you do not want to rely on the package-level default
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

## Example

See [cmd/main.go](cmd/main.go) for a runnable example covering `Serve`, `Go`,
`Timer`, and termination-triggered shutdown.
