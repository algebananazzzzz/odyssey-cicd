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
| 5 | odyssey-cli: Go generator (Bubble Tea TUI + headless flags) | wizard poc |

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
| credentials | never a Terraform variable, nothing `sensitive`; providers read their own env vars. `TF_VAR_` carries identifiers only (amended session 3) |
| identifiers | org-shaped identifiers became GitHub variables in session 3; project-shaped values stay in committed tfvars |
| secrets | Cloudflare `CLOUDFLARE_API_TOKEN` + `R2_*` for state; AWS `AWS_ROLE_ARN` over OIDC |
| targets | `infra` `infra-plan` `infra-check`; `infra-init` takes the state bucket from `STATE_BUCKET` (amended session 3: no local runs, no `infra/.env`) |
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
| variables | no registry: discovery by scanning; see the variables table below |

## Session 3, settled — variables

| | |
|---|---|
| kinds | engine vars (`PROJECT`, `ENV`, `ENV_LIST`, `PREPROD_ENV`, `PRD_ENV`) may land anywhere; custom `{{VAR}}`s only in `infra/**/config/*` and workflow files; org-shaped values are GitHub variables, credentials are GitHub secrets |
| declarations | the manifest declares the wizard ask list (`inputs:`, with `optional:` meaning empty allowed) and the GitHub contract (`github.variables` / `github.secrets`, always required). The validator scans and proves declarations and usage agree both ways: an undeclared reference and a dead declaration both fail. Declarations live on the axis entry owning the fragment; base workflow vars at top level |
| substitution | one regex, `\{\{[A-Z][A-Z0-9_]*\}\}`, is discovery, substitution and the leftover check; uppercase-only, so it can never match a `${{ ... }}` GitHub expression. `text/template` rejected: needs `{{.VAR}}` and parses GitHub expressions |
| filenames | only `{{ENV}}` may appear in a file name |
| per-env | a custom var in a `{{ENV}}`-named file is asked once per environment, so prd and preprod tfvars diverge; the wizard defaults each later env to the first answer. Runtime values diverge per env through GitHub environment scoping |
| org values | `STATE_BUCKET`, `CLOUDFLARE_ACCOUNT_ID`, `ZONE_NAME`, `AWS_REGION` are GitHub variables fed at runtime: `AWS_REGION` and `AWS_ENDPOINT_URL_S3` read natively by provider and backend, bucket via `-backend-config`, `TF_VAR_` for module variables. Set once at org level; environment scope overrides where prd differs |
| tfvars | committed tfvars carry `env`, `project_code` and project-shaped architecture values only; both providers' base tfvars are identical |
| urls | `PREPROD_URL` / `PRD_URL` stay template inputs in the deploy workflows; a deploy-output URL pattern was rejected as not universal |
| optionality | `optional:` in the manifest marks an ask that accepts empty; env-specific stays placement-derived, never declared. A derivable value is committed derived (`worker_name = "{{ENV}}-web-worker-{{PROJECT}}"`), a fixed value is committed and edited after scaffold (`r2_location`), optional behavior lives in the terraform layer (`custom_domain` may be empty; `enable_custom_domain` validates against it) |
| no local runs | `.env.example`, the `infra/.env` include and its gitignore line are gone; CI is the only place `make infra` runs |
| setup checklist | the wizard prints the GitHub variables and secrets to configure from the selected fragments' `github:` blocks, which validation has already proven match the workflows (session 5) |

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
| wizard | hand-rolled Bubble Tea model, one question per screen: waterfall selects, architecture filtered by provider, stack by architecture, options computed synchronously per step; environment shapes listed from `workflows/deploy/`. `huh` rejected: grouped layout and racy async options. `odyssey-cli new` validates then runs it; selection only, render next |
| charm stack | `huh` forms on Bubble Tea, `lipgloss` styling |


## Open questions

- Does a major bump stay automatic, or get capped at minor?
- Should `2-ci-merge` also run `infra-plan` against preprod? It needs credentials
  in a PR run, so it would be limited to pull requests from this repo.
