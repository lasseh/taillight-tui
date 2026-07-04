GO ?= go
GOFLAGS ?=
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

# Detect gotestsum for nicer test output, fall back to plain go test.
GOTESTSUM := $(shell command -v gotestsum 2>/dev/null)
ifdef GOTESTSUM
  TEST_CMD := gotestsum --format testdox --
else
  TEST_CMD := $(GO) test
endif

.PHONY: build build-tui build-wish build-linux build-tui-linux build-wish-linux \
	docker run-tui test lint fix tidy vulncheck coverage clean help

##@ General
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build
build: build-tui build-wish ## Build both binaries

build-tui: ## Build the taillight-tui client
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o taillight-tui ./cmd/taillight-tui

build-wish: ## Build the taillight-wish SSH server
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o taillight-wish ./cmd/taillight-wish

build-linux: build-tui-linux build-wish-linux ## Build both Linux amd64 binaries

build-tui-linux: ## Build taillight-tui for Linux amd64
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(LDFLAGS) -o taillight-tui-linux-amd64 ./cmd/taillight-tui

build-wish-linux: ## Build taillight-wish for Linux amd64
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) $(LDFLAGS) -o taillight-wish-linux-amd64 ./cmd/taillight-wish

docker: ## Build the taillight-wish Docker image stamped with VERSION
	docker build --build-arg VERSION=$(VERSION) -t taillight-wish .

run-tui: ## Build and run the TUI (pass ARGS="-s URL -k KEY")
	$(GO) run ./cmd/taillight-tui $(ARGS)

##@ Quality
test: ## Run tests with the race detector
	$(TEST_CMD) -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

coverage: ## Run tests with coverage and write an HTML report
	$(TEST_CMD) -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

##@ Utility
fix: ## Apply Go version modernizations (go fix)
	$(GO) fix ./...

tidy: ## Verify go.mod and go.sum are tidy
	$(GO) mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy" && exit 1)

vulncheck: ## Scan for known vulnerabilities
	govulncheck ./...

clean: ## Remove build artifacts
	rm -f taillight-tui taillight-tui-linux-amd64 taillight-wish taillight-wish-linux-amd64 coverage.out coverage.html

.DEFAULT_GOAL := help
