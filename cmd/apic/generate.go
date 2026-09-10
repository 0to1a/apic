package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0to1a/apic/internal/contract"
	"github.com/0to1a/apic/internal/gen"
	"github.com/0to1a/apic/internal/ir"
)

func newGenerateCmd() *cobra.Command {
	var stubs bool
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate the gen/ package from apic.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(".", stubs, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVar(&stubs, "stubs", false, "also write a service.go stub implementation")
	return cmd
}

// loadAndResolve reads apic.yaml in dir, parses it, validates it, and
// resolves it into IR. Every ir.Validate error is printed to stderr (not
// just the first) before returning a generic error, so one command run
// gives complete feedback on a broken contract.
func loadAndResolve(dir string, stderr io.Writer) (*contract.Contract, *ir.ContractIR, error) {
	path := filepath.Join(dir, "apic.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	c, err := contract.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return nil, nil, fmt.Errorf("parse failed")
	}

	if errs := ir.Validate(c); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "%s: %v\n", path, e)
		}
		return nil, nil, fmt.Errorf("validation failed (%d error(s))", len(errs))
	}

	resolved, err := ir.Resolve(c)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", path, err)
		return nil, nil, fmt.Errorf("resolve failed")
	}

	out := outDir(dir, c)
	if filepath.Clean(out) == filepath.Clean(dir) {
		return nil, nil, fmt.Errorf("out: %q must not be the same directory as apic.yaml — generated files would collide with service.go and apic.yaml itself", c.Out)
	}

	return c, resolved, nil
}

// outDir returns c.Out resolved against dir, defaulting to "gen/" when
// c.Out is empty (contract.Parse never applies this default itself).
func outDir(dir string, c *contract.Contract) string {
	out := c.Out
	if out == "" {
		out = "gen/"
	}
	return filepath.Join(dir, out)
}

// runGenerate implements `apic generate`: parse, validate, resolve, then
// write the generated package to the contract's out directory.
func runGenerate(dir string, stubs bool, stdout, stderr io.Writer) error {
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
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(out, name), content, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "generated %d files in %s/\n", len(files), out)

	if stubs {
		if err := writeStubs(dir, out, resolved, stdout); err != nil {
			return err
		}
	}
	return nil
}
