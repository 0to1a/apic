package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindModulePath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	modulePath, moduleRoot, err := findModulePath(sub)
	if err != nil {
		t.Fatalf("findModulePath: %v", err)
	}
	if modulePath != "example.com/foo" {
		t.Errorf("modulePath = %q, want %q", modulePath, "example.com/foo")
	}
	wantRoot, _ := filepath.EvalSymlinks(dir)
	gotRoot, _ := filepath.EvalSymlinks(moduleRoot)
	if gotRoot != wantRoot {
		t.Errorf("moduleRoot = %q, want %q", moduleRoot, dir)
	}
}

func TestGenImportPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := genImportPath(dir, filepath.Join(dir, "gen"))
	if err != nil {
		t.Fatalf("genImportPath: %v", err)
	}
	if path != "example.com/foo/gen" {
		t.Errorf("path = %q, want %q", path, "example.com/foo/gen")
	}
}
