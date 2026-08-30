# Deploy: AWS ECS

Deploys a container to ECS on Fargate. `make deploy ENV=<env>` runs
`scripts/deploy-ecs.sh`: build the image, push it to that env's ECR
repository, register a new task definition revision pointing at it. The
revision ARN is the script's stdout; everything else goes to stderr.

## Contract

Two files live at the repo root:

- `Dockerfile` — the image, built from the repo root with no build args;
  the stack fragment ships it.
- `taskdef.json` — the `register-task-definition` input. Static choices
  (cpu/memory, `awsvpc`, `FARGATE`, the `awslogs` skeleton) are real values;
  env-scoped fields hold `INJECTED` and are overwritten by the script at
  deploy time: family, both role ARNs, the image, and the `awslogs`
  group/region/stream-prefix.

The script derives everything it injects from `ENV`, `PROJECT` and the
caller's AWS identity — nothing env-specific is committed. `TAG` defaults to
`git describe --tags --always`.

## Infra vs deploy

Terraform owns what the deploy cannot create: the ECR repository
(`{env}-app-ecr-{project}`), the cluster, the log group
(`{env}/{project}/app/ecs-logs`), and the IAM roles
(`{env}-app-iamrole-{project}-execution` / `-task`). The deploy creates only
task definition revisions; a Terraform-managed service or schedule references
the family and picks up new revisions on its own terms.

App configuration goes in `taskdef.json` — `environment` for plain values,
`secrets` with SSM parameter ARNs for credentials. Never in the Dockerfile.
