BINARY      := waitfor
PKG         := github.com/0xProgress/waitfor
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
               -X $(PKG)/internal/version.Version=$(VERSION) \
               -X $(PKG)/internal/version.Commit=$(COMMIT) \
               -X $(PKG)/internal/version.Date=$(DATE)

GOBIN       ?= $(shell go env GOPATH)/bin
BIN_DIR     := bin

.PHONY: all build install test race vet fmt lint check clean release help

all: build

build: ## build the binary into ./bin
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

install: ## install into $GOBIN
	go install -trimpath -ldflags "$(LDFLAGS)" .

test: ## run unit tests
	go test ./... -count=1

race: ## run tests with the race detector
	go test -race ./... -count=1

vet: ## go vet
	go vet ./...

fmt: ## gofmt all Go files
	gofmt -s -w .

lint: vet ## vet + check for unformatted files
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
	  echo "unformatted files:"; echo "$$out"; exit 1; \
	fi

check: lint race ## run all checks (lint + race tests) — same as CI
	@echo "✓ All checks passed"

clean: ## remove build artifacts
	rm -rf $(BIN_DIR) dist/

release: ## snapshot release (writes to ./dist)
	goreleaser release --snapshot --clean

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} \
	     /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' \
	     $(MAKEFILE_LIST)
