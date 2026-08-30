#!/usr/bin/env bash
# Build the image, push it to this env's ECR repository and register a new
# task definition revision pointing at it. Prints the revision ARN; diagnostics
# go to stderr.
#
#   ENV      required. Environment being deployed.
#   PROJECT  required. Project code; picks the ECR repository.
#   TAG      optional. Image tag, default `git describe`.
#
# Expects taskdef.json at the repo root: the register-task-definition input,
# with the env-scoped fields (family, roles, image, log options) overwritten here.
set -euo pipefail

: "${ENV:?ENV is required}"
: "${PROJECT:?PROJECT is required}"
TAG="${TAG:-$(git describe --tags --always)}"

region="${AWS_REGION:-$(aws configure get region)}"
account="$(aws sts get-caller-identity --query Account --output text)"
registry="${account}.dkr.ecr.${region}.amazonaws.com"
image="${registry}/${ENV}-app-ecr-${PROJECT}:${TAG}"

aws ecr get-login-password --region "$region" |
  docker login --username AWS --password-stdin "$registry" >&2

docker build -t "$image" . >&2
docker push "$image" >&2

taskdef="$(mktemp)"
trap 'rm -f "$taskdef"' EXIT
jq --arg img "$image" \
   --arg family "${ENV}-app-ecstaskdefn-${PROJECT}" \
   --arg execution "arn:aws:iam::${account}:role/${ENV}-app-iamrole-${PROJECT}-execution" \
   --arg task "arn:aws:iam::${account}:role/${ENV}-app-iamrole-${PROJECT}-task" \
   --arg logs "${ENV}/${PROJECT}/app/ecs-logs" \
   --arg region "$region" '
  .family = $family
  | .executionRoleArn = $execution
  | .taskRoleArn = $task
  | .containerDefinitions[0].image = $img
  | .containerDefinitions[0].logConfiguration.options = {
      "awslogs-group": $logs,
      "awslogs-region": $region,
      "awslogs-stream-prefix": "app"
    }' taskdef.json > "$taskdef"

aws ecs register-task-definition --cli-input-json "file://$taskdef" \
  --query 'taskDefinition.taskDefinitionArn' --output text
