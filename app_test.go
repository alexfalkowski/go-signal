package signal_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/stretchr/testify/require"
)

func TestNewAppBuildsBeforeStarting(t *testing.T) {
	t.Parallel()

	events := make([]string, 0, 3)
	app, err := signal.NewApp(signal.Module(
		signal.Register(func(p componentParams) {
			require.Equal(t, "custom", p.Config.value)
			require.Nil(t, p.Logger)

			slices.Sort(p.Validations)
			require.Equal(t, []string{"size", "time"}, p.Validations)

			events = append(events, "register")
			p.Lifecycle.Append(signal.Hook{
				OnStart: func(context.Context) error {
					events = append(events, "start")
					return nil
				},
				OnStop: func(context.Context) error {
					events = append(events, "stop")
					return nil
				},
			})
		}),
		signal.Decorate(func(c *customConfig) *config {
			return c.config
		}),
		signal.Constructor(func() *config {
			return &config{value: "default"}
		}),
		signal.Constructor(func() *customConfig {
			return &customConfig{config: &config{value: "custom"}}
		}),
		signal.Constructor(func() validationResult {
			return validationResult{Value: "size"}
		}),
		signal.Constructor(func() validationResult {
			return validationResult{Value: "time"}
		}),
	))
	require.NoError(t, err)
	require.Equal(t, []string{"register"}, events)

	require.NoError(t, app.Run(t.Context()))
	require.Equal(t, []string{"register", "start", "stop"}, events)
}

func TestNewAppReturnsInitializationErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("configuration failed")
	for _, tc := range []struct {
		name string
		opts []signal.Option
	}{
		{name: "constructor", opts: []signal.Option{
			signal.Constructor(func() (*config, error) {
				return nil, want
			}),
			signal.Register(func(*config) {
			}),
		}},
		{name: "decorator", opts: []signal.Option{
			signal.Constructor(func() *config {
				return &config{}
			}),
			signal.Decorate(func(*config) (*config, error) {
				return nil, want
			}),
			signal.Register(func(*config) {
			}),
		}},
		{name: "registration", opts: []signal.Option{
			signal.Register(func() error {
				return want
			}),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, err := signal.NewApp(tc.opts...)
			require.Nil(t, app)
			require.ErrorIs(t, err, want)
			require.ErrorIs(t, signal.RootCause(err), want)
		})
	}
}

func TestNewAppRecoversPanics(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		opts []signal.Option
	}{
		{name: "constructor", opts: []signal.Option{
			signal.Constructor(func() *config {
				panic("bad config")
			}),
			signal.Register(func(*config) {
			}),
		}},
		{name: "decorator", opts: []signal.Option{
			signal.Constructor(func() *config {
				return &config{}
			}),
			signal.Decorate(func(*config) *config {
				panic("bad config")
			}),
			signal.Register(func(*config) {
			}),
		}},
		{name: "registration", opts: []signal.Option{
			signal.Register(func() {
				panic("bad config")
			}),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, err := signal.NewApp(tc.opts...)
			require.Nil(t, app)
			require.ErrorContains(t, err, "bad config")
		})
	}
}

func TestNewAppRejectsInvalidApplications(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		opts []signal.Option
	}{
		{name: "missing dependency", opts: []signal.Option{
			signal.Register(func(*config) {
			}),
		}},
		{name: "invalid constructor", opts: []signal.Option{signal.Constructor(42)}},
		{name: "duplicate decorator", opts: []signal.Option{
			signal.Decorate(func() string {
				return "a"
			}, func() string {
				return "b"
			}),
		}},
		{name: "duplicate constructor", opts: []signal.Option{
			signal.Constructor(func() string {
				return "a"
			}, func() string {
				return "b"
			}),
		}},
		{name: "cycle", opts: []signal.Option{
			signal.Constructor(func(*config) *customConfig {
				return &customConfig{}
			}),
			signal.Constructor(func(*customConfig) *config {
				return &config{}
			}),
			signal.Register(func(*config) {
			}),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, err := signal.NewApp(tc.opts...)
			require.Nil(t, app)
			require.Error(t, err)
		})
	}

	for _, opt := range []signal.Option{signal.StartTimeout(0), signal.StopTimeout(-time.Second)} {
		_, err := signal.NewApp(opt)
		require.ErrorIs(t, err, signal.ErrInvalidTimeout)
	}
}

func TestRunPreservesExitCodeAndCleanupErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("cleanup failed")
	stops := 0
	module := signal.Register(func(lc signal.Lifecycle, sh signal.Shutdowner) {
		lc.Append(signal.Hook{
			OnStart: func(context.Context) error {
				sh.Shutdown(3)
				sh.Shutdown(9)
				return nil
			},
			OnStop: func(context.Context) error {
				stops++
				return want
			},
		})
	})

	for range 2 {
		app, err := signal.NewApp(module)
		require.NoError(t, err)

		err = app.Run(t.Context())
		require.ErrorIs(t, err, want)
		require.ErrorContains(t, err, "signal: shutdown with exit code 3")

		var exit *signal.ExitError
		require.ErrorAs(t, err, &exit)
		require.Equal(t, 3, exit.Code, "first shutdown request determines the exit code")
	}

	require.Equal(t, 2, stops, "each application cleans up once")
}

func TestCleanupPreservesShutdownCode(t *testing.T) {
	cleanupErr := errors.New("cleanup failed")

	for _, mode := range []string{"run", "serve"} {
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				code int
				err  error
			}{
				{name: "nonzero", code: 7},
				{name: "nonzero with cleanup error", code: 7, err: cleanupErr},
				{name: "zero"},
				{name: "zero with cleanup error", err: cleanupErr},
			} {
				t.Run(tc.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()

					app, err := signal.NewApp(signal.Register(func(r signal.Lifecycle, sh signal.Shutdowner) {
						r.Append(signal.Hook{
							OnStart: func(ctx context.Context) error {
								// Startup context cancellation follows successful startup.
								// Cancel the service then so it proceeds to normal cleanup.
								context.AfterFunc(ctx, cancel)
								return nil
							},
							OnStop: func(context.Context) error {
								sh.Shutdown(tc.code)
								sh.Shutdown(9)
								return tc.err
							},
						})
					}))
					require.NoError(t, err)

					if mode == "serve" {
						err = app.Serve(ctx)
					} else {
						err = app.Run(ctx)
					}
					if tc.err != nil {
						require.ErrorIs(t, err, tc.err)
					}

					var exit *signal.ExitError
					if tc.code != 0 {
						require.ErrorAs(t, err, &exit)
						require.Equal(t, tc.code, exit.Code, "the first request during cleanup determines the exit code")
					} else {
						require.NotErrorAs(t, err, &exit, "a zero request during cleanup means success")
						if tc.err == nil {
							require.NoError(t, err)
						}
					}
				})
			}
		})
	}
}

func TestShutdownIsLocalAndBuffered(t *testing.T) {
	t.Parallel()

	app, err := signal.NewApp(signal.Register(func(sh signal.Shutdowner) {
		sh.Shutdown(7)
	}))
	require.NoError(t, err)

	other, err := signal.NewApp()
	require.NoError(t, err)

	err = app.Run(t.Context())
	var exit *signal.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 7, exit.Code, "shutdown requested during construction is preserved")
	require.NoError(t, other.Run(t.Context()), "shutdown belongs to its application")
}

func TestCanceledStartupPreservesShutdownCode(t *testing.T) {
	for _, tc := range []struct {
		name  string
		serve bool
		code  int
	}{
		{name: "run nonzero", code: 7},
		{name: "serve nonzero", serve: true, code: 7},
		{name: "run zero"},
		{name: "serve zero", serve: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := signal.NewApp(signal.Register(func(sh signal.Shutdowner) {
				sh.Shutdown(tc.code)
				sh.Shutdown(9)
			}))
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if tc.serve {
				err = app.Serve(ctx)
			} else {
				err = app.Run(ctx)
			}
			require.ErrorIs(t, err, context.Canceled)

			var exit *signal.ExitError
			if tc.code == 0 {
				require.NotErrorAs(t, err, &exit, "a success request does not hide the startup error")
			} else {
				require.ErrorAs(t, err, &exit)
				require.Equal(t, tc.code, exit.Code, "the first buffered shutdown code is preserved")
			}
		})
	}
}

func TestServeHandlesShutdownDuringStartup(t *testing.T) {
	app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle, sh signal.Shutdowner) {
		lc.Append(signal.Hook{
			OnStart: func(context.Context) error {
				sh.Shutdown(5)
				return nil
			},
		})
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err = app.Serve(ctx)
	var exit *signal.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 5, exit.Code, "Serve preserves shutdown requested before it starts waiting")
}

func TestServeCancellationPreservesShutdownCode(t *testing.T) {
	want := errors.New("stop failed")

	// Repeat the simultaneous-ready case to exercise either select branch.
	for range 50 {
		ctx, cancel := context.WithCancel(t.Context())
		stopped := make(chan struct{})
		app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle, sh signal.Shutdowner) {
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					sh.Shutdown(7)
					cancel()
					return nil
				},
				OnStop: func(context.Context) error {
					close(stopped)
					return want
				},
			})
		}))
		require.NoError(t, err)

		err = app.Serve(ctx)
		cancel()

		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("application did not clean up after cancellation")
		}

		var exit *signal.ExitError
		require.ErrorAs(t, err, &exit)
		require.Equal(t, 7, exit.Code, "cancellation must preserve a buffered shutdown code")

		// A canceled startup wait can return before its hook finishes.
		if errors.Is(err, context.Canceled) {
			continue
		}

		require.ErrorIs(t, err, want)
	}
}

func TestServeCancellationCleansUp(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan error, 1)
	app, err := signal.NewApp(signal.Register(func(r signal.Lifecycle) {
		r.Append(signal.Hook{
			OnStart: func(context.Context) error {
				close(started)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				stopped <- ctx.Err()
				return nil
			},
		})
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- app.Serve(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("application did not start")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			require.ErrorIs(t, err, context.Canceled, "cancellation can end the startup wait")
		}
	case <-time.After(time.Second):
		t.Fatal("application did not return after cancellation")
	}

	select {
	case err := <-stopped:
		require.NoError(t, err, "cleanup uses a fresh context")
	case <-time.After(time.Second):
		t.Fatal("application did not clean up after cancellation")
	}
}

func TestServeSignalDuringStartupRollsBack(t *testing.T) {
	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	for _, tc := range []struct {
		name  string
		sig   signal.Signal
		extra []signal.Signal
	}{
		{name: "interrupt", sig: signal.Interrupt},
		{name: "termination", sig: signal.Termination},
		{name: "additional hangup", sig: signal.Hangup, extra: []signal.Signal{signal.Hangup}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopped := make(chan struct{})
			var startCause error

			app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle) {
				lc.Append(signal.Hook{
					OnStart: func(ctx context.Context) error {
						if err := process.Signal(tc.sig); err != nil {
							return err
						}

						<-ctx.Done()
						startCause = ctx.Err()
						return nil
					},
					OnStop: func(context.Context) error {
						close(stopped)
						return nil
					},
				})
			}))
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			err = app.Serve(ctx, tc.extra...)
			require.True(t, err == nil || errors.Is(err, context.Canceled), "unexpected Serve error: %v", err)

			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("signal cancellation did not roll back the started hook")
			}

			require.ErrorIs(t, startCause, context.Canceled)
		})
	}
}

func TestApplicationTimeoutsBoundWaiting(t *testing.T) {
	t.Parallel()

	for _, phase := range []string{"start", "stop"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			release := make(chan struct{})
			finished := make(chan struct{})
			cleanupContext := make(chan error, 1)
			app, err := signal.NewApp(signal.StartTimeout(20*time.Millisecond), signal.StopTimeout(20*time.Millisecond),
				signal.Register(func(lc signal.Lifecycle) {
					lc.Append(signal.Hook{
						OnStart: func(context.Context) error {
							if phase == "start" {
								<-release
							}

							return nil
						},
						OnStop: func(ctx context.Context) error {
							if phase == "stop" {
								<-release
							}

							cleanupContext <- ctx.Err()
							close(finished)
							return nil
						},
					})
				}),
			)
			require.NoError(t, err)

			err = app.Run(t.Context())

			close(release)
			require.ErrorIs(t, err, signal.ErrTimeout)

			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("timed-out lifecycle did not finish cleanup after release")
			}

			if phase == "start" {
				require.NoError(t, <-cleanupContext, "late startup cleanup needs a fresh context")
			}
		})
	}
}

func TestRunStartupFailureRollsBack(t *testing.T) {
	t.Parallel()

	want := errors.New("startup failed")
	cleanupErr := errors.New("cleanup failed")
	for _, tc := range []struct {
		name       string
		panic      any
		wrapsError bool
	}{
		{name: "error", wrapsError: true},
		{name: "panic error", panic: want, wrapsError: true},
		{name: "panic string", panic: "startup failed"},
		{name: "panic number", panic: 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			events := make([]string, 0, 3)
			app, err := signal.NewApp(signal.Register(func(r signal.Lifecycle) {
				r.Append(signal.Hook{
					OnStart: func(context.Context) error {
						events = append(events, "first")
						return nil
					},
					OnStop: func(ctx context.Context) error {
						events = append(events, "rollback")
						return errors.Join(ctx.Err(), cleanupErr)
					},
				})
				r.Append(signal.Hook{
					OnStart: func(context.Context) error {
						events = append(events, "failure")
						if tc.panic != nil {
							panic(tc.panic)
						}

						return want
					},
					OnStop: func(context.Context) error {
						events = append(events, "failed hook cleanup")
						return nil
					},
				})
				r.Append(signal.Hook{OnStart: func(context.Context) error {
					events = append(events, "skipped")
					return nil
				}})
			}))
			require.NoError(t, err)

			err = app.Run(t.Context())
			require.ErrorIs(t, err, cleanupErr)
			if tc.wrapsError {
				require.ErrorIs(t, err, want)
			}
			require.Equal(t, tc.panic != nil, errors.Is(err, signal.ErrRecovered), "panics are marked as recovered")
			require.Equal(t, []string{"first", "failure", "rollback"}, events)
		})
	}
}

func TestRunStopsHooksAfterPanic(t *testing.T) {
	t.Parallel()

	want := errors.New("cleanup failed")
	events := make([]string, 0, 3)
	app, err := signal.NewApp(signal.Register(func(r signal.Lifecycle) {
		r.Append(signal.Hook{
			OnStop: func(context.Context) error {
				events = append(events, "first")
				return nil
			},
		})
		r.Append(signal.Hook{
			OnStop: func(context.Context) error {
				events = append(events, "second")
				panic("stop failed")
			},
		})
		r.Append(signal.Hook{
			OnStop: func(context.Context) error {
				events = append(events, "third")
				return want
			},
		})
	}))
	require.NoError(t, err)

	err = app.Run(t.Context())
	require.ErrorIs(t, err, want)
	require.ErrorIs(t, err, signal.ErrRecovered)
	require.Equal(t, []string{"third", "second", "first"}, events)
}

func TestLateStartupFailureRollsBack(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	stopped := make(chan error, 1)
	app, err := signal.NewApp(signal.StartTimeout(20*time.Millisecond),
		signal.Register(func(r signal.Lifecycle) {
			r.Append(signal.Hook{
				OnStop: func(ctx context.Context) error {
					stopped <- ctx.Err()
					return nil
				},
			})
			r.Append(signal.Hook{
				OnStart: func(context.Context) error {
					<-release
					return errors.New("late startup failure")
				},
			})
		}),
	)
	require.NoError(t, err)

	err = app.Run(t.Context())
	close(release)
	require.ErrorIs(t, err, signal.ErrTimeout)

	select {
	case err := <-stopped:
		require.NoError(t, err, "late rollback needs a fresh context")
	case <-time.After(time.Second):
		t.Fatal("failed startup did not roll back after release")
	}
}

func TestStartupRollbackTimeout(t *testing.T) {
	t.Parallel()

	want := errors.New("startup failed")
	app, err := signal.NewApp(signal.StopTimeout(20*time.Millisecond),
		signal.Register(func(r signal.Lifecycle, sh signal.Shutdowner) {
			r.Append(signal.Hook{
				OnStop: func(ctx context.Context) error {
					<-ctx.Done()
					return context.Cause(ctx)
				},
			})
			r.Append(signal.Hook{
				OnStart: func(context.Context) error {
					sh.Shutdown(7)
					return want
				},
			})
		}),
	)
	require.NoError(t, err)

	err = app.Run(t.Context())
	require.ErrorIs(t, err, want)
	require.ErrorIs(t, err, signal.ErrTimeout)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	var exit *signal.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 7, exit.Code, "startup, rollback, and shutdown errors are preserved together")
}

func TestStartupFailureUsesCleanupDeadline(t *testing.T) {
	for _, mode := range []string{"run", "serve"} {
		t.Run(mode, func(t *testing.T) {
			startupErr := errors.New("startup failed")
			cleanupErr := errors.New("cleanup failed")
			stopped := make(chan error, 1)
			app, err := signal.NewApp(signal.StartTimeout(20*time.Millisecond), signal.StopTimeout(time.Second),
				signal.Register(func(r signal.Lifecycle) {
					r.Append(signal.Hook{
						OnStop: func(ctx context.Context) error {
							timer := time.NewTimer(60 * time.Millisecond)
							defer timer.Stop()
							select {
							case <-timer.C:
								stopped <- ctx.Err()
								return cleanupErr
							case <-ctx.Done():
								stopped <- context.Cause(ctx)
								return context.Cause(ctx)
							}
						},
					})
					r.Append(signal.Hook{
						OnStart: func(context.Context) error {
							return startupErr
						},
					})
				}),
			)
			require.NoError(t, err)

			if mode == "serve" {
				err = app.Serve(t.Context())
			} else {
				err = app.Run(t.Context())
			}

			select {
			case stopErr := <-stopped:
				require.NoError(t, stopErr, "rollback gets its own cleanup deadline")
			case <-time.After(time.Second):
				t.Fatal("rollback did not complete")
			}
			require.ErrorIs(t, err, startupErr)
			require.ErrorIs(t, err, cleanupErr)
			require.NotErrorIs(t, err, signal.ErrTimeout, "cleanup completed within StopTimeout")
		})
	}
}

func TestHTTPApplication(t *testing.T) {
	address := make(chan string, 1)
	serverErr := make(chan error, 1)
	server := &http.Server{ReadHeaderTimeout: time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	app, err := signal.NewApp(signal.Register(func(r signal.Lifecycle) {
		r.Append(signal.Hook{
			OnStart: func(ctx context.Context) error {
				listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
				if err != nil {
					return err
				}

				address <- listener.Addr().String()
				go func() {
					serverErr <- server.Serve(listener)
				}()
				return nil
			},
			OnStop: server.Shutdown,
		})
	}))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- app.Serve(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	})

	var target string
	select {
	case target = <-address:
	case <-time.After(time.Second):
		t.Fatal("HTTP server did not start")
	}

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+target, nil)
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNoContent, response.StatusCode)

	cancel()
	select {
	case err := <-serverErr:
		require.ErrorIs(t, err, http.ErrServerClosed)
	case <-time.After(time.Second):
		t.Fatal("HTTP server did not stop")
	}
}

type config struct{ value string }

type customConfig struct{ *config }

type validationResult struct {
	signal.Out

	Value string `group:"validations"`
}

type componentParams struct {
	signal.In

	Lifecycle   signal.Lifecycle
	Config      *config
	Logger      *slog.Logger `optional:"true"`
	Validations []string     `group:"validations"`
}
