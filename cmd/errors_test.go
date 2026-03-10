package cmd

import "testing"

func TestShouldPrintError_SuppressesPrintedErrors(t *testing.T) {
	err := markPrinted(assertionError("already shown"))
	if shouldPrintError(err) {
		t.Fatalf("expected printed errors to be suppressed")
	}
}

func TestShouldPrintError_AllowsUnprintedErrors(t *testing.T) {
	if !shouldPrintError(assertionError("plain error")) {
		t.Fatalf("expected plain errors to be printed")
	}
}

func TestExitCode_DefaultsToGeneric(t *testing.T) {
	if got := exitCode(assertionError("plain error")); got != exitCodeGeneric {
		t.Fatalf("expected generic exit code, got %d", got)
	}
}

func TestWithExitCode_PreservesPrintedState(t *testing.T) {
	err := withExitCode(markPrinted(assertionError("already shown")), exitCodePackage)

	if shouldPrintError(err) {
		t.Fatalf("expected printed error to stay suppressed")
	}
	if got := exitCode(err); got != exitCodePackage {
		t.Fatalf("expected exit code %d, got %d", exitCodePackage, got)
	}
}

func TestClassifyProjectError_TargetAmbiguity(t *testing.T) {
	err := classifyProjectError(assertionError("multiple app targets found in Demo.xcodeproj: App, Demo — rerun with --target <name>"))
	if got := exitCode(err); got != exitCodeTarget {
		t.Fatalf("expected target exit code, got %d", got)
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
