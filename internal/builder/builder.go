// Package builder wraps xcodegen and xcodebuild operations.
package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureXcodeGen checks that xcodegen is installed, auto-installs via Homebrew if missing.
func EnsureXcodeGen() error {
	if _, err := exec.LookPath("xcodegen"); err == nil {
		return nil
	}
	// Try installing via brew
	cmd := exec.Command("brew", "install", "xcodegen")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xcodegen not found and brew install failed: %w\nInstall manually: brew install xcodegen", err)
	}
	return nil
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
func ResolvePackageDependencies(projectPath string) error {
	projectDir := filepath.Dir(projectPath)
	projectName := filepath.Base(projectPath)
	sourcePackagesDir := filepath.Join(projectDir, ".build", "SourcePackages")

	out, err := runResolvePackageDependencies(projectDir, projectName, sourcePackagesDir)
	if err == nil {
		return nil
	}

	firstErr := fmt.Errorf("resolve package dependencies: %w\n%s", err, out)
	if !looksLikeApusResolutionError(out) {
		return firstErr
	}

	if cleanupErr := resetApusResolutionState(projectDir, sourcePackagesDir); cleanupErr != nil {
		return fmt.Errorf("%w\nretry preparation failed: %v", firstErr, cleanupErr)
	}

	retryOut, retryErr := runResolvePackageDependencies(projectDir, projectName, sourcePackagesDir)
	if retryErr == nil {
		return nil
	}

	return fmt.Errorf("resolve package dependencies after cleanup: %w\n%s", retryErr, retryOut)
}

func runResolvePackageDependencies(projectDir, projectName, sourcePackagesDir string) (string, error) {
	if err := os.MkdirAll(sourcePackagesDir, 0o755); err != nil {
		return "", fmt.Errorf("create source packages dir: %w", err)
	}

	cmd := exec.Command("xcodebuild",
		"-resolvePackageDependencies",
		"-project", projectName,
		"-clonedSourcePackagesDirPath", sourcePackagesDir,
	)
	cmd.Dir = projectDir

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func resetApusResolutionState(projectDir, sourcePackagesDir string) error {
	var errs []string

	if err := os.RemoveAll(sourcePackagesDir); err != nil {
		errs = append(errs, fmt.Sprintf("remove source packages dir: %v", err))
	}

	resolvedFiles, err := findPackageResolvedFiles(projectDir)
	if err != nil {
		errs = append(errs, fmt.Sprintf("scan Package.resolved files: %v", err))
	} else {
		for _, path := range resolvedFiles {
			changed, stripErr := stripApusPinsFromResolvedFile(path)
			if stripErr != nil {
				errs = append(errs, fmt.Sprintf("update %s: %v", path, stripErr))
				continue
			}
			_ = changed
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func looksLikeApusResolutionError(output string) bool {
	lower := strings.ToLower(output)
	hasApusRef := strings.Contains(lower, "'apus'") ||
		strings.Contains(lower, "github.com/ivanhoe/apus") ||
		strings.Contains(lower, "from https://github.com/ivanhoe/apus")
	if !hasApusRef {
		return false
	}

	return strings.Contains(lower, "unresolved") ||
		strings.Contains(lower, "could not resolve package dependencies")
}

func findPackageResolvedFiles(projectDir string) ([]string, error) {
	seen := map[string]struct{}{}
	files := []string{}

	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".build" || name == "DerivedData" {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() != "Package.resolved" {
			return nil
		}
		if !strings.Contains(path, "swiftpm") {
			return nil
		}
		if _, ok := seen[path]; ok {
			return nil
		}
		seen[path] = struct{}{}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func stripApusPinsFromResolvedFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read file: %w", err)
	}

	updated, changed, err := stripApusPins(raw)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return false, fmt.Errorf("write file: %w", err)
	}
	return true, nil
}

func stripApusPins(raw []byte) ([]byte, bool, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, false, fmt.Errorf("parse Package.resolved json: %w", err)
	}

	changed := false

	if obj, ok := root["object"].(map[string]interface{}); ok {
		pins, pinChanged := filterApusPins(obj["pins"])
		if pinChanged {
			obj["pins"] = pins
			root["object"] = obj
			changed = true
		}
	} else {
		pins, pinChanged := filterApusPins(root["pins"])
		if pinChanged {
			root["pins"] = pins
			changed = true
		}
	}

	if !changed {
		return raw, false, nil
	}

	pretty, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("encode Package.resolved json: %w", err)
	}
	pretty = append(pretty, '\n')
	return pretty, true, nil
}

func filterApusPins(value interface{}) ([]interface{}, bool) {
	pins, ok := value.([]interface{})
	if !ok {
		return nil, false
	}

	filtered := make([]interface{}, 0, len(pins))
	changed := false

	for _, p := range pins {
		pin, ok := p.(map[string]interface{})
		if !ok {
			filtered = append(filtered, p)
			continue
		}
		if isApusPin(pin) {
			changed = true
			continue
		}
		filtered = append(filtered, p)
	}

	return filtered, changed
}

func isApusPin(pin map[string]interface{}) bool {
	candidates := []string{
		stringField(pin, "identity"),
		stringField(pin, "package"),
		stringField(pin, "repositoryURL"),
		stringField(pin, "location"),
	}

	for _, candidate := range candidates {
		value := strings.ToLower(strings.TrimSpace(candidate))
		if value == "" {
			continue
		}
		if value == "apus" ||
			strings.Contains(value, "github.com/ivanhoe/apus") ||
			strings.HasSuffix(value, "/apus") {
			return true
		}
	}
	return false
}

func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
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
