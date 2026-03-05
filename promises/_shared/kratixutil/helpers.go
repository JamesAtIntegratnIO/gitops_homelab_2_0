package kratixutil

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	kratix "github.com/syntasso/kratix-go"
)

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

// GetStringValueWithDefault returns defaultValue when the path is missing, empty,
// or the literal string "null". The "null" check is required because Kratix
// resources may serialize a Go nil pointer as the YAML scalar "null", which
// round-trips through JSON unmarshalling as the string "null" rather than a
// Go nil/empty-string.
func GetStringValueWithDefault(resource kratix.Resource, path, defaultValue string) string {
	val, err := GetStringValue(resource, path)
	if err != nil || val == "" || val == "null" {
		return defaultValue
	}
	return val
}

// GetIntValue handles int, int64, float64, and string (via strconv.Atoi) representations.
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

// GetIntValueWithDefault returns defaultValue only when the path is missing or not parseable;
// an explicit 0 is returned as-is.
func GetIntValueWithDefault(resource kratix.Resource, path string, defaultValue int) int {
	val, err := GetIntValue(resource, path)
	if err != nil {
		return defaultValue
	}
	return val
}

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

func GetBoolValueWithDefault(resource kratix.Resource, path string, defaultValue bool) bool {
	val, err := GetBoolValue(resource, path)
	if err != nil {
		return defaultValue
	}
	return val
}

func ExtractStringMap(resource kratix.Resource, path string) map[string]string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	if val == nil {
		return nil
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		log.Printf("warning: field %s has unexpected type %T, expected map[string]interface{}", path, val)
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

func ExtractStringSlice(resource kratix.Resource, path string) []string {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	if val == nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
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

func ExtractObjectSlice(resource kratix.Resource, path string) []map[string]interface{} {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	if val == nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
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

func ExtractSecrets(resource kratix.Resource, path string) []SecretRef {
	val, err := resource.GetValue(path)
	if err != nil {
		return nil
	}
	if val == nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
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

// ToMap converts a struct to map[string]interface{} via JSON roundtrip.
// Useful at the merge boundary where typed structs meet DeepMerge.
func ToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("toMap marshal: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("toMap unmarshal: %w", err)
	}
	return m, nil
}

// DeleteFromResource strips a Resource down to its identity fields
// (apiVersion, kind, name, namespace) for use as a Kratix delete output.
func DeleteFromResource(r Resource) Resource {
	return Resource{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Metadata: ObjectMeta{
			Name:      r.Metadata.Name,
			Namespace: r.Metadata.Namespace,
		},
	}
}

// DeleteOutputPathForResource computes the output path using the standard
// "resources/delete-{kind}-{name}.yaml" pattern.
func DeleteOutputPathForResource(prefix string, r Resource) string {
	if prefix == "" {
		prefix = "resources/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return fmt.Sprintf("%sdelete-%s-%s.yaml", prefix, strings.ToLower(r.Kind), r.Metadata.Name)
}

// WritePromiseStatus builds a Kratix status object, sets the given phase and
// message, applies any extra fields, and writes it via the SDK. This reduces
// repetitive status-setting boilerplate across promise handlers.
func WritePromiseStatus(sdk *kratix.KratixSDK, phase, message string, fields map[string]interface{}) error {
	status := kratix.NewStatus()
	status.Set("phase", phase)
	status.Set("message", message)
	for k, v := range fields {
		status.Set(k, v)
	}
	return sdk.WriteStatus(status)
}
