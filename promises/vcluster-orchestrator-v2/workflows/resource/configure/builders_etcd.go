package main

import (
	"fmt"
	"strings"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

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

	mergeServiceAccount := ku.Resource{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Metadata: ku.ResourceMeta(
			mergeSAName,
			config.TargetNamespace,
			labels("etcd-certs-merge-sa"),
			nil,
		),
	}

	mergeRole := ku.Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "Role",
		Metadata: ku.ResourceMeta(
			mergeSAName,
			config.TargetNamespace,
			labels("etcd-certs-merge-role"),
			nil,
		),
		Rules: []ku.PolicyRule{
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
		},
	}

	mergeRoleBinding := ku.Resource{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "RoleBinding",
		Metadata: ku.ResourceMeta(
			mergeSAName,
			config.TargetNamespace,
			labels("etcd-certs-merge-binding"),
			nil,
		),
		RoleRef: &ku.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     mergeSAName,
		},
		Subjects: []ku.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      mergeSAName,
				Namespace: config.TargetNamespace,
			},
		},
	}

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
							Image:   "bitnami/kubectl:latest",
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
	for i := 0; i < 3; i++ {
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
	return r.Replace(`set -e
echo "Waiting for certificates to be ready..."

# Wait for CA cert
until kubectl get secret {{NAME}}-etcd-ca -n {{NS}} 2>/dev/null; do
  echo "Waiting for CA certificate..."
  sleep 2
done

# Wait for server cert
until kubectl get secret {{NAME}}-etcd-server -n {{NS}} 2>/dev/null; do
  echo "Waiting for server certificate..."
  sleep 2
done

# Wait for peer cert
until kubectl get secret {{NAME}}-etcd-peer -n {{NS}} 2>/dev/null; do
  echo "Waiting for peer certificate..."
  sleep 2
done

echo "All certificates ready, merging..."

# Extract certs
CA_CRT=$(kubectl get secret {{NAME}}-etcd-ca -n {{NS}} -o jsonpath='{.data.tls\.crt}')
SERVER_CRT=$(kubectl get secret {{NAME}}-etcd-server -n {{NS}} -o jsonpath='{.data.tls\.crt}')
SERVER_KEY=$(kubectl get secret {{NAME}}-etcd-server -n {{NS}} -o jsonpath='{.data.tls\.key}')
PEER_CRT=$(kubectl get secret {{NAME}}-etcd-peer -n {{NS}} -o jsonpath='{.data.tls\.crt}')
PEER_KEY=$(kubectl get secret {{NAME}}-etcd-peer -n {{NS}} -o jsonpath='{.data.tls\.key}')

# Create merged secret
kubectl create secret generic {{NAME}}-etcd-certs -n {{NS}} \
  --from-literal=etcd-ca.crt="$(echo $CA_CRT | base64 -d)" \
  --from-literal=etcd-server.crt="$(echo $SERVER_CRT | base64 -d)" \
  --from-literal=etcd-server.key="$(echo $SERVER_KEY | base64 -d)" \
  --from-literal=etcd-peer.crt="$(echo $PEER_CRT | base64 -d)" \
  --from-literal=etcd-peer.key="$(echo $PEER_KEY | base64 -d)" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Certificate merge complete!"`)
}
