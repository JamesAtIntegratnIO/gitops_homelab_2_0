#!/usr/bin/env bash
# Render both addon bootstraps the way ArgoCD renders them, and print -- per
# ApplicationSet -- the clusters its generator selector actually matches.
#
#   ./addon-targets.sh > /tmp/before   # on the base revision
#   <edit addons/>
#   ./addon-targets.sh > /tmp/after
#   diff -u /tmp/before /tmp/after     # this diff is the review
#
# Reading an addon definition is NOT a substitute: the control-plane bootstrap
# renders ApplicationSets whose cluster selector can match the vcluster too, so
# a "host" addon lands inside the tenant. That leak is only visible here.
#
# Fidelity: the sum of matched clusters equals the live count of
# `-l addon=true` Applications (61 on 2026-08-28).
set -euo pipefail

REPO=$(git rev-parse --show-toplevel)
source "$REPO/.claude/skills/homelab-toolchain/scripts/tools.sh"

INV=$("$KUBECTL" --context "$CTX" -n argocd get secret \
  -l argocd.argoproj.io/secret-type=cluster -o json |
  "$JQ" -c '[.items[]|{name:(.data.name|@base64d),labels:.metadata.labels}]')

render() { # cluster_name -> ApplicationSet YAML
  local cname="$1" env role prefix f args=() files=()
  env=$("$JQ" -r --arg n "$cname" '.[]|select(.labels.cluster_name==$n)|.labels.environment' <<<"$INV")
  role=$("$JQ" -r --arg n "$cname" '.[]|select(.labels.cluster_name==$n)|.labels.cluster_role' <<<"$INV")
  # Value-file order is copied from terraform/cluster/bootstrap/addons-*.yaml.
  # The vcluster bootstrap deliberately omits environments/*/addons/addons.yaml.
  if [ "$role" = vcluster ]; then
    prefix="bootstrap-vcluster-${cname}-"
    files=("environments/$env/addons/common.yaml")
  else
    prefix="bootstrap-"
    files=("environments/$env/addons/addons.yaml" "environments/$env/addons/common.yaml")
  fi
  files+=("cluster-roles/$role/addons/addons.yaml"
          "cluster-roles/$role/addons/common.yaml"
          "clusters/$cname/addons.yaml")
  for f in "${files[@]}"; do [ -f "$REPO/addons/$f" ] && args+=(-f "$REPO/addons/$f"); done
  "$HELM" template "$REPO/addons/charts/application-sets" "${args[@]}" --set appsetPrefix="$prefix"
}

# LabelSelector semantics: matchLabels AND matchExpressions.
MATCH='
def matches($sel; $lab):
  (($sel.matchLabels // {}) | to_entries | all(.value == ($lab[.key] // null)))
  and (($sel.matchExpressions // []) | all(
        . as $e | ($lab[$e.key] // null) as $v |
        if   $e.operator == "In"           then ($v != null and ($e.values|index($v)) != null)
        elif $e.operator == "NotIn"        then ($v == null or  ($e.values|index($v)) == null)
        elif $e.operator == "Exists"       then $v != null
        elif $e.operator == "DoesNotExist" then $v == null
        else false end));
[ $inv[] | select(matches($sel; .labels)) | .labels.cluster_name ] | sort | join(",")'

for cname in $("$JQ" -r '.[].labels.cluster_name' <<<"$INV" | sort); do
  echo "=== bootstrap for $cname ==="
  render "$cname" | "$YQ" -o=json -I=0 'select(.kind == "ApplicationSet")' |
  while read -r doc; do
    name=$("$JQ" -r '.metadata.name' <<<"$doc")
    sel=$("$JQ" -c '.spec.generators[0].clusters.selector // {}' <<<"$doc")
    tgt=$("$JQ" -r --argjson inv "$INV" --argjson sel "$sel" -n "$MATCH")
    printf '%-56s -> %s\n' "$name" "${tgt:-<none>}"
  done
done
