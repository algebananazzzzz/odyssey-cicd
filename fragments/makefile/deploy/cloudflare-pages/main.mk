DIST ?= dist

deploy: ## Deploy to ENV
	npx wrangler pages deploy $(DIST)/ --project-name=$(ENV)-web-pages-{{PROJECT}} --branch=main
