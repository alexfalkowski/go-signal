package signal

import (
	"errors"
	"fmt"
)

// ErrRecovered marks an error produced from a panic recovered by [Hook.Start],
// [Hook.Tick], [Hook.Stop], [Lifecycle.Run]'s handler, or [Go]'s handler.
//
// Use [errors.Is] to detect a recovered panic:
//
//	errors.Is(err, signal.ErrRecovered)
var ErrRecovered = errors.New("signal: recovered")

func convertRecover(value any) error {
	switch recovered := value.(type) {
	case error:
		return fmt.Errorf("%w: %w", recovered, ErrRecovered)
	case string:
		return fmt.Errorf("%s: %w", recovered, ErrRecovered)
	default:
		return fmt.Errorf("%v: %w", recovered, ErrRecovered)
	}
}
