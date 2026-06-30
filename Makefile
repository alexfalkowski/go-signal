include bin/build/make/help.mak
include bin/build/make/go.mak
include bin/build/make/git.mak

# Run all the benchmarks.
benchmarks: lifecycle-benchmarks

lifecycle-benchmarks:
	@$(MAKE) benchtime=100x benchmark

# Run bounded fuzz tests. Set fuzztime=<duration-or-count> to override the default 1000x per target.
fuzzes: lifecycle-fuzz timer-fuzz terminated-fuzz

lifecycle-fuzz:
	@$(MAKE) package=. name=FuzzLifecycleRunHookMatrix fuzztime=$(or $(fuzztime),1000x) fuzz

timer-fuzz:
	@$(MAKE) package=. name=FuzzTimerEntryGuards fuzztime=$(or $(fuzztime),1000x) fuzz

terminated-fuzz:
	@$(MAKE) package=. name=FuzzTerminatedWrapping fuzztime=$(or $(fuzztime),1000x) fuzz

# Run the manual example.
run:
	@go run cmd/main.go $(param)
