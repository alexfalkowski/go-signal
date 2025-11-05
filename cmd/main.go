package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/alexfalkowski/go-signal"
)

var logger = slog.Default()

func start(ctx context.Context) error {
	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		logger.Info("process failed", "error", err)
		return err
	}
	return nil
}

func terminate(_ context.Context) error {
	time.Sleep(2 * time.Second)

	return signal.Terminated(context.Canceled)
}

func main() {
	switch os.Args[1] {
	case "start":
		signal.Register(&signal.Hook{
			OnStart: func(ctx context.Context) error {
				logger.Info("starting process")
				return signal.Go(ctx, time.Second, start)
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(time.Second)
				logger.Info("stopping process")
				return ctx.Err()
			},
		})
	case "terminate":
		signal.Register(&signal.Hook{
			OnStart: func(ctx context.Context) error {
				logger.Info("starting process")
				return signal.Go(ctx, time.Second, terminate)
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(time.Second)
				logger.Info("stopping process")
				return ctx.Err()
			},
		})
	}

	if err := signal.Serve(context.Background()); err != nil {
		logger.Info("server failed", "error", err)
	}
}
