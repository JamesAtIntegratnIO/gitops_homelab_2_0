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
          # talosctl is pinned apart from nixpkgs, to the Talos version the
          # NODES run -- Sidero's upgrade FAQ: "We recommend using the version
          # that matches the current running version of the cluster." Bumping
          # the nixpkgs input instead would move every tool at once, kubectl
          # included, which tools.sh hard-pins to the apiserver minor.
          #
          # During the upgrade ladder this trails the nodes by one rung: once
          # every node reports a new version, bump `version` to match, then
          # replace the four hashes from that release's sha256sum.txt:
          #
          #   curl -fsSL https://github.com/siderolabs/talos/releases/download/v<ver>/sha256sum.txt \
          #     | grep -E 'talosctl-(darwin|linux)-(amd64|arm64)$'
          #   nix hash convert --hash-algo sha256 --to sri <hex>
          #
          # The release binary is used rather than rebuilding from source: it is
          # the artifact Sidero ships and tests, and it needs no vendorHash.
          talosctlOverlay = final: prev: {
            talosctl =
              let
                version = "1.13.12";
                platform = {
                  x86_64-linux = "linux-amd64";
                  aarch64-linux = "linux-arm64";
                  x86_64-darwin = "darwin-amd64";
                  aarch64-darwin = "darwin-arm64";
                }.${prev.stdenv.hostPlatform.system};
                hash = {
                  darwin-arm64 = "sha256-kOOShmFoCBiXMl/x7s7sEKhUdqm8nrQ5n+dBqNf3iT0=";
                  darwin-amd64 = "sha256-VCR6f5Pino74HDFxFkXVczGeSVhlNK72lc9Pz80YI1M=";
                  linux-amd64 = "sha256-TOQhh2TCTWpr1WEJcZm2DthhnbBzeS/OW3z85/qNEvw=";
                  linux-arm64 = "sha256-NcMEw3jXDyQexdY2Et794lMF+BCOVQJWqmPzfaUQtr4=";
                }.${platform};
              in
              prev.stdenvNoCC.mkDerivation {
                pname = "talosctl";
                inherit version;
                src = prev.fetchurl {
                  url = "https://github.com/siderolabs/talos/releases/download/v${version}/talosctl-${platform}";
                  inherit hash;
                };
                dontUnpack = true;
                # Never strip: on aarch64-darwin that invalidates the ad-hoc code
                # signature the Go linker embeds, and the kernel kills the binary.
                dontStrip = true;
                installPhase = ''
                  runHook preInstall
                  install -Dm755 "$src" "$out/bin/talosctl"
                  runHook postInstall
                '';
                meta.mainProgram = "talosctl";
              };
          };

          # kubectl follows the apiserver the same way. The hashes come from
          #
          #   curl -fsSL https://dl.k8s.io/release/v<ver>/bin/<os>/<arch>/kubectl.sha256
          #
          # and when the minor moves, so does KUBECTL_WANT_MINOR in tools.sh.
          kubectlOverlay = final: prev: {
            kubectl =
              let
                version = "1.36.6";
                platform = {
                  x86_64-linux = "linux/amd64";
                  aarch64-linux = "linux/arm64";
                  x86_64-darwin = "darwin/amd64";
                  aarch64-darwin = "darwin/arm64";
                }.${prev.stdenv.hostPlatform.system};
                hash = {
                  "darwin/arm64" = "sha256-HPc1w9Ncz+wo0GGHhMsGpGFNLEKq3ii2vg6rdqL/tiM=";
                  "darwin/amd64" = "sha256-LiLTgvJCPIjZ0HYkb4YLs9XHQ90nnKY+ETR0u4g4JCY=";
                  "linux/amd64" = "sha256-KYGLZynckmK2kjB0cNBVjQJ3Jec4vl3UfHYCYYaKJjQ=";
                  "linux/arm64" = "sha256-NVB+9AjD04hJ5dMgPhj46GHbgiqmZBAanO0Iae3QYEY=";
                }.${platform};
              in
              prev.stdenvNoCC.mkDerivation {
                pname = "kubectl";
                inherit version;
                src = prev.fetchurl {
                  url = "https://dl.k8s.io/release/v${version}/bin/${platform}/kubectl";
                  inherit hash;
                };
                dontUnpack = true;
                dontStrip = true;   # same code-signature reason as talosctl
                installPhase = ''
                  runHook preInstall
                  install -Dm755 "$src" "$out/bin/kubectl"
                  runHook postInstall
                '';
                meta.mainProgram = "kubectl";
              };
          };

          pkgs = (nixpkgs.legacyPackages.${system}.extend talosctlOverlay).extend kubectlOverlay;

          hctl = pkgs.buildGoModule {
            pname = "hctl";
            version = "0.1.0";
            src = pkgs.lib.cleanSource ./cli;
            vendorHash = "sha256-c5RQoqS8Zs3Ugm2iN4e3mzlJ+XPkvYVWyo7ZLYgqnFk=";
            ldflags = [
              "-s" "-w"
              "-X github.com/jamesatintegratnio/hctl/cmd.Version=0.1.0"
            ];
            meta = {
              description = "Homelab platform CLI";
              mainProgram = "hctl";
            };
          };

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
          packages.hctl = hctl;
          packages.talosctl = pkgs.talosctl;
          packages.default = hctl;

          devShells.default = pkgs.mkShell {
            myScript = pkgs.writeShellScriptBin "my-script" ''
              #!/usr/bin/env bash
              echo "Hello, world!"
            '';

            buildInputs = with pkgs; [
              argocd
              go
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
              cilium-cli
              hubble
              jq
              yq
              minio-client
              clusterctl
              nodejs_22

              # Custom tools
              hctl
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
              # secrets.env is gitignored, so a fresh clone does not have one.
              # Sourcing it unconditionally made `nix develop` fail outright on
              # any checkout that had not been set up yet -- including a
              # disaster-recovery clone, which is precisely when you least want
              # the toolchain to be unavailable.
              if [ -f ./secrets.env ]; then
                source ./secrets.env
              else
                echo "note: ./secrets.env not found; shell is usable, but anything reading those variables will not be." >&2
              fi
              source <(kubectl completion bash)
              source <(kubecm completion bash)
              source <(helm completion bash)
              source <(argocd completion bash)
              source <(kustomize completion bash)
              source <(talosctl completion bash)
              source <(hctl completion bash)
              set +a
            '';
          };
        }
    );
}
