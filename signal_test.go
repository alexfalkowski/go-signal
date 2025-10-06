package signal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test")

func TestClientEmpty(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	h := &signal.Hook{}
	lc.Register(h)

	require.NoError(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientError(t *testing.T) {
	lc := signal.NewLifeCycle()
	h := &signal.Hook{}
	lc.Register(h)

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return errTest
	}))
}

func TestClientStartError(t *testing.T) {
	lc := signal.NewLifeCycle()
	h := &signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	}
	lc.Register(h)

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientStopError(t *testing.T) {
	lc := signal.NewLifeCycle()
	h := &signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	}
	lc.Register(h)

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestServerEmpty(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	h := &signal.Hook{}
	lc.Register(h)

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.NoError(t, lc.Server(t.Context()))
}

func TestServerStartError(t *testing.T) {
	lc := signal.NewLifeCycle()
	h := &signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	}
	lc.Register(h)

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.Error(t, lc.Server(t.Context()))
}

func TestServerStopError(t *testing.T) {
	lc := signal.NewLifeCycle()
	h := &signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	}
	lc.Register(h)

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.Error(t, lc.Server(t.Context()))
}
