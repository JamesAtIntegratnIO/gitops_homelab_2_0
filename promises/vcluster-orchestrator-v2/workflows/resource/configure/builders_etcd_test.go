package main

import (
	"fmt"
	"strings"
	"testing"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func etcdConfig() *VClusterConfig {
	config := minimalConfig()
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

	// CA cert is at index 3 (SA, Role, RoleBinding, CACert, ...)
	caCert := certs[3]
	if caCert.Kind != "Certificate" {
		t.Fatalf("expected kind Certificate, got %q", caCert.Kind)
	}
	if caCert.APIVersion != "cert-manager.io/v1" {
		t.Errorf("expected apiVersion 'cert-manager.io/v1', got %q", caCert.APIVersion)
	}
	wantName := fmt.Sprintf("%s-etcd-ca", config.Name)
	if caCert.Metadata.Name != wantName {
		t.Errorf("expected name %q, got %q", wantName, caCert.Metadata.Name)
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

	// SelfSigned issuer is at index 4
	selfSigned := certs[4]
	if selfSigned.Kind != "Issuer" {
		t.Fatalf("expected kind Issuer, got %q", selfSigned.Kind)
	}
	wantSSName := fmt.Sprintf("%s-etcd-selfsigned", config.Name)
	if selfSigned.Metadata.Name != wantSSName {
		t.Errorf("expected name %q, got %q", wantSSName, selfSigned.Metadata.Name)
	}
	ssSpec, ok := selfSigned.Spec.(IssuerSpec)
	if !ok {
		t.Fatal("expected IssuerSpec type")
	}
	if ssSpec.SelfSigned == nil {
		t.Error("expected selfSigned issuer spec")
	}

	// CA issuer is at index 5
	caIssuer := certs[5]
	if caIssuer.Kind != "Issuer" {
		t.Fatalf("expected kind Issuer, got %q", caIssuer.Kind)
	}
	wantCAName := fmt.Sprintf("%s-etcd-ca", config.Name)
	if caIssuer.Metadata.Name != wantCAName {
		t.Errorf("expected name %q, got %q", wantCAName, caIssuer.Metadata.Name)
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

	// Server cert is at index 7
	serverCert := certs[7]
	if serverCert.Kind != "Certificate" {
		t.Fatalf("expected kind Certificate, got %q", serverCert.Kind)
	}
	wantName := fmt.Sprintf("%s-etcd-server", config.Name)
	if serverCert.Metadata.Name != wantName {
		t.Errorf("expected name %q, got %q", wantName, serverCert.Metadata.Name)
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

	// Peer cert is at index 8
	peerCert := certs[8]
	if peerCert.Kind != "Certificate" {
		t.Fatalf("expected kind Certificate, got %q", peerCert.Kind)
	}
	wantName := fmt.Sprintf("%s-etcd-peer", config.Name)
	if peerCert.Metadata.Name != wantName {
		t.Errorf("expected name %q, got %q", wantName, peerCert.Metadata.Name)
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

	// SA is at index 0
	sa := certs[0]
	if sa.Kind != "ServiceAccount" {
		t.Fatalf("expected ServiceAccount, got %q", sa.Kind)
	}
	wantSAName := fmt.Sprintf("%s-etcd-certs-merge", config.Name)
	if sa.Metadata.Name != wantSAName {
		t.Errorf("expected SA name %q, got %q", wantSAName, sa.Metadata.Name)
	}
	if sa.Metadata.Namespace != config.TargetNamespace {
		t.Errorf("expected SA namespace %q, got %q", config.TargetNamespace, sa.Metadata.Namespace)
	}

	// Role is at index 1
	role := certs[1]
	if role.Kind != "Role" {
		t.Fatalf("expected Role, got %q", role.Kind)
	}
	if role.Metadata.Name != wantSAName {
		t.Errorf("expected Role name %q, got %q", wantSAName, role.Metadata.Name)
	}
	if len(role.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(role.Rules))
	}
	if len(role.Rules[0].ResourceNames) != 4 {
		t.Errorf("expected 4 resourceNames in role rule, got %d", len(role.Rules[0].ResourceNames))
	}

	// RoleBinding is at index 2
	rb := certs[2]
	if rb.Kind != "RoleBinding" {
		t.Fatalf("expected RoleBinding, got %q", rb.Kind)
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

	// Job is at index 6
	job := certs[6]
	if job.Kind != "Job" {
		t.Fatalf("expected kind Job, got %q", job.Kind)
	}
	if job.APIVersion != "batch/v1" {
		t.Errorf("expected apiVersion 'batch/v1', got %q", job.APIVersion)
	}
	wantName := fmt.Sprintf("%s-etcd-certs-merge", config.Name)
	if job.Metadata.Name != wantName {
		t.Errorf("expected name %q, got %q", wantName, job.Metadata.Name)
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
		if cert.Metadata.Labels["kratix.io/promise-name"] != config.WorkflowContext.PromiseName {
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
