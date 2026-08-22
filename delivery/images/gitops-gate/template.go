package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// ApplicationSets come in two templating dialects and a repository usually
// contains both, because the bootstrap layer predates goTemplate mode:
//
//	fasttemplate (goTemplate: false)  {{name}}  {{metadata.labels.environment}}
//	goTemplate   (goTemplate: true)   {{ .name }}  {{ .values.chart }}
//
// Guessing wrong produces plausible-looking wrong output rather than an error,
// so the dialect is read from the ApplicationSet's own `goTemplate` field
// rather than inferred from the string.

// renderGoTemplate renders an ApplicationSet template in goTemplate mode.
func renderGoTemplate(node any, data map[string]any) (any, error) {
	raw, err := yaml.Marshal(node)
	if err != nil {
		return nil, err
	}
	// missingkey=error matches the goTemplateOptions these ApplicationSets set,
	// so a typo'd key fails here rather than rendering "<no value>" into a
	// resource name.
	tpl, err := template.New("t").
		Funcs(sprig.TxtFuncMap()).
		Option("missingkey=error").
		Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}
	var out any
	if err := yaml.Unmarshal(buf.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("re-parsing rendered template: %w", err)
	}
	return out, nil
}

var fastTemplateRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.\-\[\]"']+)\s*\}\}`)

// renderFastTemplate renders the pre-goTemplate dialect, where a placeholder is
// a dotted path into the parameter map with no leading dot and no functions.
func renderFastTemplate(s string, data map[string]any) (string, error) {
	var missing []string
	out := fastTemplateRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := fastTemplateRe.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		v, ok := lookupPath(data, sub[1])
		if !ok {
			missing = append(missing, sub[1])
			return m
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("no value for %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// lookupPath walks a dotted path, tolerating the index syntax that appears in
// selectors: metadata.labels.foo and metadata.labels["foo/bar"] both work.
func lookupPath(data map[string]any, path string) (string, bool) {
	cur := any(data)
	for _, part := range splitPath(path) {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[part]
		if !ok {
			return "", false
		}
	}
	switch v := cur.(type) {
	case string:
		return v, true
	case nil:
		return "", false
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func splitPath(path string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch {
		case inQuote != 0:
			if ch == inQuote {
				inQuote = 0
			} else {
				cur.WriteByte(ch)
			}
		case ch == '"' || ch == '\'':
			inQuote = ch
		case ch == '[':
			// An index expression starts a new segment: in labels["a.b/c"] the
			// pending "labels" must be flushed, or the key concatenates onto it.
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		case ch == ']':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		case ch == '.':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
