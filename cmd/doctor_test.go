package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestShouldRenderSpinner_FalseForNonFileWriters(t *testing.T) {
	orig := isTerminalFdFn
	t.Cleanup(func() {
		isTerminalFdFn = orig
	})

	isTerminalFdFn = func(uintptr) bool {
		t.Fatal("isTerminalFdFn should not be called for non-file writers")
		return false
	}

	if shouldRenderSpinner(&bytes.Buffer{}) {
		t.Fatalf("expected spinner to be disabled for non-file writers")
	}
}

func TestShouldRenderSpinner_UsesTTYCheckForFileWriters(t *testing.T) {
	orig := isTerminalFdFn
	t.Cleanup(func() {
		isTerminalFdFn = orig
	})

	called := false
	isTerminalFdFn = func(uintptr) bool {
		called = true
		return true
	}

	if !shouldRenderSpinner(os.Stdout) {
		t.Fatalf("expected spinner to be enabled when tty check passes")
	}
	if !called {
		t.Fatalf("expected tty check to be evaluated for file writers")
	}
}
