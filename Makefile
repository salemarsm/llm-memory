.PHONY: all build test check clean install snapshot

GO ?= go
BIN_DIR ?= bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/salemarsm/llm-memory/internal/version.Version=$(VERSION) \
	-X github.com/salemarsm/llm-memory/internal/version.Commit=$(COMMIT) \
	-X github.com/salemarsm/llm-memory/internal/version.Date=$(DATE)

COMMANDS := llm-memory memctl memmcp memserver

all: check

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/llm-memory ./cmd/llm-memory
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/memctl ./cmd/memctl
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/memmcp ./cmd/memmcp
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/memserver ./cmd/memserver

test:
	$(GO) test ./...

check:
	$(GO) test ./...
	$(GO) build ./cmd/...

install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/llm-memory
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/memctl
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/memmcp
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/memserver

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN_DIR) dist
