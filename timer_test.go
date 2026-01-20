package signal_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/stretchr/testify/require"
)

func TestTimerWithTick(t *testing.T) {
	ticked := make(chan struct{})
	once := sync.Once{}
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, 0, time.Millisecond, signal.Hook{
				OnStart: func(context.Context) error {
					return nil
				},
				OnTick: func(context.Context) error {
					once.Do(func() { close(ticked) })
					return nil
				},
			})
		},
	})

	go func() {
		select {
		case <-ticked:
			cancel()
		case <-time.After(time.Second):
			cancel()
		}
	}()

	require.NoError(t, signal.Serve(ctx))

	select {
	case <-ticked:
	default:
		t.Fatal("tick not observed")
	}
}

func TestTimerWithNoTick(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			close(started)
			return signal.Timer(ctx, 0, time.Second, signal.Hook{
				OnStart: func(context.Context) error {
					return nil
				},
			})
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
}

func TestTimerStartError(t *testing.T) {
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			close(started)
			return signal.Timer(ctx, 0, time.Second, signal.Hook{
				OnStart: func(context.Context) error {
					time.Sleep(10 * time.Millisecond)
					return errServe
				},
			})
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
}

func TestTimerTickStopError(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	once := sync.Once{}
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			close(started)
			return signal.Timer(ctx, 0, time.Second, signal.Hook{
				OnStop: func(context.Context) error {
					once.Do(func() { close(stopped) })
					return errServe
				},
			})
		},
	})

	go func() {
		<-started
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))

	select {
	case <-stopped:
	default:
		t.Fatal("stop not observed")
	}
}

func TestTimerTickError(t *testing.T) {
	ticked := make(chan struct{})
	once := sync.Once{}
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, 0, time.Millisecond, signal.Hook{
				OnTick: func(context.Context) error {
					once.Do(func() { close(ticked) })
					return errServe
				},
			})
		},
	})

	go func() {
		select {
		case <-ticked:
			cancel()
		case <-time.After(time.Second):
			cancel()
		}
	}()

	require.NoError(t, signal.Serve(ctx))

	select {
	case <-ticked:
	default:
		t.Fatal("tick not observed")
	}
}
