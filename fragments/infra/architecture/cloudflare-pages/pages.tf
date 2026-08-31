resource "cloudflare_pages_project" "this" {
  account_id        = var.cloudflare_account_id
  name              = var.pages_project_name
  production_branch = "main"
}
