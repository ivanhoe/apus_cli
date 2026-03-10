package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/ivanhoe/apus_cli/internal/builder"
	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/scaffold"
	"github.com/ivanhoe/apus_cli/internal/simulator"
	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <AppName>",
	Short: "Create a new iOS project with Apus pre-integrated (SwiftUI template)",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

var (
	newTemplate string
	mcpPort     int
)

const defaultMCPPort = 9847

func init() {
	newCmd.Flags().StringVar(&newTemplate, "template", "swiftui", "project template to use (currently only: swiftui)")
	newCmd.Flags().IntVar(&mcpPort, "port", defaultMCPPort, "MCP server port to poll for health check")
}

func runNew(cmd *cobra.Command, args []string) error {
	if err := preflight.Validate(preflight.ScopeNew); err != nil {
		return err
	}

	appName := args[0]
	if err := scaffold.ValidateAppName(appName); err != nil {
		return err
	}

	if newTemplate != "swiftui" {
		return fmt.Errorf("template %q is not supported yet — currently only \"swiftui\" is available", newTemplate)
	}

	// Prevent overwriting an existing directory
	if _, err := os.Stat(appName); err == nil {
		return fmt.Errorf("directory %q already exists", appName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	p := terminal.NewProgress(6)

	// ── Step 1: Pick simulator (before generating files so UDID goes into AGENTS.md) ──
	var sim simulator.Device
	{
		done := p.Start("Selecting simulator")
		sim, err = simulator.PickBestDevice()
		done(err)
		if err != nil {
			terminal.Fatal("no iPhone simulator available", err)
			return err
		}
		terminal.Info(sim.Name + " · " + sim.UDID)
	}

	data := scaffold.NewData(appName, sim.UDID, newTemplate)

	// ── Step 2: Generate project files ──
	{
		done := p.Start("Generating project files")
		err = scaffold.Generate(data, cwd)
		done(err)
		if err != nil {
			terminal.Fatal("scaffold failed", err)
			return err
		}
	}

	projectDir := scaffold.ProjectDir(data, cwd)

	// ── Step 3: xcodegen ──
	{
		done := p.Start("Running xcodegen")
		if err = builder.EnsureXcodeGen(); err != nil {
			done(err)
			terminal.Fatal("xcodegen not available", err)
			return err
		}
		err = builder.Generate(projectDir)
		done(err)
		if err != nil {
			terminal.Fatal("xcodegen generate failed", err)
			return err
		}
	}

	// ── Step 4: Build ──
	var result *builder.BuildResult
	{
		done := p.Start("Building")
		result, err = builder.Build(projectDir, appName, sim.UDID)
		done(err)
		if err != nil {
			terminal.Fatal("build failed", err)
			return err
		}
	}

	// ── Step 5: Boot simulator + launch app ──
	{
		done := p.Start("Launching app")
		_ = simulator.OpenSimulatorApp()
		if err = simulator.ShutdownOtherBootedDevices(sim.UDID); err != nil {
			terminal.Info("warning: could not shut down other booted simulators; MCP port may conflict")
		}
		if err = simulator.Boot(sim.UDID); err != nil {
			done(err)
			terminal.Fatal("simulator boot failed", err)
			return err
		}
		if err = simulator.UninstallIfPresent(sim.UDID, result.BundleID); err != nil {
			terminal.Info("warning: cleanup old install failed, continuing with fresh install")
		}
		if err = simulator.Install(sim.UDID, result.AppPath); err != nil {
			done(err)
			terminal.Fatal("install failed", err)
			return err
		}
		if err = simulator.LaunchWithProjectRoot(sim.UDID, result.BundleID, projectDir); err != nil {
			done(err)
			terminal.Fatal("launch failed", err)
			return err
		}
		done(nil)
	}

	// ── Step 6: MCP health check ──
	{
		done := p.Start("Waiting for MCP server")
		mcpURL := fmt.Sprintf("http://127.0.0.1:%d/", mcpPort)
		err = simulator.WaitForMCPReady(mcpURL, 30*time.Second)
		done(err)
		if err != nil {
			terminal.Fatal("MCP health check failed", err)
			terminal.Info(fmt.Sprintf("The app was launched, but MCP did not respond on port %d in time.", mcpPort))
			terminal.Info(fmt.Sprintf("Try running the app again and confirm no other simulator app is using port %d.", mcpPort))
			return err
		}
	}

	terminal.Success(appName, sim.Name, fmt.Sprintf("http://localhost:%d/mcp", mcpPort))
	return nil
}
