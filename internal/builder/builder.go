// Package builder wraps xcodegen and xcodebuild operations.
package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var lookPathFn = exec.LookPath

// EnsureXcodeGen checks that xcodegen is installed.
func EnsureXcodeGen() error {
	if _, err := lookPathFn("xcodegen"); err == nil {
		return nil
	}
	return fmt.Errorf("xcodegen not found. Install it manually: brew install xcodegen")
}

// Generate runs `xcodegen generate` in the given directory.
func Generate(projectDir string) error {
	cmd := exec.Command("xcodegen", "generate")
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodegen generate: %w\n%s", err, string(out))
	}
	return nil
}

// BuildResult contains information about a successful build.
type BuildResult struct {
	AppPath  string
	BundleID string
}

// Build compiles the project for a simulator destination.
// projectDir is the directory containing the .xcodeproj.
func Build(projectDir, scheme, destinationUDID string) (*BuildResult, error) {
	derivedData := filepath.Join(projectDir, ".build", "DerivedData")

	args := []string{
		"build",
		"-scheme", scheme,
		"-destination", fmt.Sprintf("id=%s", destinationUDID),
		"-derivedDataPath", derivedData,
		"-quiet",
		"CODE_SIGN_IDENTITY=-",
		"CODE_SIGNING_REQUIRED=NO",
		"CODE_SIGNING_ALLOWED=NO",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = projectDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("xcodebuild build: %w\n%s", err, stderr.String())
	}

	appPath, err := findAppBundle(derivedData, scheme)
	if err != nil {
		return nil, err
	}

	bundleID, err := readBundleID(appPath)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		AppPath:  appPath,
		BundleID: bundleID,
	}, nil
}

// ResolvePackageDependencies runs xcodebuild -resolvePackageDependencies on a project.
// It does not modify Package.resolved or any lockfile.
func ResolvePackageDependencies(projectPath string) error {
	projectDir := filepath.Dir(projectPath)
	projectName := filepath.Base(projectPath)
	cmd := exec.Command("xcodebuild",
		"-resolvePackageDependencies",
		"-project", projectName,
	)
	cmd.Dir = projectDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resolve package dependencies: %w\n%s", err, string(out))
	}
	return nil
}

func findAppBundle(derivedData, scheme string) (string, error) {
	// Look in Build/Products/Debug-iphonesimulator/
	pattern := filepath.Join(derivedData, "Build", "Products", "*-iphonesimulator", scheme+".app")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob app bundle: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("app bundle not found at %s", pattern)
	}
	return matches[0], nil
}

func readBundleID(appPath string) (string, error) {
	infoPlist := filepath.Join(appPath, "Info.plist")
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", infoPlist).Output()
	if err != nil {
		return "", fmt.Errorf("read Info.plist: %w", err)
	}

	var plist map[string]interface{}
	if err := json.Unmarshal(out, &plist); err != nil {
		return "", fmt.Errorf("parse Info.plist: %w", err)
	}

	id, ok := plist["CFBundleIdentifier"].(string)
	if !ok {
		return "", fmt.Errorf("CFBundleIdentifier not found in Info.plist")
	}

	// Strip Xcode variable expansions
	id = strings.TrimPrefix(id, "$(")
	return id, nil
}
