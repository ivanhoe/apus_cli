package xcode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMainAttributeIndex(t *testing.T) {
	if got := findMainAttributeIndex("struct App {}"); got != -1 {
		t.Fatalf("findMainAttributeIndex() = %d, want -1", got)
	}

	if got := findMainAttributeIndex("@main\nstruct App: App {}"); got != 0 {
		t.Fatalf("findMainAttributeIndex() for @main = %d, want 0", got)
	}

	if got := findMainAttributeIndex("@UIApplicationMain\nclass AppDelegate: UIResponder {}"); got != 0 {
		t.Fatalf("findMainAttributeIndex() for @UIApplicationMain = %d, want 0", got)
	}

	src := "/* comment */\n@UIApplicationMain\nclass AppDelegate: UIResponder {}\n@main\nstruct MyApp: App {}"
	want := 14
	if got := findMainAttributeIndex(src); got != want {
		t.Fatalf("findMainAttributeIndex() = %d, want %d", got, want)
	}
}

func TestHasApusIntegration(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "App")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	file := filepath.Join(srcDir, "MyApp.swift")
	content := `import SwiftUI
import Apus

@main
struct MyApp: App {
  init() {
    Apus.shared.start(interceptNetwork: true)
  }
  var body: some Scene { WindowGroup { Text("Hi") } }
}`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ok, err := HasApusIntegration(tmp)
	if err != nil {
		t.Fatalf("HasApusIntegration() error: %v", err)
	}
	if !ok {
		t.Fatalf("expected HasApusIntegration() to return true")
	}
}

func TestHasApusIntegration_FalseWhenMissingStart(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "MyApp.swift")
	content := `import SwiftUI
import Apus

@main
struct MyApp: App {
  var body: some Scene { WindowGroup { Text("Hi") } }
}`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ok, err := HasApusIntegration(tmp)
	if err != nil {
		t.Fatalf("HasApusIntegration() error: %v", err)
	}
	if ok {
		t.Fatalf("expected HasApusIntegration() to return false")
	}
}
