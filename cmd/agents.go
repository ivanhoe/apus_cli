package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

const managedAgentsMarker = "<!-- apus-cli:managed -->"

func readManagedAgentsFile(dir string) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return isManagedAgentsContent(string(raw)), nil
}

func isManagedAgentsContent(raw string) bool {
	if strings.Contains(raw, managedAgentsMarker) {
		return true
	}
	return isLegacyManagedAgentsContent(raw)
}

func isLegacyManagedAgentsContent(raw string) bool {
	commonSignals := []string{
		"# AGENTS.md — ",
		"## MCP Server",
		"Apus runs at `http://localhost:9847/mcp`",
		"## Key MCP Tools",
		"`get_view_hierarchy`",
		"`hot_reload`",
	}
	for _, signal := range commonSignals {
		if !strings.Contains(raw, signal) {
			return false
		}
	}

	isLegacyInit := strings.Contains(raw, `xcodebuild -scheme `) &&
		strings.Contains(raw, `-destination "platform=iOS Simulator`)
	isLegacyScaffold := strings.Contains(raw, "POST http://localhost:9847/mcp") &&
		strings.Contains(raw, "SIMCTL_CHILD_APUS_PROJECT_ROOT")

	return isLegacyInit || isLegacyScaffold
}
