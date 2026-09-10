package gen

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0to1a/apic/internal/contract"
	"github.com/0to1a/apic/internal/ir"
)

var update = flag.Bool("update", false, "write golden files instead of comparing against them")

func generateExample(t *testing.T) map[string][]byte {
	t.Helper()
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
	files, err := Generate(resolved)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return files
}

// TestGenerate_Golden compares Generate's output for testdata/example.yaml
// against the committed golden files in testdata/example/. Run with
// -update to (re)write them after an intentional codegen change.
func TestGenerate_Golden(t *testing.T) {
	files := generateExample(t)
	goldenDir := "testdata/example"

	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(goldenDir, name), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}

	for name, content := range files {
		want, err := os.ReadFile(filepath.Join(goldenDir, name))
		if err != nil {
			t.Fatalf("read golden %s (run `go test ./internal/gen/... -update` if this is expected): %v", name, err)
		}
		if string(want) != string(content) {
			t.Errorf("%s does not match golden. Run `go test ./internal/gen/... -update` if this change is expected.\n--- got ---\n%s", name, content)
		}
	}
}

// TestGenerate_CompilesAgainstRealRuntime renders the example contract and
// go-builds the result against the real apic runtime package (not a mock),
// proving the generator's output is valid Go that actually integrates —
// this is the design doc's §6 "compile-check" requirement.
func TestGenerate_CompilesAgainstRealRuntime(t *testing.T) {
	files := generateExample(t)

	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(genDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module example.com/testapp\n\ngo 1.22\n\nrequire github.com/0to1a/apic v0.0.0\n\nreplace github.com/0to1a/apic => %s\n", repoRoot)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, out)
		}
	}
}
