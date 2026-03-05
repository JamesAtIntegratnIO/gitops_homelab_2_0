package kratixutil

import (
	"fmt"
	"strconv"

	kratix "github.com/syntasso/kratix-go"
)

// ============================================================================
// Value Extraction Helpers
// ============================================================================

// GetStringValue extracts a string from a Kratix resource at the given path.
func GetStringValue(resource kratix.Resource, path string) (string, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return "", err
	}
	if str, ok := val.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("%s is not a string", path)
}

// GetStringValueWithDefault extracts a string or returns the default.
// Also treats "null" (YAML null rendered as string) as empty.
func GetStringValueWithDefault(resource kratix.Resource, path, defaultValue string) string {
	val, err := GetStringValue(resource, path)
	if err != nil || val == "" || val == "null" {
		return defaultValue
	}
	return val
}

// GetIntValue extracts an integer from a Kratix resource at the given path.
// Handles int, int64, float64, and string (via strconv.Atoi) representations.
func GetIntValue(resource kratix.Resource, path string) (int, error) {
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
	case string:
		return strconv.Atoi(v)
	}
	return 0, fmt.Errorf("value at %s is not an integer", path)
}

// GetIntValueWithDefault extracts an integer or returns the default.
// Returns defaultValue only when the path is missing or not parseable;
// an explicit 0 is returned as-is.
func GetIntValueWithDefault(resource kratix.Resource, path string, defaultValue int) int {
	val, err := GetIntValue(resource, path)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetBoolValue extracts a boolean from a Kratix resource at the given path.
func GetBoolValue(resource kratix.Resource, path string) (bool, error) {
	val, err := resource.GetValue(path)
	if err != nil {
		return false, err
	}
	if b, ok := val.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("value at %s is not a bool", path)
}

// GetBoolValueWithDefault extracts a boolean or returns the default.
func GetBoolValueWithDefault(resource kratix.Resource, path string, defaultValue bool) bool {
	val, err := resource.GetValue(path)
	if err != nil {
		return defaultValue
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return defaultValue
}

// ============================================================================
// Collection Extractors
// ============================================================================

// ExtractStringMap extracts a map[string]string from the given path.
func ExtractStringMap(resource kratix.Resource, path string) map[string]string {
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

// ExtractStringSlice extracts a []string from the given path.
func ExtractStringSlice(resource kratix.Resource, path string) []string {
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

// ExtractObjectSlice extracts a []map[string]interface{} from the given path.
func ExtractObjectSlice(resource kratix.Resource, path string) []map[string]interface{} {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(arr))
	for _, v := range arr {
		if obj, ok := v.(map[string]interface{}); ok {
			result = append(result, obj)
		}
	}
	return result
}

// ExtractSecrets extracts a slice of SecretRef from the standard secrets path.
func ExtractSecrets(resource kratix.Resource, path string) []SecretRef {
	val, err := resource.GetValue(path)
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

// ============================================================================
// Map Utilities
// ============================================================================

// DeepMerge merges src into dst recursively. src values win on conflicts.
// For map values, merging recurses. For non-map or mismatched types, src wins.
func DeepMerge(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = map[string]interface{}{}
	}
	if src == nil {
		return dst
	}
	result := make(map[string]interface{})
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = DeepMerge(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// MergeStringMap merges src into dst without mutating either input. src values win.
func MergeStringMap(dst, src map[string]string) map[string]string {
	result := make(map[string]string, len(dst)+len(src))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		result[k] = v
	}
	return result
}
