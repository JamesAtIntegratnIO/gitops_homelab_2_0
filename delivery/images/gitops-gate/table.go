package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Row is one generated Application, normalized so two renders can be compared
// regardless of field ordering, whitespace, or where in the values layers a
// setting came from.
type Row struct {
	AppSet     string `json:"appset"`
	Cluster    string `json:"cluster"`
	App        string `json:"app"`
	Project    string `json:"project"`
	Namespace  string `json:"namespace"`
	SourceType string `json:"sourceType"` // helm | path | manifest
	ChartRepo  string `json:"chartRepo"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Path       string `json:"path"`
}

// Key identifies an Application across renders. Deliberately excludes Version:
// a version change is an expected, reportable event, whereas a change to this
// key means the Application itself moved, appeared or vanished.
func (r Row) Key() string {
	return r.Cluster + "\x00" + r.App
}

type Table struct {
	Rows []Row `json:"rows"`
	// Objects are the Kubernetes resources a source produced directly --
	// from a rendered-manifests branch, or from any source whose output is
	// not itself an Application. Empty when nothing in the repository is
	// rendered, which is the common case and is why the object diff is
	// reported only when there is something to report.
	Objects  []Object `json:"objects,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (t *Table) Sort() {
	sort.Slice(t.Objects, func(i, j int) bool { return t.Objects[i].ID() < t.Objects[j].ID() })
	sort.Slice(t.Rows, func(i, j int) bool {
		if t.Rows[i].Cluster != t.Rows[j].Cluster {
			return t.Rows[i].Cluster < t.Rows[j].Cluster
		}
		return t.Rows[i].App < t.Rows[j].App
	})
}

func (t *Table) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(t)
}

func WriteTableFile(path string, t *Table) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	if err := t.WriteJSON(bw); err != nil {
		return err
	}
	return bw.Flush()
}

func ReadTableFile(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading target table %s: %w", path, err)
	}
	var t Table
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("parsing target table %s: %w", path, err)
	}
	return &t, nil
}

// Describe renders a row's source in one human-readable string.
func (r Row) Describe() string {
	switch r.SourceType {
	case "helm":
		repo := r.ChartRepo
		if repo != "" && !strings.HasSuffix(repo, "/") {
			repo += "/"
		}
		return fmt.Sprintf("%s%s %s", repo, r.Chart, r.Version)
	case "path", "manifest":
		return fmt.Sprintf("%s (%s)", r.Path, r.SourceType)
	default:
		return r.SourceType
	}
}
