package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDiff_UpToDateAfterGenerate(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDiff: %v (stderr: %s)", err, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "up to date" {
		t.Fatalf("stdout = %q, want \"up to date\"", stdout.String())
	}
}

func TestRunDiff_DetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	typesPath := filepath.Join(dir, "gen", "types.go")
	if err := os.WriteFile(typesPath, []byte("package gen\n\n// tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	err := runDiff(dir, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected an error for an out-of-date gen/ directory")
	}
	if !strings.Contains(stderr.String(), "types.go") {
		t.Errorf("expected types.go to be named in stderr, got %q", stderr.String())
	}
}

func TestRunDiff_DetectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "gen", "routes.go")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err == nil {
		t.Fatalf("expected an error for a missing generated file")
	}
	if !strings.Contains(stderr.String(), "routes.go") {
		t.Errorf("expected routes.go to be named in stderr, got %q", stderr.String())
	}
}

func TestRunDiff_IgnoresNonGoFiles(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "gen", ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDiff: %v (stderr: %s)", err, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "up to date" {
		t.Fatalf("stdout = %q, want \"up to date\"", stdout.String())
	}
}

func TestRunDiff_DetectsModifiedTSFile(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, "ts: web/api.ts\n\n"+starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	tsPath := filepath.Join(dir, "web", "api.ts")
	if err := os.WriteFile(tsPath, []byte("// tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	err := runDiff(dir, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected an error for an out-of-date ts: file")
	}
	if !strings.Contains(stderr.String(), "api.ts") {
		t.Errorf("expected api.ts to be named in stderr, got %q", stderr.String())
	}
}

func TestRunDiff_DetectsMissingTSFile(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, "ts: web/api.ts\n\n"+starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "web", "api.ts")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err == nil {
		t.Fatalf("expected an error for a missing ts: file")
	}
	if !strings.Contains(stderr.String(), "api.ts") {
		t.Errorf("expected api.ts to be named in stderr, got %q", stderr.String())
	}
}

func TestRunDiff_IgnoresTSWithoutTSKey(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDiff: %v (stderr: %s)", err, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "up to date" {
		t.Fatalf("stdout = %q, want \"up to date\"", stdout.String())
	}
}

func TestRunDiff_NeverTouchesServiceGo(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runGenerate(dir, false, &stdout, &stderr); err != nil {
		t.Fatalf("runGenerate: %v", err)
	}

	const marker = "package main\n\n// untouched marker\n"
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := runDiff(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDiff: %v (stderr: %s)", err, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != marker {
		t.Fatalf("service.go was modified by runDiff, got:\n%s", data)
	}
}
