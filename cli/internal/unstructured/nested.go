// Package unstructured provides helpers for accessing nested fields in
// Kubernetes unstructured objects (map[string]interface{}).
package unstructured

// NestedField traverses a nested map using the given field path and returns
// the value found. It returns (nil, false, nil) if any intermediate key is
// missing or not a map.
func NestedField(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var current interface{} = obj
	for _, f := range fields {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}

// NestedString extracts a string from a nested unstructured object.
func NestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	val, found, err := NestedField(obj, fields...)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := val.(string)
	return s, ok, nil
}

// NestedSlice extracts a slice from a nested unstructured object.
func NestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	val, found, err := NestedField(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}
	s, ok := val.([]interface{})
	return s, ok, nil
}

// NestedInt64 extracts an int64 from a nested unstructured object.
// It handles int64, float64, and int types.
func NestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := NestedField(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}
	switch v := val.(type) {
	case int64:
		return v, true, nil
	case float64:
		return int64(v), true, nil
	case int:
		return int64(v), true, nil
	default:
		return 0, false, nil
	}
}

// NestedBool extracts a bool from a nested unstructured object.
func NestedBool(obj map[string]interface{}, fields ...string) (bool, bool, error) {
	val, found, err := NestedField(obj, fields...)
	if !found || err != nil {
		return false, found, err
	}
	b, ok := val.(bool)
	return b, ok, nil
}

// MustString returns the nested string value or "" if missing/wrong type.
// It is a convenience wrapper around NestedString for callers that don't
// need the found/error returns.
func MustString(obj map[string]interface{}, fields ...string) string {
	val, _, _ := NestedString(obj, fields...)
	return val
}

// MustSlice returns the nested slice value or nil if missing/wrong type.
// It is a convenience wrapper around NestedSlice for callers that don't
// need the found/error returns.
func MustSlice(obj map[string]interface{}, fields ...string) []interface{} {
	val, _, _ := NestedSlice(obj, fields...)
	return val
}
