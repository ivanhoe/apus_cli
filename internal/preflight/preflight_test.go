package preflight

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestValidateScopeNewFailsWhenXcodeGenMissing(t *testing.T) {
	origLook := lookPathFn
	origRun := runCombinedOutputFn
	t.Cleanup(func() {
		lookPathFn = origLook
		runCombinedOutputFn = origRun
	})

	lookPathFn = func(name string) (string, error) {
		if name == "xcodegen" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
	runCombinedOutputFn = func(name string, args ...string) (string, error) {
		if name == "xcrun" {
			payload, err := json.Marshal(map[string]any{
				"devices": map[string]any{
					"com.apple.CoreSimulator.SimRuntime.iOS-18-0": []map[string]any{
						{"name": "iPhone 16", "isAvailable": true},
					},
				},
			})
			if err != nil {
				t.Fatalf("marshal simctl payload: %v", err)
			}
			return string(payload), nil
		}
		return "/Applications/Xcode.app/Contents/Developer", nil
	}

	err := Validate(ScopeNew)
	if err == nil {
		t.Fatalf("expected Validate(ScopeNew) to fail")
	}
	if !strings.Contains(err.Error(), "xcodegen") {
		t.Fatalf("expected error to mention xcodegen, got: %v", err)
	}
}

func TestValidateScopeInitPassesWhenRequirementsArePresent(t *testing.T) {
	origLook := lookPathFn
	origRun := runCombinedOutputFn
	t.Cleanup(func() {
		lookPathFn = origLook
		runCombinedOutputFn = origRun
	})

	lookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	runCombinedOutputFn = func(name string, args ...string) (string, error) {
		return "/Applications/Xcode.app/Contents/Developer", nil
	}

	if err := Validate(ScopeInit); err != nil {
		t.Fatalf("expected Validate(ScopeInit) to pass, got: %v", err)
	}
}

func TestValidateScopeDoctorFailsWhenNoIPhoneSimulatorIsAvailable(t *testing.T) {
	origLook := lookPathFn
	origRun := runCombinedOutputFn
	t.Cleanup(func() {
		lookPathFn = origLook
		runCombinedOutputFn = origRun
	})

	lookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	runCombinedOutputFn = func(name string, args ...string) (string, error) {
		if name == "xcode-select" {
			return "/Applications/Xcode.app/Contents/Developer", nil
		}

		payload, err := json.Marshal(map[string]any{
			"devices": map[string]any{
				"com.apple.CoreSimulator.SimRuntime.iOS-18-0": []map[string]any{
					{"name": "iPad Pro (11-inch)", "isAvailable": true},
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal simctl payload: %v", err)
		}
		return string(payload), nil
	}

	err := Validate(ScopeDoctor)
	if err == nil {
		t.Fatalf("expected Validate(ScopeDoctor) to fail when no iPhone simulator exists")
	}
	if !strings.Contains(err.Error(), "simulator:iphone") {
		t.Fatalf("expected simulator failure, got: %v", err)
	}
}

func TestRunWithOptions_ProjectWarningYieldsRiskyClassification(t *testing.T) {
	origLook := lookPathFn
	origRun := runCombinedOutputFn
	origDetect := detectProjectFn
	t.Cleanup(func() {
		lookPathFn = origLook
		runCombinedOutputFn = origRun
		detectProjectFn = origDetect
	})

	lookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	runCombinedOutputFn = func(name string, args ...string) (string, error) {
		if name == "xcrun" {
			payload, err := json.Marshal(map[string]any{
				"devices": map[string]any{
					"com.apple.CoreSimulator.SimRuntime.iOS-18-0": []map[string]any{
						{"name": "iPhone 16", "isAvailable": true},
					},
				},
			})
			if err != nil {
				t.Fatalf("marshal simctl payload: %v", err)
			}
			return string(payload), nil
		}
		return "/Applications/Xcode.app/Contents/Developer", nil
	}
	detectProjectFn = func(dir, target string) (*xcode.ProjectInfo, error) {
		return &xcode.ProjectInfo{
			ProjectPath: "/tmp/Demo.xcodeproj",
			ProjectName: "Demo",
			Target:      "Demo",
		}, nil
	}

	report := RunWithOptions(Options{
		Scope:      ScopeInit,
		ProjectDir: "/tmp/demo",
	})

	if report.Classification != ClassificationRisky {
		t.Fatalf("expected risky classification, got %q", report.Classification)
	}
	if !report.HasWarnings() {
		t.Fatalf("expected project warning")
	}
}

func TestRunWithOptions_ProjectDetectionFailureYieldsUnsupportedClassification(t *testing.T) {
	origLook := lookPathFn
	origRun := runCombinedOutputFn
	origDetect := detectProjectFn
	t.Cleanup(func() {
		lookPathFn = origLook
		runCombinedOutputFn = origRun
		detectProjectFn = origDetect
	})

	lookPathFn = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	runCombinedOutputFn = func(name string, args ...string) (string, error) {
		return "/Applications/Xcode.app/Contents/Developer", nil
	}
	detectProjectFn = func(dir, target string) (*xcode.ProjectInfo, error) {
		return nil, errors.New("multiple app targets found in Demo.xcodeproj: App, Demo — rerun with --target <name>")
	}

	report := RunWithOptions(Options{
		Scope:      ScopeInit,
		ProjectDir: "/tmp/demo",
	})

	if report.Classification != ClassificationUnsupported {
		t.Fatalf("expected unsupported classification, got %q", report.Classification)
	}
	if !report.HasFailures() {
		t.Fatalf("expected project detection failure")
	}
	if report.Project == nil || report.Project.Directory != "/tmp/demo" {
		t.Fatalf("expected project assessment to preserve directory")
	}
}
