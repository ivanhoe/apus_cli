package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

type initPlan struct {
	Project                 *jsonProject             `json:"project,omitempty"`
	Classification          preflight.Classification `json:"classification"`
	CurrentDependencySource string                   `json:"current_dependency_source"`
	DesiredDependencySource string                   `json:"desired_dependency_source"`
	PackagePath             string                   `json:"package_path,omitempty"`
	Actions                 []jsonAction             `json:"actions"`
	Warnings                []string                 `json:"warnings,omitempty"`
}

func buildInitPlan(projectDir string, info *xcode.ProjectInfo, report preflight.Report, resolvedPackagePath string) (initPlan, error) {
	dependencyState, err := xcode.DetectApusDependency(info.ProjectPath)
	if err != nil {
		return initPlan{}, projectError(err)
	}

	plan := initPlan{
		Project:                 jsonProjectFromInfo(projectDir, info),
		Classification:          report.Classification,
		CurrentDependencySource: dependencySourceLabel(dependencyState),
		DesiredDependencySource: desiredDependencySource(resolvedPackagePath),
		PackagePath:             resolvedPackagePath,
	}

	for _, warning := range report.Warnings() {
		if warning.Detail != "" {
			plan.Warnings = append(plan.Warnings, warning.Detail)
		}
		if warning.Hint != "" {
			plan.Warnings = append(plan.Warnings, warning.Hint)
		}
	}

	if dependencyState.Any() {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "would_verify",
			File:   "project.pbxproj",
			Detail: "ensure the existing Apus package dependency wiring is complete",
		})
	} else {
		plan.Actions = append(plan.Actions,
			jsonAction{
				Action: "would_modify",
				File:   "project.pbxproj",
				Detail: fmt.Sprintf("add Apus %s package dependency", desiredDependencySource(resolvedPackagePath)),
			},
			jsonAction{
				Action: "would_run",
				File:   "xcodebuild -resolvePackageDependencies",
				Detail: "resolve Swift Package dependencies after editing project.pbxproj",
			},
		)
	}

	entryAction, err := buildInitEntryAction(projectDir, info)
	if err != nil {
		return initPlan{}, err
	}
	plan.Actions = append(plan.Actions, entryAction)

	agentsPath := filepath.Join(projectDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "would_create",
			File:   "AGENTS.md",
			Detail: "write MCP tool reference for the project",
		})
	} else {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "skip",
			File:   "AGENTS.md",
			Detail: "AGENTS.md already exists",
		})
	}

	return plan, nil
}

func buildInitEntryAction(projectDir string, info *xcode.ProjectInfo) (jsonAction, error) {
	if info.EntryFile == "" {
		return jsonAction{
			Action: "manual",
			File:   "app entry point",
			Detail: "no Swift entry point was detected; add Apus.shared.start() manually",
		}, nil
	}

	raw, err := os.ReadFile(info.EntryFile)
	if err != nil {
		return jsonAction{}, mutationError(fmt.Errorf("read %s: %w", info.EntryFile, err))
	}

	src := string(raw)
	if strings.Contains(src, "import Apus") && strings.Contains(src, "Apus.shared.start(") {
		return jsonAction{
			Action: "skip",
			File:   filepath.Base(info.EntryFile),
			Detail: "Apus import and start call are already present",
		}, nil
	}

	return jsonAction{
		Action: "would_modify",
		File:   filepath.Base(info.EntryFile),
		Detail: "inject import Apus and Apus.shared.start()",
	}, nil
}

func printInitPlan(plan initPlan) {
	terminal.DryRunHeader()
	if plan.Project != nil {
		terminal.Detected(filepath.Base(plan.Project.ProjectPath), plan.Project.Target)
	}
	terminal.Info("Doctor classification: " + string(plan.Classification))
	for _, warning := range plan.Warnings {
		terminal.Info("Warning: " + warning)
	}
	fmt.Println()
	for _, action := range plan.Actions {
		label := action.Action
		if action.Detail != "" {
			terminal.DryRunItem(label, action.File+" ("+action.Detail+")")
			continue
		}
		terminal.DryRunItem(label, action.File)
	}
	fmt.Println()
	terminal.Info("Run `apus init` without --dry-run to apply these changes.")
}

func desiredDependencySource(resolvedPackagePath string) string {
	if strings.TrimSpace(resolvedPackagePath) != "" {
		return "local"
	}
	return "remote"
}
