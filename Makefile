fuzztime ?= 1000x

include bin/build/make/help.mak
include bin/build/make/go.mak
include bin/build/make/git.mak
include bin/build/make/claude.mak
include bin/build/make/codex.mak

# Run all benchmarks with 100 iterations each.
benchmarks: lifecycle-benchmarks

# Run lifecycle hook benchmarks with 100 iterations.
lifecycle-benchmarks:
	@$(MAKE) package=. benchtime=100x benchmark

# Run bounded fuzz tests with 1000 executions per target by default.
# Set fuzztime=<duration-or-count> to override the default.
fuzzes: lifecycle-fuzz

# Fuzz lifecycle hook orchestration using fuzztime (default 1000x).
lifecycle-fuzz:
	@$(MAKE) package=. name=FuzzLifecycleHookMatrix fuzz

# Run the manual example with param=start|timer|terminate (required).
# The start and timer modes wait for interruption; terminate shuts down itself.
run:
	@go run cmd/main.go $(param)
