package preflight

import (
	"errors"
	"strings"
	"testing"
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
