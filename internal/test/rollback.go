package test

import (
	"context"

	"github.com/alexfalkowski/go-signal"
)

// RegisterRollbackHooks registers a fixed set of lifecycle hooks that exercise
// startup rollback behavior and returns the event log used by those hooks.
//
// The registered hooks always attempt startup in this order:
//
//   - hook 1 starts successfully and stops successfully
//   - hook 2 fails during start with hook2StartErr
//   - hook 3 starts successfully and fails during stop with hook3StopErr
//   - hook 4 fails during start with hook4StartErr
//
// Each hook appends its start and stop activity to the returned slice so callers
// can assert the exact execution order. The returned pointer remains valid for
// the lifetime of the test because the closures capture the underlying slice.
//
// This helper is intended for tests that need to verify that startup:
//
//   - attempts all registered start hooks
//   - rolls back only successfully started hooks
//   - preserves reverse registration order during rollback
//   - joins startup and rollback errors
func RegisterRollbackHooks(hook2StartErr, hook3StopErr, hook4StartErr error) *[]string {
	events := make([]string, 0, 6)

	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:1")
			return nil
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:1")
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:2")
			return hook2StartErr
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:2")
			return nil
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:3")
			return nil
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:3")
			return hook3StopErr
		},
	})
	signal.Register(signal.Hook{
		OnStart: func(context.Context) error {
			events = append(events, "start:4")
			return hook4StartErr
		},
		OnStop: func(context.Context) error {
			events = append(events, "stop:4")
			return nil
		},
	})

	return &events
}
