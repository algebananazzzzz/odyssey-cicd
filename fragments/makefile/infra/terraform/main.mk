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
