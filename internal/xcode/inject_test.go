package xcode

import (
	"os"
	"path/filepath"
	"strings"
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

func TestInjectApus_SwiftUIWithInit(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "MyApp.swift")
	content := `import SwiftUI

@main
struct MyApp: App {
    init() {
        print("setup")
    }
    var body: some Scene { WindowGroup { Text("Hi") } }
}`
	os.WriteFile(file, []byte(content), 0o644)

	if err := InjectApus(file); err != nil {
		t.Fatalf("InjectApus() error: %v", err)
	}

	data, _ := os.ReadFile(file)
	src := string(data)

	if !strings.Contains(src, "import Apus") {
		t.Fatalf("expected import Apus in output:\n%s", src)
	}
	if !strings.Contains(src, "#if DEBUG") {
		t.Fatalf("expected #if DEBUG guard:\n%s", src)
	}
	if !strings.Contains(src, "Apus.shared.start(interceptNetwork: true)") {
		t.Fatalf("expected Apus.shared.start() call:\n%s", src)
	}
}

func TestInjectApus_SwiftUIWithoutInit(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "MyApp.swift")
	content := `import SwiftUI

@main
struct MyApp: App {
    var body: some Scene { WindowGroup { Text("Hi") } }
}`
	os.WriteFile(file, []byte(content), 0o644)

	if err := InjectApus(file); err != nil {
		t.Fatalf("InjectApus() error: %v", err)
	}

	data, _ := os.ReadFile(file)
	src := string(data)

	if !strings.Contains(src, "init()") {
		t.Fatalf("expected synthesized init():\n%s", src)
	}
	if !strings.Contains(src, "Apus.shared.start(interceptNetwork: true)") {
		t.Fatalf("expected Apus.shared.start() call:\n%s", src)
	}
}

func TestInjectApus_Idempotent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "MyApp.swift")
	content := `import SwiftUI
#if DEBUG
import Apus
#endif

@main
struct MyApp: App {
    init() {
        #if DEBUG
        Apus.shared.start(interceptNetwork: true)
        #endif
    }
    var body: some Scene { WindowGroup { Text("Hi") } }
}`
	os.WriteFile(file, []byte(content), 0o644)

	if err := InjectApus(file); err != nil {
		t.Fatalf("InjectApus() error: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != content {
		t.Fatalf("InjectApus should be idempotent, but file changed")
	}
}

func TestInjectApus_NoImports(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "NoImports.swift")
	content := `@main
struct MyApp: App {
    var body: some Scene { WindowGroup { Text("Hi") } }
}`
	os.WriteFile(file, []byte(content), 0o644)

	err := InjectApus(file)
	if err == nil {
		t.Fatalf("expected error when no import statements exist")
	}
	if !strings.Contains(err.Error(), "no import statement") {
		t.Fatalf("expected 'no import statement' error, got: %v", err)
	}
}

func TestInjectApus_UIApplicationMain(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "AppDelegate.swift")
	content := `import UIKit

@UIApplicationMain
class AppDelegate: UIResponder, UIApplicationDelegate {
    init() {
        super.init()
    }
}`
	os.WriteFile(file, []byte(content), 0o644)

	if err := InjectApus(file); err != nil {
		t.Fatalf("InjectApus() error: %v", err)
	}

	data, _ := os.ReadFile(file)
	src := string(data)

	if !strings.Contains(src, "import Apus") {
		t.Fatalf("expected import Apus:\n%s", src)
	}
}

func TestInjectImport(t *testing.T) {
	src := "import SwiftUI\nimport Foundation\n\n@main\nstruct App {}"
	got, err := injectImport(src)
	if err != nil {
		t.Fatalf("injectImport() error: %v", err)
	}

	// Import block should appear after the last import (Foundation), not after SwiftUI
	foundationIdx := strings.Index(got, "import Foundation")
	apusIdx := strings.Index(got, "import Apus")
	if apusIdx < foundationIdx {
		t.Fatalf("import Apus should be after import Foundation:\n%s", got)
	}
}

func TestFindInit(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"standard init", "\n    init() {\n        print(\"hi\")\n    }", true},
		{"no space init", "\n    init(){\n        print(\"hi\")\n    }", true},
		{"override init", "\n    override init() {\n        print(\"hi\")\n    }", true},
		{"no init", "struct App {\n    var body: some Scene {}\n}", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findInit(tc.src)
			if tc.want && got == -1 {
				t.Fatalf("expected to find init()")
			}
			if !tc.want && got != -1 {
				t.Fatalf("expected no init(), but found at %d", got)
			}
		})
	}
}
