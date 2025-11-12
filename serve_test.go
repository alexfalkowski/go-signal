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
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServeStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return errServe
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

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

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

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
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return errServe
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.Error(t, signal.Serve(t.Context()))
}

func TestServeStopContextNoError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			return ctx.Err()
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServeStartContext(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	ch := make(chan bool, 1)
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				<-ctx.Done()
				ch <- true
				return nil
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
	require.True(t, <-ch)
}

func TestServeStartLoopContext(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	ch := make(chan bool, 1)
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				for {
					select {
					case <-ctx.Done():
						ch <- true
					default:
						time.Sleep(time.Millisecond)
					}
				}
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
	require.True(t, <-ch)
}

func TestTimerWithTick(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStart: func(context.Context) error {
					return nil
				},
				OnTick: func(context.Context) error {
					return nil
				},
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestTimerWithNoTick(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStart: func(context.Context) error {
					return nil
				},
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestTimerStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStart: func(context.Context) error {
					time.Sleep(10 * time.Millisecond)
					return errServe
				},
				OnTick: func(context.Context) error {
					return nil
				},
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestTimerTickError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStart: func(context.Context) error {
					return nil
				},
				OnTick: func(context.Context) error {
					return errServe
				},
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}
