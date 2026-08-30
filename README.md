# cicd-templates

Composable CI/CD boilerplate. A generator picks fragments and assembles a
project; an agent can do the same thing headlessly.

## The one idea

**Workflows never name a tool. They only call `make`.**

That single indirection is what makes composition tractable. Because every
workflow step is `make check` / `make test` / `make deploy ENV=prd`, the
workflow files are *identical across every project* and get copied verbatim.
All stack variation collapses into a flat Makefile assembled at generate time.

No YAML merging. No templating engine. No per-combination file explosion.

```
CI workflows     × 1  (shared)                            -> copied verbatim
deploy workflows × 2  (single, dual)                      -> copied verbatim
Makefile         × N  (stack × deploy × infra × services) -> concatenated text
setup action     × 1 per stack                            -> copied verbatim
```

Both shapes gate the same way — `1-ci-branch` then `2-ci-merge`. They differ
only in what happens after the merge lands.

## Layout

```
fragments/
  workflows/
    ci/            1-ci-branch.yml  2-ci-merge.yml     shared by both shapes
    deploy/dual/   3-deploy-preprod.yml  4-deploy-prd.yml
    deploy/single/ 3-deploy.yml
  stacks/
    node|go|python/.github/actions/setup/action.yml
  makefile/
    base.mk                     always included
    stack-{node,go,python}.mk   setup / check / test / build
    deploy-*.mk                 deploy
    infra-terraform.mk          infra / infra-plan
    services-compose.mk         up / down / test-integration
examples/
  next-cloudflare-dual/         a fully rendered result
```

## The Makefile contract

Fixed names, fixed meanings. CI calls these and nothing else.

| target | meaning | must not |
|---|---|---|
| `setup` | install deps + toolchain | touch the network at any other stage |
| `check` | static only — format, lint, types | run application code |
| `test` | unit + component | need a network service |
| `build` | produce the artifact | deploy |
| `deploy` | ship the artifact to `ENV` | build differently than `build` |
| `help` | list targets (default goal) | — |
| `ci` | local mirror of the pipeline | — |

Optional, present only when the matching fragment is selected:

| target | when |
|---|---|
| `test-integration` | project has a service dependency |
| `up` / `down` | that dependency runs locally in Docker |
| `infra` / `infra-plan` | project has `infra/` Terraform |

Absent target means the generator drops the matching CI step. No runtime
guards, no no-op stubs.

Rules: a recipe is 1–3 lines, longer goes to `scripts/<name>.sh`. `ENV` is the
only universal variable. Never add a target purely for CI's benefit.

## Pipeline shapes

### `dual` — promotion ladder

```
1-ci-branch      push to non-main    check, test
2-ci-merge       PR -> main          check, test, build, [integration]
3-deploy-preprod PR merged / rc tag  cut vX.Y.Z-rc, [infra], deploy preprod,
                                     cut vX.Y.Z, dispatch prd
4-deploy-prd     dispatch / tag push validate tag, [infra], deploy prd
```

A `vX.Y.Z-rc` tag means "merged". A `vX.Y.Z` tag means "passed preprod".
Deploys always check out the tag, never a branch.

**The GITHUB_TOKEN trap.** A ref pushed with the default `GITHUB_TOKEN` does
not start a workflow run — `workflow_dispatch` and `repository_dispatch` are
the only exceptions. So `3-deploy-preprod` pushes the production tag *and*
dispatches `4-deploy-prd` explicitly. A pipeline that pushes the tag and waits
ships nothing, silently.

### `single` — one environment

```
1-ci-branch  push to non-main    check, test
2-ci-merge   PR -> main          check, test, build, [integration]
3-deploy     push main /dispatch cut vX.Y.Z, [infra], deploy prd
```

Same gates as `dual`, no promotion ladder — the tag is cut and shipped in one
run. The deploy still checks out the tag, so a rollback target exists.

## Conditional blocks

The generator's only transformation beyond copying, besides `{{VAR}}`
substitution: delete marker-delimited blocks for unselected options.

```yaml
# >>> infra
- run: make infra ENV=prd
# <<< infra
```

Blocks are valid YAML in place, so every fragment file is testable as-is.

Current markers: `infra`, `integration`.

## Variables

| token | example |
|---|---|
| `{{PROJECT}}` | `intranet-web` |
| `{{PREPROD_URL}}` | `https://beta.example.com` |
| `{{PRD_URL}}` | `https://example.com` |

## Conventions this repo fixes

| decision | value |
|---|---|
| pre-release suffix | `-rc` |
| workflow extension | `.yml` |
| workflow naming | `<n>-<kind>-<scope>.yml` — `ci-*` and `deploy-*` group in the sidebar |
| branch CI trigger | `push: branches-ignore: [main]` — fires before a PR exists |
| `check` vs `test` | static vs executing, so a failed formatter costs no test run |
| prd ship | automatic on green preprod, via dispatch |
| Terraform state key | `<env>/<project>/terraform.tfstate` |
| resource naming | `{env}-{layer}-{resource}-{project}` |
