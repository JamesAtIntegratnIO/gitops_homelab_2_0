package cmd

import (
	"strings"
	"testing"

	"github.com/jamesatintegratnio/hctl/internal/kube"
)

func TestFormatNodesTable_MultipleNodes(t *testing.T) {
	nodes := []kube.NodeInfo{
		{
			Name:   "node-1",
			Ready:  true,
			IP:     "10.0.4.101",
			Roles:  []string{"control-plane"},
			CPU:    "4",
			Memory: "16Gi",
		},
		{
			Name:   "node-2",
			Ready:  false,
			IP:     "10.0.4.102",
			Roles:  []string{"control-plane", "worker"},
			CPU:    "8",
			Memory: "32Gi",
		},
	}

	out := formatNodesTable(nodes)

	// Headers should be present
	if !strings.Contains(out, "NAME") {
		t.Error("expected NAME header")
	}
	if !strings.Contains(out, "READY") {
		t.Error("expected READY header")
	}
	if !strings.Contains(out, "IP") {
		t.Error("expected IP header")
	}
	if !strings.Contains(out, "ROLES") {
		t.Error("expected ROLES header")
	}

	// Node data
	if !strings.Contains(out, "node-1") {
		t.Error("expected node-1 in output")
	}
	if !strings.Contains(out, "10.0.4.101") {
		t.Error("expected IP 10.0.4.101 in output")
	}
	if !strings.Contains(out, "node-2") {
		t.Error("expected node-2 in output")
	}
	if !strings.Contains(out, "16Gi") {
		t.Error("expected memory 16Gi in output")
	}
	if !strings.Contains(out, "control-plane,worker") {
		t.Error("expected joined roles in output")
	}
}

func TestFormatNodesTable_Empty(t *testing.T) {
	out := formatNodesTable(nil)
	if out == "" {
		t.Error("expected some output even for empty list (e.g. no data message)")
	}
	// tui.Table with empty rows returns "(no data)" dim text
	if !strings.Contains(out, "no data") {
		t.Error("expected 'no data' indicator for empty nodes")
	}
}

func TestFormatNodesTable_SingleNode(t *testing.T) {
	nodes := []kube.NodeInfo{
		{
			Name:   "solo",
			Ready:  true,
			IP:     "192.168.1.1",
			Roles:  []string{"control-plane"},
			CPU:    "2",
			Memory: "8Gi",
		},
	}

	out := formatNodesTable(nodes)
	if !strings.Contains(out, "solo") {
		t.Error("expected node name 'solo' in output")
	}
	if !strings.Contains(out, "192.168.1.1") {
		t.Error("expected IP in output")
	}
}
