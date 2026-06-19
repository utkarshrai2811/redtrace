.DEFAULT_GOAL := help

BINARY      := redtrace
PKG         := github.com/utkarshrai2811/redtrace
CMD         := ./cmd/redtrace
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     :=

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: web-install web-build build ## Full build: install + build the UI, then the binary

.PHONY: dev
dev: ## Run proxy + frontend in dev mode
	./scripts/dev.sh

.PHONY: build
build: ## Build the redtrace binary into ./bin
	@mkdir -p bin
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

.PHONY: run
run: build ## Build and run the server
	./bin/$(BINARY) serve

.PHONY: test
test: ## Run all Go tests with the race detector
	go test -race -count=1 ./...

.PHONY: cover
cover: ## Run tests with coverage report
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run ./...

.PHONY: vuln
vuln: ## Run govulncheck
	govulncheck ./...

.PHONY: tidy
tidy: ## Tidy go modules
	go mod tidy

.PHONY: fmt
fmt: ## Format Go code
	gofmt -s -w .
	goimports -w .

.PHONY: web-install
web-install: ## Install frontend dependencies
	cd web && npm ci

.PHONY: web-build
web-build: ## Build the frontend
	cd web && npm run build

.PHONY: web-lint
web-lint: ## Lint the frontend
	cd web && npm run lint

.PHONY: docker
docker: ## Build the Docker image
	docker build -t redtrace:$(VERSION) .

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin dist coverage.txt coverage.html web/dist
