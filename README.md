# cicd-templates

Composable CI/CD boilerplate. A generator picks fragments and assembles a
project.

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
Makefile         × N  (stack × deploy × infra) -> concatenated text
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
  stacks/                       one per use case, named by what it builds
    nextjs|astro|node-puppeteer|go-service/
      .github/actions/setup/    toolchain composite action
      context.md                bootstrap + Makefile-contract wiring
      files/                    -> repo root (Dockerfile, when containerised)
  workflows/scripts/          -> .github/scripts/; workflow-called only
    semver-tag.sh               next tag from the commit or PR title
    promote-tag.sh              strip the pre-release suffix and push
  makefile/                     one folder per choice; appended in this order
    base.mk                     always included, first
    stack/<use case>/           setup / check / test / build
    deploy/<target>/            deploy; ships scripts/ when a recipe outgrows
                                3 lines (aws-ecs)
    infra/terraform/            infra / infra-plan / infra-check
  infra/
    .gitignore                  -> infra/;  context.md composes into AGENTS.md
    providers/{aws,cloudflare}/ backend.tf, providers.tf, variables.tf,
                                config/, .env.example
      workflows/                deploy.yml
    architecture/<target>/      the deploy target's resources, copied in;
                                variables.tf + config/ append to the provider's
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
| `infra` / `infra-plan` / `infra-check` | project has `infra/` Terraform |

Absent target means the generator drops the matching CI step. No runtime
guards, no no-op stubs.

Rules: a recipe is 1–3 lines, longer goes to `scripts/<name>.sh`. `ENV` is the
only universal variable. Never add a target purely for CI's benefit.

## Scripts

The caller is the divide, and the caller decides where a script lives:

| dir | called by | test |
|---|---|---|
| `scripts/` | a make target, and nothing else | you would run it on a laptop |
| `.github/scripts/` | a workflow step, and nothing else | pipeline mechanics with no local meaning |

Either way, more than a few lines of shell moves out of the recipe or step into
a script. A script reads its inputs from the environment and writes its result
to stdout, and diagnostics go to stderr to keep stdout parseable.

`semver-tag.sh` is the case that forced it — both pipeline shapes compute the
next version the same way and differ only in what they feed it:

| | `TITLE` | `BODY` | `SUFFIX` |
|---|---|---|---|
| `dual` | PR title | PR body | `-beta` |
| `single` | commit message | commit message | — |

Inputs arrive through `env:`, never `${{ }}` interpolation: a fork's PR title is
attacker-controlled, and interpolation splices it into the step as shell source.

## Pipeline shapes

### `dual` — promotion ladder

```
1-ci-branch      push to non-main    check, test
2-ci-merge       PR -> main          check, test, build
3-deploy-preprod PR merged /beta tag cut vX.Y.Z-beta, [infra], deploy preprod,
                                     cut vX.Y.Z
4-deploy-prd     dispatch / tag push validate tag, [infra], deploy prd
```

A `vX.Y.Z-beta` tag means "merged". A `vX.Y.Z` tag means "passed preprod".
Deploys always check out the tag, never a branch.

**The bump level comes from the PR title.** You squash-merge, so the title is
the commit message, and the workflow reads it Conventional Commits style:

| PR title | bump | `v1.4.7-beta` becomes |
|---|---|---|
| `feat!: drop the v1 API` | major | `v2.0.0-beta` |
| `fix!: rename every route` | major | `v2.0.0-beta` |
| *(body contains `BREAKING CHANGE`)* | major | `v2.0.0-beta` |
| `feat: add booking` | minor | `v1.5.0-beta` |
| `feat(cca): allocation preview` | minor | `v1.5.0-beta` |
| `fix: rounding off by one` | patch | `v1.4.8-beta` |
| `refactor cca table` | patch | `v1.4.8-beta` |

Unrecognised titles fall through to patch, so a sloppy title can never produce
a surprise major. The title reaches the script through `env:`, never through
`${{ }}` interpolation — a fork PR title is attacker-controlled and would
otherwise be spliced in as shell source.

The `single` shape applies the same rules to the squash-merge commit subject,
since a push event carries no pull_request payload.

`vX.Y.Z` says "passed preprod", not "send this to prd". `3-deploy-preprod` cuts
it and stops; shipping is a dispatch of `4-deploy-prd` against that tag.

That is also what the default `GITHUB_TOKEN` enforces: a ref pushed with it
starts no workflow run, so the `push: tags` trigger on `4-deploy-prd` fires only
for a tag a human pushed.

### `single` — one environment

```
1-ci-branch  push to non-main    check, test
2-ci-merge   PR -> main          check, test, build
3-deploy     push main /dispatch cut vX.Y.Z, [infra], deploy prd
```

Same gates as `dual`, no promotion ladder — the tag is cut and shipped in one
run. The deploy still checks out the tag, so a rollback target exists.

## The seam

A workflow file is generic. Everything specific to a stack, cloud or provider
enters through one of two seams, and nowhere else:

| seam | for |
|---|---|
| `.github/actions/setup` | installing the stack toolchain — one composite action per stack |
| a marker block | anything a workflow step cannot express as `make <target>` |

That is the whole contract, and it is checkable: **no workflow file contains the
string `terraform`, `wrangler`, `npm` or `npx`.** Every workflow is generic;
tools are named only inside fragments.

### Conditional blocks

The generator's only transformation beyond copying, besides `{{VAR}}`
substitution: **replace a marker's body with the selected fragment's, or delete
it when nothing is selected.**

```yaml
# >>> deploy
      - name: 🚀 Deploy
        run: make deploy ENV=prd
# <<< deploy
```

Every marker body is valid YAML as written, so each fragment file parses on its
own and the default is a working pipeline — an empty `infra` marker is simply a
project with no infrastructure.

| marker | default body | filled from |
|---|---|---|
| `infra` | empty — no infra is the default | `workflows/ci/infra.yml` in `2-ci-merge`, `infra/providers/<cloud>/workflows/deploy.yml` in a deploy job |
| `deploy` | `make deploy ENV=<env>` | a deploy fragment that cannot be a CLI call, if one ever exists |

A marker's body is **replaced**, never merged, so a `make` call belongs to
exactly one of the two — the workflow's default or the fragment. `infra` puts it
in the fragment, because the fragment must run Terraform's install and the
cloud's authentication first anyway. `deploy` puts it in the workflow, because
nearly every provider stops there.

No provider overrides `deploy` today — `make deploy` is the deploy, and that is
the point. The marker exists so that a provider published by GitHub itself,
rather than by a CLI a Makefile could call, can supply its own steps without
touching a workflow file. Until one does, every project gets the default.

### Why the infra check is not path-filtered

`infra-check` needs no credentials and takes about fifteen seconds, and GitHub
filters paths per workflow rather than per job. Skipping it on pull requests
that leave `infra/` alone therefore costs either a second workflow file — which
reports no status when skipped, so it cannot be a required check without
blocking merges — or a changed-files job that burns a runner to decide whether
to burn a runner. Neither is worth fifteen seconds. It runs on every pull
request, as one job in the same gate as everything else.

## Variables

| token | example |
|---|---|
| `{{PROJECT}}` | `intranet-web` |
| `{{PREPROD_URL}}` | `https://beta.example.com` |
| `{{PRD_URL}}` | `https://example.com` |
| `{{ENV}}` | `preprod` — also expands in filenames, once per environment |
| `{{ENV_LIST}}` | `["preprod", "prd"]` |
| `{{STATE_BUCKET}}` | `com-infra-tfstate` |
| `{{CLOUDFLARE_ACCOUNT_ID}}`, `{{ZONE_NAME}}` | Cloudflare only |
| `{{AWS_REGION}}` | AWS only |
| `{{PREPROD_ENV}}`, `{{PRD_ENV}}` | `preprod` / `prd` — the GitHub environment holding that env's secrets |

`fragments/infra/` composes by copying `.gitignore` into `infra/`, then the
chosen cloud directory. Nothing cloud-agnostic sits in a `.tf` file: the backend
type follows the cloud, so `backend.tf` lives with it.

## Conventions this repo fixes

| decision | value |
|---|---|
| pre-release suffix | `-beta` |
| workflow extension | `.yml` |
| workflow naming | `<n>-<kind>-<scope>.yml` — `ci-*` and `deploy-*` group in the sidebar |
| branch CI trigger | `push: branches-ignore: [main]` — fires before a PR exists |
| `check` vs `test` | static vs executing, so a failed formatter costs no test run |
| prd ship | deliberate — a dispatch against the `vX.Y.Z` tag |
| deploy job permissions | static `contents: read` + `id-token: write` |
| Terraform state key | `<env>/<project>/terraform.tfstate` |
| resource naming | `{env}-{layer}-{resource}-{project}` |
