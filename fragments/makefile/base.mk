# {{PROJECT}} task runner.
#
# CI calls these targets and nothing else. Every workflow step is `make <target>`,
# so this file is the whole contract between the repo and its pipeline. Change a
# tool here and CI follows; CI never names a tool directly.
#
# Rules:
#   - A recipe is 1-3 lines. Longer goes to scripts/<name>.sh and the target calls it.
#   - ENV is the only universal variable. Everything else is ?= overridable.
#   - Targets are fixed names with fixed meanings. Do not invent new ones for CI.

ENV ?= preprod

.DEFAULT_GOAL := help
.PHONY: help setup check test build deploy ci

help: ## List targets
	@grep -E '^[a-z][a-z-]*:.*?##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?##"} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

ci: check test build ## Mirror CI locally
