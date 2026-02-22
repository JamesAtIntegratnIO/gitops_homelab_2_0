terraform {
  required_version = ">= 1.6.0, < 2.0.0"

  backend "pg" {
    # Connection string should be provided via init-time config:
    # tofu init -backend-config=backend.hcl
    # See backend.hcl.example for template
  }

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "2.31.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "= 2.10.1"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "4.39.0"
    }
    onepassword = {
      source  = "1Password/onepassword"
      version = "~> 3.0.2"
    }
  }
}

provider "kubernetes" {
  config_path    = "../../matchbox/assets/talos/1.11.5/kubeconfig"
  config_context = join("@", ["admin", var.cluster_name])
}

provider "helm" {
  kubernetes {
    config_path    = "../../matchbox/assets/talos/1.11.5/kubeconfig"
    config_context = join("@", ["admin", var.cluster_name])
  }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_key
}


provider "onepassword" {
  url   = "https://connect.integratn.tech"
  token = var.onepassword_token
}