package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/alexfalkowski/go-signal"
)

func process(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func main() {
	logger := slog.Default()
	lc := signal.NewLifeCycle()
	lc.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				logger.Info("starting process")
				if err := process(ctx); err != nil {
					logger.Info("process failed", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			time.Sleep(time.Second)
			logger.Info("stopping process")
			return nil
		},
	})

	if err := lc.Server(context.Background()); err != nil {
		logger.Info("server failed", "error", err)
	}
}
