package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCommandPath_DefaultsToWorkingDirectory(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	resolved, err := resolveCommandPath("")
	if err != nil {
		t.Fatalf("resolveCommandPath() error: %v", err)
	}
	expected, err := filepath.EvalSymlinks(dir)
	if err != nil {
		expected = dir
	}
	if resolved != expected {
		t.Fatalf("expected %s, got %s", expected, resolved)
	}
}

func TestResolveCommandPath_ResolvesRelativeDirectory(t *testing.T) {
	base := t.TempDir()
	projectDir := filepath.Join(base, "workspace", "MyApp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(base); err != nil {
		t.Fatalf("chdir base dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	resolved, err := resolveCommandPath(filepath.Join("workspace", "MyApp"))
	if err != nil {
		t.Fatalf("resolveCommandPath() error: %v", err)
	}
	expected, err := filepath.EvalSymlinks(projectDir)
	if err != nil {
		expected = projectDir
	}
	if resolved != expected {
		t.Fatalf("expected %s, got %s", expected, resolved)
	}
}

func TestResolveCommandPath_RejectsMissingDirectory(t *testing.T) {
	_, err := resolveCommandPath(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatalf("expected resolveCommandPath() to fail")
	}
	if !strings.Contains(err.Error(), "invalid --path") {
		t.Fatalf("expected invalid --path error, got: %v", err)
	}
}

func TestResolveCommandPath_RejectsFiles(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := resolveCommandPath(filePath)
	if err == nil {
		t.Fatalf("expected resolveCommandPath() to fail for files")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got: %v", err)
	}
}
