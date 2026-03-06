package kratixutil

import (
	"bytes"
	"fmt"
	"sort"

	kratix "github.com/syntasso/kratix-go"
	"sigs.k8s.io/yaml"
)

// WriteOrderedResources writes a map of path→Resource to the Kratix output
// directory in deterministic (sorted-key) order. This is the standard pattern
// for emitting multiple resources from a promise pipeline.
func WriteOrderedResources(sdk *kratix.KratixSDK, resources map[string]Resource) error {
	paths := make([]string, 0, len(resources))
	for p := range resources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := WriteYAML(sdk, path, resources[path]); err != nil {
			return err
		}
	}
	return nil
}

func WriteYAML(sdk *kratix.KratixSDK, path string, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := sdk.WriteOutput(path, data); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}

// WriteYAMLDocuments marshals multiple Resource objects into a single
// multi-document YAML file separated by "---".
func WriteYAMLDocuments(sdk *kratix.KratixSDK, path string, docs []Resource) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for i, doc := range docs {
		data, err := yaml.Marshal(doc)
		if err != nil {
			return fmt.Errorf("marshal %s doc %d: %w", path, i, err)
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(data)
	}

	if err := sdk.WriteOutput(path, buf.Bytes()); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}
