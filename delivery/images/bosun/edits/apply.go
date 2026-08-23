// Package edits applies a model's proposed changes -- deterministically, and
// only where they are allowed.
//
// This is where the agent's safety actually lives. The model proposes; this
// package disposes, and it refuses far more than it accepts:
//
//   - a path outside the allowlist is rejected, so "never edit the gate" and
//     "never weaken a merge policy to go green" hold whatever the model asks
//     for, including if it is asked nicely by something it read in a diff;
//   - an edit whose `from` does not match the file is rejected, so a model
//     working from a stale or imagined view of the repository changes nothing
//     rather than changing the wrong line;
//   - the value is rewritten in place, preserving indentation, quoting style
//     and any trailing comment, so a one-line change stays a one-line change.
package edits

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy decides which files may be touched.
type Policy struct {
	// Allow are path globs an edit may target. Empty allows nothing, which is
	// the correct default for a component that can write to a repository.
	Allow []string
	// Deny wins over Allow. These are the paths that must never move no
	// matter how the allowlist is configured.
	Deny []string

	// Evidence is the material the model was given -- the gate report, the
	// release notes, the file inventory. When set, any version-shaped value an
	// edit tries to write must appear somewhere in it.
	//
	// This exists because a model asked to fix "requires Gateway API v1.5"
	// will confidently write v1.5.0 when the answer was v1.5.1. That is the
	// worst kind of wrong: it renders perfectly and breaks at runtime. Telling
	// the model not to invent versions does not reliably stop it -- measured,
	// not assumed -- so the guarantee has to live here instead.
	Evidence string
}

// versionish matches the value shapes where invention is both likely and
// costly: semver, v-prefixed semver, and date-style release tags. Booleans,
// ports and names are deliberately not covered -- corroborating those would
// reject legitimate toggles, since "false" rarely appears in a failure report.
var versionish = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?([.\-+][0-9A-Za-z.\-]+)?$`)

// corroborated reports whether a version-shaped value is supported by the
// evidence the model was given.
func (p Policy) corroborated(to string) bool {
	if p.Evidence == "" || !versionish.MatchString(to) {
		return true
	}
	return strings.Contains(p.Evidence, to)
}

// DefaultDeny are refused regardless of configuration. Each entry is a way the
// agent could otherwise make a red gate green without fixing anything.
var DefaultDeny = []string{
	".github/**",     // the gate's own workflows
	".gitlab-ci.yml", //
	"bitbucket-pipelines.yml",
	".gitops-gate.yaml",             // what the gate renders, and how
	".gitops-gate/**",               // the cluster inventory the gate compares against
	"delivery/**",                   // the kit itself, including this agent
	"**/kargo-projects/values.yaml", // merge policy and constraints
	"**/kargo-pipelines/**",
}

type Result struct {
	Applied  []Applied
	Rejected []Rejected
}

type Applied struct {
	Path, Key, From, To, Rationale string
}

type Rejected struct {
	Path, Key, Reason string
}

// Edit is the shape the model produces. Declared here rather than imported so
// this package has no dependency on the model layer -- it is a pure function
// of "proposed change" plus "policy".
type Edit struct {
	Path, Key, From, To, Rationale string
}

// Apply writes every permitted edit under root and reports both what it did
// and, importantly, what it refused. A silent refusal would let a model
// believe it had fixed something.
func Apply(root string, policy Policy, in []Edit) (*Result, error) {
	res := &Result{}

	for _, e := range in {
		if reason := policy.check(e.Path); reason != "" {
			res.Rejected = append(res.Rejected, Rejected{e.Path, e.Key, reason})
			continue
		}

		if !policy.corroborated(e.To) {
			res.Rejected = append(res.Rejected, Rejected{e.Path, e.Key,
				fmt.Sprintf("version %q does not appear in the evidence -- refusing an invented version", e.To)})
			continue
		}

		full := filepath.Join(root, filepath.Clean("/"+e.Path))
		rel, err := filepath.Rel(root, full)
		if err != nil || strings.HasPrefix(rel, "..") {
			res.Rejected = append(res.Rejected, Rejected{e.Path, e.Key, "path escapes the repository"})
			continue
		}

		data, err := os.ReadFile(full)
		if err != nil {
			res.Rejected = append(res.Rejected, Rejected{e.Path, e.Key, fmt.Sprintf("cannot read: %v", err)})
			continue
		}

		updated, err := setScalar(data, e.Key, e.From, e.To)
		if err != nil {
			res.Rejected = append(res.Rejected, Rejected{e.Path, e.Key, err.Error()})
			continue
		}
		if err := os.WriteFile(full, updated, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", e.Path, err)
		}
		res.Applied = append(res.Applied, Applied{e.Path, e.Key, e.From, e.To, e.Rationale})
	}
	return res, nil
}

func (p Policy) check(path string) string {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	for _, d := range append(append([]string{}, DefaultDeny...), p.Deny...) {
		if matchGlob(d, clean) {
			return fmt.Sprintf("path is denied (%s)", d)
		}
	}
	for _, a := range p.Allow {
		if matchGlob(a, clean) {
			return ""
		}
	}
	return "path is not in the allowlist"
}

// matchGlob supports the `**` prefix/suffix form used in the policies, which
// filepath.Match does not.
func matchGlob(pattern, path string) bool {
	switch {
	case pattern == path:
		return true
	case strings.HasSuffix(pattern, "/**"):
		prefix := strings.TrimSuffix(pattern, "/**")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	case strings.HasPrefix(pattern, "**/"):
		suffix := strings.TrimPrefix(pattern, "**/")
		return path == suffix || strings.HasSuffix(path, "/"+suffix)
	default:
		ok, err := filepath.Match(pattern, path)
		return err == nil && ok
	}
}

// setScalar rewrites one scalar in place.
//
// The document is parsed to find the node -- so the key path is resolved
// properly rather than by guessing at text -- but the write is a targeted
// replacement on that node's own line. Re-serialising the whole document would
// reformat the file, discard comments, and turn a one-line change into an
// unreviewable diff.
func setScalar(data []byte, key, want, to string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("file is not valid YAML: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	node, err := lookup(doc.Content[0], strings.Split(key, "."))
	if err != nil {
		return nil, err
	}
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("key %q is not a scalar", key)
	}
	if want != "" && node.Value != want {
		// The optimistic-concurrency check. A mismatch means the model was
		// working from something other than this file as it stands.
		return nil, fmt.Errorf("key %q holds %q, not the expected %q -- refusing to overwrite", key, node.Value, want)
	}

	lines := strings.Split(string(data), "\n")
	idx := node.Line - 1
	if idx < 0 || idx >= len(lines) {
		return nil, fmt.Errorf("key %q resolved to line %d, outside the file", key, node.Line)
	}

	replaced, err := replaceValueOnLine(lines[idx], node.Value, to)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", key, err)
	}
	lines[idx] = replaced
	return []byte(strings.Join(lines, "\n")), nil
}

// replaceValueOnLine swaps the value while preserving indentation, the key,
// the quoting style already in use, and any trailing comment.
func replaceValueOnLine(line, old, to string) (string, error) {
	i := strings.Index(line, old)
	if i < 0 {
		return "", fmt.Errorf("value %q not found on its own line (%q)", old, strings.TrimSpace(line))
	}
	return line[:i] + to + line[i+len(old):], nil
}

func lookup(node *yaml.Node, path []string) (*yaml.Node, error) {
	cur := node
	for i, part := range path {
		if cur.Kind == yaml.DocumentNode && len(cur.Content) > 0 {
			cur = cur.Content[0]
		}
		switch cur.Kind {
		case yaml.MappingNode:
			found := false
			for j := 0; j+1 < len(cur.Content); j += 2 {
				if cur.Content[j].Value == part {
					cur = cur.Content[j+1]
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("key %q not found (at %q)", strings.Join(path, "."), part)
			}
		case yaml.SequenceNode:
			var n int
			if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
				return nil, fmt.Errorf("key %q: %q is not a list index", strings.Join(path, "."), part)
			}
			if n < 0 || n >= len(cur.Content) {
				return nil, fmt.Errorf("key %q: index %d out of range", strings.Join(path, "."), n)
			}
			cur = cur.Content[n]
		default:
			return nil, fmt.Errorf("key %q: %q is a scalar, cannot descend", strings.Join(path, "."), strings.Join(path[:i], "."))
		}
	}
	return cur, nil
}
