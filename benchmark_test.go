package signal_test

import (
	"context"
	"testing"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/stretchr/testify/require"
)

// BenchmarkAppRun measures construction and one lifecycle execution per application.
func BenchmarkAppRun(b *testing.B) {
	for _, tc := range []struct {
		name  string
		hooks int
	}{
		{name: "empty", hooks: 0},
		{name: "one_hook", hooks: 1},
		{name: "ten_hooks", hooks: 10},
	} {
		b.Run(tc.name, func(b *testing.B) {
			ctx := context.Background()
			option := signal.Register(func(r signal.Lifecycle) {
				for range tc.hooks {
					r.Append(signal.Hook{
						OnStart: func(context.Context) error {
							return nil
						},
						OnStop: func(context.Context) error {
							return nil
						},
					})
				}
			})

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				app, err := signal.NewApp(option)
				// Keep assertion helper work off the successful timing path.
				if err != nil {
					require.NoError(b, err)
				}

				if err := app.Run(ctx); err != nil {
					require.NoError(b, err)
				}
			}
		})
	}
}
