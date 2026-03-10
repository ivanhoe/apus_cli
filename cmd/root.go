package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "apus",
	Short: "Apus CLI — embed the MCP debug server in your iOS app",
	Long: `Apus CLI makes it trivial to get an AI-inspectable iOS app running.

  apus doctor      — verify your local toolchain first
  apus new MyApp   — create a new SwiftUI project with Apus pre-integrated
  apus init        — best-effort add Apus to an existing Xcode project
  apus remove      — remove Apus from an existing Xcode project
  apus status      — check if Apus is integrated in the current project`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(statusCmd)
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if shouldPrintError(err) {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
		}
		os.Exit(1)
	}
}
