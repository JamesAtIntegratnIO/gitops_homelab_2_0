package main

import (
	"fmt"

	ku "github.com/jamesatintegratnio/gitops_homelab_2_0/promises/_shared/kratixutil"
)

func buildNamespace(config *VClusterConfig) ku.Resource {
	labels := ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name":        "vcluster-namespace",
		"vcluster.loft.sh/namespace":    "true",
		"platform.integratn.tech/type":  "vcluster",
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

	return ku.Resource{
		APIVersion: "v1",
		Kind:       "Namespace",
		Metadata: ku.ResourceMeta(
			config.TargetNamespace,
			"",
			labels,
			map[string]string{"argocd.argoproj.io/sync-wave": "-2"},
		),
	}
}

func buildCorednsConfigMap(config *VClusterConfig) ku.Resource {
	labels := ku.MergeStringMap(map[string]string{
		"app.kubernetes.io/name":     "coredns",
		"app.kubernetes.io/instance": fmt.Sprintf("vc-%s", config.Name),
	}, ku.BaseLabels(config.WorkflowContext.PromiseName, config.Name))

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

	return ku.Resource{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata: ku.ResourceMeta(
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
