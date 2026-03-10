package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestRemoveAgentsMD(t *testing.T) {
	dir := t.TempDir()

	// Write an Apus-generated AGENTS.md
	agentsPath := filepath.Join(dir, "AGENTS.md")
	os.WriteFile(agentsPath, []byte("# AGENTS.md\nApus runs at localhost:9847\n"), 0o644)

	if err := removeAgentsMD(dir); err != nil {
		t.Fatalf("removeAgentsMD() error: %v", err)
	}

	if _, err := os.Stat(agentsPath); !os.IsNotExist(err) {
		t.Fatalf("expected AGENTS.md to be deleted")
	}
}

func TestRemoveAgentsMD_NotOurs(t *testing.T) {
	dir := t.TempDir()

	agentsPath := filepath.Join(dir, "AGENTS.md")
	content := "# AGENTS.md\nThis is a custom agents file.\n"
	os.WriteFile(agentsPath, []byte(content), 0o644)

	if err := removeAgentsMD(dir); err != nil {
		t.Fatalf("removeAgentsMD() error: %v", err)
	}

	// Should NOT delete a non-Apus AGENTS.md
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md should still exist: %v", err)
	}
	if string(data) != content {
		t.Fatalf("AGENTS.md content should be unchanged")
	}
}

func TestRemoveAgentsMD_NotExists(t *testing.T) {
	dir := t.TempDir()

	if err := removeAgentsMD(dir); err != nil {
		t.Fatalf("removeAgentsMD() on non-existent file error: %v", err)
	}
}

func TestRemoveInjectRoundTrip(t *testing.T) {
	dir := t.TempDir()

	entryFile := filepath.Join(dir, "MyApp.swift")
	original := "import SwiftUI\n\n@main\nstruct MyApp: App {\n\tvar body: some Scene {\n\t\tWindowGroup {\n\t\t\tContentView()\n\t\t}\n\t}\n}\n"
	os.WriteFile(entryFile, []byte(original), 0o644)

	// Inject
	if err := xcode.InjectApus(entryFile); err != nil {
		t.Fatalf("InjectApus error: %v", err)
	}

	data, _ := os.ReadFile(entryFile)
	if !strings.Contains(string(data), "import Apus") {
		t.Fatalf("expected import Apus after inject")
	}

	// Uninject
	if err := xcode.UninjectApus(entryFile); err != nil {
		t.Fatalf("UninjectApus error: %v", err)
	}

	data, _ = os.ReadFile(entryFile)
	src := string(data)

	if strings.Contains(src, "import Apus") {
		t.Fatalf("expected import Apus removed after uninject")
	}
	if strings.Contains(src, "Apus.shared.start") {
		t.Fatalf("expected Apus.shared.start removed after uninject")
	}
	if strings.Contains(src, "init()") {
		t.Fatalf("expected synthesized empty init removed after uninject")
	}
}
