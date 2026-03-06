package main

import (
	"fmt"
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func etcdConfig() *VClusterConfig {
	config := minimalConfig()
	config.EtcdEnabled = true
	config.BackingStore = map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	return config
}

func TestBuildEtcdCertificates_CACert(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantName := fmt.Sprintf("%s-etcd-ca", config.Name)
	caCert := ku.FindResource(certs, "Certificate", wantName)
	if caCert == nil {
		t.Fatalf("Certificate %q not found in resources", wantName)
	}
	if caCert.APIVersion != "cert-manager.io/v1" {
		t.Errorf("expected apiVersion 'cert-manager.io/v1', got %q", caCert.APIVersion)
	}
	if caCert.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("expected namespace %q, got %q", config.TargetNamespace, caCert.Metadata.Namespace)
	}

	spec, ok := caCert.Spec.(CertificateSpec)
	if !ok {
		t.Fatal("expected CertificateSpec type")
	}
	if !spec.IsCA {
		t.Error("expected CA cert to have isCA=true")
	}
	if spec.CommonName != wantName {
		t.Errorf("expected commonName %q, got %q", wantName, spec.CommonName)
	}
	if spec.SecretName != wantName {
		t.Errorf("expected secretName %q, got %q", wantName, spec.SecretName)
	}
	if spec.PrivateKey == nil || spec.PrivateKey.Algorithm != "RSA" || spec.PrivateKey.Size != 2048 {
		t.Error("expected RSA 2048 private key")
	}
	if spec.IssuerRef.Kind != "Issuer" {
		t.Errorf("expected IssuerRef kind 'Issuer', got %q", spec.IssuerRef.Kind)
	}
}

func TestBuildEtcdCertificates_Issuers(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantSSName := fmt.Sprintf("%s-etcd-selfsigned", config.Name)
	selfSigned := ku.FindResource(certs, "Issuer", wantSSName)
	if selfSigned == nil {
		t.Fatalf("Issuer %q not found in resources", wantSSName)
	}
	ssSpec, ok := selfSigned.Spec.(IssuerSpec)
	if !ok {
		t.Fatal("expected IssuerSpec type")
	}
	if ssSpec.SelfSigned == nil {
		t.Error("expected selfSigned issuer spec")
	}

	wantCAName := fmt.Sprintf("%s-etcd-ca", config.Name)
	caIssuer := ku.FindResource(certs, "Issuer", wantCAName)
	if caIssuer == nil {
		t.Fatalf("Issuer %q not found in resources", wantCAName)
	}
	caSpec, ok := caIssuer.Spec.(IssuerSpec)
	if !ok {
		t.Fatal("expected IssuerSpec type for CA issuer")
	}
	if caSpec.CA == nil {
		t.Fatal("expected CA issuer spec")
	}
	if caSpec.CA.SecretName != wantCAName {
		t.Errorf("expected CA secretName %q, got %q", wantCAName, caSpec.CA.SecretName)
	}
}

func TestBuildEtcdCertificates_ServerCert(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantName := fmt.Sprintf("%s-etcd-server", config.Name)
	serverCert := ku.FindResource(certs, "Certificate", wantName)
	if serverCert == nil {
		t.Fatalf("Certificate %q not found in resources", wantName)
	}

	spec, ok := serverCert.Spec.(CertificateSpec)
	if !ok {
		t.Fatal("expected CertificateSpec type")
	}
	if spec.IsCA {
		t.Error("server cert should not be CA")
	}
	if spec.SecretName != wantName {
		t.Errorf("expected secretName %q, got %q", wantName, spec.SecretName)
	}
	if len(spec.DNSNames) == 0 {
		t.Error("expected DNS names on server cert")
	}
	if len(spec.IPAddresses) != 1 || spec.IPAddresses[0] != "127.0.0.1" {
		t.Errorf("expected IPAddresses [127.0.0.1], got %v", spec.IPAddresses)
	}
	if len(spec.Usages) != 2 {
		t.Errorf("expected 2 usages, got %d", len(spec.Usages))
	}
	wantUsages := map[string]bool{"server auth": true, "client auth": true}
	for _, u := range spec.Usages {
		if !wantUsages[u] {
			t.Errorf("unexpected usage %q", u)
		}
	}
	if spec.IssuerRef.Name != fmt.Sprintf("%s-etcd-ca", config.Name) {
		t.Errorf("expected server cert issuer %q, got %q", fmt.Sprintf("%s-etcd-ca", config.Name), spec.IssuerRef.Name)
	}
	if spec.SecretTemplate == nil {
		t.Fatal("expected secretTemplate on server cert")
	}
	if spec.SecretTemplate.Labels["app.kubernetes.io/name"] != "etcd-server-cert" {
		t.Error("expected secretTemplate label app.kubernetes.io/name=etcd-server-cert")
	}
}

func TestBuildEtcdCertificates_PeerCert(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantName := fmt.Sprintf("%s-etcd-peer", config.Name)
	peerCert := ku.FindResource(certs, "Certificate", wantName)
	if peerCert == nil {
		t.Fatalf("Certificate %q not found in resources", wantName)
	}

	spec, ok := peerCert.Spec.(CertificateSpec)
	if !ok {
		t.Fatal("expected CertificateSpec type")
	}
	if spec.SecretName != wantName {
		t.Errorf("expected secretName %q, got %q", wantName, spec.SecretName)
	}
	// Peer cert should not have IPAddresses
	if len(spec.IPAddresses) != 0 {
		t.Errorf("expected no IPAddresses on peer cert, got %v", spec.IPAddresses)
	}
	if spec.SecretTemplate == nil {
		t.Fatal("expected secretTemplate on peer cert")
	}
	if spec.SecretTemplate.Labels["app.kubernetes.io/name"] != "etcd-peer-cert" {
		t.Error("expected secretTemplate label app.kubernetes.io/name=etcd-peer-cert")
	}
}

func TestBuildEtcdCertificates_RBAC(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantSAName := fmt.Sprintf("%s-etcd-certs-merge", config.Name)
	sa := ku.FindResource(certs, "ServiceAccount", wantSAName)
	if sa == nil {
		t.Fatalf("ServiceAccount %q not found in resources", wantSAName)
	}
	if sa.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("expected SA namespace %q, got %q", config.TargetNamespace, sa.Metadata.Namespace)
	}

	role := ku.FindResource(certs, "Role", wantSAName)
	if role == nil {
		t.Fatalf("Role %q not found in resources", wantSAName)
	}
	if len(role.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(role.Rules))
	}
	if len(role.Rules[0].ResourceNames) != 4 {
		t.Errorf("expected 4 resourceNames in role rule, got %d", len(role.Rules[0].ResourceNames))
	}

	rb := ku.FindResource(certs, "RoleBinding", wantSAName)
	if rb == nil {
		t.Fatalf("RoleBinding %q not found in resources", wantSAName)
	}
	if rb.RoleRef == nil {
		t.Fatal("expected roleRef")
	}
	if rb.RoleRef.Name != wantSAName {
		t.Errorf("expected roleRef name %q, got %q", wantSAName, rb.RoleRef.Name)
	}
	if len(rb.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(rb.Subjects))
	}
	if rb.Subjects[0].Name != wantSAName {
		t.Errorf("expected subject name %q, got %q", wantSAName, rb.Subjects[0].Name)
	}
}

func TestBuildEtcdCertificates_MergeJob(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	wantName := fmt.Sprintf("%s-etcd-certs-merge", config.Name)
	job := ku.FindResource(certs, "Job", wantName)
	if job == nil {
		t.Fatalf("Job %q not found in resources", wantName)
	}
	if job.APIVersion != "batch/v1" {
		t.Errorf("expected apiVersion 'batch/v1', got %q", job.APIVersion)
	}

	spec, ok := job.Spec.(ku.JobSpec)
	if !ok {
		t.Fatal("expected JobSpec type")
	}
	if spec.Template.Spec.RestartPolicy != "OnFailure" {
		t.Errorf("expected restartPolicy 'OnFailure', got %q", spec.Template.Spec.RestartPolicy)
	}
	wantSA := fmt.Sprintf("%s-etcd-certs-merge", config.Name)
	if spec.Template.Spec.ServiceAccountName != wantSA {
		t.Errorf("expected serviceAccountName %q, got %q", wantSA, spec.Template.Spec.ServiceAccountName)
	}
	if len(spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(spec.Template.Spec.Containers))
	}
	container := spec.Template.Spec.Containers[0]
	if container.Image != ku.DefaultKubectlImage {
		t.Errorf("expected image %q, got %q", ku.DefaultKubectlImage, container.Image)
	}
}

func TestBuildEtcdDNSNames_ContainsAllVariants(t *testing.T) {
	config := minimalConfig()
	dns := buildEtcdDNSNames(config)

	dnsSet := make(map[string]bool)
	for _, d := range dns {
		dnsSet[d] = true
	}

	// Base names
	wantBase := []string{
		fmt.Sprintf("%s-etcd", config.Name),
		fmt.Sprintf("%s-etcd.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc.cluster.local", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless", config.Name),
		fmt.Sprintf("%s-etcd-headless.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc.cluster.local", config.Name, config.TargetNamespace),
	}
	for _, name := range wantBase {
		if !dnsSet[name] {
			t.Errorf("missing base DNS name: %q", name)
		}
	}

	// Per-replica names (DefaultEtcdReplicas = 3)
	for i := 0; i < ku.DefaultEtcdReplicas; i++ {
		perReplica := []string{
			fmt.Sprintf("%s-etcd-%d", config.Name, i),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc.cluster.local", config.Name, i, config.Name, config.TargetNamespace),
		}
		for _, name := range perReplica {
			if !dnsSet[name] {
				t.Errorf("missing replica %d DNS name: %q", i, name)
			}
		}
	}

	if !dnsSet["localhost"] {
		t.Error("expected localhost in DNS names")
	}
}

func TestBuildEtcdDNSNames_ExactCount(t *testing.T) {
	config := minimalConfig()
	dns := buildEtcdDNSNames(config)

	// 8 base + (DefaultEtcdReplicas * 4) per-replica + 1 localhost
	expected := 8 + (ku.DefaultEtcdReplicas * 4) + 1
	if len(dns) != expected {
		t.Errorf("expected %d DNS names, got %d", expected, len(dns))
	}
}

func TestBuildEtcdMergeScript_Replacements(t *testing.T) {
	config := minimalConfig()
	script := buildEtcdMergeScript(config)

	if strings.Contains(script, "{{NAME}}") {
		t.Error("script still contains {{NAME}} placeholder")
	}
	if strings.Contains(script, "{{NS}}") {
		t.Error("script still contains {{NS}} placeholder")
	}
	if !strings.Contains(script, config.Name) {
		t.Error("expected config.Name in merge script")
	}
	if !strings.Contains(script, config.TargetNamespace) {
		t.Error("expected config.TargetNamespace in merge script")
	}
}

func TestBuildEtcdCertificates_AllInTargetNamespace(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	for i, cert := range certs {
		if cert.Metadata.Namespace != config.TargetNamespace {
			t.Errorf("resource %d (%s %q): expected namespace %q, got %q",
				i, cert.Kind, cert.Metadata.Name, config.TargetNamespace, cert.Metadata.Namespace)
		}
	}
}

func TestBuildEtcdCertificates_AllHaveBaseLabels(t *testing.T) {
	config := etcdConfig()
	certs := buildEtcdCertificates(config)

	for i, cert := range certs {
		if cert.Metadata.Labels["kratix.io/promise-name"] != config.PromiseName {
			t.Errorf("resource %d (%s): missing kratix.io/promise-name label", i, cert.Kind)
		}
		if cert.Metadata.Labels["kratix.io/resource-name"] != config.Name {
			t.Errorf("resource %d (%s): missing kratix.io/resource-name label", i, cert.Kind)
		}
		if cert.Metadata.Labels["app.kubernetes.io/instance"] != config.Name {
			t.Errorf("resource %d (%s): missing app.kubernetes.io/instance label", i, cert.Kind)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge-case / nil-config tests for etcd builders
// ---------------------------------------------------------------------------

func TestEtcdEnabled_NilBackingStore(t *testing.T) {
	config := &VClusterConfig{BackingStore: nil}
	if config.EtcdEnabled {
		t.Error("expected etcdEnabled=false with nil BackingStore")
	}
}

func TestEtcdEnabled_EmptyBackingStore(t *testing.T) {
	config := &VClusterConfig{BackingStore: map[string]interface{}{}}
	if config.EtcdEnabled {
		t.Error("expected etcdEnabled=false with empty BackingStore")
	}
}

func TestEtcdEnabled_MalformedEtcdKey(t *testing.T) {
	// etcd key exists but is wrong type (string instead of map)
	config := &VClusterConfig{BackingStore: map[string]interface{}{
		"etcd": "not-a-map",
	}}
	if config.EtcdEnabled {
		t.Error("expected etcdEnabled=false when etcd key is not a map")
	}
}

func TestEtcdEnabled_MissingDeployKey(t *testing.T) {
	config := &VClusterConfig{BackingStore: map[string]interface{}{
		"etcd": map[string]interface{}{
			"something": "else",
		},
	}}
	if config.EtcdEnabled {
		t.Error("expected etcdEnabled=false when deploy key is missing")
	}
}

func TestEtcdEnabled_DeployEnabledNotBool(t *testing.T) {
	config := &VClusterConfig{BackingStore: map[string]interface{}{
		"etcd": map[string]interface{}{
			"deploy": map[string]interface{}{
				"enabled": "yes", // string instead of bool
			},
		},
	}}
	if config.EtcdEnabled {
		t.Error("expected etcdEnabled=false when enabled is not a bool")
	}
}

func TestBuildEtcdCertificates_NilBackingStore(t *testing.T) {
	config := minimalConfig()
	config.BackingStore = nil
	certs := buildEtcdCertificates(config)
	if certs != nil {
		t.Errorf("expected nil when BackingStore is nil, got %d resources", len(certs))
	}
}

func TestBuildEtcdCertificates_EmptyBackingStore(t *testing.T) {
	config := minimalConfig()
	config.BackingStore = map[string]interface{}{}
	certs := buildEtcdCertificates(config)
	if certs != nil {
		t.Errorf("expected nil when BackingStore is empty, got %d resources", len(certs))
	}
}

func TestBuildEtcdDNSNames_IncludesHeadlessVariants(t *testing.T) {
	config := minimalConfig()
	dns := buildEtcdDNSNames(config)

	dnsSet := make(map[string]bool)
	for _, d := range dns {
		dnsSet[d] = true
	}

	// Verify headless service DNS entries exist
	headlessBase := fmt.Sprintf("%s-etcd-headless", config.Name)
	if !dnsSet[headlessBase] {
		t.Errorf("missing headless base name: %q", headlessBase)
	}
	headlessFQDN := fmt.Sprintf("%s-etcd-headless.%s.svc.cluster.local", config.Name, config.TargetNamespace)
	if !dnsSet[headlessFQDN] {
		t.Errorf("missing headless FQDN: %q", headlessFQDN)
	}
}

func TestBuildEtcdMergeScript_ContainsKubectlCommands(t *testing.T) {
	config := minimalConfig()
	script := buildEtcdMergeScript(config)

	if !strings.Contains(script, "kubectl") {
		t.Error("expected kubectl command in merge script")
	}
	if !strings.Contains(script, config.Name) {
		t.Error("expected vcluster name in merge script")
	}
}
