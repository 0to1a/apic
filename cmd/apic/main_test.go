package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRootCmd_RegistersAllSubcommands(t *testing.T) {
	root := newRootCmd()
	want := []string{"init", "generate", "diff"}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("expected %q subcommand to be registered, got commands: %v", name, root.Commands())
		}
	}
}

func TestRunDefault_AutoInitsWhenApicYAMLMissing(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := runDefault(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDefault: %v (stderr: %s)", err, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "apic.yaml")); err != nil {
		t.Fatalf("expected apic.yaml to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gen", "types.go")); err != nil {
		t.Fatalf("expected gen/types.go to be created: %v", err)
	}
	if !strings.Contains(stdout.String(), "Created") {
		t.Errorf("expected a Created message, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `Edit apic.yaml and run "apic" again.`) {
		t.Errorf("expected the first-run hint, got %q", stdout.String())
	}
}

func TestRunDefault_JustGeneratesWhenApicYAMLExists(t *testing.T) {
	dir := t.TempDir()
	writeApicYAML(t, dir, starterContract)

	var stdout, stderr bytes.Buffer
	if err := runDefault(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDefault: %v (stderr: %s)", err, stderr.String())
	}

	if strings.Contains(stdout.String(), "Created") {
		t.Errorf("did not expect apic.yaml to be (re)created, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "Edit apic.yaml") {
		t.Errorf("did not expect the first-run hint when apic.yaml already existed, got %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "gen", "types.go")); err != nil {
		t.Fatalf("expected gen/types.go to be created: %v", err)
	}
}

func TestRunDefault_NeverWritesStubs(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	if err := runDefault(dir, &stdout, &stderr); err != nil {
		t.Fatalf("runDefault: %v (stderr: %s)", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "service.go")); !os.IsNotExist(err) {
		t.Fatalf("expected no service.go from bare invocation, stat err = %v", err)
	}
}
