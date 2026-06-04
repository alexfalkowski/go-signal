package signal_test

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/alexfalkowski/go-signal/internal/test"
	"github.com/stretchr/testify/require"
)

var errSignal = errors.New("signal: test error")

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
			return errSignal
		},
	})

	require.Error(t, signal.Serve(t.Context()))
}

func TestServeStartRollback(t *testing.T) {
	hook2StartErr := errors.New("signal: serve hook 2 start error")
	hook3StopErr := errors.New("signal: serve hook 3 stop error")
	hook4StartErr := errors.New("signal: serve hook 4 start error")

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	events := test.RegisterRollbackHooks(hook2StartErr, hook3StopErr, hook4StartErr)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err := signal.Serve(ctx)
	require.ErrorIs(t, err, hook2StartErr)
	require.ErrorIs(t, err, hook3StopErr)
	require.ErrorIs(t, err, hook4StartErr)

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

func TestServeSIGTERM(t *testing.T) {
	started := make(chan struct{})

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			close(started)
			return nil
		},
	})

	go func() {
		<-started
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(syscall.SIGTERM)
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServeGoError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Minute, func(context.Context) error {
				return errSignal
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
				return signal.Terminated(errSignal)
			})
		},
	})

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServeStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return errSignal
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

func TestServeStopTimeoutCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Microsecond))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			cancel()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			<-ctx.Done()
			return context.Cause(ctx)
		},
	})

	err := signal.Serve(ctx)

	require.ErrorIs(t, err, signal.ErrTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestServeStartContext(t *testing.T) {
	canceled := make(chan struct{})

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				<-ctx.Done()
				close(canceled)
				return nil
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
	<-canceled
}

func TestServeStartLoopContext(t *testing.T) {
	canceled := make(chan struct{})

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Second, func(ctx context.Context) error {
				for {
					select {
					case <-ctx.Done():
						close(canceled)
						return nil
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
	<-canceled
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
					return errSignal
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
			return errSignal
		},
		OnStop: func(context.Context) error {
			stopped = true
			return stopErr
		},
	})

	require.ErrorIs(t, err, errSignal)
	require.ErrorIs(t, err, stopErr)
	require.True(t, stopped)
}

func TestTimerCancelStopsHook(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{})
	stopped := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- signal.Timer(ctx, time.Second, time.Millisecond, signal.Hook{
			OnStart: func(context.Context) error {
				close(started)
				return nil
			},
			OnStop: func(context.Context) error {
				close(stopped)
				return nil
			},
		})
	}()

	<-started
	cancel()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		require.Fail(t, "Timer did not stop hook after cancellation")
	}

	require.NoError(t, <-done)
}

func TestTimerTickStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnStop: func(context.Context) error {
					return errSignal
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
	events := make([]string, 0, 3)
	stopErr := errors.New("signal: timer tick stop error")

	err := signal.Timer(t.Context(), time.Second, time.Millisecond, signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start")
			return nil
		},
		OnTick: func(context.Context) error {
			events = append(events, "tick")
			return errSignal
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop")
			return stopErr
		},
	})

	require.ErrorIs(t, err, errSignal)
	require.ErrorIs(t, err, stopErr)
	require.Equal(t, []string{"start", "tick", "stop"}, events)
}

func TestTimerTickError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Timer(ctx, time.Millisecond, time.Millisecond, signal.Hook{
				OnTick: func(context.Context) error {
					return errSignal
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

func TestTerminatedNil(t *testing.T) {
	err := signal.Terminated(nil)

	require.ErrorIs(t, err, signal.ErrTerminated)
	require.EqualError(t, err, signal.ErrTerminated.Error())
}

func TestTerminatedError(t *testing.T) {
	err := signal.Terminated(errSignal)

	require.True(t, signal.IsTerminated(err))
	require.ErrorIs(t, err, signal.ErrTerminated)
	require.ErrorIs(t, err, errSignal)
}
