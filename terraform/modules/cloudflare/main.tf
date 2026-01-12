data "cloudflare_zone" "zone" {
  name = var.cloudflare_zone_name
}

resource "cloudflare_record" "records" {
  for_each = var.cloudflare_records
  zone_id  = data.cloudflare_zone.zone.id
  name     = each.value.name
  content  = each.value.content
  type     = each.value.type
  ttl      = each.value.ttl
  proxied  = each.value.proxied
}