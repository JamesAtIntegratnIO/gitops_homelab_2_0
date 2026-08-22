package edits

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scalar is one addressable value in a file: the dotted key an Edit must use,
// and the exact current value an Edit must quote in `From`.
type Scalar struct {
	Key   string
	Value string
	Line  int
}

// Inventory lists every scalar in a YAML document, as key/value pairs in the
// exact form an Edit has to use.
//
// This is the single biggest lever on making a small model usable here. Asked
// to invent a key path and reproduce a value, a 9B model will return a file
// path in the key field and a paraphrased block as the value -- observed, not
// theorised. Handed an inventory, the same model is choosing from a list
// instead of generating from memory, and a key that does not exist is no
// longer expressible.
//
// It also removes a whole class of failure that costs a round trip: `From` is
// copied from text the model was given, so the applier's equality check passes
// instead of rejecting a paraphrase.
func Inventory(data []byte, subtree string) ([]Scalar, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("not valid YAML: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	var out []Scalar
	walk(doc.Content[0], nil, &out)

	if subtree != "" {
		var filtered []Scalar
		for _, s := range out {
			if s.Key == subtree || strings.HasPrefix(s.Key, subtree+".") {
				filtered = append(filtered, s)
			}
		}
		out = filtered
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out, nil
}

func walk(n *yaml.Node, path []string, out *[]Scalar) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			walk(c, path, out)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			walk(n.Content[i+1], append(append([]string{}, path...), n.Content[i].Value), out)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			walk(c, append(append([]string{}, path...), fmt.Sprintf("%d", i)), out)
		}
	case yaml.ScalarNode:
		if len(path) == 0 {
			return
		}
		*out = append(*out, Scalar{Key: strings.Join(path, "."), Value: n.Value, Line: n.Line})
	}
}

// Render formats one file's scalars for a prompt.
//
// The `path:` line is repeated on every scalar rather than stated once as a
// header. That looks redundant and is not: given a header, a 9B model
// reconstructs the path from memory when it writes the edit and drops a
// segment -- `a/b/c/addons.yaml` becomes `a/b/addons.yaml`, the applier
// cannot open it, and the fix is silently lost. Repeating it puts the exact
// string adjacent to every key it might choose, which turns writing the path
// into copying rather than recalling.
func Render(path string, scalars []Scalar) string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- editable scalars in %s ---\n", path)
	for _, s := range scalars {
		fmt.Fprintf(&b, "  path=%s  key=%s  from=%s\n", path, s.Key, s.Value)
	}
	return b.String()
}
