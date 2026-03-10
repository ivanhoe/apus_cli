package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/preflight"
)

func TestDryRunInit_JSONOutput(t *testing.T) {
	output := captureStdout(t, func() {
		err := dryRunInit(initPlan{
			Classification: preflight.ClassificationSupported,
			Actions: []jsonAction{
				{Action: "would_modify", File: "project.pbxproj"},
			},
		}, true)
		if err != nil {
			t.Fatalf("dryRunInit() error: %v", err)
		}
	})

	if !strings.Contains(output, `"mode": "dry-run"`) {
		t.Fatalf("expected dry-run JSON mode, got:\n%s", output)
	}
	if !strings.Contains(output, `"project.pbxproj"`) {
		t.Fatalf("expected dry-run JSON actions, got:\n%s", output)
	}
}

func TestDryRunRemove_JSONOutput(t *testing.T) {
	origRemoveDryRun := removeDryRun
	removeDryRun = true
	t.Cleanup(func() {
		removeDryRun = origRemoveDryRun
	})

	output := captureStdout(t, func() {
		err := dryRunRemove(removePlan{
			Integrated: true,
			Actions: []jsonAction{
				{Action: "would_delete", File: "AGENTS.md"},
			},
		}, true)
		if err != nil {
			t.Fatalf("dryRunRemove() error: %v", err)
		}
	})

	if !strings.Contains(output, `"mode": "dry-run"`) {
		t.Fatalf("expected dry-run JSON mode, got:\n%s", output)
	}
	if !strings.Contains(output, `"AGENTS.md"`) {
		t.Fatalf("expected dry-run JSON actions, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}

	os.Stdout = writer

	fn()

	os.Stdout = origStdout
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close(): %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll(): %v", err)
	}

	return string(data)
}
