variable "r2_location" {
  description = "R2 location hint."
  type        = string

  validation {
    condition     = contains(["apac", "eeur", "enam", "weur", "wnam", "oc"], var.r2_location)
    error_message = "r2_location is not a location R2 offers."
  }
}

variable "enable_custom_domain" {
  description = "Attach the custom domain. Requires the Worker to be deployed."
  type        = bool
  default     = false

  validation {
    condition     = !var.enable_custom_domain || var.custom_domain != ""
    error_message = "enable_custom_domain needs custom_domain set."
  }
}

variable "custom_domain" {
  description = "Hostname routed to this environment's Worker. Empty when the Worker has none."
  type        = string
}

variable "worker_name" {
  description = "Name of the Worker the deploy tool creates."
  type        = string
}
