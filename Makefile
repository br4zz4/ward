.PHONY: test test.integration build

build:
	go build -o build/ward ./cmd/ward

test:
	go test ./...

test.integration:
	go test -tags integration ./test/integration/...
