include .commons/make/shell.makefile

.PHONY: test test.integration test.e2e build

build:
	go build -o build/ward ./cmd/ward

# Unit tests — fast, no external processes or file I/O beyond what Go tests do
test:
	go test ./...

# Integration tests — test internal components together with real files on disk
test.integration:
	go test -tags integration ./test/integration/...

# E2E tests — build the binary and test via CLI (slow)
test.e2e:
	go test -tags e2e ./test/e2e/...
