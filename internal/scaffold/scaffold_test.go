package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAppName(t *testing.T) {
	tests := []struct {
		name    string
		appName string
		wantErr bool
	}{
		{name: "valid", appName: "MyApp", wantErr: false},
		{name: "starts with digit", appName: "1App", wantErr: true},
		{name: "contains space", appName: "My App", wantErr: true},
		{name: "swift keyword", appName: "class", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAppName(tc.appName)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestGenerateAtomicCreatesProject(t *testing.T) {
	dir := t.TempDir()
	data := NewData("MyApp", "SIM-UDID", "swiftui")

	if err := Generate(data, dir); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	projectDir := filepath.Join(dir, "MyApp")
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("expected project dir to exist: %v", err)
	}

	wantFiles := []string{
		filepath.Join(projectDir, "project.yml"),
		filepath.Join(projectDir, "AGENTS.md"),
		filepath.Join(projectDir, "Sources", "MyAppApp.swift"),
		filepath.Join(projectDir, "Sources", "ContentView.swift"),
	}
	for _, f := range wantFiles {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("expected generated file %s: %v", f, err)
		}
	}

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}

	agentsText := string(agents)
	mustContain := []string{
		"-derivedDataPath \"$DERIVED_DATA\"",
		"simctl install",
		"SIMCTL_CHILD_APUS_PROJECT_ROOT=\"$PROJECT_ROOT\"",
		"--terminate-running-process",
	}

	for _, fragment := range mustContain {
		if !strings.Contains(agentsText, fragment) {
			t.Fatalf("AGENTS.md should contain %q", fragment)
		}
	}

	projectYMLPath := filepath.Join(projectDir, "project.yml")
	projectYML, err := os.ReadFile(projectYMLPath)
	if err != nil {
		t.Fatalf("read project.yml: %v", err)
	}

	projectYMLText := string(projectYML)
	if !strings.Contains(projectYMLText, `url: "https://github.com/ivanhoe/apus"`) {
		t.Fatalf("project.yml should default to remote Apus package source")
	}
	if strings.Contains(projectYMLText, "path:") {
		t.Fatalf("project.yml should not use local package path by default")
	}
}

func TestGenerateFailsWhenDirectoryExists(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "MyApp")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir existing dir: %v", err)
	}

	data := NewData("MyApp", "SIM-UDID", "swiftui")
	if err := Generate(data, dir); err == nil {
		t.Fatalf("expected Generate() to fail when directory exists")
	}
}

func TestGenerateFailsForUnsupportedTemplate(t *testing.T) {
	dir := t.TempDir()
	data := NewData("MyApp", "SIM-UDID", "uikit")
	if err := Generate(data, dir); err == nil {
		t.Fatalf("expected unsupported template error")
	}
}

func TestGenerateUsesNearestLocalApusPackageWhenPresent(t *testing.T) {
	root := t.TempDir()
	localApusPath := filepath.Join(root, "apus")
	if err := os.MkdirAll(localApusPath, 0o755); err != nil {
		t.Fatalf("mkdir local apus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localApusPath, "Package.swift"), []byte("let package = Package(name: \"Apus\")\n"), 0o644); err != nil {
		t.Fatalf("write Package.swift: %v", err)
	}

	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	data := NewData("MyApp", "SIM-UDID", "swiftui")
	if err := Generate(data, workspace); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	projectYMLPath := filepath.Join(workspace, "MyApp", "project.yml")
	projectYML, err := os.ReadFile(projectYMLPath)
	if err != nil {
		t.Fatalf("read project.yml: %v", err)
	}

	projectYMLText := string(projectYML)
	if !strings.Contains(projectYMLText, `path: "`+localApusPath+`"`) {
		t.Fatalf("project.yml should use nearest local Apus path, got:\n%s", projectYMLText)
	}
	if strings.Contains(projectYMLText, `url: "https://github.com/ivanhoe/apus"`) {
		t.Fatalf("project.yml should not include remote Apus URL when local path is used")
	}
}

func TestGenerateUsesConfiguredApusPackagePath(t *testing.T) {
	root := t.TempDir()
	localApusPath := filepath.Join(root, "custom-apus")
	if err := os.MkdirAll(localApusPath, 0o755); err != nil {
		t.Fatalf("mkdir local apus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localApusPath, "Package.swift"), []byte("// package\n"), 0o644); err != nil {
		t.Fatalf("write Package.swift: %v", err)
	}

	t.Setenv("APUS_PACKAGE_PATH", "custom-apus")

	data := NewData("MyApp", "SIM-UDID", "swiftui")
	if err := Generate(data, root); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	projectYMLPath := filepath.Join(root, "MyApp", "project.yml")
	projectYML, err := os.ReadFile(projectYMLPath)
	if err != nil {
		t.Fatalf("read project.yml: %v", err)
	}

	projectYMLText := string(projectYML)
	if !strings.Contains(projectYMLText, `path: "`+localApusPath+`"`) {
		t.Fatalf("project.yml should use APUS_PACKAGE_PATH when set, got:\n%s", projectYMLText)
	}
}

func TestGenerateFailsForInvalidConfiguredApusPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APUS_PACKAGE_PATH", "./missing-apus")

	data := NewData("MyApp", "SIM-UDID", "swiftui")
	err := Generate(data, root)
	if err == nil {
		t.Fatalf("expected Generate() to fail for invalid APUS_PACKAGE_PATH")
	}
	if !strings.Contains(err.Error(), "invalid APUS_PACKAGE_PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
}
