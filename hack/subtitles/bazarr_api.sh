#!/bin/zsh
# bazarr_api.sh METHOD PATH [curl args]: call Bazarr's API from inside its pod (vcluster-media, ns media).
# The API key is read from /config/config/config.yaml inside the pod and never printed.
# usage: api.sh METHOD PATH [extra curl args]; key read inside the pod, never printed
m=$1; p=$2; shift 2
kubectl --context admin@media-cluster -n media exec deploy/bazarr -c bazarr -- sh -c 'K=$(awk "/^auth:/{f=1;next} /^[^ ]/{f=0} f&&/^ *apikey:/{print \$2}" /config/config/config.yaml | tr -d "\"'"'"'"); m=$1; p=$2; shift 2; curl -sS -m 120 -w "\nHTTP %{http_code}\n" -X "$m" -H "X-API-KEY: $K" "$@" "http://localhost:6767$p"' sh "$m" "$p" "$@"
