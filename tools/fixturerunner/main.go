package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ivanhoe/apus_cli/internal/fixturematrix"
	"github.com/ivanhoe/apus_cli/internal/xcode"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("fixturerunner", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	manifestPath := fs.String("manifest", filepath.Join("fixtures", "matrix.json"), "Path to the fixture manifest")
	fixtureID := fs.String("fixture", "", "Run only a single fixture id")
	apusBin := fs.String("apus-bin", "", "Path to the apus binary to execute")
	apusPackagePath := fs.String("apus-package-path", "", "Local path to the Apus Swift package")
	workRoot := fs.String("work-root", filepath.Join(".tmp", "fixtures"), "Workspace for copied fixture runs")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*apusBin) == "" {
		fmt.Fprintln(os.Stderr, "--apus-bin is required")
		return 2
	}
	if strings.TrimSpace(*apusPackagePath) == "" {
		fmt.Fprintln(os.Stderr, "--apus-package-path is required")
		return 2
	}

	manifest, err := fixturematrix.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	manifestDir := filepath.Dir(*manifestPath)
	if err := manifest.ValidatePaths(manifestDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fixtures := manifest.ReadyFixtures()
	if *fixtureID != "" {
		fixtures = filterFixture(fixtures, *fixtureID)
		if len(fixtures) == 0 {
			fmt.Fprintf(os.Stderr, "fixture %q not found or not ready\n", *fixtureID)
			return 1
		}
	}

	if err := os.MkdirAll(*workRoot, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	runner := fixtureRunner{
		manifestDir:     manifestDir,
		workRoot:        *workRoot,
		apusBin:         *apusBin,
		apusPackagePath: *apusPackagePath,
	}

	for i, fixture := range fixtures {
		fmt.Printf("[%d/%d] %s\n", i+1, len(fixtures), fixture.ID)
		if err := runner.runFixture(fixture); err != nil {
			fmt.Fprintf(os.Stderr, "fixture %s failed: %v\n", fixture.ID, err)
			return 1
		}
		fmt.Printf("PASS %s\n\n", fixture.ID)
	}

	fmt.Printf("Completed %d fixture(s)\n", len(fixtures))
	return 0
}

type fixtureRunner struct {
	manifestDir     string
	workRoot        string
	apusBin         string
	apusPackagePath string
}

func (r fixtureRunner) runFixture(fixture fixturematrix.Fixture) error {
	if fixture.SourceKind != fixturematrix.SourceSynthetic {
		return fmt.Errorf("external fixtures are not executable yet")
	}

	srcDir := filepath.Join(r.manifestDir, fixture.Path)
	workDir, err := os.MkdirTemp(r.workRoot, fixture.ID+"-")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}

	if err := copyDir(srcDir, workDir); err != nil {
		return fmt.Errorf("copy fixture: %w", err)
	}

	if err := prepareFixture(workDir); err != nil {
		return err
	}

	switch fixture.ExpectedOutcome {
	case fixturematrix.OutcomeSupported, fixturematrix.OutcomeSupportedWithTarget:
		return r.runSupportedFixture(workDir, fixture)
	case fixturematrix.OutcomeUnsupportedCleanly:
		return r.runUnsupportedFixture(workDir, fixture)
	default:
		return fmt.Errorf("unsupported fixture outcome %q", fixture.ExpectedOutcome)
	}
}

func (r fixtureRunner) runSupportedFixture(workDir string, fixture fixturematrix.Fixture) error {
	info, err := xcode.DetectProjectWithTarget(workDir, fixture.Target)
	if err != nil {
		return fmt.Errorf("detect baseline project: %w", err)
	}

	pbxPath := filepath.Join(info.ProjectPath, "project.pbxproj")
	pbxBefore, err := os.ReadFile(pbxPath)
	if err != nil {
		return fmt.Errorf("read baseline pbxproj: %w", err)
	}

	entryBefore := []byte(nil)
	if info.EntryFile != "" {
		entryBefore, err = os.ReadFile(info.EntryFile)
		if err != nil {
			return fmt.Errorf("read baseline entry file: %w", err)
		}
	}

	statusArgs := []string{"status"}
	if fixture.Target != "" {
		statusArgs = append(statusArgs, "--target", fixture.Target)
	}
	statusResult, err := runCommand(workDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("baseline status failed: %w\n%s", err, statusResult.combined())
	}
	if !strings.Contains(statusResult.stdout, "not integrated") {
		return fmt.Errorf("expected baseline status to report not integrated, got:\n%s", statusResult.combined())
	}

	if fixture.TargetRequired {
		if _, err := runCommand(workDir, r.apusBin, "init", "--package-path", r.apusPackagePath); err == nil {
			return fmt.Errorf("expected init without --target to fail for %s", fixture.ID)
		}
	}

	initArgs := []string{"init", "--package-path", r.apusPackagePath}
	if fixture.Target != "" {
		initArgs = append(initArgs, "--target", fixture.Target)
	}
	initResult, err := runCommand(workDir, r.apusBin, initArgs...)
	if err != nil {
		return fmt.Errorf("init failed: %w\n%s", err, initResult.combined())
	}
	if !strings.Contains(initResult.stdout, "Done.") {
		return fmt.Errorf("expected init success output, got:\n%s", initResult.combined())
	}

	state, err := xcode.DetectApusDependency(info.ProjectPath)
	if err != nil {
		return fmt.Errorf("detect dependency after init: %w", err)
	}
	if !state.Any() {
		return fmt.Errorf("expected Apus dependency after init")
	}

	integrated, err := xcode.HasApusIntegration(workDir)
	if err != nil {
		return fmt.Errorf("detect Swift integration after init: %w", err)
	}
	if !integrated {
		return fmt.Errorf("expected Swift integration after init")
	}

	if _, err := os.Stat(filepath.Join(workDir, "AGENTS.md")); err != nil {
		return fmt.Errorf("expected AGENTS.md after init: %w", err)
	}

	statusAfterInit, err := runCommand(workDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("status after init failed: %w\n%s", err, statusAfterInit.combined())
	}
	if !strings.Contains(statusAfterInit.stdout, "Apus integration:") {
		return fmt.Errorf("expected integrated status output, got:\n%s", statusAfterInit.combined())
	}

	removeArgs := []string{"remove"}
	if fixture.Target != "" {
		removeArgs = append(removeArgs, "--target", fixture.Target)
	}
	removeResult, err := runCommand(workDir, r.apusBin, removeArgs...)
	if err != nil {
		return fmt.Errorf("remove failed: %w\n%s", err, removeResult.combined())
	}
	if !strings.Contains(removeResult.stdout, "Done.") {
		return fmt.Errorf("expected remove success output, got:\n%s", removeResult.combined())
	}

	state, err = xcode.DetectApusDependency(info.ProjectPath)
	if err != nil {
		return fmt.Errorf("detect dependency after remove: %w", err)
	}
	if state.Any() {
		return fmt.Errorf("expected Apus dependency to be removed")
	}

	integrated, err = xcode.HasApusIntegration(workDir)
	if err != nil {
		return fmt.Errorf("detect Swift integration after remove: %w", err)
	}
	if integrated {
		return fmt.Errorf("expected Swift integration to be removed")
	}

	if _, err := os.Stat(filepath.Join(workDir, "AGENTS.md")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("expected AGENTS.md to be removed")
	}

	pbxAfter, err := os.ReadFile(pbxPath)
	if err != nil {
		return fmt.Errorf("read pbxproj after remove: %w", err)
	}
	if !bytes.Equal(normalizePBXProjForComparison(pbxBefore), normalizePBXProjForComparison(pbxAfter)) {
		return fmt.Errorf("pbxproj did not roundtrip cleanly")
	}

	if info.EntryFile != "" {
		entryAfter, err := os.ReadFile(info.EntryFile)
		if err != nil {
			return fmt.Errorf("read entry file after remove: %w", err)
		}
		if !bytes.Equal(entryBefore, entryAfter) {
			return fmt.Errorf("entry file did not roundtrip cleanly")
		}
	}

	finalStatus, err := runCommand(workDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("final status failed: %w\n%s", err, finalStatus.combined())
	}
	if !strings.Contains(finalStatus.stdout, "not integrated") {
		return fmt.Errorf("expected final status to report not integrated, got:\n%s", finalStatus.combined())
	}

	return nil
}

func (r fixtureRunner) runUnsupportedFixture(workDir string, fixture fixturematrix.Fixture) error {
	before, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash baseline files: %w", err)
	}

	args := []string{"init", "--package-path", r.apusPackagePath}
	if fixture.Target != "" {
		args = append(args, "--target", fixture.Target)
	}
	result, err := runCommand(workDir, r.apusBin, args...)
	if err == nil {
		return fmt.Errorf("expected init to fail for unsupported fixture, got:\n%s", result.combined())
	}

	after, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash files after failed init: %w", err)
	}
	if before != after {
		return fmt.Errorf("unsupported fixture modified files unexpectedly")
	}

	return nil
}

func prepareFixture(workDir string) error {
	if _, err := os.Stat(filepath.Join(workDir, "project.yml")); err == nil {
		result, err := runCommand(workDir, "xcodegen", "generate")
		if err != nil {
			return fmt.Errorf("xcodegen generate failed: %w\n%s", err, result.combined())
		}
	}
	return nil
}

func filterFixture(fixtures []fixturematrix.Fixture, id string) []fixturematrix.Fixture {
	for _, fixture := range fixtures {
		if fixture.ID == id {
			return []fixturematrix.Fixture{fixture}
		}
	}
	return nil
}

type commandResult struct {
	stdout string
	stderr string
}

func (r commandResult) combined() string {
	if strings.TrimSpace(r.stderr) == "" {
		return r.stdout
	}
	if strings.TrimSpace(r.stdout) == "" {
		return r.stderr
	}
	return r.stdout + "\n" + r.stderr
}

func runCommand(dir string, name string, args ...string) (commandResult, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}, err
}

func copyDir(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		info, err := d.Info()
		if err != nil {
			return err
		}

		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}
		return nil
	})
}

func hashRegularFiles(root string) (string, error) {
	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	}); err != nil {
		return "", err
	}

	sort.Strings(files)
	hash := sha256.New()
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		_, _ = hash.Write([]byte(rel))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(data)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func normalizePBXProjForComparison(data []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.TrimRight(line, " \t")
	}

	normalized := strings.Join(lines, "\n")
	normalized = regexp.MustCompile(`(/\* End [^\n]+ section \*/)\n(?:[ \t]*\n)+([ \t]*\};)`).ReplaceAllString(normalized, "$1\n$2")
	normalized = regexp.MustCompile(`\n{3,}`).ReplaceAllString(normalized, "\n\n")
	return []byte(normalized)
}
