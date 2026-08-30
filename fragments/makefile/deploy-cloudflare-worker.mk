# --- deploy: cloudflare worker (opennext) ---
# prd uses the top-level wrangler config; other envs pass --env.

ENVF := $(if $(filter-out prd,$(ENV)),--env $(ENV),)

deploy: ## Deploy to ENV
	npx opennextjs-cloudflare build $(ENVF)
	npx opennextjs-cloudflare deploy $(ENVF)
