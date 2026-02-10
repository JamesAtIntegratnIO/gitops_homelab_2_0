package main

import (
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
