locals {
  gitops_addons_url      = "${var.gitops_addons_org}/${var.gitops_addons_repo}"
  gitops_addons_basepath = var.gitops_addons_basepath
  gitops_addons_path     = var.gitops_addons_path
  gitops_addons_revision = var.gitops_addons_revision


  addons_metadata = merge(
    {
      addons_repo_url      = local.gitops_addons_url
      addons_repo_basepath = local.gitops_addons_basepath
      addons_repo_path     = local.gitops_addons_path
      addons_repo_revision = local.gitops_addons_revision
      managed-by           = "argocd.argoproj.io"
    },
    {
      external_dns_namespace                    = "external-dns"
      cert_manager_namespace                    = "cert-manager"
      nfs_subdir_external_provisioner_namespace = "nfs-provisioner"

    }
  )

  addons = {
    enable_argocd                          = true
    enable_argocd_image_updater            = true
    enable_ingress_nginx                   = true
    enable_metallb                         = true
    enable_external_dns                    = true
    enable_cert_manager                    = true
    enable_nfs_subdir_external_provisioner = true
    enable_stakater_reloader               = true
    enable_kube_prometheus_stack           = true
    enable_cluster_api_operator            = true
    enable_external_secrets                = true
    enable_headlamp                        = true
    enable_gateway_api_crds                = true
    enable_nginx_gateway_fabric            = true
  }

}


module "cloudflare" {
  source = "../modules/cloudflare"

  cloudflare_zone_name = var.cloudflare_zone_name
  cloudflare_records   = var.cloudflare_records
}

module "argocd" {
  source = "git::https://github.com/jamesAtIntegratnIO/terraform-helm-gitops-bridge.git?ref=homelab"

  argocd = {
    chart_version = "9.0.3"
  }

  cluster = {
    cluster_name = var.cluster_name
    environment  = "production"
    metadata     = local.addons_metadata

    addons = local.addons
  }
  apps = {
    addons = file("${path.module}/bootstrap/addons.yaml")
  }

}

resource "kubernetes_secret" "docker-config" {
  metadata {
    name      = "ghcr-login-secret"
    namespace = "argocd"
  }

  data = {
    ".dockerconfigjson" = "${file("${path.module}/dockerconfig.json")}"
  }

  type = "kubernetes.io/dockerconfigjson"

  depends_on = [module.argocd]
}