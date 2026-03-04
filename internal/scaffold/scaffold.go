// Package scaffold generates a new iOS project pre-wired with Apus.
package scaffold

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// Data holds values substituted into every template.
type Data struct {
	AppName       string
	AppNameLower  string
	BundleOrg     string
	SimulatorUDID string
}

// NewData returns a Data struct derived from appName.
func NewData(appName, simulatorUDID string) Data {
	return Data{
		AppName:       appName,
		AppNameLower:  strings.ToLower(appName),
		BundleOrg:     "dev",
		SimulatorUDID: simulatorUDID,
	}
}

// Generate creates the project directory tree from embedded templates.
// outputDir is the parent directory; the project is placed in outputDir/AppName.
func Generate(data Data, outputDir string) error {
	root := filepath.Join(outputDir, data.AppName)
	if err := os.MkdirAll(filepath.Join(root, "Sources"), 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	type fileSpec struct {
		tmplName string
		outPath  string
	}

	files := []fileSpec{
		{"project.yml.tmpl", filepath.Join(root, "project.yml")},
		{"App.swift.tmpl", filepath.Join(root, "Sources", data.AppName+"App.swift")},
		{"ContentView.swift.tmpl", filepath.Join(root, "Sources", "ContentView.swift")},
		{"AGENTS.md.tmpl", filepath.Join(root, "AGENTS.md")},
	}

	for _, f := range files {
		if err := renderTemplate(f.tmplName, f.outPath, data); err != nil {
			return err
		}
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
