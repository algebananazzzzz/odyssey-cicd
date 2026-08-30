# --- deploy: aws ecs ---

deploy: ## Deploy to ENV
	ENV=$(ENV) PROJECT={{PROJECT}} scripts/deploy-ecs.sh
