package signal_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("test")

func TestHTTPServer(t *testing.T) {
	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: time.Hour,
	}
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	h := &signal.Hook{
		OnStart: func(ctx context.Context) error {
			cfg := &net.ListenConfig{}

			ln, err := cfg.Listen(ctx, "tcp", srv.Addr)
			if err != nil {
				return err
			}

			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					panic(err)
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	}
	lc.Register(h)

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.NoError(t, lc.Server(t.Context()))
}

func TestExec(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	h := &signal.Hook{
		OnStart: func(ctx context.Context) error {
			return exec.CommandContext(ctx, "echo", "hello").Run()
		},
	}
	lc.Register(h)

	require.NoError(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

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
