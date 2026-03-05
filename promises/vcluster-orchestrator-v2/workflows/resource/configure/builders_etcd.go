package main

import (
	"fmt"
	"strings"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func etcdEnabled(config *VClusterConfig) bool {
	if config.BackingStore == nil {
		return false
	}
	etcd, ok := config.BackingStore["etcd"].(map[string]interface{})
	if !ok {
		return false
	}
	deploy, ok := etcd["deploy"].(map[string]interface{})
	if !ok {
		return false
	}
	enabled, ok := deploy["enabled"].(bool)
	return ok && enabled
}

func buildEtcdCertificates(config *VClusterConfig) []ku.Resource {
	if !etcdEnabled(config) {
		return nil
	}

	labels := func(name string) map[string]string {
		return ku.MergeStringMap(map[string]string{
			"app.kubernetes.io/instance": config.Name,
			"app.kubernetes.io/name":     name,
		}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))
	}

	caCert := ku.Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca"),
			nil,
		),
		Spec: CertificateSpec{
			IsCA:       true,
			CommonName: fmt.Sprintf("%s-etcd-ca", config.Name),
			SecretName: fmt.Sprintf("%s-etcd-ca", config.Name),
			PrivateKey: &PrivateKeySpec{
				Algorithm: "RSA",
				Size:      2048,
			},
			IssuerRef: IssuerRef{
				Name:  fmt.Sprintf("%s-etcd-selfsigned", config.Name),
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	selfsignedIssuer := ku.Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Issuer",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-selfsigned", config.Name),
			config.TargetNamespace,
			labels("etcd-issuer"),
			nil,
		),
		Spec: IssuerSpec{
			SelfSigned: &SelfSignedIssuer{},
		},
	}

	caIssuer := ku.Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Issuer",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca-issuer"),
			nil,
		),
		Spec: IssuerSpec{
			CA: &CAIssuer{
				SecretName: fmt.Sprintf("%s-etcd-ca", config.Name),
			},
		},
	}

	// ServiceAccount, Role, and RoleBinding for the merge job.
	// We create a dedicated SA instead of relying on vc-{name} which is
	// only created when the vcluster Helm chart is deployed (circular dependency).
	mergeSAName := fmt.Sprintf("%s-etcd-certs-merge", config.Name)

	mergeServiceAccount := ku.BuildServiceAccount(mergeSAName, config.TargetNamespace, labels("etcd-certs-merge-sa"))

	mergeRole := ku.BuildRole(mergeSAName, config.TargetNamespace, labels("etcd-certs-merge-role"), []ku.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			ResourceNames: []string{
				fmt.Sprintf("%s-etcd-ca", config.Name),
				fmt.Sprintf("%s-etcd-server", config.Name),
				fmt.Sprintf("%s-etcd-peer", config.Name),
				fmt.Sprintf("%s-etcd-certs", config.Name),
			},
			Verbs: []string{"get", "list", "create", "update", "patch"},
		},
	})

	mergeRoleBinding := ku.BuildRoleBinding(mergeSAName, config.TargetNamespace, labels("etcd-certs-merge-binding"),
		ku.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     mergeSAName,
		},
		[]ku.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      mergeSAName,
				Namespace: config.TargetNamespace,
			},
		},
	)

	mergeJob := ku.Resource{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-certs-merge", config.Name),
			config.TargetNamespace,
			labels("etcd-certs-job"),
			nil,
		),
		Spec: ku.JobSpec{
			Template: ku.PodTemplateSpec{
				Metadata: &ku.ObjectMeta{
					Labels: map[string]string{
						"app": "etcd-certs-merge",
					},
				},
				Spec: ku.PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: mergeSAName,
					Containers: []ku.Container{
						{
							Name:    "merge-certs",
							Image:   ku.DefaultKubectlImage,
							Command: []string{"/bin/bash", "-c", buildEtcdMergeScript(config)},
						},
					},
				},
			},
		},
	}

	serverCert := ku.Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
			labels("etcd-server-cert"),
			nil,
		),
		Spec: CertificateSpec{
			SecretName:  fmt.Sprintf("%s-etcd-server", config.Name),
			CommonName:  fmt.Sprintf("%s-etcd", config.Name),
			DNSNames:    buildEtcdDNSNames(config),
			IPAddresses: []string{"127.0.0.1"},
			PrivateKey: &PrivateKeySpec{
				Algorithm: "RSA",
				Size:      2048,
			},
			Usages: []string{"server auth", "client auth"},
			IssuerRef: IssuerRef{
				Name:  fmt.Sprintf("%s-etcd-ca", config.Name),
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
			SecretTemplate: &SecretTemplate{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "etcd-server-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	peerCert := ku.Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: ku.ResourceMeta(
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
			labels("etcd-peer-cert"),
			nil,
		),
		Spec: CertificateSpec{
			SecretName: fmt.Sprintf("%s-etcd-peer", config.Name),
			CommonName: fmt.Sprintf("%s-etcd", config.Name),
			DNSNames:   buildEtcdDNSNames(config),
			PrivateKey: &PrivateKeySpec{
				Algorithm: "RSA",
				Size:      2048,
			},
			Usages: []string{"server auth", "client auth"},
			IssuerRef: IssuerRef{
				Name:  fmt.Sprintf("%s-etcd-ca", config.Name),
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
			SecretTemplate: &SecretTemplate{
				Labels: map[string]string{
					"app.kubernetes.io/name":     "etcd-peer-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	return []ku.Resource{mergeServiceAccount, mergeRole, mergeRoleBinding, caCert, selfsignedIssuer, caIssuer, mergeJob, serverCert, peerCert}
}

func buildEtcdDNSNames(config *VClusterConfig) []string {
	base := []string{
		fmt.Sprintf("%s-etcd", config.Name),
		fmt.Sprintf("%s-etcd.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd.%s.svc.cluster.local", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless", config.Name),
		fmt.Sprintf("%s-etcd-headless.%s", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc", config.Name, config.TargetNamespace),
		fmt.Sprintf("%s-etcd-headless.%s.svc.cluster.local", config.Name, config.TargetNamespace),
	}
	for i := 0; i < ku.DefaultEtcdReplicas; i++ {
		base = append(base,
			fmt.Sprintf("%s-etcd-%d", config.Name, i),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc", config.Name, i, config.Name, config.TargetNamespace),
			fmt.Sprintf("%s-etcd-%d.%s-etcd-headless.%s.svc.cluster.local", config.Name, i, config.Name, config.TargetNamespace),
		)
	}
	base = append(base, "localhost")
	return base
}

func buildEtcdMergeScript(config *VClusterConfig) string {
	r := strings.NewReplacer("{{NAME}}", config.Name, "{{NS}}", config.TargetNamespace)
	return r.Replace(mergeEtcdCertsScript)
}
