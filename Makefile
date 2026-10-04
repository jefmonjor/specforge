MODULE  := specforge
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X $(MODULE)/internal/buildinfo.Version=$(VERSION) \
  -X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) \
  -X $(MODULE)/internal/buildinfo.Date=$(DATE)

.PHONY: build test test-integration lint fmt tidy-check release-snapshot

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o specforge .

test:
	go test -race -cover ./...

## Runs the tests that shell out to real tools (golangci-lint, npx, markitdown).
test-integration:
	go test -tags integration ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt: files need formatting:"; gofmt -l .; exit 1; }
	go vet ./...

fmt:
	gofmt -w .

tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

## Cross-compiles every platform into dist/ without publishing (needs goreleaser).
release-snapshot:
	goreleaser release --snapshot --clean
