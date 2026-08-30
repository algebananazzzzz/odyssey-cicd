# intranet-web task runner.
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

# --- nextjs ---

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

TF := terraform -chdir=infra

ifneq (,$(wildcard infra/.env))
include infra/.env
export $(shell sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)=.*/\1/p' infra/.env)
endif

.PHONY: infra infra-plan infra-check infra-init

# -reconfigure: the backend key changes with ENV, and without it Terraform
# offers to migrate the previous env's state into the new key.
infra-init:
	$(TF) init -reconfigure -input=false -backend-config=config/$(ENV).tfbackend

infra-check: ## Check formatting + validate terraform; no backend, no credentials
	$(TF) fmt -check -recursive
	$(TF) init -backend=false -input=false >/dev/null
	$(TF) validate

infra-plan: infra-init ## Show pending infra changes for ENV
	$(TF) plan -input=false -var-file=config/$(ENV).tfvars

infra: infra-init ## Apply infra for ENV
	$(TF) apply -input=false -auto-approve -var-file=config/$(ENV).tfvars
