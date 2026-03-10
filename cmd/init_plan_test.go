package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestBuildInitPlan_RemoteDependency(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "Demo.xcodeproj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.pbxproj"), []byte("// empty pbxproj"), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	entryFile := filepath.Join(dir, "DemoApp.swift")
	entrySrc := "import SwiftUI\n\n@main\nstruct DemoApp: App {\n    var body: some Scene { WindowGroup { Text(\"Demo\") } }\n}\n"
	if err := os.WriteFile(entryFile, []byte(entrySrc), 0o644); err != nil {
		t.Fatalf("write entry file: %v", err)
	}

	plan, err := buildInitPlan(dir, &xcode.ProjectInfo{
		ProjectPath: projectDir,
		ProjectName: "Demo",
		Target:      "Demo",
		EntryFile:   entryFile,
		IsSwiftUI:   true,
	}, preflight.Report{
		Classification: preflight.ClassificationSupported,
	}, "")
	if err != nil {
		t.Fatalf("buildInitPlan() error: %v", err)
	}

	if plan.CurrentDependencySource != "none" {
		t.Fatalf("expected no current dependency source, got %q", plan.CurrentDependencySource)
	}
	if plan.DesiredDependencySource != "remote" {
		t.Fatalf("expected remote desired dependency source, got %q", plan.DesiredDependencySource)
	}
	if len(plan.Actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(plan.Actions))
	}
	if plan.Actions[0].File != "project.pbxproj" || !strings.Contains(plan.Actions[0].Detail, "remote") {
		t.Fatalf("unexpected first action: %+v", plan.Actions[0])
	}
	if plan.Actions[2].File != "DemoApp.swift" || plan.Actions[2].Action != "would_modify" {
		t.Fatalf("unexpected entry action: %+v", plan.Actions[2])
	}
}

func TestBuildInitPlan_AlreadyIntegratedLocalDependency(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "Demo.xcodeproj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	localPBX := `1234567890ABCDEF12345678 /* XCLocalSwiftPackageReference "../apus" */ = {
		isa = XCLocalSwiftPackageReference;
		relativePath = ../apus;
	};`
	if err := os.WriteFile(filepath.Join(projectDir, "project.pbxproj"), []byte(localPBX), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	entryFile := filepath.Join(dir, "DemoApp.swift")
	entrySrc := "import SwiftUI\n#if DEBUG\nimport Apus\n#endif\n\n@main\nstruct DemoApp: App {\n    init() {\n        #if DEBUG\n        Apus.shared.start(interceptNetwork: true)\n        #endif\n    }\n\n    var body: some Scene { WindowGroup { Text(\"Demo\") } }\n}\n"
	if err := os.WriteFile(entryFile, []byte(entrySrc), 0o644); err != nil {
		t.Fatalf("write entry file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	plan, err := buildInitPlan(dir, &xcode.ProjectInfo{
		ProjectPath: projectDir,
		ProjectName: "Demo",
		Target:      "Demo",
		EntryFile:   entryFile,
		IsSwiftUI:   true,
	}, preflight.Report{
		Classification: preflight.ClassificationRisky,
		Checks: []preflight.Check{
			{
				Name:   "project:entrypoint",
				Status: preflight.CheckStatusWarn,
				Detail: "warning detail",
				Hint:   "warning hint",
			},
		},
	}, filepath.Join(dir, "..", "apus"))
	if err != nil {
		t.Fatalf("buildInitPlan() error: %v", err)
	}

	if plan.CurrentDependencySource != "local" {
		t.Fatalf("expected local current dependency source, got %q", plan.CurrentDependencySource)
	}
	if plan.Actions[0].Action != "would_verify" {
		t.Fatalf("expected first action to verify existing dependency, got %+v", plan.Actions[0])
	}
	if plan.Actions[1].Action != "skip" {
		t.Fatalf("expected entry action to skip existing integration, got %+v", plan.Actions[1])
	}
	if len(plan.Warnings) != 2 {
		t.Fatalf("expected warning detail and hint, got %v", plan.Warnings)
	}
}
