package signal

import (
	"os"
	"syscall"
)

// Signal aliases [os.Signal] for selecting shutdown signals.
type Signal = os.Signal

// Interrupt is SIGINT, one of the signals [App.Serve] supports by default.
var Interrupt Signal = os.Interrupt

// Termination is SIGTERM, one of the signals [App.Serve] supports by default.
var Termination Signal = syscall.SIGTERM

// Hangup is SIGHUP, an optional signal callers can pass to [App.Serve] to extend
// its default signal set.
var Hangup Signal = syscall.SIGHUP
