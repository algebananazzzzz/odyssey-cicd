# --- node-puppeteer ---

setup: ## Install dependencies
	npm ci --prefer-offline --no-audit --no-fund

check: ## Format + lint + types
	npx prettier --check .
	npx eslint .
	npx tsc --noEmit

test: ## Unit + component tests
	npx vitest run

build: ## Build artifact
	npm run build

