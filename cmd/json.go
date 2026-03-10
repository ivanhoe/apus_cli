package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

type jsonProject struct {
	Directory   string `json:"directory,omitempty"`
	ProjectPath string `json:"project_path,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Target      string `json:"target,omitempty"`
	EntryFile   string `json:"entry_file,omitempty"`
	IsSwiftUI   bool   `json:"is_swiftui,omitempty"`
}

type jsonAction struct {
	Action string `json:"action"`
	File   string `json:"file"`
	Detail string `json:"detail,omitempty"`
}

func writeJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func writeJSONResult(v any, err error) error {
	if jsonErr := writeJSON(v); jsonErr != nil {
		return mutationError(jsonErr)
	}
	if err != nil {
		return markPrinted(err)
	}
	return nil
}

func jsonProjectFromInfo(dir string, info *xcode.ProjectInfo) *jsonProject {
	if info == nil {
		return &jsonProject{Directory: dir}
	}

	entryFile := info.EntryFile
	if entryFile != "" {
		entryFile = filepath.Base(entryFile)
	}

	return &jsonProject{
		Directory:   dir,
		ProjectPath: info.ProjectPath,
		ProjectName: info.ProjectName,
		Target:      info.Target,
		EntryFile:   entryFile,
		IsSwiftUI:   info.IsSwiftUI,
	}
}

func jsonActionsFromChanges(changes []terminal.FileChange) []jsonAction {
	actions := make([]jsonAction, 0, len(changes))
	for _, change := range changes {
		actions = append(actions, jsonAction{
			Action: change.Action,
			File:   change.File,
			Detail: change.Detail,
		})
	}
	return actions
}
