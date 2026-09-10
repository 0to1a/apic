package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// starterContract is what `apic init` writes: a minimal, immediately
// generate-able apic.yaml (contract YAML plus the CLI-only out: key).
const starterContract = `out: gen/

version: 1
service: Service

groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
          resp:
            status: str!
`

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter apic.yaml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(".", force, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite apic.yaml if it already exists")
	return cmd
}

// runInit writes a starter apic.yaml in dir. If apic.yaml already exists and
// force is false, it returns an error instead of overwriting it.
func runInit(dir string, force bool, stdout io.Writer) error {
	path := filepath.Join(dir, "apic.yaml")
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.WriteFile(path, []byte(starterContract), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s\n", path)
	return nil
}
