# Infrastructure

One flat Terraform module in `infra/`, parameterized by `var.env`, applied once
per environment against its own state key. No workspaces, no `modules/`, no
per-environment directories.

The module is assembled from two layers:

- **provider** (`providers/<cloud>/`) — the cloud itself: `backend.tf`,
  `providers.tf`, `variables.tf`, `config/`. One concern per
  file. A cloud swap replaces this whole layer and `config/` with it — the
  backend type is a property of the cloud, not something shared above it.
- **architecture** (`architecture/<deploy-target>/`) — the resources the deploy
  target needs but cannot create for itself, named after the deploy target it
  serves (`cloudflare-worker`, `aws-ecs`). Its subject `.tf` files are copied in
  beside the provider's; its `variables.tf` and `config/<env>.tfvars` are
  appended to the provider's files of the same name.

```
infra/
├── backend.tf                required_version + partial backend
├── providers.tf              required_providers + provider block
├── variables.tf              env, project_code + this cloud's variables
├── <subject>.tf              the architecture's resources
├── .gitignore
└── config/
    ├── <env>.tfbackend       state key
    └── <env>.tfvars          provider values + architecture values
```

The append is plain concatenation, the same mechanism as the Makefile: the
provider declares `env`, `project_code` and its cloud's identifiers, and the
architecture's `variables.tf` adds its own below them. Nothing is ever edited
into a base file.

There is no `main.tf`, `locals.tf` or `outputs.tf` until something needs them.
Resources go in files named by subject — `dns.tf`, `storage.tf` — not in one
catch-all. Resources the app accumulates later — a table, a queue — follow the
same pattern in the generated repo; fragments carry only what the deploy target
requires.

## Naming

Resources: `{env}-{layer}-{resource}-{project}`, written as
`"${var.env}-web-cache-${var.project_code}"`. layer is the tier the resource serves —
web, api, data, ops.

## State

Key: `<env>/<project>/terraform.tfstate`, in a bucket shared across projects.

The `s3` backend is partial. `config/<env>.tfbackend` commits what is
project-shaped: the key, plus each flavor's flags (R2 sets `region = "auto"`
and the `skip_*` flags; S3 sets `encrypt` and `use_lockfile`). What is
org-shaped arrives at init time from GitHub variables: `make infra` passes
`bucket` from `STATE_BUCKET` via `-backend-config`, and the backend reads the
region and the R2 endpoint from `AWS_REGION` / `AWS_ENDPOINT_URL_S3` in the job
environment. Credentials appear in neither place.

## Credentials

One rule: **a credential is never a Terraform variable.** Providers read their
own environment variables, so the provider block takes no credential arguments
and nothing in the module is marked `sensitive`. `TF_VAR_` carries identifiers
only, never anything that authenticates.

Org-shaped identifiers (account id, zone, region, state bucket) are GitHub
**variables**, fed to Terraform at runtime by the deploy workflow: `AWS_REGION`
through the provider's own env lookup, `TF_VAR_cloudflare_account_id` and
`TF_VAR_cloudflare_zone` for module variables, `STATE_BUCKET` through
`-backend-config`. Project-shaped values stay in the committed
`config/<env>.tfvars`.

| Cloud | Secrets | Variables |
|---|---|---|
| Cloudflare | `CLOUDFLARE_API_TOKEN`; `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` for state | `CLOUDFLARE_ACCOUNT_ID`, `ZONE_NAME`, `STATE_BUCKET` |
| AWS | `AWS_ROLE_ARN`, assumed over OIDC; provider and backend share the chain | `AWS_REGION`, `STATE_BUCKET` |

Set secrets per GitHub **environment**, not per repository, so a preprod
credential cannot reach a prd apply. Variables are org-shaped: set them once at
org or repo level, and scope one to an environment only when prd differs from
preprod. There is no local path: CI is the only place `make infra` runs.

The `s3` backend reads no names but `AWS_*`, so R2 state credentials are stored
under their own names and mapped onto `AWS_*` at the step that needs them.

## What Terraform owns

Terraform owns what the deploy tool cannot create for itself: buckets, domains, queues, roles,
DNS. Not the Worker script or the function package — the deploy tool creates
those, so reference them by name.

A resource that can only exist after the first deploy goes behind a bool
variable defaulting to false. Deploy once, flip it in the tfvars, apply again.

## Commands

```bash
make infra-check               # fmt -check + validate; no credentials, no state
make infra ENV=<env>           # init + apply -auto-approve
```

`init` always passes `-reconfigure`, because the backend key changes with ENV.
`infra-check` only reports bad formatting; run `terraform -chdir=infra fmt` to
fix it.

`2-ci-merge` runs `infra-check` on every pull request, as its own job. It needs
no secrets, so it runs on forks too.

Deploys run `make infra ENV=<env>` inside the deploy job, before the app deploy,
so anything the deploy tool binds to already exists.

## Committed vs not

Committed: `config/*.tfvars`, `config/*.tfbackend`, `.terraform.lock.hcl`.
Ignored: state, plans, `.terraform/`.
