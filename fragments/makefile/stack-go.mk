# --- go ---

setup: ## Install dependencies
	go mod download

check: ## Format + vet
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

test: ## Unit tests
	go test -race -coverprofile=coverage.out ./...

build: ## Build artifact
	go build -o bin/{{PROJECT}} ./cmd/{{PROJECT}}
