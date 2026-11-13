package signaltest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/alexfalkowski/go-signal/v2/signaltest"
	"github.com/stretchr/testify/require"
)

func TestLifecycleCleanupAfterTestCancellation(t *testing.T) {
	stopped := false

	t.Run("component", func(t *testing.T) {
		lc := signaltest.NewLifecycle(t)
		lc.Append(signal.Hook{
			OnStop: func(ctx context.Context) error {
				require.ErrorIs(t, t.Context().Err(), context.Canceled)
				require.NoError(t, ctx.Err())
				stopped = true
				return nil
			},
		})
		t.Cleanup(lc.RequireStop)
		lc.RequireStart()
	})

	require.True(t, stopped, "cleanup must stop the real lifecycle after the test returns")
}

func TestLifecycleReportsFailures(t *testing.T) {
	t.Parallel()

	want := errors.New("component failed")
	for _, phase := range []string{"start", "stop"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			tb := &recordingTB{TB: t}
			lc := signaltest.NewLifecycle(tb)
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					if phase == "start" {
						return want
					}

					return nil
				},
				OnStop: func(context.Context) error {
					return want
				},
			})

			lc.RequireStart()
			if phase == "stop" {
				lc.RequireStop()
			}

			require.Contains(t, tb.message, "lifecycle "+phase+" failed")
			require.Contains(t, tb.message, want.Error())
		})
	}
}

func TestLifecycleReturnsExpectedErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("expected startup failure")
	cleanupErr := errors.New("expected cleanup failure")
	lc := signaltest.NewLifecycle(t)
	lc.Append(signal.Hook{
		OnStop: func(context.Context) error {
			return cleanupErr
		},
	})
	lc.Append(signal.Hook{
		OnStart: func(context.Context) error {
			return want
		},
	})

	err := lc.Start(t.Context())
	require.ErrorIs(t, err, want)
	require.ErrorIs(t, err, cleanupErr)
	require.NoError(t, lc.Stop(t.Context()))
}

func TestLifecycleStopsOnce(t *testing.T) {
	t.Parallel()

	stops := 0
	lc := signaltest.NewLifecycle(t)
	lc.Append(signal.Hook{
		OnStop: func(context.Context) error {
			stops++
			return nil
		},
	})
	t.Cleanup(lc.RequireStop)

	lc.RequireStart()
	require.ErrorIs(t, lc.Start(t.Context()), signal.ErrStarted)
	lc.RequireStop()
	lc.RequireStop()
	require.Equal(t, 1, stops, "component cleanup executes once")
}

func TestLifecyclePreservesCancellationCause(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		cause   error
		between bool
	}{
		{name: "before custom", cause: errors.New("startup aborted")},
		{name: "between custom", cause: errors.New("startup aborted"), between: true},
		{name: "before timeout", cause: signal.ErrTimeout},
		{name: "between timeout", cause: signal.ErrTimeout, between: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if !tc.between {
				cancel(tc.cause)
			}

			events := make([]string, 0, 3)
			lc := signaltest.NewLifecycle(t)
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					events = append(events, "start")
					cancel(tc.cause)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					require.NoError(t, ctx.Err(), "rollback uses a fresh context after startup cancellation")
					events = append(events, "stop")
					return nil
				},
			})
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					events = append(events, "skipped")
					return nil
				},
			})

			require.ErrorIs(t, lc.Start(ctx), tc.cause)
			if tc.between {
				require.Equal(t, []string{"start", "stop"}, events)
			} else {
				require.Empty(t, events)
			}
			require.NoError(t, lc.Stop(t.Context()))
		})
	}
}

type recordingTB struct {
	testing.TB

	message string
}

func (t *recordingTB) Fatalf(format string, args ...any) {
	t.message = fmt.Sprintf(format, args...)
}
