# Standard entry points — `make check` must pass before any commit.

.DEFAULT_GOAL := help

.PHONY: help build test lint fmt vet check tidy clean preview

help: ## Show this help
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

build: ## Build the CLI into ./bin/staticart
	go build -o bin/staticart ./cmd/staticart

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format code (gofumpt via golangci-lint formatters)
	gofumpt -w .
	goimports -local github.com/jaminalder/go-graphics -w .

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Format, vet, lint, and test — the pre-commit gate

tidy: ## go mod tidy
	go mod tidy

clean: ## Remove build artifacts and rendered output
	rm -rf bin out

# Example render targets (available once cmd/staticart exists):
preview: ## Render the contour sketch at preview size into out/
	go run ./cmd/staticart render contour --profile preview --out out
