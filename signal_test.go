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

			err = signal.Go(ctx, time.Second, func(context.Context) error {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}

				return nil
			})

			return err
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

func TestExec(t *testing.T) {
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

func TestClientEmpty(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{})

	require.NoError(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return errTest
	}))
}

func TestClientStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestClientStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	})

	require.Error(t, signal.Run(t.Context(), func(context.Context) error {
		return nil
	}))
}

func TestServerEmpty(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.NoError(t, signal.Serve(t.Context()))
}

func TestServerStartError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStart: func(context.Context) error {
			return errTest
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.Error(t, signal.Serve(t.Context()))
}

func TestServerGoError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			return signal.Go(ctx, time.Minute, func(context.Context) error {
				return errTest
			})
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.Error(t, signal.Serve(t.Context()))
}

func TestServerStopError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
		OnStop: func(context.Context) error {
			return errTest
		},
	})

	go func() {
		time.Sleep(time.Second)
		_ = signal.Shutdown()
	}()

	require.Error(t, signal.Serve(t.Context()))
}

func TestServerStopContextNoError(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	signal.Register(&signal.Hook{
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

func TestServerStartContext(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	ch := make(chan bool, 1)
	signal.Register(&signal.Hook{
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

func TestServerStartLoopContext(t *testing.T) {
	signal.SetDefault(signal.NewLifeCycle(time.Minute))
	ch := make(chan bool, 1)
	signal.Register(&signal.Hook{
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
