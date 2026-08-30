# Roadmap

## The idea

CI workflows never name a tool, they only call `make`. So workflow files are
identical in every project and get copied verbatim, while all per-project
variation collapses into a flat Makefile assembled from text fragments. A
generator picks fragments, copies, concatenates, substitutes a few variables.

Scaffold only. Nothing updates a project after creation.

## Sessions

| # | scope | state |
|---|---|---|
| 1 | Workflow catalog + Makefile contract | done |
| 2 | Makefiles + infra files, Terraform tied to the Makefile | infra done |
| 3 | Fragments: manifest schema, how they link and validate | drafted |
| 4 | Stacks: use-case fragments (nextjs, astro, node-puppeteer, go-service) | poc |
| 5 | odyssey-cli: Go generator (Bubble Tea TUI + headless flags) | ideation |

## Session 1, settled

| | |
|---|---|
| composition | workflows copied verbatim, Makefile concatenated, marker blocks stripped |
| makefile layout | `makefile/<axis>/<choice>/main.mk` (+ optional `scripts/`); append order base, stack, deploy, infra; each axis owns its contract targets |
| Makefile contract | `setup check test build deploy help ci`, optional `infra` targets |
| shapes | `single` (3 files), `dual` (4 files), CI files shared between them |
| naming | `1-ci-branch` `2-ci-merge` `3-deploy-preprod` `4-deploy-prd` |
| versioning | `-beta` ladder, bump level from the PR title, promote by stripping the suffix |
| prd trigger | preprod cuts `vX.Y.Z` and stops; shipping is a dispatch against that tag |
| wizard order | environments, provider, architecture, stack — waterfall, each answer filters the next |
| docs | AGENTS.md canonical, composed from per-fragment `context.md`; CLAUDE.md points at it |

## Session 2, settled — infra

| | |
|---|---|
| module | one flat `infra/`, parameterized by `var.env`, applied once per env |
| files | `backend.tf` `providers.tf` `variables.tf` `config/<env>.{tfbackend,tfvars}` |
| no placeholders | no `main.tf`, `locals.tf` or `outputs.tf` until something needs them |
| clouds | `cloudflare` and `aws`; a swap replaces `providers.tf`, `variables.tf`, `config/` |
| layers | `providers/<cloud>` base + `architecture/<deploy-target>` resources copied in beside it |
| style | `.claude/skills/terraform` — resource labels (`this` rule), variables, file layout |
| architecture | subject `.tf` files copied in; `variables.tf` and `config/<env>.tfvars` append to the provider's |
| backend | `s3` for both — R2 is reached through it with `endpoints` + `skip_*` |
| state key | `<env>/<project_code>/terraform.tfstate` |
| variables | `env`, `project_code`, plus that cloud's; all lower-case |
| credentials | never a Terraform variable; providers read their own env vars, no `TF_VAR_`, nothing `sensitive` |
| identifiers | account id, zone, region live in the committed tfvars |
| secrets | Cloudflare `CLOUDFLARE_API_TOKEN` + `R2_*` for state; AWS `AWS_ROLE_ARN` over OIDC |
| targets | `infra` `infra-plan` `infra-check`, plus `infra/.env` included by the Makefile |
| validation | `infra-check` needs no credentials, so merge CI runs it on every PR, forks included |
| apply | inline in the deploy job, before the app deploy — the binding is a hard dependency |
| workflow | `# >>> infra` is replaced by the cloud's `workflows/deploy.yml` |
| infra CI | a job in `2-ci-merge` behind the `infra` marker; not path-filtered, since the check is cheaper than the machinery to skip it |

## Session 2, settled — the seam

| | |
|---|---|
| rule | a workflow step is `actions/checkout`, `uses: ./.github/actions/<x>`, `run: make <target>`, or `run: .github/scripts/<name>.sh` |
| seams | `.github/actions/setup` for the stack toolchain; marker blocks for everything else |
| markers | `infra` `deploy` |
| checkable | no workflow file contains `terraform`, `wrangler`, `npm` or `npx` |
| marker semantics | replace the body with the selected fragment's, or delete it when nothing is selected |
| no duplication | a `make` call lives in the workflow default or in the fragment, never both |
| defaults | every marker body is valid YAML as written, so the un-generated fragment is a working pipeline |
| `deploy` | defaults to `make deploy`; the marker exists for a provider that cannot be a CLI call |
| permissions | static on the deploy job: `contents: read` + `id-token: write`, no marker |
| git identity | never configured: tags are lightweight refs, which carry no tagger |
| env name | `{{PREPROD_ENV}}` / `{{PRD_ENV}}`, so the environment holding the secrets is nameable |
| scripts | the caller decides the home: make-called in `scripts/`, workflow-called in `.github/scripts/`; never both. Inputs from env, result on stdout, diagnostics on stderr |
| `semver-tag.sh` | one script for both shapes, parameterised by `TITLE` / `BODY` / `SUFFIX` |

Proven by rendering every workflow across cloudflare, aws and no-infra and
parsing each result, asserting no `make infra` is duplicated — plus a no-infra
pass showing no Terraform or AWS residue survives marker deletion.

## Session 3, settled so far — manifest

| | |
|---|---|
| principle | convention over configuration: the layout is the contract, the engine knows what every path means |
| manifest | declares only what exists and what fits together: environments, providers, architectures (each names its provider), stacks (each lists its architectures) |
| conventions | `workflows/ci/` always copied; `workflows/deploy/<environments>/` by choice; `workflows/scripts/` → `.github/scripts/`; `makefile/base.mk` + selected `<axis>/<choice>/main.mk` appended in order, their `scripts/` → `scripts/`, `files/` → repo root; `infra/providers/<p>/` → `infra/` with `workflows/deploy.yml` as the deploy `infra` marker body and `workflows/ci/infra.yml` as the ci one; `infra/architecture/<a>/` `.tf` copied, `variables.tf` + `config/<env>.tfvars` appended; `stacks/<s>/.github/` → `.github/`, `stacks/<s>/files/` → repo root; selected `context.md`s compose AGENTS.md |
| variables | no registry: the engine scans written files for `{{VAR}}`s and asks by name; `ENV`/`ENV_LIST` derived; a `{{VAR}}` in a filename expands per environment |

## Session 5, settled so far — odyssey-cli

| | |
|---|---|
| name | the framework is **odyssey**; the binary is `odyssey-cli`, in `cmd/odyssey-cli` here |
| engine | generic: walks the fragment manifest, renders forms from it; a new pattern is a fragment dir + manifest entry, never a code change |
| templates source | tarball of this repo's `main`, fetched and cached (`~/.cache/odyssey/`); `--templates <dir>` overrides for local dev |
| release channel | `main` — so this repo's CI validating fragments is load-bearing |
| flow | waterfall wizard (environments, provider, architecture, stack) → rendered-plan preview → confirm → write |
| headless | flags mirror the wizard; `--yes` skips the preview |
| bootstrap | a stack fragment may declare a bootstrap section (commands + intent). `--bootstrap` runs it; the default prints it as a continuation prompt to take elsewhere |
| charm stack | `huh` forms on Bubble Tea, `lipgloss` styling |


## Open questions

- Does a major bump stay automatic, or get capped at minor?
- Should `2-ci-merge` also run `infra-plan` against preprod? It needs credentials
  in a PR run, so it would be limited to pull requests from this repo.
