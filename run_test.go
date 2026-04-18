package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/alexfalkowski/go-signal/internal/test"
	"github.com/alexfalkowski/go-sync"
	"github.com/stretchr/testify/require"
)

var errRun = errors.New("signal: run error")

func TestRunEmpty(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{})

	require.NoError(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestSetDefaultNilResetsLifecycle(t *testing.T) {
	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return errRun
		},
	})

	signal.SetDefault(lifecycle)
	signal.SetDefault(nil)

	started := false
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			started = true
			return nil
		},
	})

	require.NoError(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
	require.True(t, started)
}

func TestNewDefaultLifecycle(t *testing.T) {
	lifecycle := signal.NewDefaultLifecycle()
	started := false

	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			started = true
			return nil
		},
	})

	require.NoError(t, lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	}))
	require.True(t, started)
}

func TestRunError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	stopped := false
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			stopped = true
			return nil
		},
	})

	err := signal.Run(t.Context(), func(context.Context) error {
		return errRun
	})

	require.ErrorIs(t, err, errRun)
	require.True(t, stopped)
}

func TestRunStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return errRun
		},
	})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestRunStartRollback(t *testing.T) {
	startErr1 := errors.New("signal: run start error 1")
	startErr2 := errors.New("signal: run start error 2")
	stopErr := errors.New("signal: run stop error")
	handlerCalled := false

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	events := test.RegisterRollbackHooks(startErr1, startErr2, stopErr)

	err := signal.Run(t.Context(), func(context.Context) error {
		handlerCalled = true
		return nil
	})

	require.ErrorIs(t, err, startErr1)
	require.ErrorIs(t, err, startErr2)
	require.ErrorIs(t, err, stopErr)
	require.False(t, handlerCalled)
	require.Equal(t, []string{
		"start:1",
		"start:2",
		"start:3",
		"start:4",
		"stop:3",
		"stop:1",
	}, *events)
}

func TestRunStartRollbackFreshStopContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stopped := false

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			stopped = true
			return ctx.Err()
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			cancel()
			return errRun
		},
	})

	err := signal.Run(ctx, func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, errRun)
	require.NotErrorIs(t, err, context.Canceled)
	require.True(t, stopped)
}

func TestRunStopOrder(t *testing.T) {
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

	require.NoError(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
	require.Equal(t, []string{"stop:3", "stop:2", "stop:1"}, events)
}

func TestRunStopFreshContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			return ctx.Err()
		},
	})

	require.NoError(t, signal.Run(ctx, func(context.Context) error {
		cancel()
		return nil
	}))
}

func TestRunStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return errRun
		},
	})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestRunHandlerAndStopError(t *testing.T) {
	stopErr := errors.New("signal: stop error")

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return stopErr
		},
	})

	err := signal.Run(t.Context(), func(context.Context) error {
		return errRun
	})

	require.ErrorIs(t, err, errRun)
	require.ErrorIs(t, err, stopErr)
}

func TestRunStopTimeoutCause(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Microsecond))
	signal.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			<-ctx.Done()
			return context.Cause(ctx)
		},
	})

	err := signal.Run(t.Context(), func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, signal.ErrTimeout)
	require.ErrorIs(t, err, sync.ErrTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
