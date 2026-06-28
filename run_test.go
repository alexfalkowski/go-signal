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
	t.Parallel()

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{})

	require.NoError(t, lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestRunOrder(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 5)

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:1")
			return nil
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:1")
			return nil
		},
	})
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:2")
			return nil
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:2")
			return nil
		},
	})

	require.NoError(t, lifecycle.Run(t.Context(), func(context.Context) error {
		events = append(events, "handler")
		return nil
	}))
	require.Equal(t, []string{"start:1", "start:2", "handler", "stop:2", "stop:1"}, events)
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
	t.Parallel()

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
	t.Parallel()

	lifecycle := signal.NewLifeCycle(time.Minute)
	stopped := false
	lifecycle.Register(signal.Hook{
		OnStop: func(context.Context) error {
			stopped = true
			return nil
		},
	})

	err := lifecycle.Run(t.Context(), func(context.Context) error {
		return errRun
	})

	require.ErrorIs(t, err, errRun)
	require.True(t, stopped)
}

func TestRunStartError(t *testing.T) {
	t.Parallel()

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return errRun
		},
	})

	require.Error(t, lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestRunStartRollback(t *testing.T) {
	t.Parallel()

	hook2StartErr := errors.New("signal: run hook 2 start error")
	hook3StopErr := errors.New("signal: run hook 3 stop error")
	hook4StartErr := errors.New("signal: run hook 4 start error")
	handlerCalled := false

	lifecycle := signal.NewLifeCycle(time.Minute)
	events := test.RegisterRollbackHooks(lifecycle, hook2StartErr, hook3StopErr, hook4StartErr)

	err := lifecycle.Run(t.Context(), func(context.Context) error {
		handlerCalled = true
		return nil
	})

	require.ErrorIs(t, err, hook2StartErr)
	require.ErrorIs(t, err, hook3StopErr)
	require.ErrorIs(t, err, hook4StartErr)
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
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	stopped := false

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			stopped = true
			return ctx.Err()
		},
	})
	lifecycle.Register(signal.Hook{
		OnStart: func(context.Context) error {
			cancel()
			return errRun
		},
	})

	err := lifecycle.Run(ctx, func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, errRun)
	require.NotErrorIs(t, err, context.Canceled)
	require.True(t, stopped)
}

func TestRunStopOrder(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 3)

	lifecycle := signal.NewLifeCycle(time.Minute)
	for _, event := range []string{"stop:1", "stop:2", "stop:3"} {
		lifecycle.Register(signal.Hook{
			OnStop: func(context.Context) error {
				events = append(events, event)
				return nil
			},
		})
	}

	require.NoError(t, lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	}))
	require.Equal(t, []string{"stop:3", "stop:2", "stop:1"}, events)
}

func TestRunStopFreshContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			return ctx.Err()
		},
	})

	require.NoError(t, lifecycle.Run(ctx, func(context.Context) error {
		cancel()
		return nil
	}))
}

func TestRunStopError(t *testing.T) {
	t.Parallel()

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return errRun
		},
	})

	require.Error(t, lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestRunHandlerAndStopError(t *testing.T) {
	t.Parallel()

	stopErr := errors.New("signal: stop error")

	lifecycle := signal.NewLifeCycle(time.Minute)
	lifecycle.Register(signal.Hook{
		OnStop: func(context.Context) error {
			return stopErr
		},
	})

	err := lifecycle.Run(t.Context(), func(context.Context) error {
		return errRun
	})

	require.ErrorIs(t, err, errRun)
	require.ErrorIs(t, err, stopErr)
}

func TestRunStopTimeoutCause(t *testing.T) {
	t.Parallel()

	lifecycle := signal.NewLifeCycle(time.Microsecond)
	lifecycle.Register(signal.Hook{
		OnStop: func(ctx context.Context) error {
			<-ctx.Done()
			return context.Cause(ctx)
		},
	})

	err := lifecycle.Run(t.Context(), func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, signal.ErrTimeout)
	require.ErrorIs(t, err, sync.ErrTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
