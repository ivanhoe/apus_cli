package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether Apus is integrated in the current project",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func runStatus(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	info, err := xcode.DetectProject(cwd)
	if err != nil {
		terminal.Fatal("project detection failed", err)
		return err
	}

	terminal.Detected(filepath.Base(info.ProjectPath), info.Target)

	pbxPath := filepath.Join(info.ProjectPath, "project.pbxproj")
	pbxRaw, _ := os.ReadFile(pbxPath)
	pbxHasApus := strings.Contains(string(pbxRaw), "github.com/ivanhoe/apus")

	hasSwift, _ := xcode.HasApusIntegration(cwd)

	agentsPath := filepath.Join(cwd, "AGENTS.md")
	agentsRaw, _ := os.ReadFile(agentsPath)
	hasAgents := strings.Contains(string(agentsRaw), "Apus runs at")

	if !pbxHasApus && !hasSwift && !hasAgents {
		terminal.StatusNotIntegrated()
		return nil
	}

	terminal.StatusIntegrated(pbxHasApus, hasSwift, hasAgents)
	return nil
}
