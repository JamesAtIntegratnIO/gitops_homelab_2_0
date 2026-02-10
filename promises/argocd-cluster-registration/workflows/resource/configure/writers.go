package main

import (
	"bytes"
	"fmt"

	kratix "github.com/syntasso/kratix-go"
	"sigs.k8s.io/yaml"
)

func writeYAML(sdk *kratix.KratixSDK, path string, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := sdk.WriteOutput(path, data); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}

func writeYAMLDocuments(sdk *kratix.KratixSDK, path string, docs []Resource) error {
	if len(docs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for i, doc := range docs {
		data, err := yaml.Marshal(doc)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", path, err)
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
