resource "cloudflare_pages_domain" "this" {
  count = var.enable_custom_domain ? 1 : 0

  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.this.name
  name         = var.custom_domain
}
