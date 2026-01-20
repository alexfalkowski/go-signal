package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/stretchr/testify/require"
)

var errServe = errors.New("signal: serve error")

func TestServeEmpty(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			close(started)
			return nil
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
}

func TestServeStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return errServe
		},
	})

	require.Error(t, signal.Serve(t.Context()))
}

func TestServeGoError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Minute, func(context.Context) error {
				return errServe
			})
		},
	})

	require.Error(t, signal.Serve(t.Context()))
}

func TestServeGoTerminated(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Second, func(context.Context) error {
				time.Sleep(2 * time.Second)
				return signal.Terminated(errServe)
			})
		},
	})

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServeStopError(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			close(started)
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return errServe
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.Error(t, signal.Serve(ctx))
}

func TestServeStopContextNoError(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			close(started)
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			return ctx.Err()
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
}

func TestServeStartContext(t *testing.T) {
	started := make(chan struct{})
	ch := make(chan bool, 1)
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			close(started)
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				<-ctx.Done()
				ch <- true
				return nil
			})
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
	require.True(t, <-ch)
}

func TestServeStartLoopContext(t *testing.T) {
	started := make(chan struct{})
	ch := make(chan bool, 1)
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			close(started)
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				for {
					select {
					case <-ctx.Done():
						ch <- true
						return nil
					default:
						time.Sleep(time.Millisecond)
					}
				}
			})
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
	require.True(t, <-ch)
}
