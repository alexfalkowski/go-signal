package signal_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/alexfalkowski/go-signal"
	"github.com/stretchr/testify/require"
)

func FuzzLifecycleRunHookMatrix(f *testing.F) {
	f.Add([]byte{}, false, false, false)
	f.Add([]byte{0}, false, false, false)
	f.Add([]byte{0x05, 0x05}, false, false, false)
	f.Add([]byte{0x07, 0x05}, false, false, false)
	f.Add([]byte{0x05, 0x0f, 0x07}, true, false, false)
	f.Add([]byte{0x15, 0x05}, false, true, false)
	f.Add([]byte{0x15, 0x05}, false, false, true)

	f.Fuzz(func(t *testing.T, raw []byte, handlerFails, cancelBeforeRun, handlerCancels bool) {
		const maxHooks = 8
		if len(raw) > maxHooks {
			raw = raw[:maxHooks]
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if cancelBeforeRun {
			cancel()
		}

		lifecycle := signal.NewLifeCycle(time.Minute)
		run := newLifecycleRunFuzz(raw)
		run.register(t, lifecycle)
		handlerErr := errors.New("handler error")
		run.expect(handlerErr, handlerFails)

		err := lifecycle.Run(ctx, func(context.Context) error {
			run.events = append(run.events, "handler")
			if handlerCancels {
				cancel()
			}
			if handlerFails {
				return handlerErr
			}
			return nil
		})

		require.Equal(t, run.expectedEvents, run.events)
		if len(run.expectedErrs) == 0 {
			require.NoError(t, err)
		}
		for expectedErr := range run.expectedErrs {
			require.ErrorIs(t, err, expectedErr)
		}
		for _, possibleErr := range run.allErrs {
			if _, ok := run.expectedErrs[possibleErr]; !ok {
				require.NotErrorIs(t, err, possibleErr)
			}
		}
		require.NotErrorIs(t, err, context.Canceled)
	})
}

func FuzzTimerEntryGuards(f *testing.F) {
	f.Add(int64(time.Second), int64(0), false)
	f.Add(int64(time.Second), int64(-time.Nanosecond), false)
	f.Add(int64(0), int64(time.Nanosecond), false)
	f.Add(int64(-time.Nanosecond), int64(time.Nanosecond), false)
	f.Add(int64(time.Second), int64(time.Nanosecond), true)

	f.Fuzz(func(t *testing.T, timeoutNanos, intervalNanos int64, canceled bool) {
		timeout := time.Duration(timeoutNanos)
		interval := time.Duration(intervalNanos)
		if interval > 0 && timeout > 0 && !canceled {
			t.Skip("positive timer ticking is intentionally covered by deterministic tests")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if canceled {
			cancel()
		}

		called := 0
		err := signal.Timer(ctx, timeout, interval, signal.Hook{
			OnStart: func(context.Context) error {
				called++
				return errors.New("unexpected start")
			},
			OnTick: func(context.Context) error {
				called++
				return errors.New("unexpected tick")
			},
			OnStop: func(context.Context) error {
				called++
				return errors.New("unexpected stop")
			},
		})

		require.Zero(t, called)
		if interval <= 0 {
			require.ErrorIs(t, err, signal.ErrInvalidInterval)
			return
		}
		require.NoError(t, err)
	})
}

func FuzzTerminatedWrapping(f *testing.F) {
	f.Add("", false, 0)
	f.Add("signal: test error", false, 1)
	f.Add("signal: test error", true, 2)
	f.Add("signal: terminated", false, 3)

	f.Fuzz(func(t *testing.T, message string, alreadyTerminated bool, wraps int) {
		require.False(t, signal.IsTerminated(nil))
		require.ErrorIs(t, signal.Terminated(nil), signal.ErrTerminated)

		base := errors.New(message)
		err := base
		if alreadyTerminated {
			err = signal.Terminated(err)
		}
		for i := range wraps % 4 {
			err = fmt.Errorf("wrap %d: %w", i, err)
		}

		terminated := signal.Terminated(err)

		require.True(t, signal.IsTerminated(terminated))
		require.ErrorIs(t, terminated, signal.ErrTerminated)
		require.ErrorIs(t, terminated, base)
	})
}

type lifecycleRunFuzz struct {
	raw            []byte
	events         []string
	expectedEvents []string
	expectedErrs   map[error]struct{}
	allErrs        []error
	stopErrs       []error
	started        []bool
	stopEnabled    []bool
	stopFails      []bool
	startFailed    bool
}

func newLifecycleRunFuzz(raw []byte) *lifecycleRunFuzz {
	return &lifecycleRunFuzz{
		raw:            raw,
		events:         make([]string, 0, len(raw)*2+1),
		expectedEvents: make([]string, 0, len(raw)*2+1),
		expectedErrs:   make(map[error]struct{}),
		allErrs:        make([]error, 0, len(raw)*2+1),
		stopErrs:       make([]error, len(raw)),
		started:        make([]bool, len(raw)),
		stopEnabled:    make([]bool, len(raw)),
		stopFails:      make([]bool, len(raw)),
	}
}

func (r *lifecycleRunFuzz) register(t *testing.T, lifecycle *signal.Lifecycle) {
	t.Helper()

	for i, flags := range r.raw {
		lifecycle.Register(r.hook(i, flags))
	}
}

func (r *lifecycleRunFuzz) hook(i int, flags byte) signal.Hook {
	startEnabled := flags&0x01 != 0
	startFails := flags&0x02 != 0
	r.stopEnabled[i] = flags&0x04 != 0
	r.stopFails[i] = flags&0x08 != 0
	stopReturnsContextErr := flags&0x10 != 0
	startErr := fmt.Errorf("start %d", i)
	stopErr := fmt.Errorf("stop %d", i)
	r.stopErrs[i] = stopErr
	r.allErrs = append(r.allErrs, startErr, stopErr)

	hook := signal.Hook{}
	if startEnabled {
		r.expectedEvents = append(r.expectedEvents, fmt.Sprintf("start:%d", i))
		hook.OnStart = func(context.Context) error {
			r.events = append(r.events, fmt.Sprintf("start:%d", i))
			if startFails {
				return startErr
			}
			return nil
		}
	}
	if startEnabled && startFails {
		r.startFailed = true
		r.expectedErrs[startErr] = struct{}{}
	} else {
		r.started[i] = true
	}
	if r.stopEnabled[i] {
		hook.OnStop = func(ctx context.Context) error {
			r.events = append(r.events, fmt.Sprintf("stop:%d", i))
			if r.stopFails[i] {
				return stopErr
			}
			if stopReturnsContextErr {
				return ctx.Err()
			}
			return nil
		}
	}

	return hook
}

func (r *lifecycleRunFuzz) expect(handlerErr error, handlerFails bool) {
	r.allErrs = append(r.allErrs, handlerErr)
	if r.startFailed {
		r.expectRollbackStops()
		return
	}

	r.expectedEvents = append(r.expectedEvents, "handler")
	if handlerFails {
		r.expectedErrs[handlerErr] = struct{}{}
	}
	r.expectSuccessfulStops()
}

func (r *lifecycleRunFuzz) expectRollbackStops() {
	for i := range slices.Backward(r.raw) {
		if !r.started[i] || !r.stopEnabled[i] {
			continue
		}
		r.expectStop(i)
	}
}

func (r *lifecycleRunFuzz) expectSuccessfulStops() {
	for i := range slices.Backward(r.raw) {
		if r.stopEnabled[i] {
			r.expectStop(i)
		}
	}
}

func (r *lifecycleRunFuzz) expectStop(i int) {
	r.expectedEvents = append(r.expectedEvents, fmt.Sprintf("stop:%d", i))
	if r.stopFails[i] {
		r.expectedErrs[r.stopErrs[i]] = struct{}{}
	}
}
