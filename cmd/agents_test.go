package cmd

import "testing"

func TestIsManagedAgentsContent_WithMarker(t *testing.T) {
	content := managedAgentsMarker + "\n# AGENTS.md — Demo\n"
	if !isManagedAgentsContent(content) {
		t.Fatalf("expected managed marker to classify AGENTS.md as ours")
	}
}

func TestIsManagedAgentsContent_LegacyInitFingerprint(t *testing.T) {
	content := `# AGENTS.md — Demo

## MCP Server

Apus runs at ` + "`http://localhost:9847/mcp`" + ` when the app is running in the simulator or on a device.

## Build & Run

` + "```bash" + `
xcodebuild -scheme Demo \
  -destination "platform=iOS Simulator,name=iPhone 16e" \
  -quiet CODE_SIGN_IDENTITY="-"
` + "```" + `

## Key MCP Tools

| Tool | Description |
|------|-------------|
| ` + "`get_view_hierarchy`" + ` | Full SwiftUI/UIKit view tree |
| ` + "`hot_reload`" + ` | Inject Swift changes without recompile |
`

	if !isManagedAgentsContent(content) {
		t.Fatalf("expected legacy init AGENTS.md fingerprint to be recognized")
	}
}

func TestIsManagedAgentsContent_CustomFileMentioningApus(t *testing.T) {
	content := `# AGENTS.md

This is a custom file.
Apus runs at http://localhost:9847/mcp.
`

	if isManagedAgentsContent(content) {
		t.Fatalf("expected custom AGENTS.md to remain unmanaged")
	}
}
