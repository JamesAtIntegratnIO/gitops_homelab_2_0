package kratixutil

import (
	"fmt"
	"log"
	"strconv"

	kratix "github.com/syntasso/kratix-go"
)

// ---------------------------------------------------------------------------
// Resource-level accessors: convenience wrappers around kratix.Resource.GetValue
// that return typed Go values. Each has a "WithDefault" variant for optional
// fields.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Legacy Resource-based collection extractors. These delegate to the
// error-returning E variants in extractors.go and log warnings on type
// mismatches. Prefer the E variants in new code.
// ---------------------------------------------------------------------------

func ExtractStringMap(resource kratix.Resource, path string) map[string]string {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return nil
	}
	wrapper := map[string]interface{}{"_v": val}
	result, err := ExtractStringMapE(wrapper, "_v")
	if err != nil {
		log.Printf("warning: field %s has unexpected type %T, expected map[string]interface{}", path, val)
		return nil
	}
	return result
}

func ExtractStringSlice(resource kratix.Resource, path string) []string {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return nil
	}
	wrapper := map[string]interface{}{"_v": val}
	result, err := ExtractStringSliceE(wrapper, "_v")
	if err != nil {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
		return nil
	}
	return result
}

func ExtractObjectSlice(resource kratix.Resource, path string) []map[string]interface{} {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return nil
	}
	wrapper := map[string]interface{}{"_v": val}
	result, err := ExtractObjectSliceE(wrapper, "_v")
	if err != nil {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
		return nil
	}
	return result
}

func ExtractSecrets(resource kratix.Resource, path string) []SecretRef {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return nil
	}
	wrapper := map[string]interface{}{"_v": val}
	result, err := ExtractSecretsE(wrapper, "_v")
	if err != nil {
		log.Printf("warning: field %s has unexpected type %T, expected []interface{}", path, val)
		return nil
	}
	return result
}
