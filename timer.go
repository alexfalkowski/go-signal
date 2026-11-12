package signal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexfalkowski/go-signal/v2/internal/lifecycle"
)

// ErrInvalidInterval reports a non-positive Timer interval.
var ErrInvalidInterval = errors.New("signal: invalid interval")

// Timer calls tick at interval until cancellation or a callback error. The first
// call occurs after one interval. Calls run synchronously and never overlap;
// slow callbacks may cause ticks to be dropped, as with time.NewTicker.
//
// Cancellation observed between callbacks returns nil. A callback error takes
// precedence over cancellation and is returned unchanged. Callback panics become
// errors marked with ErrRecovered. The interval must be positive and tick must
// be non-nil. Callbacks must honor their context; cancellation cannot interrupt
// a blocked callback.
//
// Timer blocks until it finishes. Launch it through go-sync.Go from a lifecycle
// start hook, then cancel its context and await the task from a stop hook.
func Timer(ctx context.Context, interval time.Duration, tick Handler) (err error) {
	if interval <= 0 {
		return fmt.Errorf("%w: %s", ErrInvalidInterval, interval)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = lifecycle.RecoveredError(recovered)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil
			}

			if err := tick(ctx); err != nil {
				return err
			}
		}
	}
}
