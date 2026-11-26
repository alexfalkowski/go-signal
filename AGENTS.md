# AGENTS.md

## Shared guidance

Use `bin/AGENTS.md` for shared skills and cross-repository defaults.

## Accepted design

Before proposing a change or recording a finding or audit entry, read the
[accepted-design index](docs/agents/accepted-design.md) and the area files
relevant to your scope. Apply each decision within its documented scope and
record newly agreed design choices in the relevant area file.

## Repo

- Module: `github.com/alexfalkowski/go-signal/v2`.
- Read current toolchain details from repo files such as `go.mod`,
  `.circleci/config.yml`, and `bin/`; do not duplicate them here.
- Purpose: small library for startup/shutdown hooks around OS signals.
- Main files: `signal.go`, `app.go`, `options.go`, `hook.go`, `timer.go`, `app_test.go`, `timer_test.go`,
  `internal/lifecycle/`, `signaltest/`,
  `cmd/main.go`, `README.md`, `.circleci/config.yml`.

## Commands

Initialize shared tooling first when needed:

Use `make submodule` once the shared `bin` checkout is present; see
`bin/AGENTS.md` for fresh-clone bootstrap details.

Common targets: `make dep`, `make lint`, `make sec`, `make specs`,
`make coverage`, `make run param=start|timer|terminate`.

CI order: `make source-key`, `make clean`, `make dep`, `make clean`,
`make lint`, `make sec`, `make specs`, `make fuzzes`, `make coverage`,
`make benchmarks`, `make codecov-upload`.
