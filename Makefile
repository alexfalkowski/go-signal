include bin/build/make/go.mak
include bin/build/make/git.mak

# Run a test.
run:
	@go run cmd/main.go $(param)

# Diagram generated from https://github.com/loov/goda.
diagram:
	@make package=. create-diagram
