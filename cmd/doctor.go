package cmd

import (
	"fmt"

	"github.com/ivanhoe/apus_cli/internal/preflight"
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

func runDoctor(_ *cobra.Command, _ []string) error {
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

	report := preflight.RunWithOptions(preflight.Options{
		Scope:      preflight.ScopeDoctor,
		ProjectDir: projectDir,
		Target:     doctorTarget,
	})

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

	fmt.Printf("Doctor report: %s\n", report.Classification)
	fmt.Println()
	fmt.Println("Checks:")
	for _, c := range report.Checks {
		switch c.Status {
		case preflight.CheckStatusPass:
			fmt.Printf("  [ok]   %s\n", c.Name)
		case preflight.CheckStatusWarn:
			fmt.Printf("  [warn] %s\n", c.Name)
		default:
			fmt.Printf("  [fail] %s\n", c.Name)
		}
		if c.Detail != "" {
			fmt.Printf("         %s\n", c.Detail)
		}
		if c.Hint != "" {
			fmt.Printf("         %s\n", c.Hint)
		}
	}

	if report.Project != nil && report.Project.ProjectPath != "" {
		fmt.Println()
		fmt.Println("Project:")
		fmt.Printf("  directory: %s\n", report.Project.Directory)
		fmt.Printf("  project:   %s\n", report.Project.ProjectPath)
		fmt.Printf("  target:    %s\n", report.Project.Target)
		if report.Project.EntryFile != "" {
			fmt.Printf("  entry:     %s\n", report.Project.EntryFile)
		}
	}

	if report.HasFailures() {
		fmt.Println()
		fmt.Println("Doctor found blocking issues.")
		return classifyPreflightReportError(report)
	}

	fmt.Println()
	if report.HasWarnings() {
		fmt.Println("Environment looks usable, but this project is in a risky state.")
		return nil
	}

	fmt.Println("Environment looks good.")
	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
