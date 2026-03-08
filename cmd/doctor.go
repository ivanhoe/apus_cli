package cmd

import (
	"fmt"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run environment checks required by apus commands",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) error {
	report := preflight.Run(preflight.ScopeDoctor)

	fmt.Println("Preflight checks:")
	for _, c := range report.Checks {
		if c.OK {
			fmt.Printf("  [ok]   %s\n", c.Name)
			continue
		}
		fmt.Printf("  [fail] %s\n", c.Name)
		if c.Hint != "" {
			fmt.Printf("         %s\n", c.Hint)
		}
	}

	if report.HasFailures() {
		return fmt.Errorf("preflight checks failed")
	}

	fmt.Println("\nEnvironment looks good.")
	return nil
}
