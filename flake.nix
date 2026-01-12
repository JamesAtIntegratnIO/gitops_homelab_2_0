{
  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    utils = {
      url = "github:numtide/flake-utils";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (
      system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          scaleDownNamespace = pkgs.writeShellScriptBin "scale-down-namespace" ''
            #!/usr/bin/env bash
            set -euo pipefail

            if [[ $# -ne 1 ]]; then
              echo "Usage: scale-down-namespace <namespace>" >&2
              exit 1
            fi

            ns="$1"

            # Validate namespace exists
            if ! kubectl get ns "$ns" &>/dev/null; then
              echo "❌ Namespace '$ns' does not exist." >&2
              exit 1
            fi

            echo "🔻 Disabling ArgoCD auto-sync and scaling deployments in namespace '$ns' to 0..."

            for deploy in $(kubectl -n "$ns" get deploy -o jsonpath='{.items[*].metadata.name}'); do
              echo "→ Processing deployment: $deploy"

              # Get ArgoCD tracking annotation
              tracking=$(kubectl -n "$ns" get deploy "$deploy" -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/tracking-id}' 2>/dev/null || true)

              if [[ -z "$tracking" ]]; then
                echo "⚠️  No ArgoCD tracking ID on $deploy, skipping sync disable"
              else
                app_name=$(echo "$tracking" | cut -d: -f1)
                echo "→ Disabling auto-sync for ArgoCD app: $app_name"
                kubectl -n argocd patch application "$app_name" \
                  -p '{"spec": {"syncPolicy": null}}' --type merge || echo "⚠️  Failed to patch app: $app_name"
              fi

              echo "→ Scaling $deploy to 0"
              kubectl -n "$ns" scale deploy "$deploy" --replicas=0
            done

            echo "✅ All deployments in '$ns' scaled down"
          '';


          scaleUpNamespace = pkgs.writeShellScriptBin "scale-up-namespace" ''
            #!/usr/bin/env bash
            set -euo pipefail

            if [[ $# -ne 1 ]]; then
              echo "Usage: scale-up-namespace <namespace>" >&2
              exit 1
            fi

            ns="$1"

            # Validate namespace exists
            if ! kubectl get ns "$ns" &>/dev/null; then
              echo "❌ Namespace '$ns' does not exist." >&2
              exit 1
            fi

            echo "🔼 Scaling up deployments in '$ns' (if replicas == 0) and re-enabling ArgoCD sync..."

            for deploy in $(kubectl -n "$ns" get deploy -o jsonpath='{.items[*].metadata.name}'); do
              current=$(kubectl -n "$ns" get deploy "$deploy" -o jsonpath='{.spec.replicas}')

              if [[ "$current" == "0" ]]; then
                echo "→ Scaling $deploy to 1"
                kubectl -n "$ns" scale deploy "$deploy" --replicas=1
              else
                echo "→ $deploy already has $current replicas, skipping scale"
              fi

              tracking=$(kubectl -n "$ns" get deploy "$deploy" -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/tracking-id}' 2>/dev/null || true)

              if [[ -z "$tracking" ]]; then
                echo "⚠️  No ArgoCD tracking ID on $deploy, skipping sync enable"
              else
                app_name=$(echo "$tracking" | cut -d: -f1)
                echo "→ Re-enabling auto-sync for ArgoCD app: $app_name"
                kubectl -n argocd patch application "$app_name" \
                  -p '{"spec": {"syncPolicy": {"automated": {"prune": true, "selfHeal": true}}}}' --type merge || echo "⚠️  Failed to patch app: $app_name"
              fi
            done

            echo "✅ All deployments in '$ns' scaled up (if needed)"
          '';




        in {
          devShells.default = pkgs.mkShell {
            myScript = pkgs.writeShellScriptBin "my-script" ''
              #!/usr/bin/env bash
              echo "Hello, world!"
            '';

            buildInputs = with pkgs; [
              argocd
              opentofu
              tflint
              terraform-docs
              kubecm
              curl
              kubectl
              kustomize
              kubernetes-helm
              krew
              k9s
              talosctl
              jq
              yq
              minio-client
              clusterctl

              # Custom tools
              scaleDownNamespace
              scaleUpNamespace

              (pkgs.writeShellScriptBin "yolo" ''
                #!/usr/bin/env bash
                set -euo pipefail

                SCRIPTDIR="$(${pkgs.git}/bin/git rev-parse --show-toplevel)"
                ROOTDIR=$SCRIPTDIR
                [[ -n "''${DEBUG:-}" ]] && set -x

                ''${ROOTDIR}/terraform/hub/deploy.sh

                for dir in ''${ROOTDIR}/terraform/spokes/*/; do
                  if [[ -d "$dir" && -f "''${dir}deploy.sh" ]]; then
                    echo "Running deploy.sh in $dir"
                    chmod +x "''${dir}deploy.sh"
                    "''${dir}deploy.sh"
                  else
                    echo "Skipping $dir, no deploy.sh found"
                  fi
                done
              '')

              (pkgs.writeShellScriptBin "get_secret_data" ''
                #!/usr/bin/env bash
                set -euo pipefail

                namespace=$1
                secret_name=$2
                ${pkgs.kubectl}/bin/kubectl -n "$namespace" get secret "$secret_name" -o json |
                  ${pkgs.jq}/bin/jq -r '.data | map_values(@base64d)'
              '')
            ];

            shellHook = ''
              set -a
              source ./secrets.env
              source <(kubectl completion bash)
              source <(kubecm completion bash)
              source <(helm completion bash)
              source <(argocd completion bash)
              source <(kustomize completion bash)
              source <(talosctl completion bash)
              set +a
            '';
          };
        }
    );
}
