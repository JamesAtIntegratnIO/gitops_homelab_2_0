package main

import (
	"testing"
)

func TestVclusterRBACName(t *testing.T) {
	tests := []struct {
		name      string
		vcName    string
		namespace string
		want      string
	}{
		{
			name:      "simple",
			vcName:    "media",
			namespace: "vcluster-media",
			want:      "vc-media-v-vcluster-media",
		},
		{
			name:      "with hyphens",
			vcName:    "my-cluster",
			namespace: "vcluster-my-cluster",
			want:      "vc-my-cluster-v-vcluster-my-cluster",
		},
		{
			name:      "short names",
			vcName:    "a",
			namespace: "b",
			want:      "vc-a-v-b",
		},
		{
			name:      "test-vc from minimalConfig",
			vcName:    "test-vc",
			namespace: "vcluster-test-vc",
			want:      "vc-test-vc-v-vcluster-test-vc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vclusterRBACName(tt.vcName, tt.namespace)
			if got != tt.want {
				t.Errorf("vclusterRBACName(%q, %q) = %q, want %q", tt.vcName, tt.namespace, got, tt.want)
			}
		})
	}
}
