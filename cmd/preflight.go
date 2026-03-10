package cmd

import (
	"errors"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/terminal"
)

func classifyPreflightReportError(report preflight.Report) error {
	err := report.Validate()
	if err == nil {
		return nil
	}

	for _, failure := range report.Failures() {
		if failure.Name == "project:detect" {
			return classifyProjectError(errors.New(failure.Detail))
		}
	}

	return preflightError(err)
}

func printPreflightWarnings(report preflight.Report) {
	for _, warning := range report.Warnings() {
		if warning.Detail != "" {
			terminal.Info("Warning: " + warning.Detail)
		}
		if warning.Hint != "" {
			terminal.Info(warning.Hint)
		}
	}
}
