package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ivanhoe/apus-cli/internal/builder"
	"github.com/ivanhoe/apus-cli/internal/terminal"
	"github.com/ivanhoe/apus-cli/internal/xcode"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Add Apus to an existing Xcode project",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// ── Detect project ──
	info, err := xcode.DetectProject(cwd)
	if err != nil {
		terminal.Fatal("project detection failed", err)
		return err
	}

	terminal.Detected(filepath.Base(info.ProjectPath), info.Target+" (SwiftUI)")

	p := terminal.NewProgress(4)

	// ── Step 1: Modify .pbxproj ──
	{
		done := p.Start("Adding Apus to project.pbxproj")
		err = xcode.AddApusDependency(info.ProjectPath, info.Target)
		done(err)
		if err != nil {
			terminal.Fatal("pbxproj modification failed", err)
			return err
		}
	}

	// ── Step 2: Resolve package dependencies ──
	{
		done := p.Start("Resolving Swift Package dependencies")
		err = builder.ResolvePackageDependencies(info.ProjectPath)
		done(err)
		if err != nil {
			terminal.Fatal("package resolution failed", err)
			terminal.Info("You can resolve manually: xcodebuild -resolvePackageDependencies")
			// Non-fatal: user can open Xcode to resolve
		}
	}

	// ── Step 3: Inject Apus.shared.start() ──
	{
		done := p.Start("Injecting Apus.shared.start() in " + filepath.Base(info.EntryFile))
		if info.EntryFile == "" {
			done(fmt.Errorf("no entry file detected"))
			terminal.Info("Add `Apus.shared.start()` manually in your App init()")
		} else {
			err = xcode.InjectApus(info.EntryFile)
			done(err)
			if err != nil {
				terminal.Fatal("Swift injection failed", err)
				return err
			}
		}
	}

	// ── Step 4: Write AGENTS.md ──
	{
		done := p.Start("Writing AGENTS.md")
		err = writeAgentsMD(cwd, info)
		done(err)
		if err != nil {
			terminal.Fatal("AGENTS.md write failed", err)
			// Non-fatal
		}
	}

	terminal.InitSuccess(info.ProjectName)
	return nil
}

const agentsMDTemplate = `# AGENTS.md — {{ .ProjectName }}

## MCP Server

Apus runs at ` + "`http://localhost:9847/mcp`" + ` when the app is running in the simulator or on a device.

## Build & Run

` + "```bash" + `
xcodebuild -scheme {{ .ProjectName }} \
  -destination "platform=iOS Simulator,name=iPhone 16e" \
  -quiet CODE_SIGN_IDENTITY="-"
` + "```" + `

## Key MCP Tools

| Tool | Description |
|------|-------------|
| ` + "`get_logs`" + ` | App console output |
| ` + "`get_screenshot`" + ` | Current screen as JPEG |
| ` + "`get_view_hierarchy`" + ` | Full SwiftUI/UIKit view tree |
| ` + "`ui_interact`" + ` | tap, double_tap, swipe, type_text |
| ` + "`get_network_history`" + ` | All HTTP/HTTPS traffic |
| ` + "`hot_reload`" + ` | Inject Swift changes without recompile |
`

func writeAgentsMD(dir string, info *xcode.ProjectInfo) error {
	outPath := filepath.Join(dir, "AGENTS.md")

	// Skip if already exists
	if _, err := os.Stat(outPath); err == nil {
		return nil
	}

	tmpl, err := template.New("agents").Parse(agentsMDTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := struct {
		ProjectName string
		BundleOrg   string
	}{
		ProjectName: info.ProjectName,
		BundleOrg:   strings.ToLower(info.ProjectName),
	}
	return tmpl.Execute(f, data)
}
