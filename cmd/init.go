package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/ivanhoe/apus_cli/internal/builder"
	"github.com/ivanhoe/apus_cli/internal/preflight"
	"github.com/ivanhoe/apus_cli/internal/terminal"
	"github.com/ivanhoe/apus_cli/internal/xcode"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Best-effort add Apus to an existing Xcode project (backs up files first)",
	Long: `apus init is a best-effort command.
It currently works best with SwiftUI-style app entry files and standard Xcode layouts.
If preflight checks fail or injection cannot be performed safely, prefer manual integration.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func runInit(_ *cobra.Command, _ []string) error {
	if err := preflight.Validate(preflight.ScopeInit); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// ── Detect project ──
	info, err := xcode.DetectProject(cwd)
	if err != nil {
		terminal.Fatal("project detection failed", err)
		return err
	}

	terminal.Info("`apus init` is best-effort and currently optimized for SwiftUI-style entry points.")
	terminal.Info("A backup of modified files will be created before changes are applied.")
	terminal.Detected(filepath.Base(info.ProjectPath), info.Target+" (SwiftUI)")

	backup, err := createProjectBackup(cwd, backupCandidates(info))
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	if backup.fileCount > 0 {
		terminal.Info("Backup created: " + backup.dir)
	}

	p := terminal.NewProgress(4)

	// ── Step 1: Modify .pbxproj ──
	{
		done := p.Start("Adding Apus to project.pbxproj")
		err = xcode.AddApusDependency(info.ProjectPath, info.Target)
		done(err)
		if err != nil {
			terminal.Fatal("pbxproj modification failed", err)
			_ = backup.restore()
			return err
		}
	}

	// ── Step 2: Resolve package dependencies ──
	{
		done := p.Start("Resolving Swift Package dependencies")
		err = builder.ResolvePackageDependencies(info.ProjectPath)
		done(err)
		if err != nil {
			terminal.Fatal("package resolution failed", err)
			if rollbackErr := backup.restore(); rollbackErr != nil {
				return fmt.Errorf("package resolution failed: %w (rollback failed: %v)", err, rollbackErr)
			}
			return fmt.Errorf("package resolution failed; changes were rolled back: %w", err)
		}
	}

	// ── Step 3: Inject Apus.shared.start() ──
	{
		entryLabel := "app entry point"
		if info.EntryFile != "" {
			entryLabel = filepath.Base(info.EntryFile)
		}
		done := p.Start("Injecting Apus.shared.start() in " + entryLabel)
		if info.EntryFile == "" {
			integrated, checkErr := xcode.HasApusIntegration(cwd)
			if checkErr != nil {
				done(fmt.Errorf("no entry file detected"))
				terminal.Info("Add `Apus.shared.start()` manually in your App init()")
				terminal.Info("Entry-point verification failed: " + checkErr.Error())
			} else if integrated {
				done(nil)
				terminal.Info("Apus integration already present; skipping injection")
			} else {
				done(fmt.Errorf("no entry file detected"))
				terminal.Info("Add `Apus.shared.start()` manually in your App init()")
			}
		} else {
			err = xcode.InjectApus(info.EntryFile)
			done(err)
			if err != nil {
				terminal.Fatal("Swift injection failed", err)
				if rollbackErr := backup.restore(); rollbackErr != nil {
					return fmt.Errorf("swift injection failed: %w (rollback failed: %v)", err, rollbackErr)
				}
				return fmt.Errorf("swift injection failed; changes were rolled back: %w", err)
			}
		}
	}

	// ── Step 4: Write AGENTS.md ──
	{
		done := p.Start("Writing AGENTS.md")
		err = writeAgentsMD(cwd, info)
		done(err)
		if err != nil {
			terminal.Fatal("AGENTS.md write failed", err)
			// Non-fatal
		}
	}

	terminal.InitSuccess(info.ProjectName)
	return nil
}

type projectBackup struct {
	dir       string
	files     []backupFile
	fileCount int
}

type backupFile struct {
	original string
	backup   string
}

func backupCandidates(info *xcode.ProjectInfo) []string {
	files := []string{filepath.Join(info.ProjectPath, "project.pbxproj")}
	if info.EntryFile != "" {
		files = append(files, info.EntryFile)
	}
	return files
}

func createProjectBackup(cwd string, files []string) (*projectBackup, error) {
	backupDir := filepath.Join(cwd, ".apus-backups", time.Now().Format("20060102-150405"))

	entries := make([]backupFile, 0, len(files))
	for i, original := range files {
		if _, err := os.Stat(original); err != nil {
			continue
		}

		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return nil, fmt.Errorf("create backup dir: %w", err)
		}

		backupPath := filepath.Join(backupDir, fmt.Sprintf("%02d_%s", i+1, filepath.Base(original)))
		if err := copyFile(original, backupPath); err != nil {
			return nil, fmt.Errorf("backup %s: %w", original, err)
		}
		entries = append(entries, backupFile{original: original, backup: backupPath})
	}

	return &projectBackup{
		dir:       backupDir,
		files:     entries,
		fileCount: len(entries),
	}, nil
}

func (b *projectBackup) restore() error {
	var rollbackErr error
	for _, f := range b.files {
		if err := copyFile(f.backup, f.original); err != nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

const agentsMDTemplate = `# AGENTS.md — {{ .ProjectName }}

## MCP Server

Apus runs at ` + "`http://localhost:9847/mcp`" + ` when the app is running in the simulator or on a device.

## Build & Run

` + "```bash" + `
xcodebuild -scheme {{ .ProjectName }} \
  -destination "platform=iOS Simulator,name=iPhone 16e" \
  -quiet CODE_SIGN_IDENTITY="-"
` + "```" + `

## Key MCP Tools

| Tool | Description |
|------|-------------|
| ` + "`get_logs`" + ` | App console output |
| ` + "`get_screenshot`" + ` | Current screen as JPEG |
| ` + "`get_view_hierarchy`" + ` | Full SwiftUI/UIKit view tree |
| ` + "`ui_interact`" + ` | tap, double_tap, swipe, type_text |
| ` + "`get_network_history`" + ` | All HTTP/HTTPS traffic |
| ` + "`hot_reload`" + ` | Inject Swift changes without recompile |
`

func writeAgentsMD(dir string, info *xcode.ProjectInfo) error {
	outPath := filepath.Join(dir, "AGENTS.md")

	// Skip if already exists
	if _, err := os.Stat(outPath); err == nil {
		return nil
	}

	tmpl, err := template.New("agents").Parse(agentsMDTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := struct {
		ProjectName string
		BundleOrg   string
	}{
		ProjectName: info.ProjectName,
		BundleOrg:   strings.ToLower(info.ProjectName),
	}
	return tmpl.Execute(f, data)
}
