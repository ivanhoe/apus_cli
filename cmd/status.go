package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
	"github.com/spf13/cobra"
)

var statusTarget string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether Apus is integrated in the current project",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusTarget, "target", "", "App target to inspect when the project contains multiple app targets")
}

func runStatus(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	info, err := xcode.DetectProjectWithTarget(cwd, statusTarget)
	if err != nil {
		terminal.Fatal("project detection failed", err)
		return markPrinted(err)
	}

	terminal.Detected(filepath.Base(info.ProjectPath), info.Target)

	dependencyState, depErr := xcode.DetectApusDependency(info.ProjectPath)
	if depErr != nil {
		return depErr
	}
	pbxHasApus := dependencyState.Any()

	hasSwift, _ := xcode.HasApusIntegration(cwd)

	hasAgents, _ := readManagedAgentsFile(cwd)

	if !pbxHasApus && !hasSwift && !hasAgents {
		terminal.StatusNotIntegrated()
		return nil
	}

	terminal.StatusIntegrated(pbxHasApus, hasSwift, hasAgents)
	return nil
}
