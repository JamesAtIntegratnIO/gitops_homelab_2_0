variable "cloudflare_zone_name" {
  description = "The name of the Cloudflare zone"
  type        = string
}

variable "cloudflare_records" {
  description = "A map of Cloudflare records to create"
  type = map(object({
    name    = string
    content   = string
    type    = string
    ttl     = number
    proxied = bool
  }))
}