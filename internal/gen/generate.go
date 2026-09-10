package gen

import (
	"bytes"
	"fmt"
	"go/format"
	"text/template"

	"github.com/0to1a/apic/internal/ir"
)

// Generate renders a resolved contract into the gen/ package files. Callers
// are expected to have already run ir.Validate + ir.Resolve successfully.
func Generate(c *ir.ContractIR) (map[string][]byte, error) {
	files := map[string]*template.Template{
		"types.go":      typesTmpl,
		"service.go":    serviceTmpl,
		"middleware.go": middlewareTmpl,
		"routes.go":     routesTmpl,
	}
	out := make(map[string][]byte, len(files))
	for name, tmpl := range files {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, c); err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("format %s: %w\n--- unformatted source ---\n%s", name, err, buf.String())
		}
		out[name] = formatted
	}
	return out, nil
}
