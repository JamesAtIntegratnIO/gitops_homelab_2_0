package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Object is one Kubernetes resource that will exist in a cluster, identified
// the way the API server identifies it.
//
// This is a different and stronger signal than the Application table. Knowing
// that a chart moved from 0.15.2 to 0.16.0 tells you a version changed;
// knowing that the change removes two containers and adds a DaemonSet, four
// CRDs and a webhook tells you what will happen. Every incident this gate was
// built for was of the second kind -- a pull request that renders fine and
// breaks at runtime -- and none of them are visible at Application level.
type Object struct {
	Source     string `json:"source"`
	Cluster    string `json:"cluster,omitempty"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	// Hash is of the whole object, so a changed field is detectable without
	// storing the object itself. The table is an artifact a pull request
	// carries around; embedding every manifest would make it enormous.
	Hash string `json:"hash"`
}

// ID identifies an object across revisions. Deliberately excludes apiVersion:
// a resource whose API version moved is the SAME resource being migrated, and
// reporting it as one removal plus one addition hides exactly the migration a
// reviewer needs to see.
func (o Object) ID() string {
	return o.Cluster + "|" + o.Kind + "|" + o.Namespace + "|" + o.Name
}

func (o Object) Describe() string {
	base := fmt.Sprintf("%s/%s", o.Kind, o.Name)
	if o.Namespace != "" {
		base += " in " + o.Namespace
	}
	return base
}

// isTestHook reports whether an object is a Helm test hook.
//
// Test hooks are never applied by a sync -- they run on `helm test` and
// nothing else -- so reporting them as deployed resources is wrong on its own.
// They are also the one place charts routinely generate a random name, so
// every render produces a different one and the diff shows the same three
// pods added and removed on every single bump. Other hooks (pre-install,
// post-upgrade) ARE applied, and are deliberately still reported.
func isTestHook(meta map[string]any) bool {
	ann, _ := meta["annotations"].(map[string]any)
	h, _ := ann["helm.sh/hook"].(string)
	for _, part := range strings.Split(h, ",") {
		switch strings.TrimSpace(part) {
		case "test", "test-success", "test-failure":
			return true
		}
	}
	return false
}

// objectFrom builds an Object, returning false for anything that is not a
// Kubernetes resource or is not something a sync would apply.
//
// defaultNS is the Application's destination namespace. A namespaced resource
// whose manifest omits `metadata.namespace` lands there when ArgoCD applies
// it, so that is its real identity -- and whether a chart stamps the field at
// all varies BETWEEN VERSIONS OF THE SAME CHART. podinfo 6.7.0 omits it and
// 6.14.1 sets it, which made every object in the chart read as one removal
// plus one addition rather than a change.
func objectFrom(source, cluster, defaultNS string, obj map[string]any) (Object, bool) {
	kind, _ := obj["kind"].(string)
	apiVersion, _ := obj["apiVersion"].(string)
	if kind == "" || apiVersion == "" {
		return Object{}, false
	}
	meta, _ := obj["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	if name == "" {
		return Object{}, false
	}
	if isTestHook(meta) {
		return Object{}, false
	}
	ns, _ := meta["namespace"].(string)
	if ns == "" {
		ns = defaultNS
	}

	raw, err := yaml.Marshal(normalise(obj))
	if err != nil {
		return Object{}, false
	}
	sum := sha256.Sum256(raw)

	return Object{
		Source: source, Cluster: cluster,
		APIVersion: apiVersion, Kind: kind, Namespace: ns, Name: name,
		Hash: hex.EncodeToString(sum[:8]),
	}, true
}

// ObjectChange is one difference between two renders of the same object set.
type ObjectChange struct {
	Kind    string `json:"kind"` // added | removed | changed | apiVersion
	Object  string `json:"object"`
	Cluster string `json:"cluster,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
}

// diffObjects compares two object sets.
func diffObjects(base, head []Object) []ObjectChange {
	byID := func(in []Object) map[string]Object {
		m := make(map[string]Object, len(in))
		for _, o := range in {
			m[o.ID()] = o
		}
		return m
	}
	b, h := byID(base), byID(head)

	var out []ObjectChange
	for id, o := range h {
		prev, ok := b[id]
		switch {
		case !ok:
			out = append(out, ObjectChange{Kind: "added", Object: o.Describe(), Cluster: o.Cluster, To: o.APIVersion})
		case prev.APIVersion != o.APIVersion:
			// Called out separately because it is never routine: an API
			// version moving under a resource is a migration, and it is the
			// single most reliable signal that a bump needs a human.
			out = append(out, ObjectChange{
				Kind: "apiVersion", Object: o.Describe(), Cluster: o.Cluster,
				From: prev.APIVersion, To: o.APIVersion,
			})
		case prev.Hash != o.Hash:
			out = append(out, ObjectChange{Kind: "changed", Object: o.Describe(), Cluster: o.Cluster})
		}
	}
	for id, o := range b {
		if _, ok := h[id]; !ok {
			out = append(out, ObjectChange{Kind: "removed", Object: o.Describe(), Cluster: o.Cluster, From: o.APIVersion})
		}
	}

	// The same resource changing identically on several clusters is one
	// finding, not one per cluster. Collapse them and say where.
	collapsed := map[string]*ObjectChange{}
	var order []string
	for i := range out {
		c := out[i]
		k := c.Kind + "|" + c.Object + "|" + c.From + "|" + c.To
		if prev, ok := collapsed[k]; ok {
			if c.Cluster != "" && !strings.Contains(prev.Cluster, c.Cluster) {
				prev.Cluster += ", " + c.Cluster
			}
			continue
		}
		cp := c
		collapsed[k] = &cp
		order = append(order, k)
	}
	out = out[:0]
	for _, k := range order {
		out = append(out, *collapsed[k])
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Object < out[j].Object
	})
	return out
}

// versionStamps are labels Helm writes into EVERY object it renders, carrying
// the chart and app version.
//
// Hashing them makes a version bump report every single resource as changed --
// measured at 101 of 105 on one cert-manager bump -- which buries the four
// that actually changed. They are stripped before hashing, so "changed" means
// something a reader should look at. The version itself is already reported,
// in the Versions table, once.
var versionStamps = []string{
	"helm.sh/chart",
	"app.kubernetes.io/version",
	"app.kubernetes.io/managed-by",
	"chart",
}

// normalise returns a copy of an object with version stamps removed.
func normalise(obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return out
	}
	newMeta := make(map[string]any, len(meta))
	for k, v := range meta {
		newMeta[k] = v
	}
	for _, field := range []string{"labels", "annotations"} {
		m, ok := meta[field].(map[string]any)
		if !ok {
			continue
		}
		cleaned := make(map[string]any, len(m))
		for k, v := range m {
			stamp := false
			for _, s := range versionStamps {
				if k == s {
					stamp = true
					break
				}
			}
			if !stamp {
				cleaned[k] = v
			}
		}
		newMeta[field] = cleaned
	}
	out["metadata"] = newMeta
	return out
}
