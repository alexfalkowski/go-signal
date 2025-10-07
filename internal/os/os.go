package os

import (
	"os"
	"syscall"
)

var (
	// Interrupt is an alias for os.Interrupt.
	Interrupt = os.Interrupt

	// Terminate an alias for syscall.SIGTERM.
	Terminate os.Signal = syscall.SIGTERM
)

// FindProcess is an alias for os.FindProcess.
func FindProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}

// Getpid is an alias for os.Getpid.
func Getpid() int {
	return os.Getpid()
}
