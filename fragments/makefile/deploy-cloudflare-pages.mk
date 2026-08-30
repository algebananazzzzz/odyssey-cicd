# --- deploy: cloudflare pages ---

deploy: build ## Deploy to ENV
	npx wrangler pages deploy dist/ --project-name=$(ENV)-web-pages-{{PROJECT}}
