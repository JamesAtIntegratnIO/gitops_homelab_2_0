# ARGOCD

variable "cluster_name" {
  type = string
}

variable "gitops_addons_org" {
  type    = string
  default = "https://github.com/jamesatintegratnio"
}

variable "gitops_addons_repo" {
  type    = string
  default = "gitops-homelab"
}

variable "gitops_addons_basepath" {
  type    = string
  default = "gitops"
}

variable "gitops_addons_path" {
  type    = string
  default = "bootstrap/control-plane/addons"
}

variable "gitops_addons_revision" {
  type    = string
  default = "homelab"
}

variable "onepassword_token" {
  type      = string
  sensitive = true
}

# cloudflare

variable "cloudflare_api_key" {
  type      = string
  sensitive = true
}

variable "cloudflare_zone_name" {
  type = string
}

variable "cloudflare_records" {
  type = map(object({
    name    = string
    content = string
    type    = string
    ttl     = number
    proxied = bool
  }))
}