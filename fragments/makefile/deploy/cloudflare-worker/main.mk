ENVF := $(if $(filter-out prd,$(ENV)),--env $(ENV),)

deploy: ## Deploy to ENV
	npx opennextjs-cloudflare build $(ENVF)
	npx opennextjs-cloudflare deploy $(ENVF)
