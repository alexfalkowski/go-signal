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
		"stop:1",
		"stop:3",
	}, *events)
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
