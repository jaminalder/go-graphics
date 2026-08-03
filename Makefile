# Standard entry points — `make check` must pass before any commit.

.DEFAULT_GOAL := help

.PHONY: help build test lint fmt vet check tidy golden clean preview preview-qql preview-pools preview-foam preview-scree preview-riffle hatchbook sweep

help: ## Show this help
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

build: ## Build the CLI into ./bin/staticart
	go build -o bin/staticart ./cmd/staticart

test: ## Run all tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format code (gofumpt + goimports via golangci-lint formatters)
	golangci-lint fmt

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Format, vet, lint, and test — the pre-commit gate

tidy: ## go mod tidy
	go mod tidy

golden: ## Regenerate all golden images (eyeball diffs before committing)
	go test ./internal/sketch/... -run TestGolden -update

clean: ## Remove build artifacts and rendered output
	rm -rf bin out

# Example render targets (available once cmd/staticart exists):
preview: ## Render the contour sketch at preview size into out/
	go run ./cmd/staticart render contour --profile preview --out out

preview-qql: ## Render the qql sketch at its native 4:5 preview size into out/
	go run ./cmd/staticart render qql --profile preview-tall --out out

preview-pools: ## Render the pools sketch at preview size into out/
	go run ./cmd/staticart render pools --profile preview --palette tchelitchew-hide-and-seek --out out

preview-foam: ## Render the foam sketch at preview size into out/
	go run ./cmd/staticart render foam --profile preview --out out

preview-scree: ## Render the scree sketch at preview size into out/
	go run ./cmd/staticart render scree --profile preview --out out

preview-riffle: ## Render the riffle sketch (a river from above) at preview size into out/
	go run ./cmd/staticart render riffle --profile preview --out out

HATCHBOOK_OUT ?= out/agent-hatch
HATCHBOOK_ARGS = --aa 3 --palette hopper-night-windows --out $(HATCHBOOK_OUT)

hatchbook: ## Render the internal/hatch specimen sheets + manifest into $(HATCHBOOK_OUT)
	go run ./cmd/staticart render hatchbook --page structures --width 2000 --height 2000 $(HATCHBOOK_ARGS)
	go run ./cmd/staticart render hatchbook --page parameters --width 2400 --height 1620 $(HATCHBOOK_ARGS)
	go run ./cmd/staticart render hatchbook --page variation  --width 2400 --height 1620 $(HATCHBOOK_ARGS)
	go run ./cmd/staticart render hatchbook --page colour     --width 2000 --height 2000 $(HATCHBOOK_ARGS)
	go run ./cmd/staticart render hatchbook --page shapes     --width 2400 --height 1820 $(HATCHBOOK_ARGS)
	go run ./tools/hatchbook > $(HATCHBOOK_OUT)/manifest.txt

sweep: ## Sweep 12 seeds of a sketch into a contact sheet (S=pools)
	go run ./cmd/staticart sweep $(or $(S),pools) --seeds 1-12 --out out/sweep
