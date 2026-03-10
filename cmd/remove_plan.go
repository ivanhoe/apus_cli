package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

type removePlan struct {
	Project    *jsonProject `json:"project,omitempty"`
	Integrated bool         `json:"integrated"`
	Actions    []jsonAction `json:"actions"`
}

func buildRemovePlan(projectDir string, info *xcode.ProjectInfo, hasSwift, hasPbx, hasAgents bool) removePlan {
	plan := removePlan{
		Project:    jsonProjectFromInfo(projectDir, info),
		Integrated: hasSwift || hasPbx || hasAgents,
	}

	if info.EntryFile != "" && hasSwift {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "would_modify",
			File:   filepath.Base(info.EntryFile),
			Detail: "remove import Apus and Apus.shared.start()",
		})
	} else if info.EntryFile != "" {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "skip",
			File:   filepath.Base(info.EntryFile),
			Detail: "no Apus code found in the detected entry file",
		})
	}

	if hasPbx {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "would_modify",
			File:   "project.pbxproj",
			Detail: "remove Apus Swift Package dependency",
		})
	} else {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "skip",
			File:   "project.pbxproj",
			Detail: "no Apus package dependency found",
		})
	}

	if hasAgents {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "would_delete",
			File:   "AGENTS.md",
			Detail: "remove the Apus-managed AGENTS.md file",
		})
	} else {
		plan.Actions = append(plan.Actions, jsonAction{
			Action: "skip",
			File:   "AGENTS.md",
			Detail: "not found or not managed by Apus",
		})
	}

	return plan
}

func printRemovePlan(plan removePlan) {
	terminal.DryRunHeader()
	if plan.Project != nil {
		terminal.Detected(filepath.Base(plan.Project.ProjectPath), plan.Project.Target)
	}
	for _, action := range plan.Actions {
		if action.Detail != "" {
			terminal.DryRunItem(action.Action, fmt.Sprintf("%s (%s)", action.File, action.Detail))
			continue
		}
		terminal.DryRunItem(action.Action, action.File)
	}
	fmt.Println()
	terminal.Info("Run `apus remove` without --dry-run to apply these changes.")
}
