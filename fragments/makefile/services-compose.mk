# --- local services: docker compose ---
# Same services CI starts, so `make ci` locally means what it says.

.PHONY: up down test-integration

up: ## Start local services
	docker compose up -d --wait

down: ## Stop local services
	docker compose down -v

test-integration: ## Integration tests (needs `make up`)
	npx vitest run --config vitest.integration.config.ts
