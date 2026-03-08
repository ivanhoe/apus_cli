package builder

import (
	"strings"
	"testing"
)

func TestEnsureXcodeGen_WhenPresent(t *testing.T) {
	orig := lookPathFn
	t.Cleanup(func() { lookPathFn = orig })

	lookPathFn = func(name string) (string, error) {
		if name != "xcodegen" {
			t.Fatalf("expected lookup for xcodegen, got %q", name)
		}
		return "/usr/local/bin/xcodegen", nil
	}

	if err := EnsureXcodeGen(); err != nil {
		t.Fatalf("expected EnsureXcodeGen() to pass, got: %v", err)
	}
}

func TestEnsureXcodeGen_WhenMissing(t *testing.T) {
	orig := lookPathFn
	t.Cleanup(func() { lookPathFn = orig })

	lookPathFn = func(name string) (string, error) { return "", errNotFound{} }

	err := EnsureXcodeGen()
	if err == nil {
		t.Fatalf("expected EnsureXcodeGen() to fail")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "Install it manually") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }
