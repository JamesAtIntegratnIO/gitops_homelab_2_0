package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

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
	if o.Namespace != "" {
		return fmt.Sprintf("%s/%s in %s", o.Kind, o.Name, o.Namespace)
	}
	return fmt.Sprintf("%s/%s", o.Kind, o.Name)
}

// objectFrom builds an Object, returning false for anything that is not a
// Kubernetes resource.
func objectFrom(source, cluster string, obj map[string]any) (Object, bool) {
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
	ns, _ := meta["namespace"].(string)

	raw, err := yaml.Marshal(obj)
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

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Object < out[j].Object
	})
	return out
}
