package main

import _ "embed"

//go:embed scripts/merge-etcd-certs.sh
var mergeEtcdCertsScript string
