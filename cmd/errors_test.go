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

type assertionError string

func (e assertionError) Error() string { return string(e) }
