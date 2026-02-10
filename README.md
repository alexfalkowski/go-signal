![Gopher](assets/gopher.png)
[![CircleCI](https://circleci.com/gh/alexfalkowski/go-signal.svg?style=shield)](https://circleci.com/gh/alexfalkowski/go-signal)
[![codecov](https://codecov.io/gh/alexfalkowski/go-signal/graph/badge.svg?token=Q7B3VZYL9K)](https://codecov.io/gh/alexfalkowski/go-signal)
[![Go Report Card](https://goreportcard.com/badge/github.com/alexfalkowski/go-signal)](https://goreportcard.com/report/github.com/alexfalkowski/go-signal)
[![Go Reference](https://pkg.go.dev/badge/github.com/alexfalkowski/go-signal.svg)](https://pkg.go.dev/github.com/alexfalkowski/go-signal)
[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)

# go-signal

A library for coordinating application start/stop hooks around OS signals.

## Background

This library has been inspired by the following articles:

- <https://gobyexample.com/signals>
- <https://goperf.dev/01-common-patterns/context/>
- <https://pkg.go.dev/golang.org/x/sync/errgroup>

## Go

Go waits for the handler to complete or for timeout to elapse. As an example:

```go
import (
    "context"
    "time"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(ctx context.Context) error {
        return signal.Go(ctx, time.Second, func(context.Context) error {
            // Do something that starts.
            return nil
        })
    },
})
```

## Timer

Timer runs a hook that ticks at an interval until its context is done. As an example:

```go
import (
    "context"
    "time"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(ctx context.Context) error {
        return signal.Timer(ctx, time.Second, time.Second, signal.Hook{
            OnStart: func(context.Context) error {
                // Do something that starts.
                return nil
            },
            OnTick: func(context.Context) error {
                // Do something that ticks.
                return nil
            },
            OnStop: func(context.Context) error {
                // Do something that stops.
                return nil
            },
        })
    },
})
```

## Run

Run runs start hooks, then your handler, then stop hooks. As an example:

```go
import (
    "context"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(context.Context) error {
        // Do something that starts.
        return nil
    },
    OnStop: func(context.Context) error {
        // Do something that stops.
        return nil
    },
})

// Do something with err.
err := signal.Run(context.Background(), func(context.Context) error {
    // Your own app.
    return nil
})
```

## Serve

Serve runs start hooks, waits for SIGINT/SIGTERM, then runs stop hooks. As an example:

```go
import (
    "context"

    "github.com/alexfalkowski/go-signal"
)

signal.Register(signal.Hook{
    OnStart: func(context.Context) error {
        // Do something that starts.
        return nil
    },
    OnStop: func(context.Context) error {
        // Do something that stops.
        return nil
    },
})

// Do something with err.
err := signal.Serve(context.Background())
```

## Example

Check out the [example](cmd/main.go) for more information.
