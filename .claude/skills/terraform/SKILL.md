---
name: terraform-conventions
description: Use when writing, editing, or reviewing Terraform in this repo — any .tf, .tfvars, or .tfbackend under fragments/infra, or a rendered infra/ module
---

# Terraform Conventions

## Overview

One flat module, assembled from two fragment layers: `providers/<cloud>` (backend, provider, base variables) plus `architecture/<deploy-target>` (the resources that target needs). `fragments/infra/context.md` is the design document; this skill is the style contract for the `.tf` code itself.

## Resource labels

- Only resource of its type, and no name more descriptive than the type itself → label it `this`: `aws_ecs_cluster.this`, `data.cloudflare_zone.this`.
- A descriptive label wins when it carries meaning the type doesn't: `cloudflare_r2_bucket.cache` (its purpose), `aws_iam_role.execution` / `aws_iam_role.task` (two of a type).
- Never repeat the type in the label: `aws_ecr_repository.this`, not `.ecr_repo`.

## Cloud resource names

`{env}-{layer}-{resource}-{project}`, written `"${var.env}-web-cache-${var.project_code}"`. `layer` is the tier served: `web`, `api`, `data`, `ops`. Log groups use paths: `"${var.env}/${var.project_code}/app/ecs-logs"`.

## File layout

- Subject files (`cache.tf`, `ecr.tf`, `iam.tf`) hold resources and data sources only.
- Every `variable` block lives in `variables.tf`. An architecture's `variables.tf` and `config/<env>.tfvars` are appended to the provider's at generate time — never edit a provider file from an architecture.
- No `main.tf`, `locals.tf`, `outputs.tf` until something needs them.
- No comments except a non-obvious constraint the code cannot show (e.g. R2's `skip_*` backend flags).

## Variables

- lower_case names, `description` on every one, `validation` when the value set is finite.
- A credential is never a variable: no `sensitive`, no `TF_VAR_`. Providers and backend read their own environment variables.
- Non-secret identifiers (account id, zone, region) go in committed `config/<env>.tfvars`.
- A default only for a true knob (`log_retention_days = 7`); everything env-shaped comes from tfvars.
- A resource that can only exist after the first deploy sits behind `enable_*` bool defaulting `false`.

## Verify

Render provider + architecture into a scratch dir (copy subject files, concatenate `variables.tf` and tfvars), then `terraform fmt -check` and `terraform init -backend=false && terraform validate`.
