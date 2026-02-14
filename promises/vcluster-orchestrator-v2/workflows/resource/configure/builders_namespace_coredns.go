package main

import "fmt"

func buildNamespace(config *VClusterConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":        "vcluster-namespace",
		"vcluster.loft.sh/namespace":    "true",
		"platform.integratn.tech/type":  "vcluster",
	}, baseLabels(config, config.Name))

	return Resource{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: resourceMeta(
			config.TargetNamespace,
			"",
			labels,
			map[string]string{"argocd.argoproj.io/sync-wave": "-2"},
		),
	}
}

func buildCorednsConfigMap(config *VClusterConfig) Resource {
	labels := mergeStringMap(map[string]string{
		"app.kubernetes.io/name":     "coredns",
		"app.kubernetes.io/instance": fmt.Sprintf("vc-%s", config.Name),
	}, baseLabels(config, config.Name))

	corefile := fmt.Sprintf(`.:1053 {
    errors
    health
    ready
    kubernetes %s in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
    }
    hosts /etc/coredns/NodeHosts {
        ttl 60
        reload 15s
        fallthrough
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}

import /etc/coredns/custom/*.server
`, config.ClusterDomain)

	return Resource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: resourceMeta(
			fmt.Sprintf("vc-%s-coredns", config.Name),
			config.TargetNamespace,
			labels,
			nil,
		),
		Data: map[string]string{
			"Corefile":  corefile,
			"NodeHosts": "",
		},
	}
}
