include bin/build/make/help.mak
include bin/build/make/go.mak
include bin/build/make/git.mak

# Run all the benchmarks.
benchmarks: lifecycle-benchmarks

lifecycle-benchmarks:
	@$(MAKE) benchtime=100x benchmark

# Run bounded fuzz smoke tests. Set fuzztime=<duration> to override the default 1s per target.
fuzz-smoke: lifecycle-fuzz timer-fuzz terminated-fuzz

lifecycle-fuzz:
	@$(MAKE) package=. name=FuzzLifecycleRunHookMatrix fuzztime=$(or $(fuzztime),1s) fuzz

timer-fuzz:
	@$(MAKE) package=. name=FuzzTimerEntryGuards fuzztime=$(or $(fuzztime),1s) fuzz

terminated-fuzz:
	@$(MAKE) package=. name=FuzzTerminatedWrapping fuzztime=$(or $(fuzztime),1s) fuzz

# Run the manual example.
run:
	@go run cmd/main.go $(param)
