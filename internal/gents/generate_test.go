package gents

import (
	"flag"
	"os"
	"testing"

	"github.com/0to1a/apic/internal/contract"
	"github.com/0to1a/apic/internal/ir"
)

var update = flag.Bool("update", false, "write the golden file instead of comparing against it")

// TestGenerate_Golden compares Generate's output for testdata/example.yaml
// against the committed golden file testdata/example.ts. Run with -update
// to (re)write it after an intentional codegen change.
func TestGenerate_Golden(t *testing.T) {
	data, err := os.ReadFile("testdata/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if errs := ir.Validate(c); len(errs) != 0 {
		t.Fatalf("Validate: %v", errs)
	}
	resolved, err := ir.Resolve(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got, err := Generate(resolved)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	const goldenPath = "testdata/example.ts"
	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run `go test ./internal/gents/... -update` if this is expected): %v", err)
	}
	if string(want) != string(got) {
		t.Errorf("output does not match golden. Run `go test ./internal/gents/... -update` if this change is expected.\n--- got ---\n%s", got)
	}
}
