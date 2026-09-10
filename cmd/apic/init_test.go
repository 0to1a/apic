package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/0to1a/apic/internal/contract"
)

func TestRunInit_WritesValidApicYAML(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	if err := runInit(dir, false, &stdout); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "apic.yaml"))
	if err != nil {
		t.Fatalf("read apic.yaml: %v", err)
	}
	c, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse(generated apic.yaml): %v", err)
	}
	if c.Out != "gen/" {
		t.Fatalf("Out = %q, want %q", c.Out, "gen/")
	}
	if len(c.Groups) == 0 {
		t.Fatalf("expected at least one group in the starter contract")
	}

	wantStdout := "Created " + filepath.Join(dir, "apic.yaml") + "\n"
	if stdout.String() != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout.String(), wantStdout)
	}
}

func TestRunInit_FailsIfExistsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	if err := runInit(dir, false, &stdout); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(dir, false, &stdout); err == nil {
		t.Fatalf("expected an error when apic.yaml already exists without --force")
	}
}

func TestRunInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	if err := runInit(dir, false, &stdout); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(dir, true, &stdout); err != nil {
		t.Fatalf("forced runInit: %v", err)
	}
}
