package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/alexfalkowski/go-signal/internal/test"
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

	require.Error(t, signal.Serve(t.Context()))
}

func TestServeStartRollback(t *testing.T) {
	startErr1 := errors.New("signal: serve start error 1")
	startErr2 := errors.New("signal: serve start error 2")
	stopErr := errors.New("signal: serve stop error")

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	events := test.RegisterRollbackHooks(startErr1, startErr2, stopErr)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- signal.Serve(ctx)
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, startErr1)
		require.ErrorIs(t, err, startErr2)
		require.ErrorIs(t, err, stopErr)
	case <-time.After(100 * time.Millisecond):
		require.Fail(t, "Serve blocked after startup failure")
	}

	require.Equal(t, []string{
		"start:1",
		"start:2",
		"start:3",
		"start:4",
		"stop:3",
		"stop:1",
	}, *events)
}

func TestServeStopOrder(t *testing.T) {
	events := make([]string, 0, 3)

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	for _, event := range []string{"stop:1", "stop:2", "stop:3"} {
		signal.Register(signal.Hook{
			OnStop: func(context.Context) error {
				events = append(events, event)
				return nil
			},
		})
	}

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	require.NoError(t, signal.Serve(ctx))
	require.Equal(t, []string{"stop:3", "stop:2", "stop:1"}, events)
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
	ch := make(chan bool, 1)

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
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
	ch := make(chan bool, 1)

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
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
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestTimerStartErrorStopsHook(t *testing.T) {
	stopped := false
	stopErr := errors.New("signal: timer start stop error")

	err := signal.Timer(t.Context(), time.Second, time.Millisecond, signal.Hook{
		OnStart: func(context.Context) error {
			return errServe
		},
		OnStop: func(context.Context) error {
			stopped = true
			return stopErr
		},
	})

	require.ErrorIs(t, err, errServe)
	require.ErrorIs(t, err, stopErr)
	require.True(t, stopped)
}

func TestTimerTickStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStop: func(context.Context) error {
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

func TestTimerTickErrorStopsHook(t *testing.T) {
	stopped := false
	stopErr := errors.New("signal: timer tick stop error")

	err := signal.Timer(t.Context(), time.Second, time.Millisecond, signal.Hook{
		OnTick: func(context.Context) error {
			return errServe
		},
		OnStop: func(context.Context) error {
			stopped = true
			return stopErr
		},
	})

	require.ErrorIs(t, err, errServe)
	require.ErrorIs(t, err, stopErr)
	require.True(t, stopped)
}

func TestTimerTickError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
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

func TestTimerZeroInterval(t *testing.T) {
	err := signal.Timer(t.Context(), time.Second, 0, signal.Hook{})
	require.ErrorIs(t, err, signal.ErrInvalidInterval)
}

func TestTimerNegativeInterval(t *testing.T) {
	err := signal.Timer(t.Context(), time.Second, -time.Second, signal.Hook{})
	require.ErrorIs(t, err, signal.ErrInvalidInterval)
}
