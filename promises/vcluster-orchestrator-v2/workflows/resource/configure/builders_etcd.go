package main

import "fmt"

func buildEtcdCertificates(config *VClusterConfig) []Resource {
	if !etcdEnabled(config) {
		return nil
	}

	labels := func(name string) map[string]string {
		return mergeStringMap(map[string]string{
			"app.kubernetes.io/instance": config.Name,
			"app.kubernetes.io/name":     name,
		}, baseLabels(config, config.Name))
	}

	caCert := Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: resourceMeta(
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

	selfsignedIssuer := Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Issuer",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-etcd-selfsigned", config.Name),
			config.TargetNamespace,
			labels("etcd-issuer"),
			nil,
		),
		Spec: IssuerSpec{
			SelfSigned: &SelfSignedIssuer{},
		},
	}

	caIssuer := Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Issuer",
		Metadata: resourceMeta(
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

	mergeJob := Resource{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: resourceMeta(
			fmt.Sprintf("%s-etcd-certs-merge", config.Name),
			config.TargetNamespace,
			labels("etcd-certs-job"),
			nil,
		),
		Spec: JobSpec{
			Template: PodTemplateSpec{
				Metadata: &ObjectMeta{
					Labels: map[string]string{
						"app": "etcd-certs-merge",
					},
				},
				Spec: PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: fmt.Sprintf("vc-%s", config.Name),
					Containers: []Container{
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

	serverCert := Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: resourceMeta(
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

	peerCert := Resource{
		APIVersion: "cert-manager.io/v1",
		Kind:       "Certificate",
		Metadata: resourceMeta(
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

	return []Resource{caCert, selfsignedIssuer, caIssuer, mergeJob, serverCert, peerCert}
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
	return fmt.Sprintf(`set -e
echo "Waiting for certificates to be ready..."

# Wait for CA cert
until kubectl get secret %s-etcd-ca -n %s 2>/dev/null; do
  echo "Waiting for CA certificate..."
  sleep 2
done

# Wait for server cert
until kubectl get secret %s-etcd-server -n %s 2>/dev/null; do
  echo "Waiting for server certificate..."
  sleep 2
done

# Wait for peer cert
until kubectl get secret %s-etcd-peer -n %s 2>/dev/null; do
  echo "Waiting for peer certificate..."
  sleep 2
done

echo "All certificates ready, merging..."

# Extract certs
CA_CRT=$(kubectl get secret %s-etcd-ca -n %s -o jsonpath='{.data.tls\.crt}')
SERVER_CRT=$(kubectl get secret %s-etcd-server -n %s -o jsonpath='{.data.tls\.crt}')
SERVER_KEY=$(kubectl get secret %s-etcd-server -n %s -o jsonpath='{.data.tls\.key}')
PEER_CRT=$(kubectl get secret %s-etcd-peer -n %s -o jsonpath='{.data.tls\.crt}')
PEER_KEY=$(kubectl get secret %s-etcd-peer -n %s -o jsonpath='{.data.tls\.key}')

# Create merged secret
kubectl create secret generic %s-etcd-certs -n %s \
  --from-literal=etcd-ca.crt="$(echo $CA_CRT | base64 -d)" \
  --from-literal=etcd-server.crt="$(echo $SERVER_CRT | base64 -d)" \
  --from-literal=etcd-server.key="$(echo $SERVER_KEY | base64 -d)" \
  --from-literal=etcd-peer.crt="$(echo $PEER_CRT | base64 -d)" \
  --from-literal=etcd-peer.key="$(echo $PEER_KEY | base64 -d)" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Certificate merge complete!"`,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
		config.Name, config.TargetNamespace,
	)
}
