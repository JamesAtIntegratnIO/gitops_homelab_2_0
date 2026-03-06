package kratixutil

import (
	"encoding/json"
	"fmt"
	"strings"

	kratix "github.com/syntasso/kratix-go"
)

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

// ParseSyncPolicyE converts an untyped value (from resource.GetValue) into a typed
// SyncPolicy via JSON round-trip. Returns an error if the value cannot be
// deserialized into SyncPolicy.
func ParseSyncPolicyE(raw interface{}) (*SyncPolicy, error) {
	if raw == nil {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("syncPolicy: failed to serialize: %w", err)
	}
	var sp SyncPolicy
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("syncPolicy: %w", err)
	}
	return &sp, nil
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
