VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/arm64 linux/amd64
MARKETPLACE := harness
PLUGIN := 90p@$(MARKETPLACE)

COVERAGE_FLOOR ?= 85

.PHONY: build dist test cover check install-claude

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

# A failing test stops the target before the floor check. A trap removes the profile on exit and interrupt.
cover:
	profile=$$(mktemp) || exit 1; \
	trap 'rm -f "$$profile"' EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	go test -coverprofile="$$profile" ./... || exit 1; \
	sh scripts/coverfloor.sh "$$profile" "$(COVERAGE_FLOOR)"

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	$(MAKE) cover

# Claude Code copies the plugin folder into its cache, so the binary must be in dist/ before the install.
install-claude:
	$(MAKE) dist PLATFORMS=$$(go env GOOS)/$$(go env GOARCH)
	@claude plugin marketplace list | grep -Eq " $(MARKETPLACE)$$" || claude plugin marketplace add "$(CURDIR)"
	claude plugin marketplace update $(MARKETPLACE)
	@if claude plugin list | grep -Eq " $(PLUGIN)$$"; then claude plugin uninstall $(PLUGIN); fi
	claude plugin install $(PLUGIN)
