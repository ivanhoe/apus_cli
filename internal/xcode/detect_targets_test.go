package xcode

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFilterAppTargets(t *testing.T) {
	in := []string{
		"MyApp",
		"MyAppTests",
		"MyAppUITests",
		"MyAppNotificationExtension",
		"MyAppShareExtension",
	}
	got := filterAppTargets(in)
	want := []string{"MyApp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterAppTargets() = %v, want %v", got, want)
	}
}

func TestChooseAppTarget_PrefersProjectName(t *testing.T) {
	projPath := "/tmp/FancyApp.xcodeproj"
	targets := []string{"OtherApp", "FancyApp"}

	got, err := chooseAppTarget(targets, projPath, "")
	if err != nil {
		t.Fatalf("chooseAppTarget() returned error: %v", err)
	}
	if got != "FancyApp" {
		t.Fatalf("chooseAppTarget() = %q, want %q", got, "FancyApp")
	}
}

func TestChooseAppTarget_NoAppTarget(t *testing.T) {
	projPath := "/tmp/FancyApp.xcodeproj"
	targets := []string{"FancyAppTests", "FancyAppUITests"}

	_, err := chooseAppTarget(targets, projPath, "")
	if err == nil {
		t.Fatalf("expected chooseAppTarget() to fail when only test targets exist")
	}
}

func TestChooseAppTarget_UsesPreferredTarget(t *testing.T) {
	projPath := "/tmp/FancyApp.xcodeproj"
	targets := []string{"OtherApp", "FancyApp"}

	got, err := chooseAppTarget(targets, projPath, "OtherApp")
	if err != nil {
		t.Fatalf("chooseAppTarget() returned error: %v", err)
	}
	if got != "OtherApp" {
		t.Fatalf("chooseAppTarget() = %q, want %q", got, "OtherApp")
	}
}

func TestChooseAppTarget_FailsWhenAmbiguous(t *testing.T) {
	projPath := "/tmp/Workspace.xcodeproj"
	targets := []string{"Alpha", "Beta"}

	_, err := chooseAppTarget(targets, projPath, "")
	if err == nil {
		t.Fatalf("expected chooseAppTarget() to fail for ambiguous targets")
	}
	if !strings.Contains(err.Error(), "--target") {
		t.Fatalf("expected ambiguity hint in error, got: %v", err)
	}
}

func TestFormatXcodebuildListError(t *testing.T) {
	err := errors.New("exit status 74")
	got := formatXcodebuildListError(err, "line1\nline2\n")
	if !strings.Contains(got, "xcodebuild -list: exit status 74") {
		t.Fatalf("missing command error in output: %q", got)
	}
	if !strings.Contains(got, "line1\nline2") {
		t.Fatalf("missing stderr details in output: %q", got)
	}
}

func TestListTargetsFromPBXProj(t *testing.T) {
	tmp := t.TempDir()
	projPath := filepath.Join(tmp, "Demo.xcodeproj")
	if err := os.MkdirAll(projPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	pbx := `// !$*UTF8*$!
{
objects = {
		AAAAAAAAAAAAAAAAAAAAAAAA /* Demo */ = {
			isa = PBXNativeTarget;
		};
		BBBBBBBBBBBBBBBBBBBBBBBB /* DemoTests */ = {
			isa = PBXNativeTarget;
		};
		CCCCCCCCCCCCCCCCCCCCCCCC /* Demo */ = {
			isa = PBXNativeTarget;
		};
};
}
`
	if err := os.WriteFile(filepath.Join(projPath, "project.pbxproj"), []byte(pbx), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	got, err := listTargetsFromPBXProj(projPath)
	if err != nil {
		t.Fatalf("listTargetsFromPBXProj() error: %v", err)
	}
	want := []string{"Demo", "DemoTests"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("listTargetsFromPBXProj() = %v, want %v", got, want)
	}
}
