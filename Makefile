VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/arm64 linux/amd64

.PHONY: build dist test check

build:
	go build -ldflags "$(LDFLAGS)" -o dist/harness ./cmd/harness

dist:
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dist/harness-$$os-$$arch ./cmd/harness || exit 1; \
	done

test:
	go test ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
