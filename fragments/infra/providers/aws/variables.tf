variable "env" {
  description = "Environment this apply targets. First segment of every resource name."
  type        = string

  validation {
    condition     = contains({{ENV_LIST}}, var.env)
    error_message = "env is not one of the environments this project defines."
  }
}

variable "project_code" {
  description = "Project code. Last segment of every resource name, and the middle segment of the state key."
  type        = string
}
