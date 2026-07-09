include .commons/make/shell.makefile

.PHONY: test test.integration test.e2e build dev

build:
	go build -o build/ward ./cmd/ward

# Build and install a local dev copy (ward-dev) on the user's PATH, so it can be
# tested side by side with a released ward. Delegates to the //go:generate
# directive in cmd/ward so the build recipe lives with the app, not here.
dev:
	go generate ./cmd/ward

# Unit tests — fast, no external processes or file I/O beyond what Go tests do
test:
	go test ./...

# Integration tests — test internal components together with real files on disk
test.integration:
	go test -tags integration ./test/integration/...

# E2E tests — build the binary and test via CLI (slow)
test.e2e:
	go test -tags e2e ./test/e2e/...
