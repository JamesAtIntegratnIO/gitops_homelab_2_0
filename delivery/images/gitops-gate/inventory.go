package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Cluster mirrors the fields of an ArgoCD cluster Secret that generators and
// templates can actually see: the name and server, plus labels and
// annotations. Selectors match on labels; templates read both.
type Cluster struct {
	Name   string `json:"name"`
	Server string `json:"server"`
	// ArgoCD names the instance this cluster is registered with, for fleets
	// running more than one -- per region, per tenant, per business unit.
	// Empty means "the only one", which is the common case.
	ArgoCD      string            `json:"argocd,omitempty"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// Inventory is the checked-in copy of the live cluster Secrets.
//
// This is the gate's weakest joint and it is worth being honest about: CI has
// no cluster access, so this file is a snapshot that can silently go stale. If
// a cluster's labels change, or a new cluster appears, the targeting diff
// keeps answering confidently and wrongly. `gitops-gate clusters export`
// regenerates it; running that check somewhere that *does* have cluster access
// is how the drift gets caught.
type Inventory struct {
	Clusters []Cluster `json:"clusters"`
	// GeneratedAt records when the export ran, so a reviewer can see the age.
	GeneratedAt string `json:"generatedAt,omitempty"`
}

// Validate checks that the inventory can answer the questions the selectors
// ask of it.
//
// This is what makes a stale inventory an ERROR rather than a wrong answer.
// Selectors match on labels, and a label the inventory has never seen simply
// does not match -- so a fixture written before someone added `cluster_role`
// renders a fraction of the real Applications and then reports "no targeting
// change" with total confidence. Measured: one missing label took a render
// from 62 Applications to 7, silently.
//
// The known set is derived from the clusters themselves rather than recorded
// separately. Recording it would be the same information written twice, and
// a stale file's own record is stale too -- so it would detect nothing.
func (inv *Inventory) Validate(selectorKeys []string, knownAbsent []string) error {
	known := map[string]bool{}
	for _, c := range inv.Clusters {
		for k := range c.Labels {
			known[k] = true
		}
	}
	for _, k := range knownAbsent {
		known[k] = true
	}

	var unknown []string
	for _, k := range selectorKeys {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf(
		"selectors match on label(s) no cluster in the inventory carries: %s\n\n"+
			"Either the inventory is stale -- re-run `gitops-gate clusters export` --\n"+
			"or those labels genuinely exist nowhere, in which case the Applications\n"+
			"selecting on them are generated for no cluster at all and that is worth\n"+
			"knowing.\n\n"+
			"If it is deliberate, list them under `clustersExport.knownAbsentLabels`\n"+
			"in .gitops-gate.yaml.\n\n"+
			"This is refused rather than assumed because the failure is silent: a\n"+
			"missing label shrinks the render, and the gate would then compare two\n"+
			"almost-empty sets and find no difference",
		strings.Join(unknown, ", "))
}

func LoadInventory(path string) (*Inventory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cluster inventory %s: %w", path, err)
	}
	var inv Inventory
	if err := yaml.UnmarshalStrict(raw, &inv); err != nil {
		return nil, fmt.Errorf("parsing cluster inventory %s: %w", path, err)
	}
	if len(inv.Clusters) == 0 {
		return nil, fmt.Errorf("%s: no clusters -- the gate cannot expand a generator against an empty inventory", path)
	}
	for i, c := range inv.Clusters {
		if c.Name == "" {
			return nil, fmt.Errorf("%s: clusters[%d] has no name", path, i)
		}
		if c.Labels == nil {
			inv.Clusters[i].Labels = map[string]string{}
		}
		if c.Annotations == nil {
			inv.Clusters[i].Annotations = map[string]string{}
		}
		// Every ArgoCD cluster Secret carries this label, and generators in the
		// wild routinely select on it. Adding it if the export omitted it keeps
		// those selectors matching.
		if _, ok := inv.Clusters[i].Labels["argocd.argoproj.io/secret-type"]; !ok {
			inv.Clusters[i].Labels["argocd.argoproj.io/secret-type"] = "cluster"
		}
	}
	sort.Slice(inv.Clusters, func(i, j int) bool { return inv.Clusters[i].Name < inv.Clusters[j].Name })
	return &inv, nil
}

// TemplateData is what an ApplicationSet cluster generator exposes to its
// template for one cluster, in goTemplate mode.
func (c Cluster) TemplateData(values map[string]string) map[string]any {
	md := map[string]any{
		"labels":      toAny(c.Labels),
		"annotations": toAny(c.Annotations),
	}
	d := map[string]any{
		"name":           c.Name,
		"server":         c.Server,
		"metadata":       md,
		"nameNormalized": c.Name,
	}
	if values != nil {
		d["values"] = toAny(values)
	}
	return d
}

func toAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
