package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alexfalkowski/go-signal"
)

var logger = slog.Default()

const usageMessage = "usage: go run cmd/main.go [start|timer|terminate]"

var (
	errUsage       = errors.New(usageMessage)
	errInvalidMode = errors.New("invalid mode")
)

func start(ctx context.Context) error {
	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		logger.Info("process failed", "error", err)
		return err
	}
	return nil
}

func ticker(context.Context) error {
	logger.Info("ticking")
	return nil
}

func terminate(_ context.Context) error {
	time.Sleep(2 * time.Second)

	return signal.Terminated(context.Canceled)
}

func configure(mode string) error {
	switch mode {
	case "start":
		signal.Register(signal.Hook{
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
	case "timer":
		signal.Register(signal.Hook{
			OnStart: func(ctx context.Context) error {
				logger.Info("starting process")
				return signal.Timer(ctx, time.Second, time.Second, signal.Hook{
					OnStart: func(_ context.Context) error {
						logger.Info("starting timer")
						return nil
					},
					OnTick: ticker,
					OnStop: func(_ context.Context) error {
						logger.Info("stopping timer")
						return nil
					},
				})
			},
			OnStop: func(ctx context.Context) error {
				time.Sleep(time.Second)
				logger.Info("stopping process")
				return ctx.Err()
			},
		})
	case "terminate":
		signal.Register(signal.Hook{
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
	default:
		return fmt.Errorf("%w %q: %s", errInvalidMode, mode, usageMessage)
	}

	return nil
}

func run(args []string) error {
	if len(args) < 2 {
		return errUsage
	}

	if err := configure(args[1]); err != nil {
		return err
	}

	return signal.Serve(context.Background())
}

func main() {
	err := run(os.Args)
	if err == nil {
		return
	}

	if errors.Is(err, errUsage) || errors.Is(err, errInvalidMode) {
		logger.Error(err.Error())
		os.Exit(2)
	}

	logger.Info("server failed", "error", err)
}
