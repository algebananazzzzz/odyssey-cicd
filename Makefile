.DEFAULT_GOAL := help
.PHONY: help setup check test build ci

help: ## List targets
	@grep -E '^[a-z][a-z-]*:.*?##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?##"} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

setup: ## Install dependencies
	go mod download

check: ## Format + vet
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

test: ## Unit tests + manifest validation
	go test ./...
	go run . validate

build: ## Build artifact
	CGO_ENABLED=0 go build -o dist/odyssey-cli .

ci: check test build ## Mirror CI locally
