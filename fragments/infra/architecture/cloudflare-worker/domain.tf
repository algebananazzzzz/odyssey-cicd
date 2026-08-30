data "cloudflare_zone" "this" {
  filter = {
    name = var.cloudflare_zone
  }
}

resource "cloudflare_workers_custom_domain" "this" {
  count = var.enable_custom_domain ? 1 : 0

  account_id = var.cloudflare_account_id
  zone_id    = data.cloudflare_zone.this.id
  hostname   = var.custom_domain
  service    = var.worker_name
}
