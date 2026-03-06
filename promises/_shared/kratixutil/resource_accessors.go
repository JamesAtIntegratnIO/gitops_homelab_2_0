package kratixutil

import (
	"fmt"
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
//
// Design note: this intentionally does not distinguish between a missing field
// and a wrong-type field — both return defaultValue. This suits optional
// configuration fields where callers only need the resolved value.
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
//
// Design note: this intentionally does not distinguish between a missing field
// and a wrong-type field — both return defaultValue. This suits optional
// configuration fields where callers only need the resolved value.
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

// GetBoolValueWithDefault returns defaultValue when the path is missing or the
// value is not a bool.
//
// Design note: this intentionally does not distinguish between a missing field
// and a wrong-type field — both return defaultValue. This suits optional
// configuration fields where callers only need the resolved value.
func GetBoolValueWithDefault(resource kratix.Resource, path string, defaultValue bool) bool {
	val, err := GetBoolValue(resource, path)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetOptionalStringValue returns (value, nil) if the field exists and is a string,
// ("", nil) if the field is absent, or ("", error) if the field exists but has wrong type.
// Use for optional string fields that should error on type mismatch but not on absence.
func GetOptionalStringValue(resource kratix.Resource, path string) (string, error) {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return "", nil
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("field %s: expected string, got %T", path, val)
	}
	return s, nil
}

// GetOptionalBoolValue returns (value, nil) if the field exists and is a bool,
// (false, nil) if the field is absent, or (false, error) if the field exists but has wrong type.
func GetOptionalBoolValue(resource kratix.Resource, path string) (bool, error) {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return false, nil
	}
	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("field %s: expected bool, got %T", path, val)
	}
	return b, nil
}

// GetOptionalIntValue returns (value, nil) if the field exists and is numeric,
// (0, nil) if the field is absent, or (0, error) if the field exists but has wrong type.
func GetOptionalIntValue(resource kratix.Resource, path string) (int, error) {
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("field %s: expected numeric, got %T", path, val)
	}
}

// ---------------------------------------------------------------------------
// Resource-level typed extraction helpers: bridge kratix.Resource → Extract*E.
// These return (nil, nil) when the path is absent and (nil, error) on type
// mismatch.
// ---------------------------------------------------------------------------

// extractFromResource is a generic helper that bridges kratix.Resource → an
// ExtractE-style function. It eliminates repeated boilerplate across the
// Extract*FromResource family.
func extractFromResource[T any](resource kratix.Resource, path string, extractFn func(map[string]interface{}, string) (T, error)) (T, error) {
	var zero T
	val, err := resource.GetValue(path)
	if err != nil || val == nil {
		return zero, nil
	}
	wrapper := map[string]interface{}{"_v": val}
	result, extractErr := extractFn(wrapper, "_v")
	if extractErr != nil {
		return zero, fmt.Errorf("%s: %w", path, extractErr)
	}
	return result, nil
}

// ExtractStringMapFromResource extracts a map[string]string from a resource at the given path.
// Returns (nil, nil) when the path is absent, (nil, error) on type mismatch.
func ExtractStringMapFromResource(resource kratix.Resource, path string) (map[string]string, error) {
	return extractFromResource(resource, path, ExtractStringMapE)
}

// ExtractStringSliceFromResource extracts a []string from a resource at the given path.
// Returns (nil, nil) when the path is absent, (nil, error) on type mismatch.
func ExtractStringSliceFromResource(resource kratix.Resource, path string) ([]string, error) {
	return extractFromResource(resource, path, ExtractStringSliceE)
}

// ExtractObjectSliceFromResource extracts a []map[string]interface{} from a resource at the given path.
// Returns (nil, nil) when the path is absent, (nil, error) on type mismatch.
func ExtractObjectSliceFromResource(resource kratix.Resource, path string) ([]map[string]interface{}, error) {
	return extractFromResource(resource, path, ExtractObjectSliceE)
}

// ExtractSecretsFromResource extracts a []SecretRef from a resource at the given path.
// Returns (nil, nil) when the path is absent, (nil, error) on type mismatch.
func ExtractSecretsFromResource(resource kratix.Resource, path string) ([]SecretRef, error) {
	return extractFromResource(resource, path, ExtractSecretsE)
}

// ExtractMapFromResource extracts a map[string]interface{} from a resource at the given path.
// Returns (nil, nil) when the path is absent, (nil, error) on type mismatch.
func ExtractMapFromResource(resource kratix.Resource, path string) (map[string]interface{}, error) {
	return extractFromResource(resource, path, ExtractMapE)
}
