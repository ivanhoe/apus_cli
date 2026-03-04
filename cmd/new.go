package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/ivanhoe/apus-cli/internal/builder"
	"github.com/ivanhoe/apus-cli/internal/scaffold"
	"github.com/ivanhoe/apus-cli/internal/simulator"
	"github.com/ivanhoe/apus-cli/internal/terminal"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new <AppName>",
	Short: "Create a new iOS project with Apus pre-integrated",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

var appNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

func runNew(cmd *cobra.Command, args []string) error {
	appName := args[0]
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid app name %q — use letters, digits, underscores; start with a letter", appName)
	}

	// Prevent overwriting an existing directory
	if _, err := os.Stat(appName); err == nil {
		return fmt.Errorf("directory %q already exists", appName)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	p := terminal.NewProgress(5)

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

	data := scaffold.NewData(appName, sim.UDID)

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
		if err = simulator.Boot(sim.UDID); err != nil {
			done(err)
			terminal.Fatal("simulator boot failed", err)
			return err
		}
		if err = simulator.Install(sim.UDID, result.AppPath); err != nil {
			done(err)
			terminal.Fatal("install failed", err)
			return err
		}
		if err = simulator.Launch(sim.UDID, result.BundleID); err != nil {
			done(err)
			terminal.Fatal("launch failed", err)
			return err
		}
		done(nil)
	}

	terminal.Success(appName, sim.Name, "http://localhost:9847/mcp")
	return nil
}
