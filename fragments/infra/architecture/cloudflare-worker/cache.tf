resource "cloudflare_r2_bucket" "cache" {
  account_id    = var.cloudflare_account_id
  name          = "${var.env}-web-cache-${var.project_code}"
  location      = var.r2_location
  storage_class = "Standard"
}
