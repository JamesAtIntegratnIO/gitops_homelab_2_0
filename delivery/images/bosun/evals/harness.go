package evals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/JamesAtIntegratnIO/delivery-kit/bosun/edits"
	"github.com/JamesAtIntegratnIO/delivery-kit/bosun/llm"
)

// Result is one case's outcome.
type Result struct {
	Case      string
	Class     string
	WantClass string
	Elapsed   time.Duration

	ClassOK bool
	EditsOK bool
	// Unsafe means something wrong actually landed on disk. That is the only
	// failure that would matter in production -- a wrong classification whose
	// edits the applier refused costs a human two minutes, a wrong edit that
	// lands renders green and breaks at runtime.
	Unsafe bool

	// Applied is what actually landed after the applier's checks -- which is
	// the only measure that matters. A model that proposes a perfect fix in
	// the wrong shape has fixed nothing.
	Applied  []string
	Rejected []string
	Notes    []string
}

func (r Result) Pass() bool { return r.ClassOK && r.EditsOK }

// BuildPrompt renders the user-side prompt for a case.
//
// The scalar inventory is the important part. Handed one, a model chooses a
// key from a list; without one it invents a key path and paraphrases a value,
// and the applier -- correctly -- throws the result away.
func BuildPrompt(c Case, withInventory bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PULL REQUEST: %s\n\n%s\n\n", c.Subject, c.GateReport)

	paths := make([]string, 0, len(c.Files))
	for p := range c.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	if withInventory {
		b.WriteString("Repository files this pull request may change.\n")
		b.WriteString("Use these keys and values EXACTLY as written.\n\n")
		for _, p := range paths {
			inv, err := edits.Inventory([]byte(c.Files[p]), "")
			if err != nil {
				continue
			}
			b.WriteString(edits.Render(p, inv))
			b.WriteString("\n")
		}
	} else {
		b.WriteString("Repository files this pull request may change:\n\n")
		for _, p := range paths {
			fmt.Fprintf(&b, "--- %s ---\n%s\n", p, c.Files[p])
		}
	}
	b.WriteString("Classify this pull request and, if mechanical, give the edits.")
	return b.String()
}

// Run executes one case and scores it by applying whatever the model proposed
// to a throwaway copy of the fixture.
func Run(ctx context.Context, p llm.Provider, system string, c Case, withInventory bool) Result {
	res := Result{Case: c.Name, WantClass: c.WantClass}

	start := time.Now()
	v, err := p.Classify(ctx, system, BuildPrompt(c, withInventory))
	res.Elapsed = time.Since(start)
	if err != nil {
		res.Notes = append(res.Notes, "provider error: "+err.Error())
		return res
	}

	res.Class = v.Classification
	res.ClassOK = v.Classification == c.WantClass

	root, err := os.MkdirTemp("", "eval")
	if err != nil {
		res.Notes = append(res.Notes, err.Error())
		return res
	}
	defer os.RemoveAll(root)
	for path, content := range c.Files {
		full := filepath.Join(root, path)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		_ = os.WriteFile(full, []byte(content), 0o644)
	}

	in := make([]edits.Edit, 0, len(v.Edits))
	for _, e := range v.Edits {
		in = append(in, edits.Edit{Path: e.Path, Key: e.Key, From: e.From, To: e.To, Rationale: e.Rationale})
	}
	// The same policy the agent runs with, including the evidence the model
	// was shown -- so the eval measures what would actually land, not what the
	// model wished for.
	policy := edits.Policy{Allow: []string{"addons/**"}, Evidence: BuildPrompt(c, withInventory)}
	applied, err := edits.Apply(root, policy, in)
	if err != nil {
		res.Notes = append(res.Notes, err.Error())
		return res
	}
	for _, a := range applied.Applied {
		res.Applied = append(res.Applied, fmt.Sprintf("%s=%s", a.Key, a.To))
	}
	for _, r := range applied.Rejected {
		res.Rejected = append(res.Rejected, fmt.Sprintf("%s (%s)", r.Key, r.Reason))
	}

	if c.WantClass != llm.ClassMechanical {
		// Proposing edits here is miscalibration; landing them is unsafe.
		res.EditsOK = len(v.Edits) == 0
		res.Unsafe = len(applied.Applied) > 0
		if !res.EditsOK {
			res.Notes = append(res.Notes, fmt.Sprintf("proposed %d edit(s) on a %s case, %d landed",
				len(v.Edits), c.WantClass, len(applied.Applied)))
		}
		return res
	}

	// Did the right values land, and only those?
	got := map[string]string{}
	for _, a := range applied.Applied {
		got[a.Key] = a.To
	}
	res.EditsOK = len(got) == len(c.WantEdits)
	for k, want := range c.WantEdits {
		if got[k] != want {
			res.EditsOK = false
			res.Notes = append(res.Notes, fmt.Sprintf("expected %s=%s, got %q", k, want, got[k]))
		}
	}
	return res
}

// Summary scores a whole run.
type Summary struct {
	Model      string
	Total      int
	ClassRight int
	FullPass   int
	Unsafe     int
	Elapsed    time.Duration
	Results    []Result
}

func Summarise(model string, results []Result) Summary {
	s := Summary{Model: model, Total: len(results), Results: results}
	for _, r := range results {
		if r.ClassOK {
			s.ClassRight++
		}
		if r.Pass() {
			s.FullPass++
		}
		if r.Unsafe {
			s.Unsafe++
		}
		s.Elapsed += r.Elapsed
	}
	return s
}

func (s Summary) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n", s.Model)
	fmt.Fprintf(&b, "  classification %d/%d   full pass %d/%d   UNSAFE %d   total %s\n",
		s.ClassRight, s.Total, s.FullPass, s.Total, s.Unsafe, s.Elapsed.Round(time.Second))
	for _, r := range s.Results {
		mark := "FAIL"
		if r.Pass() {
			mark = "pass"
		}
		fmt.Fprintf(&b, "  %-4s %-34s %-11s (want %-11s) %5s\n",
			mark, r.Case, r.Class, r.WantClass, r.Elapsed.Round(time.Second))
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "         %s\n", n)
		}
		for _, rj := range r.Rejected {
			fmt.Fprintf(&b, "         rejected: %s\n", rj)
		}
	}
	return b.String()
}
