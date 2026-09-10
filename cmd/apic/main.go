package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the apic command tree.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "apic",
		Short:        "apic parses a contract YAML and generates a Go HTTP server",
		SilenceUsage: true, // SilenceErrors stays false (cobra default) so any bubbled-up error is always printed — see Task 2
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDefault(".", cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	root.AddCommand(newInitCmd())
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newDiffCmd())
	return root
}

// runDefault implements bare `apic` invocation: if apic.yaml doesn't exist
// yet, create it (via runInit) first, then always run generate (never with
// stubs — that stays an explicit `--stubs` opt-in).
func runDefault(dir string, stdout, stderr io.Writer) error {
	path := filepath.Join(dir, "apic.yaml")
	created := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := runInit(dir, false, stdout); err != nil {
			return err
		}
		created = true
	} else if err != nil {
		return err
	}

	if err := runGenerate(dir, false, stdout, stderr); err != nil {
		return err
	}

	if created {
		fmt.Fprintln(stdout, `Edit apic.yaml and run "apic" again.`)
	}
	return nil
}
