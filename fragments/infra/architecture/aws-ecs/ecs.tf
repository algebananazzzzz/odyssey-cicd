resource "aws_ecs_cluster" "this" {
  name = "${var.env}-app-ecscluster-${var.project_code}"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name = aws_ecs_cluster.this.name

  capacity_providers = ["FARGATE"]

  default_capacity_provider_strategy {
    base              = 1
    weight            = 100
    capacity_provider = "FARGATE"
  }
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "${var.env}/${var.project_code}/app/ecs-logs"
  retention_in_days = var.log_retention_days
}
