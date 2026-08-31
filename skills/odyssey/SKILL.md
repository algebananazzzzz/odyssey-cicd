---
name: odyssey
description: Use when a user asks to set up CI/CD, GitHub Actions pipelines, deploy workflows, or release automation for a project, or mentions odyssey-cli, odyssey templates, or a make-based pipeline contract.
---

# Odyssey CLI

odyssey-cli scaffolds composable CI/CD into a new directory: GitHub Actions workflows that only ever call `make`, one flat assembled Makefile, optional Terraform under `infra/`, and semver tags cut from squash-merge PR titles.

## Setup

```bash
command -v odyssey-cli || curl -fsSL https://raw.githubusercontent.com/algebananazzzzz/odyssey-cicd/main/install.sh | sh
```

The templates are embedded in the binary; `find` and `new` need no other files. Pass `--templates <path>` only to use a modified checkout of the templates repo. `odyssey-cli update` self-updates the binary.

## Discover: find

```bash
odyssey-cli find
```

Prints a STACK / ARCHITECTURE / PROVIDER table (nextjs on cloudflare-worker, astro on cloudflare-pages, go-service and node-puppeteer on aws-ecs). Filter with substring terms (`find nextjs`) or exact axis matches (`find provider=aws`; axes are stack, architecture, provider). When exactly one row matches, it prints a card instead: environment shapes, required and optional inputs (`?` marks optional, `(per env)` means it can differ per environment), the GitHub variables and secrets the provider needs, and a ready-to-run `new` command. No match prints `no rows match` and exits 1.

## Generate: new

In a terminal `new` opens a wizard; without a TTY (how an agent runs it) it is headless and driven entirely by flags.

```bash
odyssey-cli new --stack nextjs --environments dual --project my-web --yes
```

| flag | meaning |
|---|---|
| `--stack`, `--architecture`, `--provider` | any subset; the rest is derived when unambiguous (nextjs implies cloudflare-worker implies cloudflare) |
| `--environments` | `dual` or `single` |
| `--project` | lowercase letters, digits, hyphens; starts with a letter |
| `--dir` | target directory, default `./<project>`; must not exist non-empty |
| `--var NAME=VALUE` | answer an input for all environments; repeatable |
| `--var env:NAME=VALUE` | override one environment, e.g. `--var prd:CUSTOM_DOMAIN=example.com` |
| `--yes` | apply; without it the plan is printed and nothing is written |
| `--bootstrap` | run the stack's bootstrap commands inside the new directory after apply |

With answers missing, `new` prints what it has (derived values tagged), the missing flags with their valid options, and the `--var` inputs split into required and optional, then exits 2. Rerun with the flags added. Optional inputs default to empty when omitted. Contradictions fail fast, e.g. `stack "nextjs" has no architecture on provider "aws"`.

Decide answers from the user's repo: `next` in package.json means `--stack nextjs`, astro means `astro`, a Go service means `go-service`, a puppeteer worker means `node-puppeteer`. Pick `dual` when the user wants a preprod stage before prd, `single` for one environment. Ask the user instead of guessing for: the project name, the environments shape when they have not implied one, optional inputs like CUSTOM_DOMAIN, and the stack or architecture when the repo is ambiguous.

## The generated contract

Workflows never name a tool; they only run `make`. The Makefile targets are fixed:

| target | meaning | called by |
|---|---|---|
| `setup` | install deps + toolchain | every workflow, via `.github/actions/setup` |
| `check` | static only: format, lint, types | 1-ci-branch, 2-ci-merge |
| `test` | unit + component | 1-ci-branch, 2-ci-merge |
| `build` | produce the artifact | 2-ci-merge |
| `deploy` | ship to `ENV`, e.g. `make deploy ENV=prd` | deploy workflows |
| `infra` / `infra-check` | Terraform apply / static check, only when `infra/` exists | deploy workflows / 2-ci-merge |
| `ci` | local mirror: check test build | you, locally |

`dual` generates 1-ci-branch, 2-ci-merge, 3-deploy-preprod, 4-deploy-prd: merging to main cuts `vX.Y.Z-beta`, deploys preprod, then cuts `vX.Y.Z`; shipping prd is a deliberate dispatch of 4-deploy-prd against that tag. `single` generates 3-deploy instead: push to main cuts `vX.Y.Z` and deploys prd in one run.

The bump comes from the squash-merge PR title, Conventional Commits style: `feat!:`, `fix!:` or a `BREAKING CHANGE` body means major, `feat:` means minor, anything else falls through to patch. Tell the user their PR titles now drive versioning.

## After generating

1. Read the generated `AGENTS.md`: it carries the stack bootstrap commands (when not run via `--bootstrap`) and how to keep code fitting the Makefile contract.
2. Configure GitHub before the first deploy. The apply output prints the exact commands per provider, e.g. `gh variable set STATE_BUCKET --body <value>` and `gh secret set AWS_ROLE_ARN`; use environment scope where prd differs.
3. Keep the contract: extend the fixed targets, never add targets purely for CI, keep recipes to 1-3 lines with longer logic in `scripts/`.

`odyssey-cli validate --templates <path>` checks a templates checkout (manifest plus fragments), not a generated project; verify a generated project with `make help` and `make ci`.
