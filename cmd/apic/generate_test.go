package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeApicYAML is a shared test helper used by every cmd/apic test file
// (generate_test.go, diff_test.go, main_test.go). Tests use starterContract
// (defined in init.go) as their default valid apic.yaml content.
func writeApicYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "apic.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunGenerate_WritesFilesToOutDir(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}
	for _, name := range []string{"types.go", "service.go", "middleware.go", "routes.go"} {
		if _, err := os.Stat(filepath.Join(dir, "gen", name)); err != nil {
			t.Errorf("expected gen/%s to exist: %v", name, err)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "gen/") {
		t.Errorf("expected stdout to mention the out dir, got %q", stdout.String())
	}
}

func TestRunGenerate_RespectsCustomOut(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, strings.Replace(starterContract, "out: gen/", "out: build/server", 1))

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "server", "types.go")); err != nil {
		t.Errorf("expected build/server/types.go to exist: %v", err)
	}
}

func TestRunGenerate_RejectsOutEqualToProjectDir(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, strings.Replace(starterContract, "out: gen/", "out: .", 1))

	var stdout, stderr bytes.Buffer
	err := runGenerate(dir, false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected an error for out: . (same directory as apic.yaml)")
	}
	if !strings.Contains(err.Error(), "must not be the same directory as apic.yaml") {
		t.Errorf("expected a clear out-equals-dir error, got %q", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "types.go")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no generated files written into the project dir, stat err = %v", statErr)
	}
}

func TestRunGenerate_WritesTSFileWhenTSKeyPresent(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, "ts: web/api.ts\n\n"+starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "web", "api.ts"))
	if err != nil {
		t.Fatalf("expected web/api.ts to exist: %v", err)
	}
	if !strings.Contains(string(data), "export function createClient") {
		t.Errorf("expected generated TS to declare createClient, got:\n%s", data)
	}
	if !strings.Contains(stdout.String(), "web/api.ts") {
		t.Errorf("expected stdout to mention the ts file, got %q", stdout.String())
	}
}

func TestRunGenerate_NoTSFileWithoutTSKey(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			t.Errorf("expected no .ts file without a ts: key, found %s", e.Name())
		}
	}
}

func TestRunGenerate_RejectsTSEqualToApicYAML(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, "ts: apic.yaml\n\n"+starterContract)

	var stdout, stderr bytes.Buffer
	err := runGenerate(dir, false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected an error for ts: apic.yaml (same file as the contract)")
	}
	if !strings.Contains(err.Error(), "must not be the same file as apic.yaml") {
		t.Errorf("expected a clear ts-equals-apic.yaml error, got %q", err.Error())
	}
}

func TestRunGenerate_PrintsAllValidationErrors(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, `out: gen/

version: 1
service: Service
groups:
  - routes:
      - GET /health:
`)
	var stdout, stderr bytes.Buffer
	err := runGenerate(dir, false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected an error for a group missing prefix/use")
	}
	if !strings.Contains(stderr.String(), `missing "prefix"`) {
		t.Errorf("expected a missing-prefix error in stderr, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `missing "use"`) {
		t.Errorf("expected a missing-use error in stderr, got %q", stderr.String())
	}
}

func TestRunGenerate_StubsWritesServiceGo(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)
	writeTestGoMod(t, dir)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, true, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "service.go"))
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "package main") {
		t.Errorf("expected service.go to be package main, got:\n%s", src)
	}
	if !strings.Contains(src, "func (serviceImpl) GetHealth(") {
		t.Errorf("expected a GetHealth method stub, got:\n%s", src)
	}
	if !strings.Contains(src, "gen.Unimplemented") {
		t.Errorf("expected the stub body to use gen.Unimplemented, got:\n%s", src)
	}
	if strings.Contains(src, "func main") {
		t.Errorf("service.go must not declare func main (the user's own main.go does that), got:\n%s", src)
	}
	if strings.Contains(src, "// Code generated by apic. DO NOT EDIT.") {
		t.Errorf("service.go must not carry the generated-code header — it's a plain, editable file, got:\n%s", src)
	}
}

func TestRunGenerate_StubsNeverOverwritesExistingServiceGo(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)
	writeTestGoMod(t, dir)

	const handwritten = "package main\n\n// hand-written, do not touch\n"
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(handwritten), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, true, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != handwritten {
		t.Fatalf("service.go was overwritten, got:\n%s", data)
	}
	if !strings.Contains(stdout.String(), "already exists, skipped") {
		t.Errorf("expected a skip message in stdout, got %q", stdout.String())
	}
}

func TestRunGenerate_StubsCompileAgainstGeneratedPackage(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)
	writeTestGoMod(t, dir)
	writeTestMainGo(t, dir)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, true, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... in %s failed: %v\n%s", dir, err, out)
	}
}

// TestRunGenerate_StubsCompileAgainstGeneratedPackage_CustomOut is the
// regression test for a contract with a custom `out:` directory (e.g.
// out: build/server): the generated package is always declared `package
// gen` (see internal/gen's templates) regardless of the directory name
// it's written to, so service.go's references to it (e.g. gen.HealthRequest)
// must use the identifier "gen", not the out directory's basename.
func TestRunGenerate_StubsCompileAgainstGeneratedPackage_CustomOut(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, strings.Replace(starterContract, "out: gen/", "out: build/server", 1))
	writeTestGoMod(t, dir)
	writeTestMainGo(t, dir)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, true, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v (stderr: %s)", err, stderr.String())
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... in %s failed: %v\n%s", dir, err, out)
	}
}

// writeTestMainGo writes a trivial companion main.go into dir, supplying the
// func main a real project would have alongside a --stubs-generated
// service.go (which deliberately does not declare its own func main — see
// TestRunGenerate_StubsWritesServiceGo).
func writeTestMainGo(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTestGoMod gives dir its own go.mod that replaces github.com/0to1a/apic
// with this repo, so genImportPath (and the compile-check test) have a real
// module to resolve the gen/ import path against.
func writeTestGoMod(t *testing.T, dir string) {
	t.Helper()
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module example.com/testapp\n\ngo 1.22\n\nrequire github.com/0to1a/apic v0.0.0\n\nreplace github.com/0to1a/apic => %s\n", repoRoot)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
}
