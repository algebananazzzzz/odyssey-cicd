# --- python (uv + ruff) ---

setup: ## Install dependencies
	uv sync --frozen

check: ## Format + lint + types
	uv run ruff format --check .
	uv run ruff check .
	uv run mypy .

test: ## Unit tests
	uv run pytest

build: ## Build artifact
	uv build
