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
