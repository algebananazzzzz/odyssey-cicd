TF := terraform -chdir=infra

.PHONY: infra infra-check infra-init

infra-init:
	@test -n "$(STATE_BUCKET)" || { echo "STATE_BUCKET is not set" >&2; exit 1; }
	$(TF) init -reconfigure -input=false -backend-config=config/$(ENV).tfbackend -backend-config="bucket=$(STATE_BUCKET)"

infra-check: ## Check formatting + validate terraform; no backend, no credentials
	$(TF) fmt -check -recursive
	$(TF) init -backend=false -input=false >/dev/null
	$(TF) validate

infra: infra-init ## Apply infra for ENV
	$(TF) apply -input=false -auto-approve -var-file=config/$(ENV).tfvars
