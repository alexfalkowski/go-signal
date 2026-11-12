package signal_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/alexfalkowski/go-sync"
	"github.com/stretchr/testify/require"
)

func TestTimerTicksUntilCancellation(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		started := time.Now()
		ticks := make([]time.Duration, 0, 3)
		err := signal.Timer(ctx, time.Second, func(context.Context) error {
			ticks = append(ticks, time.Since(started))
			if len(ticks) == 3 {
				cancel()
			}

			return nil
		})

		require.NoError(t, err)
		require.Equal(t, []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}, ticks)
	})
}

func TestTimerRunsCallbacksSequentially(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		started := time.Now()
		ticks := make([]time.Duration, 0, 2)
		err := signal.Timer(ctx, time.Second, func(context.Context) error {
			ticks = append(ticks, time.Since(started))
			time.Sleep(3 * time.Second)
			if len(ticks) >= 2 {
				cancel()
			}

			return nil
		})

		require.NoError(t, err)
		require.Len(t, ticks, 2, "a slow callback must not overlap another tick")
		require.GreaterOrEqual(t, ticks[1]-ticks[0], 3*time.Second, "the previous callback finishes before the next starts")
	})
}

func TestTimerRejectsInvalidInterval(t *testing.T) {
	t.Parallel()

	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			t.Parallel()

			called := false
			err := signal.Timer(t.Context(), interval, func(context.Context) error {
				called = true
				return nil
			})

			require.ErrorIs(t, err, signal.ErrInvalidInterval)
			require.False(t, called, "invalid intervals must not invoke the callback")
		})
	}
}

func TestTimerSkipsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	err := signal.Timer(ctx, time.Nanosecond, func(context.Context) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	require.False(t, called, "cancellation prevents a callback even when a tick is ready")
}

func TestTimerReturnsCallbackErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		cancel bool
		panic  bool
	}{
		{name: "error"},
		{name: "error after cancellation", cancel: true},
		{name: "panic", panic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				want := errors.New("tick failed")
				calls := 0
				err := signal.Timer(ctx, time.Second, func(context.Context) error {
					calls++
					if tc.cancel {
						cancel()
					}
					if tc.panic {
						panic(want)
					}

					return want
				})

				require.ErrorIs(t, err, want)
				if tc.panic {
					require.ErrorIs(t, err, signal.ErrRecovered)
				} else {
					require.NotErrorIs(t, err, signal.ErrRecovered)
				}
				require.Equal(t, 1, calls, "a callback failure ends the timer")
			})
		})
	}
}

func TestTimerWaitsForRunningCallback(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		entered := make(chan struct{})
		canceled := make(chan struct{})
		release := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- signal.Timer(ctx, time.Second, func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				close(canceled)
				<-release
				return nil
			})
		}()

		<-entered
		cancel()
		<-canceled
		synctest.Wait()

		select {
		case err := <-done:
			t.Fatalf("Timer returned before its callback completed: %v", err)
		default:
		}

		close(release)
		require.NoError(t, <-done)
	})
}

func TestServeStopsTimer(t *testing.T) {
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var task *sync.Task
	ticks := 0
	app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle, sh signal.Shutdowner) {
		lc.Append(signal.Hook{
			OnStart: func(context.Context) error {
				task = sync.Go(workerCtx, func(ctx context.Context) error {
					return signal.Timer(ctx, time.Millisecond, func(context.Context) error {
						ticks++
						sh.Shutdown(0)
						return nil
					})
				})
				return nil
			},
			OnStop: func(ctx context.Context) error {
				cancel()
				return task.Await(ctx)
			},
		})
	}))
	require.NoError(t, err)

	ctx, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	require.NoError(t, app.Serve(ctx))
	require.Positive(t, ticks, "the timer requests application shutdown from a tick")

	select {
	case <-task.Done():
	default:
		t.Fatal("application cleanup did not await its timer")
	}
}
