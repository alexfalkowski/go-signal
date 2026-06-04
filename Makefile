include bin/build/make/go.mak
include bin/build/make/git.mak

# Run the manual example.
run:
	@go run cmd/main.go $(param)
