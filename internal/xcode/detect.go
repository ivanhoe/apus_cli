// Package xcode provides helpers to detect and modify Xcode projects.
package xcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
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
// then resolves targets and locates the app entry point for the selected target.
func DetectProject(dir string) (*ProjectInfo, error) {
	return DetectProjectWithTarget(dir, "")
}

// DetectProjectWithTarget walks the current directory (depth 1) to find an Xcode
// project, then resolves targets and locates the app entry point for the selected target.
func DetectProjectWithTarget(dir, preferredTarget string) (*ProjectInfo, error) {
	projPath, err := findXcodeProj(dir)
	if err != nil {
		return nil, err
	}

	projectName := strings.TrimSuffix(filepath.Base(projPath), ".xcodeproj")

	target, err := pickTarget(projPath, preferredTarget)
	if err != nil {
		return nil, err
	}

	info := &ProjectInfo{
		ProjectPath: projPath,
		ProjectName: projectName,
		Target:      target,
	}

	// Find the entry-point file for the selected target
	entryFile, isSwiftUI, err := findEntryPoint(dir, target)
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
		Name           string   `json:"name"`
		Targets        []string `json:"targets"`
		Configurations []string `json:"configurations"`
		Schemes        []string `json:"schemes"`
	} `json:"project"`
}

const xcodebuildListTimeout = 10 * time.Second

var runXcodebuildListFn = runXcodebuildList

// pickTarget returns the primary app target (excludes *Tests, *UITests, *Extension*).
func pickTarget(projPath string, preferredTarget string) (string, error) {
	out, stderr, err := runXcodebuildListFn(projPath)
	if err == nil {
		var list xcodebuildList
		if parseErr := json.Unmarshal(out, &list); parseErr == nil {
			return chooseAppTarget(list.Project.Targets, projPath, preferredTarget)
		}
	}

	// Fallback: parse native targets directly from project.pbxproj when xcodebuild
	// is unavailable or fails due local Xcode environment issues.
	pbxTargets, pbxErr := listTargetsFromPBXProj(projPath)
	if pbxErr == nil {
		return chooseAppTarget(pbxTargets, projPath, preferredTarget)
	}

	xcodeErr := formatXcodebuildListError(err, stderr)
	if xcodeErr == "" {
		xcodeErr = "xcodebuild -list failed for unknown reason"
	}
	return "", fmt.Errorf("%s\npbxproj fallback failed: %v", xcodeErr, pbxErr)
}

func runXcodebuildList(projPath string) ([]byte, string, error) {
	projectDir := filepath.Dir(projPath)
	projectFile := filepath.Base(projPath)

	ctx, cancel := context.WithTimeout(context.Background(), xcodebuildListTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "xcodebuild", "-list", "-project", projectFile, "-json")
	cmd.Dir = projectDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return out, stderr.String(), fmt.Errorf("timed out after %s", xcodebuildListTimeout)
	}
	return out, stderr.String(), err
}

func chooseAppTarget(targets []string, projPath string, preferredTarget string) (string, error) {
	appTargets := filterAppTargets(targets)
	if len(appTargets) == 0 {
		return "", fmt.Errorf("no app target found in project — targets: %v", targets)
	}

	if preferredTarget != "" {
		for _, t := range appTargets {
			if t == preferredTarget {
				return t, nil
			}
		}
		return "", fmt.Errorf("target %q not found — app targets: %s", preferredTarget, strings.Join(appTargets, ", "))
	}

	if len(appTargets) == 1 {
		return appTargets[0], nil
	}

	// Prefer the one matching the project name
	projectName := strings.TrimSuffix(filepath.Base(projPath), ".xcodeproj")
	for _, t := range appTargets {
		if t == projectName {
			return t, nil
		}
	}

	return "", fmt.Errorf("multiple app targets found in %s: %s — rerun with --target <name>", filepath.Base(projPath), strings.Join(appTargets, ", "))
}

func filterAppTargets(targets []string) []string {
	var appTargets []string
	for _, t := range targets {
		lower := strings.ToLower(t)
		if strings.HasSuffix(lower, "tests") ||
			strings.HasSuffix(lower, "uitests") ||
			strings.Contains(lower, "extension") {
			continue
		}
		appTargets = append(appTargets, t)
	}
	return appTargets
}

func formatXcodebuildListError(execErr error, stderr string) string {
	if execErr == nil {
		return ""
	}
	details := strings.TrimSpace(stderr)
	if details == "" {
		return fmt.Sprintf("xcodebuild -list: %v", execErr)
	}
	return fmt.Sprintf("xcodebuild -list: %v\n%s", execErr, details)
}

func listTargetsFromPBXProj(projPath string) ([]string, error) {
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return nil, fmt.Errorf("read pbxproj: %w", err)
	}

	// Example:
	// AAAAAA /* MyApp */ = {
	//     isa = PBXNativeTarget;
	re := regexp.MustCompile(`(?s)[0-9A-F]{24} /\* ([^*]+) \*/ = \{\s*isa = PBXNativeTarget;`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no PBXNativeTarget entries found in %s", pbxPath)
	}

	seen := make(map[string]struct{}, len(matches))
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		targets = append(targets, name)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no target names parsed from PBXNativeTarget entries in %s", pbxPath)
	}
	return targets, nil
}

type entryPointCandidate struct {
	path      string
	isSwiftUI bool
	score     int
}

// findEntryPoint walks swift files looking for @main-like entry points and picks
// the candidate that best matches the selected target.
func findEntryPoint(dir, target string) (path string, isSwiftUI bool, err error) {
	candidates := make([]entryPointCandidate, 0, 4)

	err = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable dirs
		}

		base := filepath.Base(p)
		if info.IsDir() {
			if base == ".build" || base == "DerivedData" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(p, ".swift") {
			return nil
		}

		content, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		s := string(content)
		if !hasMainAttribute(s) {
			return nil
		}

		swiftUI := strings.Contains(s, ": App") || strings.Contains(s, ":App")
		candidates = append(candidates, entryPointCandidate{
			path:      p,
			isSwiftUI: swiftUI,
			score:     entryPointScore(p, s, target),
		})
		return nil
	})
	if err != nil {
		return "", false, err
	}

	if len(candidates) == 0 {
		return "", false, fmt.Errorf("no @main Swift file found in %s", dir)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			if candidates[i].isSwiftUI != candidates[j].isSwiftUI {
				return candidates[i].isSwiftUI
			}
			return len(candidates[i].path) < len(candidates[j].path)
		}
		return candidates[i].score > candidates[j].score
	})

	best := candidates[0]
	return best.path, best.isSwiftUI, nil
}

func entryPointScore(path, src, target string) int {
	score := 0
	pathLower := strings.ToLower(filepath.ToSlash(path))
	targetLower := strings.ToLower(target)
	baseLower := strings.ToLower(filepath.Base(path))

	if targetLower != "" {
		if strings.Contains(pathLower, targetLower) {
			score += 80
		}
		if baseLower == targetLower+".swift" || baseLower == targetLower+"app.swift" {
			score += 120
		}
		if strings.Contains(src, "struct "+target) || strings.Contains(src, "class "+target) {
			score += 60
		}
	}

	if strings.Contains(src, ": App") || strings.Contains(src, ":App") {
		score += 20
	}
	if strings.Contains(pathLower, "/app/main/") || strings.Contains(pathLower, "/app/") {
		score += 15
	}
	if strings.Contains(pathLower, "widget") && !strings.Contains(targetLower, "widget") {
		score -= 70
	}
	if strings.Contains(pathLower, "extension") && !strings.Contains(targetLower, "extension") {
		score -= 40
	}

	return score
}

func hasMainAttribute(src string) bool {
	return strings.Contains(src, "@main") || strings.Contains(src, "@UIApplicationMain")
}
