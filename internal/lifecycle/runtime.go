package lifecycle

import (
	"errors"
	"fmt"
)

// ErrRecovered marks an error produced by recovering a callback panic.
//
// Use [errors.Is] to detect a recovered panic:
//
//	errors.Is(err, lifecycle.ErrRecovered)
var ErrRecovered = errors.New("signal: recovered")

// RecoveredError marks a recovered panic with ErrRecovered and preserves an
// error value's identity for errors.Is.
func RecoveredError(value any) error {
	switch recovered := value.(type) {
	case error:
		return fmt.Errorf("%w: %w", recovered, ErrRecovered)
	case string:
		return fmt.Errorf("%s: %w", recovered, ErrRecovered)
	default:
		return fmt.Errorf("%v: %w", recovered, ErrRecovered)
	}
}
