package signal_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
)

func BenchmarkLifecycleRun(b *testing.B) {
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
			handler := func(context.Context) error {
				return nil
			}
			lifecycle := signal.NewLifeCycle(time.Minute)
			for range tc.hooks {
				lifecycle.Register(signal.Hook{
					OnStart: func(context.Context) error {
						return nil
					},
					OnStop: func(context.Context) error {
						return nil
					},
				})
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := lifecycle.Run(ctx, handler); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
