# Roadmap

## The idea

CI workflows never name a tool, they only call `make`. So workflow files are
identical in every project and get copied verbatim, while all per-project
variation collapses into a flat Makefile assembled from text fragments. A
generator picks fragments, copies, concatenates, substitutes a few variables.

Scaffold only. Once generated, the repo is handed off to an agent, which builds
the app. Nothing updates a project after creation.

## Sessions

| # | scope | state |
|---|---|---|
| 1 | Workflow catalog + Makefile contract | done |
| 2 | Makefiles + infra files, Terraform tied to the Makefile | next |
| 3 | Fragments: manifest schema, how they link and validate | |
| 4 | Stack boilerplate (minimal green scaffold per stack) | |
| 5 | Go generator (Bubble Tea TUI + headless flags) | |

## Session 1, settled

| | |
|---|---|
| composition | workflows copied verbatim, Makefile concatenated, marker blocks stripped |
| Makefile contract | `setup check test build deploy help ci`, optional `test-integration up down infra` |
| shapes | `single` (3 files), `dual` (4 files), CI files shared between them |
| naming | `1-ci-branch` `2-ci-merge` `3-deploy-preprod` `4-deploy-prd` |
| versioning | `-beta` ladder, bump level from the PR title, promote by stripping the suffix |
| prd trigger | auto-ship, dispatched from preprod (a GITHUB_TOKEN tag push starts no run) |
| wizard order | envs, stack, deploy, infra, services |
| docs | AGENTS.md canonical, composed from per-fragment `context.md`; CLAUDE.md points at it |

## Session 2 agenda

- Terraform file set: `terraform.tf`, `providers.tf`, `variables.tf`, `locals.tf`,
  `outputs.tf`, `config/{env}.tfbackend`, `config/{env}.tfvars`
- How `make infra ENV=` drives init and apply, and what `infra-plan` is for
- State backend: R2, key convention `<env>/<project>/terraform.tfstate`
- Resource naming convention `{env}-{layer}-{resource}-{project}`
- Which credentials come from environment vs `-backend-config`, and the secret names
- Single vs dual: how many `config/` files get written
- Deploy fragments finalised: cloudflare-worker, cloudflare-pages, container
- `scripts/` conventions, and when a recipe moves out of the Makefile

## Open questions

- Does a major bump stay automatic, or get capped at minor?
- Terraform apply inline in the deploy job, or its own job behind an environment approval?
