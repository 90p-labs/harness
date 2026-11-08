VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test check

build:
	go build -ldflags "$(LDFLAGS)" -o dist/harness ./cmd/harness

test:
	go test ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
