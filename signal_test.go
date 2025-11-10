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

func TestHTTPServe(t *testing.T) {
	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: time.Minute,
	}
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			cfg := &net.ListenConfig{}

			ln, err := cfg.Listen(ctx, "tcp", srv.Addr)
			if err != nil {
				return err
			}

			return signal.Go(ctx, time.Second, func(context.Context) error {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}

				return nil
			})
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestCommandRun(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			return exec.CommandContext(ctx, "echo", "hello").Run()
		},
	})

	require.NoError(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}
