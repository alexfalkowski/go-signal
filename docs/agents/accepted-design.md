# Accepted design

This index records go-signal's accepted design choices, tradeoffs, and support
boundaries. The area files carry the same weight as [AGENTS.md](../../AGENTS.md).

Read the relevant area files before proposing a change or recording a finding
or audit entry. Revisit a decision when the owner asks or new evidence changes
the documented contract or tradeoff.

- [Application lifecycle](accepted-design/application-lifecycle.md) covers
  construction, hook registration, rollback, deadlines, and shutdown ownership.
- [Timer](accepted-design/timer.md) covers periodic callbacks, cancellation, and
  worker ownership.
- [Testing](accepted-design/testing.md) covers component helpers, signal and
  HTTP tests, and the scope of the manual application.
