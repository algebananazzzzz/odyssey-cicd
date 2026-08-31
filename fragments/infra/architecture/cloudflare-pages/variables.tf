variable "enable_custom_domain" {
  description = "Attach the custom domain. Requires a production deployment to exist."
  type        = bool
  default     = false

  validation {
    condition     = !var.enable_custom_domain || var.custom_domain != ""
    error_message = "enable_custom_domain needs custom_domain set."
  }
}

variable "custom_domain" {
  description = "Hostname routed to this environment's Pages project. Empty when it has none."
  type        = string
}

variable "pages_project_name" {
  description = "Name of the Pages project the deploy tool uploads into."
  type        = string
}
