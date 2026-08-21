# Kratix promises

* [Kratix platform layer](kratix.md) - flow, public state repo, destination, pipeline conventions, status reconciler.
* [vcluster-orchestrator-v2](vcluster-orchestrator-v2.md) - composite: full tenant vcluster from one CR.
* [http-service](http-service.md) - composite: HTTP app product (Stakater chart + sub-requests).
* [argocd-application](argocd-application.md) - leaf: ArgoCD Application renderer.
* [argocd-project](argocd-project.md) - leaf: AppProject renderer.
* [argocd-cluster-registration](argocd-cluster-registration.md) - the kubeconfig → 1Password → ArgoCD credential loop.
* [gateway-route](gateway-route.md) - leaf: HTTPRoute + HTTP redirect renderer.
* [external-secret](external-secret.md) - leaf: 1Password-backed ExternalSecret renderer.
