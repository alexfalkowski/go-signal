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
	lc.Register(&signal.Hook{
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
	})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.NoError(t, lc.Server(t.Context()))
}

func TestExec(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	lc.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			return exec.CommandContext(ctx, "echo", "hello").Run()
		},
	})

	require.NoError(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientEmpty(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	lc.Register(&signal.Hook{})

	require.NoError(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{})

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return errTest
	}))
}

func TestClientStartError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	})

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientStopError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	})

	require.Error(t, lc.Client(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestServerEmpty(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Hour))
	lc.Register(&signal.Hook{})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.NoError(t, lc.Server(t.Context()))
}

func TestServerStartError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.Error(t, lc.Server(t.Context()))
}

func TestServerStartTimeout(t *testing.T) {
	lc := signal.NewLifeCycle(signal.WithTimeout(time.Millisecond))
	lc.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			time.Sleep(2 * time.Second)
			return ctx.Err()
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.Error(t, lc.Server(t.Context()))
}

func TestServerStopError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.Error(t, lc.Server(t.Context()))
}

func TestServerStopContextNoError(t *testing.T) {
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStop: func(ctx context.Context) error {
			return ctx.Err()
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = lc.Terminate()
	}()

	require.NoError(t, lc.Server(t.Context()))
}
