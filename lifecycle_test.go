package signal_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/stretchr/testify/require"
)

func TestRunFreshApplications(t *testing.T) {
	t.Parallel()

	for range 2 {
		events := make([]string, 0, 2)
		app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle) {
			lc.Append(signal.Hook{
				OnStart: func(context.Context) error {
					events = append(events, "start")
					return nil
				},
				OnStop: func(context.Context) error {
					events = append(events, "stop")
					return nil
				},
			})
		}))
		require.NoError(t, err)

		require.NoError(t, app.Run(t.Context()))
		require.Equal(t, []string{"start", "stop"}, events)
	}
}

func TestRunCancellationRollsBack(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	events := make([]string, 0, 3)
	stopped := make(chan error, 1)
	app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle) {
		lc.Append(signal.Hook{
			OnStart: func(context.Context) error {
				events = append(events, "start")
				cancel()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				events = append(events, "stop")
				stopped <- ctx.Err()
				return nil
			},
		})
		lc.Append(signal.Hook{
			OnStart: func(context.Context) error {
				events = append(events, "dependent")
				return nil
			},
		})
	}))
	require.NoError(t, err)

	require.ErrorIs(t, app.Run(ctx), context.Canceled)
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("startup cancellation did not roll back the started hook")
	}
	require.Equal(t, []string{"start", "stop"}, events)
}
