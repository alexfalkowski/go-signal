package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunWithoutMode(t *testing.T) {
	require.ErrorIs(t, run([]string{"go-signal"}), errUsage)
}

func TestRunWithInvalidMode(t *testing.T) {
	err := run([]string{"go-signal", "unknown"})

	require.ErrorIs(t, err, errInvalidMode)
	require.ErrorContains(t, err, usageMessage)
}
