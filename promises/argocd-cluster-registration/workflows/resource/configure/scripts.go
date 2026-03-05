package main

import _ "embed"

//go:embed scripts/sync-kubeconfig.sh
var syncKubeconfigScript string
