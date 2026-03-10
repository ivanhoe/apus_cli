// Package scaffold generates a new iOS project pre-wired with Apus.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// Data holds values substituted into every template.
type Data struct {
	AppName           string
	AppNameLower      string
	BundleOrg         string
	MCPPort           int
	SimulatorUDID     string
	Template          string
	ApusPackageSource string
	ApusPackageURL    string
	ApusPackageBranch string
	ApusPackagePath   string
}

const (
	defaultApusPackageURL    = "https://github.com/ivanhoe/apus"
	defaultApusPackageBranch = "main"
	apusPackagePathEnv       = "APUS_PACKAGE_PATH"
	apusPackageSourceRemote  = "remote"
	apusPackageSourceLocal   = "local"
	defaultMCPPort           = 9847
)

// NewData returns a Data struct derived from appName.
func NewData(appName, simulatorUDID, template string, mcpPort int) Data {
	if template == "" {
		template = "swiftui"
	}
	if mcpPort == 0 {
		mcpPort = defaultMCPPort
	}

	return Data{
		AppName:           appName,
		AppNameLower:      strings.ToLower(appName),
		BundleOrg:         "dev",
		MCPPort:           mcpPort,
		SimulatorUDID:     simulatorUDID,
		Template:          template,
		ApusPackageSource: apusPackageSourceRemote,
		ApusPackageURL:    defaultApusPackageURL,
		ApusPackageBranch: defaultApusPackageBranch,
	}
}

// Generate creates the project directory tree from embedded templates.
// outputDir is the parent directory; the project is placed in outputDir/AppName.
func Generate(data Data, outputDir string) error {
	if err := ValidateAppName(data.AppName); err != nil {
		return err
	}

	if data.Template != "swiftui" {
		return fmt.Errorf("template %q is not supported yet — currently only \"swiftui\" is available", data.Template)
	}

	data, err := resolveApusPackage(data, outputDir)
	if err != nil {
		return err
	}

	root := filepath.Join(outputDir, data.AppName)
	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("project directory %q already exists", root)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing project dir: %w", err)
	}

	tmpRoot, err := os.MkdirTemp(outputDir, ".apus-scaffold-*")
	if err != nil {
		return fmt.Errorf("create temporary scaffold dir: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	workRoot := filepath.Join(tmpRoot, data.AppName)
	if err := os.MkdirAll(filepath.Join(workRoot, "Sources"), 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	type fileSpec struct {
		tmplName string
		outPath  string
	}

	files := []fileSpec{
		{"project.yml.tmpl", filepath.Join(workRoot, "project.yml")},
		{"App.swift.tmpl", filepath.Join(workRoot, "Sources", data.AppName+"App.swift")},
		{"ContentView.swift.tmpl", filepath.Join(workRoot, "Sources", "ContentView.swift")},
		{"AGENTS.md.tmpl", filepath.Join(workRoot, "AGENTS.md")},
	}

	for _, f := range files {
		if err := renderTemplate(f.tmplName, f.outPath, data); err != nil {
			return err
		}
	}

	if err := os.Rename(workRoot, root); err != nil {
		return fmt.Errorf("move scaffold into destination: %w", err)
	}
	return nil
}

// ProjectDir returns the absolute path of the generated project.
func ProjectDir(data Data, outputDir string) string {
	return filepath.Join(outputDir, data.AppName)
}

func renderTemplate(name, destPath string, data Data) error {
	content, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		return fmt.Errorf("read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", name, err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("execute template %s: %w", name, err)
	}
	return nil
}

var appNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

var swiftReservedKeywords = map[string]struct{}{
	"actor": {}, "any": {}, "as": {}, "associatedtype": {}, "async": {}, "await": {},
	"break": {}, "case": {}, "catch": {}, "class": {}, "continue": {},
	"default": {}, "defer": {}, "deinit": {}, "do": {}, "else": {},
	"enum": {}, "extension": {}, "fallthrough": {}, "false": {}, "fileprivate": {},
	"for": {}, "func": {}, "guard": {}, "if": {}, "import": {}, "in": {},
	"infix": {}, "init": {}, "inout": {}, "internal": {}, "is": {},
	"isolated": {}, "let": {}, "nil": {}, "nonisolated": {}, "open": {},
	"operator": {}, "postfix": {}, "precedencegroup": {}, "prefix": {}, "private": {},
	"protocol": {}, "public": {}, "repeat": {}, "rethrows": {}, "return": {},
	"self": {}, "some": {}, "static": {}, "struct": {}, "subscript": {},
	"super": {}, "switch": {}, "throw": {}, "throws": {}, "true": {},
	"try": {}, "typealias": {}, "var": {}, "where": {}, "while": {},
}

// ValidateAppName validates that appName is safe for filesystem + Swift usage.
func ValidateAppName(appName string) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid app name %q — use letters, digits, underscores; start with a letter", appName)
	}

	if _, reserved := swiftReservedKeywords[strings.ToLower(appName)]; reserved {
		return fmt.Errorf("invalid app name %q — reserved Swift keyword", appName)
	}

	return nil
}

func resolveApusPackage(data Data, outputDir string) (Data, error) {
	data.ApusPackageSource = apusPackageSourceRemote
	data.ApusPackageURL = defaultApusPackageURL
	data.ApusPackageBranch = defaultApusPackageBranch
	data.ApusPackagePath = ""

	if configuredPath := strings.TrimSpace(os.Getenv(apusPackagePathEnv)); configuredPath != "" {
		resolvedPath, err := resolveExplicitApusPackagePath(configuredPath, outputDir)
		if err != nil {
			return data, fmt.Errorf("resolve %s: %w", apusPackagePathEnv, err)
		}
		if err := validateApusPackagePath(resolvedPath); err != nil {
			return data, fmt.Errorf("invalid %s=%q: %w", apusPackagePathEnv, configuredPath, err)
		}

		data.ApusPackageSource = apusPackageSourceLocal
		data.ApusPackagePath = resolvedPath
		return data, nil
	}

	if localPath, ok := findNearestApusPackagePath(outputDir); ok {
		data.ApusPackageSource = apusPackageSourceLocal
		data.ApusPackagePath = localPath
	}

	return data, nil
}

func resolveExplicitApusPackagePath(rawPath, outputDir string) (string, error) {
	candidate := strings.TrimSpace(rawPath)
	if candidate == "" {
		return "", fmt.Errorf("path is empty")
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(outputDir, candidate)
	}

	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("normalize path: %w", err)
	}
	return absolute, nil
}

func findNearestApusPackagePath(outputDir string) (string, bool) {
	current, err := filepath.Abs(outputDir)
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(current, "apus")
		if err := validateApusPackagePath(candidate); err == nil {
			return candidate, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", false
}

func validateApusPackagePath(path string) error {
	manifest := filepath.Join(path, "Package.swift")
	info, err := os.Stat(manifest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Package.swift not found at %s", manifest)
		}
		return fmt.Errorf("cannot access %s: %w", manifest, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected file but found directory at %s", manifest)
	}
	return nil
}
