package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0to1a/apic/internal/gen"
)

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Check whether the generated package is up to date with apic.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(".", cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}

// runDiff regenerates the gen/ package in memory and compares it byte for
// byte against what's on disk in the contract's out directory. It never
// touches service.go (the stub file) — that file is out of scope for this
// check entirely.
func runDiff(dir string, stdout, stderr io.Writer) error {
	c, resolved, err := loadAndResolve(dir, stderr)
	if err != nil {
		return err
	}

	files, err := gen.Generate(resolved)
	if err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return fmt.Errorf("generate failed")
	}
	out := outDir(dir, c)

	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var diffs []string
	for _, name := range names {
		want := files[name]
		got, readErr := os.ReadFile(filepath.Join(out, name))
		if readErr != nil {
			diffs = append(diffs, name+" (missing)")
			continue
		}
		if !bytes.Equal(got, want) {
			diffs = append(diffs, name+" (modified)")
		}
	}

	produced := map[string]bool{}
	for _, name := range names {
		produced[name] = true
	}
	if entries, readErr := os.ReadDir(out); readErr == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !produced[e.Name()] {
				diffs = append(diffs, e.Name()+" (stale, no longer generated)")
			}
		}
	}

	if len(diffs) == 0 {
		fmt.Fprintln(stdout, "up to date")
		return nil
	}
	sort.Strings(diffs)
	for _, name := range diffs {
		fmt.Fprintf(stderr, "%s: out of date\n", filepath.Join(out, name))
	}
	return fmt.Errorf("%s is out of date with apic.yaml (%d file(s))", out, len(diffs))
}
