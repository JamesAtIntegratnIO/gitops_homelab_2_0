// gitops-gate answers one question about a pull request: does this change what
// actually gets deployed, and is what it produces still valid?
//
// It is CI-agnostic on purpose. Run the binary, read the exit code:
//
//	0  no blocking change
//	1  blocking change -- targeting moved, or validation failed
//	2  the gate could not run
//
// Exit 2 is deliberately distinct from 1. "This change is bad" and "the gate is
// broken" want opposite reactions, and a CI system that shows them identically
// teaches people to ignore the check.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	exitOK       = 0
	exitBlocking = 1
	exitBroken   = 2
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(exitBroken)
	}
	var err error
	var blocking bool

	switch os.Args[1] {
	case "render":
		err = cmdRender(os.Args[2:])
	case "diff":
		blocking, err = cmdDiff(os.Args[2:])
	case "validate":
		blocking, err = cmdValidate(os.Args[2:])
	case "clusters":
		err = cmdClusters(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(exitBroken)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "gitops-gate: %v\n", err)
		os.Exit(exitBroken)
	}
	if blocking {
		os.Exit(exitBlocking)
	}
	os.Exit(exitOK)
}

func usage() {
	fmt.Fprint(os.Stderr, `gitops-gate -- what does this pull request actually change?

  render    Expand every ApplicationSet into the Applications it generates.
  diff      Compare two renders. Fails when cluster targeting changed.
  validate  Schema-validate rendered manifests.
  clusters  export -- regenerate the cluster inventory from live ArgoCD.

Run a command with -h for its flags.
`)
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	root := fs.String("repo", ".", "path to the repository worktree to render")
	cfgPath := fs.String("config", "", "path to .gitops-gate.yaml (default: <repo>/.gitops-gate.yaml)")
	out := fs.String("out", "", "write the target table here (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, inv, err := load(*root, *cfgPath)
	if err != nil {
		return err
	}

	table, err := Render(*root, cfg, inv)
	if err != nil {
		return err
	}

	if *out == "" {
		return table.WriteJSON(os.Stdout)
	}
	if err := WriteTableFile(*out, table); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "rendered %d Applications across %d clusters -> %s\n",
		len(table.Rows), len(inv.Clusters), *out)
	for _, w := range table.Warnings {
		fmt.Fprintf(os.Stderr, "  not covered: %s\n", w)
	}
	return nil
}

func cmdDiff(args []string) (bool, error) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	basePath := fs.String("base", "", "target table for the base revision (required)")
	headPath := fs.String("head", "", "target table for the head revision (required)")
	repo := fs.String("repo", "", "repository worktree; enables chart-diff -- renders every chart whose version moved, at BOTH versions, and diffs the resources")
	cfgPath := fs.String("config", "", "path to .gitops-gate.yaml (default: <repo>/.gitops-gate.yaml)")
	report := fs.String("report", "", "write a markdown report here (default: stdout)")
	jsonOut := fs.String("json", "", "write the machine-readable diff here")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	if *basePath == "" || *headPath == "" {
		return false, fmt.Errorf("-base and -head are both required")
	}

	base, err := ReadTableFile(*basePath)
	if err != nil {
		return false, err
	}
	head, err := ReadTableFile(*headPath)
	if err != nil {
		return false, err
	}

	// Chart-diff turns "the version moved" into "here is what the version
	// moving does". It needs the repository for the value files, so it is
	// opt-in via -repo rather than assumed.
	if *repo != "" {
		cfg, _, err := load(*repo, *cfgPath)
		if err != nil {
			return false, err
		}
		beforeOb, afterOb, warns := ChartDiff(*repo, cfg, base, head)
		base.Objects = append(base.Objects, beforeOb...)
		head.Objects = append(head.Objects, afterOb...)
		base.Warnings = append(base.Warnings, warns...)
	}

	res := Diff(base, head)

	w := os.Stdout
	if *report != "" {
		f, err := os.Create(*report)
		if err != nil {
			return false, err
		}
		defer f.Close()
		w = f
	}
	res.Report(w)

	if *jsonOut != "" {
		if err := writeJSONFile(*jsonOut, res); err != nil {
			return false, err
		}
	}

	if res.Blocking() {
		fmt.Fprintf(os.Stderr, "\ngitops-gate: %d targeting change(s), %d other source change(s) -- blocking\n",
			len(res.Targeting), len(res.Other))
		return true, nil
	}
	fmt.Fprintf(os.Stderr, "\ngitops-gate: no targeting change; %d version change(s)\n", len(res.Versions))
	return false, nil
}

func load(root, cfgPath string) (*Config, *Inventory, error) {
	if cfgPath == "" {
		cfgPath = filepath.Join(root, ".gitops-gate.yaml")
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	inv, err := LoadInventory(filepath.Join(root, cfg.Clusters))
	if err != nil {
		return nil, nil, err
	}
	return cfg, inv, nil
}
