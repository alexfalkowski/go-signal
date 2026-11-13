// Package signaltest exercises component hooks in tests and benchmarks.
// Register hooks or pass the helper as signal.Lifecycle to component constructors,
// register RequireStop with testing.TB.Cleanup, then call RequireStart. The
// helpers use the application's internal hook engine and fresh cleanup contexts.
package signaltest
