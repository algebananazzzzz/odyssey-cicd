# intranet-web — task runner.
#
# CI calls these targets and nothing else. Every workflow step is `make <target>`,
# so this file is the whole contract between the repo and its pipeline. Change a
# tool here and CI follows; CI never names a tool directly.
#
# Rules:
#   - A recipe is 1-3 lines. Longer goes to scripts/<name>.sh and the target calls it.
#   - ENV is the only universal variable. Everything else is ?= overridable.
#   - Targets are fixed names with fixed meanings. Do not invent new ones for CI.

ENV ?= preprod

.DEFAULT_GOAL := help
.PHONY: help setup check test build deploy ci

help: ## List targets
	@grep -E '^[a-z][a-z-]*:.*?##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?##"} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

ci: check test build ## Mirror CI locally

# --- node ---

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

# --- deploy: cloudflare worker (opennext) ---
# prd uses the top-level wrangler config; other envs pass --env.

ENVF := $(if $(filter-out prd,$(ENV)),--env $(ENV),)

deploy: ## Deploy to ENV
	npx opennextjs-cloudflare build $(ENVF)
	npx opennextjs-cloudflare deploy $(ENVF)

# --- infra: terraform ---
# Per-env config lives in infra/config/$(ENV).tfbackend and $(ENV).tfvars.
# Credentials come from the environment, never from a committed file.

TF := terraform -chdir=infra

.PHONY: infra infra-plan

infra-plan: ## Plan infrastructure for ENV
	$(TF) init -reconfigure -backend-config=config/$(ENV).tfbackend
	$(TF) plan -var-file=config/$(ENV).tfvars

infra: ## Apply infrastructure for ENV
	$(TF) init -reconfigure -backend-config=config/$(ENV).tfbackend
	$(TF) apply -var-file=config/$(ENV).tfvars -auto-approve

