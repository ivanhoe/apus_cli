package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestCreateProjectBackup(t *testing.T) {
	dir := t.TempDir()

	// Create fake files to back up
	file1 := filepath.Join(dir, "project.pbxproj")
	file2 := filepath.Join(dir, "AppEntry.swift")
	os.WriteFile(file1, []byte("pbxproj-content"), 0o644)
	os.WriteFile(file2, []byte("swift-content"), 0o644)

	backup, err := createProjectBackup(dir, []string{file1, file2})
	if err != nil {
		t.Fatalf("createProjectBackup() error: %v", err)
	}

	if backup.fileCount != 2 {
		t.Fatalf("expected 2 backed-up files, got %d", backup.fileCount)
	}

	if !strings.Contains(backup.dir, ".apus-backups") {
		t.Fatalf("backup dir should be under .apus-backups, got: %s", backup.dir)
	}

	// Verify backup files exist with correct content
	for _, f := range backup.files {
		data, err := os.ReadFile(f.backup)
		if err != nil {
			t.Fatalf("backup file %s not readable: %v", f.backup, err)
		}
		if len(data) == 0 {
			t.Fatalf("backup file %s is empty", f.backup)
		}
	}
}

func TestCreateProjectBackup_SkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	existing := filepath.Join(dir, "exists.swift")
	os.WriteFile(existing, []byte("content"), 0o644)
	missing := filepath.Join(dir, "nope.swift")

	backup, err := createProjectBackup(dir, []string{existing, missing})
	if err != nil {
		t.Fatalf("createProjectBackup() error: %v", err)
	}

	if backup.fileCount != 1 {
		t.Fatalf("expected 1 backed-up file (skipping missing), got %d", backup.fileCount)
	}
}

func TestCreateProjectBackup_EmptyList(t *testing.T) {
	dir := t.TempDir()

	backup, err := createProjectBackup(dir, []string{})
	if err != nil {
		t.Fatalf("createProjectBackup() error: %v", err)
	}

	if backup.fileCount != 0 {
		t.Fatalf("expected 0 backed-up files, got %d", backup.fileCount)
	}
}

func TestProjectBackup_Restore(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "project.pbxproj")
	os.WriteFile(file1, []byte("original-content"), 0o644)

	backup, err := createProjectBackup(dir, []string{file1})
	if err != nil {
		t.Fatalf("createProjectBackup() error: %v", err)
	}

	// Overwrite the original file (simulating a failed modification)
	os.WriteFile(file1, []byte("modified-content"), 0o644)

	// Restore should bring back the original
	if err := backup.restore(); err != nil {
		t.Fatalf("restore() error: %v", err)
	}

	data, _ := os.ReadFile(file1)
	if string(data) != "original-content" {
		t.Fatalf("restore should recover original content, got: %s", string(data))
	}
}

func TestProjectBackup_RestoreError(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "project.pbxproj")
	os.WriteFile(file1, []byte("content"), 0o644)

	backup, err := createProjectBackup(dir, []string{file1})
	if err != nil {
		t.Fatalf("createProjectBackup() error: %v", err)
	}

	// Delete the backup file to force a restore error
	os.Remove(backup.files[0].backup)

	err = backup.restore()
	if err == nil {
		t.Fatalf("expected restore to fail when backup file is missing")
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "sub", "dst.txt")

	os.WriteFile(src, []byte("hello"), 0o644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(data))
	}
}

func TestCopyFile_SrcMissing(t *testing.T) {
	dir := t.TempDir()

	err := copyFile(filepath.Join(dir, "nope"), filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatalf("expected error when source is missing")
	}
}

func TestBackupCandidates(t *testing.T) {
	t.Run("with entry file", func(t *testing.T) {
		info := &xcode.ProjectInfo{
			ProjectPath: "/tmp/MyApp.xcodeproj",
			EntryFile:   "/tmp/MyApp/MyAppApp.swift",
		}
		candidates := backupCandidates(info)
		if len(candidates) != 2 {
			t.Fatalf("expected 2 candidates, got %d", len(candidates))
		}
	})

	t.Run("without entry file", func(t *testing.T) {
		info := &xcode.ProjectInfo{
			ProjectPath: "/tmp/MyApp.xcodeproj",
		}
		candidates := backupCandidates(info)
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
	})
}
