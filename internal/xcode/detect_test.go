package xcode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasMainAttribute(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{name: "swiftui main", src: "@main\nstruct AppEntry: App {}", want: true},
		{name: "uikit app main", src: "@UIApplicationMain\nclass AppDelegate: UIResponder {}", want: true},
		{name: "no main", src: "struct NotMain {}", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasMainAttribute(tc.src)
			if got != tc.want {
				t.Fatalf("hasMainAttribute() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindEntryPoint_FindsMainFile(t *testing.T) {
	tmp := t.TempDir()
	mainDir := filepath.Join(tmp, "App")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mainFile := filepath.Join(mainDir, "MyApp.swift")
	src := "import SwiftUI\n\n@main\nstruct MyApp: App { var body: some Scene { WindowGroup { Text(\"Hi\") } } }\n"
	if err := os.WriteFile(mainFile, []byte(src), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	path, isSwiftUI, err := findEntryPoint(tmp)
	if err != nil {
		t.Fatalf("findEntryPoint() error: %v", err)
	}
	if path != mainFile {
		t.Fatalf("findEntryPoint() path = %q, want %q", path, mainFile)
	}
	if !isSwiftUI {
		t.Fatalf("expected isSwiftUI=true")
	}
}
