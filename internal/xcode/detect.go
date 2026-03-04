// Package xcode provides helpers to detect and modify Xcode projects.
package xcode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectInfo contains the detected Xcode project details.
type ProjectInfo struct {
	ProjectPath string // absolute path to .xcodeproj
	ProjectName string // e.g. "MyApp"
	Target      string // primary app target name
	EntryFile   string // absolute path to the @main Swift file
	IsSwiftUI   bool   // true if SwiftUI App protocol detected
}

// DetectProject walks the current directory (depth 1) to find an Xcode project,
// then queries xcodebuild for targets and locates the SwiftUI entry point.
func DetectProject(dir string) (*ProjectInfo, error) {
	projPath, err := findXcodeProj(dir)
	if err != nil {
		return nil, err
	}

	projectName := strings.TrimSuffix(filepath.Base(projPath), ".xcodeproj")

	target, err := pickTarget(projPath)
	if err != nil {
		return nil, err
	}

	info := &ProjectInfo{
		ProjectPath: projPath,
		ProjectName: projectName,
		Target:      target,
	}

	// Find Swift entry point (file with @main)
	entryFile, isSwiftUI, err := findEntryPoint(dir)
	if err != nil {
		// Non-fatal: inject may still work heuristically
		_ = err
	} else {
		info.EntryFile = entryFile
		info.IsSwiftUI = isSwiftUI
	}

	return info, nil
}

// findXcodeProj looks for a *.xcodeproj at depth 1 (current dir + immediate children).
func findXcodeProj(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read dir %s: %w", dir, err)
	}

	// Check current dir first
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	// Check one level down
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		subEntries, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if se.IsDir() && strings.HasSuffix(se.Name(), ".xcodeproj") {
				return filepath.Join(sub, se.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("no .xcodeproj found in %s — run `apus init` from your project root", dir)
}

type xcodebuildList struct {
	Project struct {
		Name          string   `json:"name"`
		Targets       []string `json:"targets"`
		Configurations []string `json:"configurations"`
		Schemes        []string `json:"schemes"`
	} `json:"project"`
}

// pickTarget returns the primary app target (excludes *Tests, *UITests, *Extension*).
func pickTarget(projPath string) (string, error) {
	projectDir := filepath.Dir(projPath)
	projectFile := filepath.Base(projPath)

	cmd := exec.Command("xcodebuild", "-list", "-project", projectFile, "-json")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("xcodebuild -list: %w", err)
	}

	var list xcodebuildList
	if err := json.Unmarshal(out, &list); err != nil {
		return "", fmt.Errorf("parse xcodebuild -list output: %w", err)
	}

	var appTargets []string
	for _, t := range list.Project.Targets {
		lower := strings.ToLower(t)
		if strings.HasSuffix(lower, "tests") ||
			strings.HasSuffix(lower, "uitests") ||
			strings.Contains(lower, "extension") {
			continue
		}
		appTargets = append(appTargets, t)
	}

	if len(appTargets) == 0 {
		return "", fmt.Errorf("no app target found in project — targets: %v", list.Project.Targets)
	}
	if len(appTargets) > 1 {
		// Prefer the one matching the project name
		projectName := strings.TrimSuffix(filepath.Base(projPath), ".xcodeproj")
		for _, t := range appTargets {
			if t == projectName {
				return t, nil
			}
		}
	}
	return appTargets[0], nil
}

// findEntryPoint walks swift files looking for @main + App protocol.
func findEntryPoint(dir string) (path string, isSwiftUI bool, err error) {
	err = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable dirs
		}
		// Skip build artifacts and hidden dirs
		base := filepath.Base(p)
		if base == ".build" || base == "DerivedData" || strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}
		if info.IsDir() || !strings.HasSuffix(p, ".swift") {
			return nil
		}

		content, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		s := string(content)
		if strings.Contains(s, "@main") {
			if strings.Contains(s, ": App") || strings.Contains(s, ":App") {
				isSwiftUI = true
			}
			path = p
			return filepath.SkipAll
		}
		return nil
	})
	if path == "" && err == nil {
		err = fmt.Errorf("no @main Swift file found in %s", dir)
	}
	return
}
