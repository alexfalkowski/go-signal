// Package signal provides a small lifecycle for coordinating application
// startup and shutdown work around process signals.
//
// A [Lifecycle] runs registered hooks in three phases:
//
//   - start, by calling each hook's [Hook.OnStart]
//   - run, by executing user code through [Run] or waiting for shutdown through [Serve]
//   - stop, by calling each hook's [Hook.OnStop]
//
// The package-level helpers operate on a process-wide default lifecycle that is
// initialized with a 30-second stop timeout.
package signal
