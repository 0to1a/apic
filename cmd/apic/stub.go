package main

import (
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0to1a/apic/internal/ir"
)

// findModulePath walks up from dir looking for a go.mod, returning the
// module path from its `module` directive and the directory it lives in.
func findModulePath(dir string) (modulePath, moduleRoot string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for d := abs; ; {
		data, readErr := os.ReadFile(filepath.Join(d, "go.mod"))
		if readErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module")), d, nil
				}
			}
			return "", "", fmt.Errorf("%s: no module directive found", filepath.Join(d, "go.mod"))
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", "", fmt.Errorf("no go.mod found above %s", abs)
		}
		d = parent
	}
}

// genImportPath computes the Go import path of out (the directory
// gen.Generate wrote to), by combining the nearest go.mod's module path
// with out's path relative to that module's root.
func genImportPath(dir, out string) (string, error) {
	modulePath, moduleRoot, err := findModulePath(dir)
	if err != nil {
		return "", err
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(moduleRoot, absOut)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return modulePath, nil
	}
	return modulePath + "/" + filepath.ToSlash(rel), nil
}

// writeStubs writes service.go in dir (package main), implementing every
// method of resolved's Service interface with a body that returns
// apic.Unimplemented. It never overwrites an existing service.go.
func writeStubs(dir, out string, resolved *ir.ContractIR, stdout io.Writer) error {
	path := filepath.Join(dir, "service.go")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(stdout, "%s already exists, skipped\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	importPath, err := genImportPath(dir, out)
	if err != nil {
		return err
	}
	// pkgName is the Go identifier used to reference the generated package,
	// which is always "gen" — every internal/gen template declares
	// `package gen` literally, regardless of the directory name (`out`)
	// gen.Generate's output is written to. Deriving it from importPath's
	// basename would break for a custom `out:` whose last path segment
	// isn't "gen" (e.g. out: build/server -> package server.HealthRequest,
	// which doesn't exist).
	const pkgName = "gen"

	var b strings.Builder
	fmt.Fprintf(&b, "package main\n\n")
	// "context" is only used inside per-route method bodies, so a contract
	// with a group but zero routes at all (a genuinely degenerate case)
	// would otherwise leave it unused and fail to compile.
	if len(resolved.Routes) > 0 {
		fmt.Fprintf(&b, "import (\n\t\"context\"\n\n\t\"github.com/0to1a/apic\"\n\t%q\n)\n\n", importPath)
	} else {
		fmt.Fprintf(&b, "import (\n\t\"github.com/0to1a/apic\"\n\t%q\n)\n\n", importPath)
	}
	fmt.Fprintf(&b, "type serviceImpl struct{}\n\n")
	for _, r := range resolved.Routes {
		if r.ResponseType == "" {
			fmt.Fprintf(&b, "func (serviceImpl) %s(ctx context.Context, req *%s.%s) error {\n\treturn apic.Errorf(apic.Unimplemented, %q)\n}\n\n",
				r.MethodName, pkgName, r.Request.Name, "TODO: implement "+r.MethodName)
		} else {
			fmt.Fprintf(&b, "func (serviceImpl) %s(ctx context.Context, req *%s.%s) (*%s.%s, error) {\n\treturn nil, apic.Errorf(apic.Unimplemented, %q)\n}\n\n",
				r.MethodName, pkgName, r.Request.Name, pkgName, r.ResponseType, "TODO: implement "+r.MethodName)
		}
	}

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("format service.go: %w\n--- unformatted source ---\n%s", err, b.String())
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s\n", path)
	return nil
}
