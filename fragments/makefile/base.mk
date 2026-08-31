ENV ?= preprod

.DEFAULT_GOAL := help
.PHONY: help setup check test build deploy ci

help: ## List targets
	@grep -E '^[a-z][a-z-]*:.*?##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?##"} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

ci: check test build ## Mirror CI locally
