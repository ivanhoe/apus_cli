package cmd

import (
	"testing"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func TestClassifyPreflightReportError_TargetFailure(t *testing.T) {
	err := classifyPreflightReportError(preflight.Report{
		Checks: []preflight.Check{
			{
				Name:   "project:detect",
				Status: preflight.CheckStatusFail,
				Detail: "multiple app targets found in Demo.xcodeproj: App, Demo - rerun with --target <name>",
			},
		},
	})

	if got := exitCode(err); got != exitCodeTarget {
		t.Fatalf("expected target exit code, got %d", got)
	}
}

func TestDependencySourceLabel(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "none", got: dependencySourceLabel(xcode.DependencyState{}), want: "none"},
		{name: "remote", got: dependencySourceLabel(xcode.DependencyState{Remote: true}), want: "remote"},
		{name: "local", got: dependencySourceLabel(xcode.DependencyState{Local: true}), want: "local"},
		{name: "both", got: dependencySourceLabel(xcode.DependencyState{Remote: true, Local: true}), want: "remote+local"},
	}

	for _, tc := range tests {
		if tc.got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, tc.got)
		}
	}
}
