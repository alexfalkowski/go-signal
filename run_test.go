package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
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

func TestRunError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return errRun
	}))
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

func TestRunStartErrorStopsStartedHooksInReverseOrder(t *testing.T) {
	order := make([]string, 0, 5)

	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			order = append(order, "start-1")
			return nil
		},
		OnStop: func(context.Context) error {
			order = append(order, "stop-1")
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			order = append(order, "start-2")
			return nil
		},
		OnStop: func(context.Context) error {
			order = append(order, "stop-2")
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			order = append(order, "start-3")
			return errRun
		},
		OnStop: func(context.Context) error {
			order = append(order, "stop-3")
			return nil
		},
	})

	err := signal.Run(t.Context(), func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, errRun)
	require.Equal(t, []string{"start-1", "start-2", "start-3", "stop-2", "stop-1"}, order)
}

func TestSetDefaultNilPanic(t *testing.T) {
	require.PanicsWithValue(t, "signal: lifecycle must not be nil", func() {
		signal.SetDefault(nil)
	})
}
