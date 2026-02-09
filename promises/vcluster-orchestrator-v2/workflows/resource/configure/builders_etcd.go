package main

import "fmt"

func buildEtcdCertificates(config *VClusterConfig) []interface{} {
	if !etcdEnabled(config) {
		return nil
	}

	labels := func(name string) map[string]string {
		return mergeStringMap(map[string]string{
			"app.kubernetes.io/instance": config.Name,
			"app.kubernetes.io/name":     name,
		}, baseLabels(config, config.Name))
	}

	caCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca"),
			nil,
		),
		"spec": map[string]interface{}{
			"isCA":       true,
			"commonName": fmt.Sprintf("%s-etcd-ca", config.Name),
			"secretName": fmt.Sprintf("%s-etcd-ca", config.Name),
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-selfsigned", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
		},
	}

	selfsignedIssuer := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-selfsigned", config.Name),
			config.TargetNamespace,
			labels("etcd-issuer"),
			nil,
		),
		"spec": map[string]interface{}{
			"selfSigned": map[string]interface{}{},
		},
	}

	caIssuer := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Issuer",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-ca", config.Name),
			config.TargetNamespace,
			labels("etcd-ca-issuer"),
			nil,
		),
		"spec": map[string]interface{}{
			"ca": map[string]interface{}{
				"secretName": fmt.Sprintf("%s-etcd-ca", config.Name),
			},
		},
	}

	mergeJob := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-certs-merge", config.Name),
			config.TargetNamespace,
			labels("etcd-certs-job"),
			nil,
		),
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": "etcd-certs-merge",
					},
				},
				"spec": map[string]interface{}{
					"restartPolicy":      "OnFailure",
					"serviceAccountName": fmt.Sprintf("vc-%s", config.Name),
					"containers": []map[string]interface{}{
						{
							"name":    "merge-certs",
							"image":   "bitnami/kubectl:latest",
							"command": []string{"/bin/bash", "-c", buildEtcdMergeScript(config)},
						},
					},
				},
			},
		},
	}

	serverCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-server", config.Name),
			config.TargetNamespace,
			labels("etcd-server-cert"),
			nil,
		),
		"spec": map[string]interface{}{
			"secretName": fmt.Sprintf("%s-etcd-server", config.Name),
			"commonName": fmt.Sprintf("%s-etcd", config.Name),
			"dnsNames":   buildEtcdDNSNames(config),
			"ipAddresses": []string{
				"127.0.0.1",
			},
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"usages": []string{"server auth", "client auth"},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-ca", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
			"secretTemplate": map[string]interface{}{
				"labels": map[string]string{
					"app.kubernetes.io/name":     "etcd-server-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	peerCert := map[string]interface{}{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": resourceMeta(
			fmt.Sprintf("%s-etcd-peer", config.Name),
			config.TargetNamespace,
			labels("etcd-peer-cert"),
			nil,
		),
		"spec": map[string]interface{}{
			"secretName": fmt.Sprintf("%s-etcd-peer", config.Name),
			"commonName": fmt.Sprintf("%s-etcd", config.Name),
			"dnsNames":   buildEtcdDNSNames(config),
			"privateKey": map[string]interface{}{
				"algorithm": "RSA",
				"size":      2048,
			},
			"usages": []string{"server auth", "client auth"},
			"issuerRef": map[string]interface{}{
				"name":  fmt.Sprintf("%s-etcd-ca", config.Name),
				"kind":  "Issuer",
				"group": "cert-manager.io",
			},
			"secretTemplate": map[string]interface{}{
				"labels": map[string]string{
					"app.kubernetes.io/name":     "etcd-peer-cert",
					"app.kubernetes.io/instance": config.Name,
				},
			},
		},
	}

	return []interface{}{caCert, selfsignedIssuer, caIssuer, mergeJob, serverCert, peerCert}
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
kubectl create secret generic %s-certs -n %s \
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
