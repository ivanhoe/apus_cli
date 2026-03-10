package cmd

import (
	"path/filepath"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestBuildRemovePlan(t *testing.T) {
	plan := buildRemovePlan("/tmp/demo", &xcode.ProjectInfo{
		ProjectPath: "/tmp/demo/Demo.xcodeproj",
		ProjectName: "Demo",
		Target:      "Demo",
		EntryFile:   "/tmp/demo/DemoApp.swift",
	}, true, true, false)

	if !plan.Integrated {
		t.Fatalf("expected plan to be integrated")
	}
	if len(plan.Actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(plan.Actions))
	}
	if plan.Actions[0].File != filepath.Base("/tmp/demo/DemoApp.swift") {
		t.Fatalf("unexpected first action: %+v", plan.Actions[0])
	}
	if plan.Actions[2].Action != "skip" {
		t.Fatalf("expected AGENTS action to be skipped, got %+v", plan.Actions[2])
	}
}
