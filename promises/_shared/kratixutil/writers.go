package kratixutil

import (
	"bytes"
	"fmt"

	kratix "github.com/syntasso/kratix-go"
	"sigs.k8s.io/yaml"
)

// WriteYAML marshals a single object to YAML and writes it to the Kratix
// output directory at the specified path.
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
