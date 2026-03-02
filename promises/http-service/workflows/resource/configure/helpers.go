package main

import (
	"fmt"

	kratix "github.com/syntasso/kratix-go"
)

func getStringValue(resource kratix.Resource, path string) (string, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return "", err
	}
	if str, ok := val.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("%s is not a string", path)
}

func getStringValueWithDefault(resource kratix.Resource, path, defaultValue string) (string, error) {
	val, err := getStringValue(resource, path)
	if err != nil || val == "" {
		return defaultValue, nil
	}
	return val, nil
}

func getIntValueWithDefault(resource kratix.Resource, path string, defaultValue int) (int, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return defaultValue, nil
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	}
	return defaultValue, nil
}

func getIntValue(resource kratix.Resource, path string) (int, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return 0, err
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	}
	return 0, fmt.Errorf("value at %s is not an integer", path)
}

func getBoolValue(resource kratix.Resource, path string) (bool, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("value at %s is not a bool", path)
}

func getBoolValueWithDefault(resource kratix.Resource, path string, defaultValue bool) (bool, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return defaultValue, nil
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return defaultValue, nil
}

func extractStringMap(resource kratix.Resource, path string) map[string]string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractStringSlice(resource kratix.Resource, path string) []string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if str, ok := v.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func extractSecrets(resource kratix.Resource) []SecretRef {
	val, err := resource.GetValue("spec.secrets")
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}

	var secrets []SecretRef
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		s := SecretRef{}
		if name, ok := m["name"].(string); ok {
			s.Name = name
		}
		if opItem, ok := m["onePasswordItem"].(string); ok {
			s.OnePasswordItem = opItem
		}

		if keys, ok := m["keys"].([]interface{}); ok {
			for _, kItem := range keys {
				km, ok := kItem.(map[string]interface{})
				if !ok {
					continue
				}
				sk := SecretKey{}
				if v, ok := km["secretKey"].(string); ok {
					sk.SecretKey = v
				}
				if v, ok := km["property"].(string); ok {
					sk.Property = v
				}
				s.Keys = append(s.Keys, sk)
			}
		}

		secrets = append(secrets, s)
	}

	return secrets
}

// deepMerge merges src into dst recursively. src values win on conflicts.
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if existing, ok := result[k]; ok {
			if existingMap, okE := existing.(map[string]interface{}); okE {
				if srcMap, okS := v.(map[string]interface{}); okS {
					result[k] = deepMerge(existingMap, srcMap)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
}
