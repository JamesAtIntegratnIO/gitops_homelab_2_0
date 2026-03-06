package main

import (
	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

// ExternalSecretConfig holds the resolved configuration from the CR.
type ExternalSecretConfig struct {
	AppName         string
	Namespace       string
	OwnerPromise    string
	PromiseName     string
	SecretStoreName string
	SecretStoreKind string
	Secrets         []ku.SecretRef
}
