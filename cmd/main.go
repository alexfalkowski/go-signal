package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/alexfalkowski/go-sync"
)

const usageMessage = "usage: make run param=start|timer|terminate"

var (
	errUsage       = errors.New(usageMessage)
	errInvalidMode = errors.New("invalid mode")
)

// RegisterProcess connects the resolved process to the application's lifecycle.
func RegisterProcess(lc signal.Lifecycle, process *Process) {
	lc.Append(signal.Hook{OnStart: process.Start, OnStop: process.Stop})
}

// NewConfig parses the required mode before constructing the application.
func NewConfig(args []string) (*Config, error) {
	if len(args) < 2 {
		return nil, errUsage
	}

	mode := args[1]
	switch mode {
	case "start", "timer", "terminate":
		return &Config{Mode: mode}, nil
	default:
		return nil, fmt.Errorf("%w %q: %s", errInvalidMode, mode, usageMessage)
	}
}

// Config selects the worker behavior for one command execution.
type Config struct {
	Mode string
}

// NewProcess constructs a process without starting its background worker.
// The application resolves its configuration, logger, and shutdown dependency.
func NewProcess(config *Config, logger *slog.Logger, shutdown signal.Shutdowner) *Process {
	return &Process{config: config, logger: logger, shutdown: shutdown}
}

// Process owns its background worker and waits for it during shutdown.
type Process struct {
	config   *Config
	logger   *slog.Logger
	shutdown signal.Shutdowner
	cancel   context.CancelFunc
	task     *sync.Task
}

// Start launches the worker with a lifetime managed by Stop.
// The startup context ends when startup completes, so the worker owns its context.
func (p *Process) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	p.logger.Info("starting process")
	p.task = sync.Go(ctx, func(ctx context.Context) error {
		if err := p.run(ctx); err != nil {
			p.shutdown.Shutdown(1)
			return err
		}

		return nil
	})
	return nil
}

// Stop cancels the worker, waits for it, and demonstrates a second of cleanup.
func (p *Process) Stop(ctx context.Context) error {
	p.cancel()

	if err := p.task.Await(ctx); err != nil {
		return err
	}

	time.Sleep(time.Second)
	p.logger.Info("stopping process")
	return context.Cause(ctx)
}

func (p *Process) run(ctx context.Context) error {
	switch p.config.Mode {
	case "start":
		<-ctx.Done()
		p.logger.Info("process failed", "error", ctx.Err())
	case "timer":
		p.logger.Info("starting timer")
		defer p.logger.Info("stopping timer")

		return signal.Timer(ctx, time.Second, p.tick)
	case "terminate":
		return signal.Timer(ctx, 2*time.Second, func(context.Context) error {
			p.shutdown.Shutdown(0)
			return nil
		})
	}

	return nil
}

func (p *Process) tick(context.Context) error {
	p.logger.Info("ticking")
	return nil
}

func run(args []string) error {
	config, err := NewConfig(args)
	if err != nil {
		return err
	}

	app, err := signal.NewApp(
		signal.Constructor(
			func() *Config {
				return config
			},
			slog.Default,
			NewProcess,
		),
		signal.Register(RegisterProcess),
	)
	if err != nil {
		return err
	}

	return app.Serve(context.Background())
}

func main() {
	err := run(os.Args)
	if err == nil {
		return
	}

	logger := slog.Default()
	if errors.Is(err, errUsage) || errors.Is(err, errInvalidMode) {
		logger.Error(err.Error())
		os.Exit(2)
	}

	logger.Info("server failed", "error", err)
}
