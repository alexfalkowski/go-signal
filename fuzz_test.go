package signal_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/alexfalkowski/go-signal/v2"
	"github.com/stretchr/testify/require"
)

// FuzzLifecycleHookMatrix explores startup, rollback, and cleanup combinations.
func FuzzLifecycleHookMatrix(f *testing.F) {
	f.Add([]byte{}, false)
	f.Add([]byte{0}, false)
	f.Add([]byte{0x05, 0x05}, false)
	f.Add([]byte{0x05, 0x07, 0x05}, false)
	f.Add([]byte{0x0d, 0x0f, 0x07}, false)
	f.Add([]byte{0x05, 0x05}, true)

	f.Fuzz(func(t *testing.T, raw []byte, canceled bool) {
		const maxHooks = 8
		if len(raw) > maxHooks {
			raw = raw[:maxHooks]
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if canceled {
			cancel()
		}

		run := newLifecycleFuzz(raw)
		app, err := signal.NewApp(signal.Register(func(lc signal.Lifecycle) {
			for i, flags := range raw {
				lc.Append(run.hook(i, flags))
			}
		}))
		require.NoError(t, err)
		run.expect(canceled)

		err = app.Run(ctx)
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
		if !canceled {
			require.NotErrorIs(t, err, context.Canceled)
		}
	})
}

type lifecycleFuzz struct {
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

func newLifecycleFuzz(raw []byte) *lifecycleFuzz {
	return &lifecycleFuzz{
		raw:            raw,
		events:         make([]string, 0, len(raw)*2),
		expectedEvents: make([]string, 0, len(raw)*2),
		expectedErrs:   make(map[error]struct{}),
		allErrs:        make([]error, 0, len(raw)*2),
		stopErrs:       make([]error, len(raw)),
		started:        make([]bool, len(raw)),
		stopEnabled:    make([]bool, len(raw)),
		stopFails:      make([]bool, len(raw)),
	}
}

func (r *lifecycleFuzz) hook(i int, flags byte) signal.Hook {
	startEnabled := flags&0x01 != 0
	startFails := flags&0x02 != 0
	r.stopEnabled[i] = flags&0x04 != 0
	r.stopFails[i] = flags&0x08 != 0
	startErr := fmt.Errorf("start %d", i)
	stopErr := fmt.Errorf("stop %d", i)
	r.stopErrs[i] = stopErr
	r.allErrs = append(r.allErrs, startErr, stopErr)

	hook := signal.Hook{}
	if startEnabled {
		if !r.startFailed {
			r.expectedEvents = append(r.expectedEvents, fmt.Sprintf("start:%d", i))
		}
		hook.OnStart = func(context.Context) error {
			r.events = append(r.events, fmt.Sprintf("start:%d", i))
			if startFails {
				return startErr
			}

			return nil
		}
	}
	if !r.startFailed {
		if startEnabled && startFails {
			r.startFailed = true
			r.expectedErrs[startErr] = struct{}{}
		} else {
			r.started[i] = true
		}
	}
	if r.stopEnabled[i] {
		hook.OnStop = func(context.Context) error {
			r.events = append(r.events, fmt.Sprintf("stop:%d", i))
			if r.stopFails[i] {
				return stopErr
			}

			return nil
		}
	}

	return hook
}

func (r *lifecycleFuzz) expect(canceled bool) {
	if canceled {
		r.expectedEvents = r.expectedEvents[:0]
		clear(r.expectedErrs)
		r.expectedErrs[context.Canceled] = struct{}{}
		return
	}

	for i := range slices.Backward(r.raw) {
		if !r.started[i] || !r.stopEnabled[i] {
			continue
		}

		r.expectedEvents = append(r.expectedEvents, fmt.Sprintf("stop:%d", i))
		if r.stopFails[i] {
			r.expectedErrs[r.stopErrs[i]] = struct{}{}
		}
	}
}
