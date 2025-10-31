package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/alexfalkowski/go-signal"
)

var logger = slog.Default()

func process(ctx context.Context) error {
	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		logger.Info("process failed", "error", err)
		return err
	}
	return nil
}

func main() {
	lc := signal.NewLifeCycle(time.Minute)
	lc.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting process")

			return signal.Go(ctx, time.Second, process)
		},
		OnStop: func(ctx context.Context) error {
			time.Sleep(time.Second)
			logger.Info("stopping process")
			return ctx.Err()
		},
	})

	if err := lc.Server(context.Background()); err != nil {
		logger.Info("server failed", "error", err)
	}
}
