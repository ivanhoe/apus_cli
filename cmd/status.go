package cmd

import (
	"path/filepath"

	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
	"github.com/spf13/cobra"
)

var statusPath string
var statusTarget string
var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether Apus is integrated in the current project",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusPath, "path", "", "Project directory to inspect instead of the current working directory")
	statusCmd.Flags().StringVar(&statusTarget, "target", "", "App target to inspect when the project contains multiple app targets")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Print the status report as JSON")
}

func runStatus(_ *cobra.Command, _ []string) error {
	projectDir, err := resolveCommandPath(statusPath)
	if err != nil {
		err = usageError(err)
		if statusJSON {
			return writeJSONResult(struct {
				Error string `json:"error"`
			}{Error: err.Error()}, err)
		}
		return err
	}

	info, err := xcode.DetectProjectWithTarget(projectDir, statusTarget)
	if err != nil {
		err = classifyProjectError(err)
		if statusJSON {
			return writeJSONResult(struct {
				Project *jsonProject `json:"project"`
				Error   string       `json:"error"`
			}{
				Project: &jsonProject{Directory: projectDir},
				Error:   err.Error(),
			}, err)
		}
		terminal.Fatal("project detection failed", err)
		return markPrinted(err)
	}

	dependencyState, depErr := xcode.DetectApusDependency(info.ProjectPath)
	if depErr != nil {
		err = projectError(depErr)
		if statusJSON {
			return writeJSONResult(struct {
				Project *jsonProject `json:"project"`
				Error   string       `json:"error"`
			}{
				Project: jsonProjectFromInfo(projectDir, info),
				Error:   err.Error(),
			}, err)
		}
		return err
	}
	pbxHasApus := dependencyState.Any()

	hasSwift, _ := xcode.HasApusIntegration(projectDir)

	hasAgents, _ := readManagedAgentsFile(projectDir)
	integrated := pbxHasApus || hasSwift || hasAgents

	if statusJSON {
		return writeJSONResult(struct {
			Project          *jsonProject `json:"project"`
			Integrated       bool         `json:"integrated"`
			DependencySource string       `json:"dependency_source"`
			Components       struct {
				Dependency bool `json:"dependency"`
				Swift      bool `json:"swift"`
				Agents     bool `json:"agents"`
			} `json:"components"`
		}{
			Project:          jsonProjectFromInfo(projectDir, info),
			Integrated:       integrated,
			DependencySource: dependencySourceLabel(dependencyState),
			Components: struct {
				Dependency bool `json:"dependency"`
				Swift      bool `json:"swift"`
				Agents     bool `json:"agents"`
			}{
				Dependency: pbxHasApus,
				Swift:      hasSwift,
				Agents:     hasAgents,
			},
		}, nil)
	}

	terminal.Detected(filepath.Base(info.ProjectPath), info.Target)

	if !integrated {
		terminal.StatusNotIntegrated()
		return nil
	}

	terminal.StatusIntegrated(pbxHasApus, hasSwift, hasAgents)
	return nil
}

func dependencySourceLabel(state xcode.DependencyState) string {
	switch {
	case state.Remote && state.Local:
		return "remote+local"
	case state.Remote:
		return "remote"
	case state.Local:
		return "local"
	default:
		return "none"
	}
}
