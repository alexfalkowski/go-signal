package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/alexfalkowski/go-signal"
)

var (
	stop   = flag.String("stop", "1m", "the stop duration")
	wait   = flag.String("wait", "1s", "the wait duration")
	logger = slog.Default()
)

func process(ctx context.Context) error {
	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		logger.Info("process failed", "error", err)
		return err
	}
	return nil
}

func main() {
	flag.Parse()

	stopDuration, err := time.ParseDuration(*stop)
	if err != nil {
		logger.Error("failed to parse stop duration", "error", err)
		os.Exit(1)
	}

	waitDuration, err := time.ParseDuration(*wait)
	if err != nil {
		logger.Error("failed to parse wait duration", "error", err)
		os.Exit(1)
	}

	signal.SetDefault(signal.NewLifeCycle(stopDuration))
	signal.Register(&signal.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting process")
			return signal.Go(ctx, waitDuration, process)
		},
		OnStop: func(ctx context.Context) error {
			time.Sleep(waitDuration)
			logger.Info("stopping process")
			return ctx.Err()
		},
	})

	if err := signal.Serve(context.Background()); err != nil {
		logger.Info("server failed", "error", err)
	}
}
