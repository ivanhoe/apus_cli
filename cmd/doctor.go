package cmd

import (
	"fmt"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/spf13/cobra"
)

var doctorPath string
var doctorJSON bool
var doctorTarget string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run environment checks required by apus commands",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().StringVar(&doctorPath, "path", "", "Project directory to inspect instead of the current working directory")
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Print the doctor report as JSON")
	doctorCmd.Flags().StringVar(&doctorTarget, "target", "", "App target to inspect when the project contains multiple app targets")
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	projectDir, err := resolveCommandPath(doctorPath)
	if err != nil {
		err = usageError(err)
		if doctorJSON {
			return writeJSONResult(struct {
				Error string `json:"error"`
			}{Error: err.Error()}, err)
		}
		return err
	}

	var spinner *terminal.Spinner
	progress := func(string) {}
	if !doctorJSON {
		spinner = terminal.NewSpinner("Checking xcodebuild")
		progress = spinner.Update
	}

	report := preflight.RunWithOptions(preflight.Options{
		Scope:      preflight.ScopeDoctor,
		ProjectDir: projectDir,
		Target:     doctorTarget,
		Progress:   progress,
	})
	if spinner != nil {
		spinner.Stop()
	}

	if doctorJSON {
		reportErr := error(nil)
		if report.HasFailures() {
			reportErr = classifyPreflightReportError(report)
		}
		return writeJSONResult(struct {
			Report preflight.Report `json:"report"`
			Error  string           `json:"error,omitempty"`
		}{
			Report: report,
			Error:  errorString(reportErr),
		}, reportErr)
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	fmt.Fprintf(out, "Doctor report: %s\n", report.Classification)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Checks:")
	for _, c := range report.Checks {
		writer := out
		if c.Status == preflight.CheckStatusFail {
			writer = errOut
		}
		switch c.Status {
		case preflight.CheckStatusPass:
			fmt.Fprintf(writer, "  [ok]   %s\n", c.Name)
		case preflight.CheckStatusWarn:
			fmt.Fprintf(writer, "  [warn] %s\n", c.Name)
		default:
			fmt.Fprintf(writer, "  [fail] %s\n", c.Name)
		}
		if c.Detail != "" {
			fmt.Fprintf(writer, "         %s\n", c.Detail)
		}
		if c.Hint != "" {
			fmt.Fprintf(writer, "         %s\n", c.Hint)
		}
	}

	if report.Project != nil && report.Project.ProjectPath != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Project:")
		fmt.Fprintf(out, "  directory: %s\n", report.Project.Directory)
		fmt.Fprintf(out, "  project:   %s\n", report.Project.ProjectPath)
		fmt.Fprintf(out, "  target:    %s\n", report.Project.Target)
		if report.Project.EntryFile != "" {
			fmt.Fprintf(out, "  entry:     %s\n", report.Project.EntryFile)
		}
	}

	if report.HasFailures() {
		fmt.Fprintln(errOut)
		fmt.Fprintln(errOut, "Doctor found blocking issues.")
		return classifyPreflightReportError(report)
	}

	fmt.Fprintln(out)
	if report.HasWarnings() {
		fmt.Fprintln(out, "Environment looks usable, but this project is in a risky state.")
		return nil
	}

	fmt.Fprintln(out, "Environment looks good.")
	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
