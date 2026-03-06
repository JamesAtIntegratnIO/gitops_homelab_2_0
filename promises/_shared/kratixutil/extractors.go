package kratixutil

import (
	"fmt"
)

// ---------------------------------------------------------------------------
// Extract*E: error-returning helpers for pulling typed values out of a plain
// map[string]interface{} (e.g. a JSON-unmarshalled object). These complement
// the kratix.Resource-based Get*Value helpers in resource_accessors.go.
//
// Semantics:
//   - Key absent or value nil → (zero, nil)        — missing is OK
//   - Key present, wrong type → (zero, error)      — caller can handle
//   - Key present, right type → (value, nil)
// ---------------------------------------------------------------------------

// ExtractStringE returns the string value for key, or an error if the value
// exists but is not a string.
func ExtractStringE(data map[string]interface{}, key string) (string, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("key %q: expected string, got %T", key, v)
	}
	return s, nil
}

// ExtractIntE returns the int value for key. Because JSON unmarshals numbers
// as float64, both int and float64 sources are accepted.
func ExtractIntE(data map[string]interface{}, key string) (int, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	default:
		return 0, fmt.Errorf("key %q: expected int (or float64), got %T", key, v)
	}
}

// ExtractBoolE returns the bool value for key, or an error if the value
// exists but is not a bool.
func ExtractBoolE(data map[string]interface{}, key string) (bool, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("key %q: expected bool, got %T", key, v)
	}
	return b, nil
}

// ExtractMapE returns the nested map for key, or an error if the value exists
// but is not a map[string]interface{}.
func ExtractMapE(data map[string]interface{}, key string) (map[string]interface{}, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q: expected map[string]interface{}, got %T", key, v)
	}
	return m, nil
}

// ExtractStringMapE returns a map[string]string for key, or an error if the
// value exists but is not a map[string]interface{}.
func ExtractStringMapE(data map[string]interface{}, key string) (map[string]string, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q: expected map[string]interface{}, got %T", key, v)
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		str, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("key %q: value for %q expected string, got %T", key, k, val)
		}
		result[k] = str
	}
	return result, nil
}

// ExtractStringSliceE returns a []string for key, or an error if the value
// exists but is not a []interface{}.
func ExtractStringSliceE(data map[string]interface{}, key string) ([]string, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q: expected []interface{}, got %T", key, v)
	}
	result := make([]string, 0, len(arr))
	for i, item := range arr {
		str, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("key %q: element [%d] expected string, got %T", key, i, item)
		}
		result = append(result, str)
	}
	return result, nil
}

// ExtractObjectSliceE returns a []map[string]interface{} for key, or an error
// if the value exists but is not a []interface{}.
func ExtractObjectSliceE(data map[string]interface{}, key string) ([]map[string]interface{}, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q: expected []interface{}, got %T", key, v)
	}
	result := make([]map[string]interface{}, 0, len(arr))
	var skippedCount int
	for _, item := range arr {
		if obj, ok := item.(map[string]interface{}); ok {
			result = append(result, obj)
		} else {
			skippedCount++
		}
	}
	var err error
	if skippedCount > 0 {
		err = fmt.Errorf("key %q: skipped %d non-map item(s)", key, skippedCount)
	}
	return result, err
}

// ExtractSecretsE returns a []SecretRef for key, or an error if the value
// exists but is not a []interface{}.
func ExtractSecretsE(data map[string]interface{}, key string) ([]SecretRef, error) {
	v, ok := data[key]
	if !ok || v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("key %q: expected []interface{}, got %T", key, v)
	}

	var secrets []SecretRef
	var skippedCount int
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			skippedCount++
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
				if sv, ok := km["secretKey"].(string); ok {
					sk.SecretKey = sv
				}
				if sv, ok := km["property"].(string); ok {
					sk.Property = sv
				}
				s.Keys = append(s.Keys, sk)
			}
		}
		secrets = append(secrets, s)
	}
	var retErr error
	if skippedCount > 0 {
		retErr = fmt.Errorf("key %q: skipped %d non-map item(s)", key, skippedCount)
	}
	return secrets, retErr
}
