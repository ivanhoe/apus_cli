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
	strictRoundtrip := fs.Bool("strict-roundtrip", true, "Require project files to roundtrip byte-cleanly after init/remove")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(*apusBin) == "" {
		fmt.Fprintln(os.Stderr, "--apus-bin is required")
		return 2
	}

	apusBinPath, err := filepath.Abs(*apusBin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve --apus-bin: %v\n", err)
		return 1
	}

	apusPackageAbsPath := ""
	if strings.TrimSpace(*apusPackagePath) != "" {
		apusPackageAbsPath, err = filepath.Abs(*apusPackagePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve --apus-package-path: %v\n", err)
			return 1
		}
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
		apusBin:         apusBinPath,
		apusPackagePath: apusPackageAbsPath,
		strictRoundtrip: *strictRoundtrip,
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
	strictRoundtrip bool
}

func (r fixtureRunner) runFixture(fixture fixturematrix.Fixture) error {
	workDir, err := os.MkdirTemp(r.workRoot, fixture.ID+"-")
	if err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	workDir, err = filepath.Abs(workDir)
	if err != nil {
		return fmt.Errorf("resolve work dir: %w", err)
	}

	projectDir := workDir
	switch fixture.SourceKind {
	case fixturematrix.SourceSynthetic:
		srcDir := filepath.Join(r.manifestDir, fixture.Path)
		if err := copyDir(srcDir, workDir); err != nil {
			return fmt.Errorf("copy fixture: %w", err)
		}
	case fixturematrix.SourceExternal:
		if err := checkoutExternalFixture(workDir, fixture); err != nil {
			return err
		}
		if fixture.Subdir != "" {
			projectDir = filepath.Join(workDir, fixture.Subdir)
		}
	default:
		return fmt.Errorf("unsupported source kind %q", fixture.SourceKind)
	}

	if err := prepareFixture(projectDir); err != nil {
		return err
	}

	switch fixture.ExpectedOutcome {
	case fixturematrix.OutcomeSupported, fixturematrix.OutcomeSupportedWithTarget:
		return r.runSupportedFixture(projectDir, fixture)
	case fixturematrix.OutcomeUnsupportedCleanly:
		return r.runUnsupportedFixture(projectDir, fixture)
	default:
		return fmt.Errorf("unsupported fixture outcome %q", fixture.ExpectedOutcome)
	}
}

func (r fixtureRunner) runSupportedFixture(workDir string, fixture fixturematrix.Fixture) error {
	info, err := xcode.DetectProjectWithTarget(workDir, fixture.Target)
	if err != nil {
		return fmt.Errorf("detect baseline project: %w", err)
	}
	invokerDir, err := os.MkdirTemp(r.workRoot, fixture.ID+"-invoker-")
	if err != nil {
		return fmt.Errorf("create invoker dir: %w", err)
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

	doctorArgs := []string{"doctor", "--json", "--path", workDir}
	if fixture.Target != "" {
		doctorArgs = append(doctorArgs, "--target", fixture.Target)
	}
	doctorResult, err := runCommand(invokerDir, r.apusBin, doctorArgs...)
	if err != nil {
		return fmt.Errorf("doctor failed: %w\n%s", err, doctorResult.combined())
	}
	if !strings.Contains(doctorResult.stdout, `"classification": "supported"`) &&
		!strings.Contains(doctorResult.stdout, `"classification": "risky"`) {
		return fmt.Errorf("expected doctor supported/risky JSON output, got:\n%s", doctorResult.combined())
	}

	statusArgs := []string{"status", "--json", "--path", workDir}
	if fixture.Target != "" {
		statusArgs = append(statusArgs, "--target", fixture.Target)
	}
	statusResult, err := runCommand(invokerDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("baseline status failed: %w\n%s", err, statusResult.combined())
	}
	if !strings.Contains(statusResult.stdout, `"integrated": false`) {
		return fmt.Errorf("expected baseline status JSON to report not integrated, got:\n%s", statusResult.combined())
	}

	beforeDryRun, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash files before init dry-run: %w", err)
	}

	dryRunArgs := appendPackagePath([]string{"init", "--dry-run", "--json", "--path", workDir}, r.apusPackagePath)
	if fixture.Target != "" {
		dryRunArgs = append(dryRunArgs, "--target", fixture.Target)
	}
	dryRunResult, err := runCommand(invokerDir, r.apusBin, dryRunArgs...)
	if err != nil {
		return fmt.Errorf("init dry-run failed: %w\n%s", err, dryRunResult.combined())
	}
	if !strings.Contains(dryRunResult.stdout, `"mode": "dry-run"`) {
		return fmt.Errorf("expected init dry-run JSON output, got:\n%s", dryRunResult.combined())
	}

	afterDryRun, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash files after init dry-run: %w", err)
	}
	if beforeDryRun != afterDryRun {
		return fmt.Errorf("init --dry-run modified files unexpectedly")
	}

	if fixture.TargetRequired {
		args := appendPackagePath([]string{"init", "--path", workDir}, r.apusPackagePath)
		result, err := runCommand(invokerDir, r.apusBin, args...)
		if err == nil {
			return fmt.Errorf("expected init without --target to fail for %s", fixture.ID)
		}
		if result.exitCode != 5 {
			return fmt.Errorf("expected init without --target to exit with code 5, got %d\n%s", result.exitCode, result.combined())
		}
	}

	initArgs := appendPackagePath([]string{"init", "--path", workDir}, r.apusPackagePath)
	if fixture.Target != "" {
		initArgs = append(initArgs, "--target", fixture.Target)
	}
	initResult, err := runCommand(invokerDir, r.apusBin, initArgs...)
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

	statusAfterInit, err := runCommand(invokerDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("status after init failed: %w\n%s", err, statusAfterInit.combined())
	}
	if !strings.Contains(statusAfterInit.stdout, `"integrated": true`) {
		return fmt.Errorf("expected integrated status JSON output, got:\n%s", statusAfterInit.combined())
	}

	beforeRemoveDryRun, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash files before remove dry-run: %w", err)
	}

	removeDryRunArgs := []string{"remove", "--dry-run", "--json", "--path", workDir}
	if fixture.Target != "" {
		removeDryRunArgs = append(removeDryRunArgs, "--target", fixture.Target)
	}
	removeDryRunResult, err := runCommand(invokerDir, r.apusBin, removeDryRunArgs...)
	if err != nil {
		return fmt.Errorf("remove dry-run failed: %w\n%s", err, removeDryRunResult.combined())
	}
	if !strings.Contains(removeDryRunResult.stdout, `"mode": "dry-run"`) {
		return fmt.Errorf("expected remove dry-run JSON output, got:\n%s", removeDryRunResult.combined())
	}

	afterRemoveDryRun, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash files after remove dry-run: %w", err)
	}
	if beforeRemoveDryRun != afterRemoveDryRun {
		return fmt.Errorf("remove --dry-run modified files unexpectedly")
	}

	removeArgs := []string{"remove", "--json", "--path", workDir}
	if fixture.Target != "" {
		removeArgs = append(removeArgs, "--target", fixture.Target)
	}
	removeResult, err := runCommand(invokerDir, r.apusBin, removeArgs...)
	if err != nil {
		return fmt.Errorf("remove failed: %w\n%s", err, removeResult.combined())
	}
	if !strings.Contains(removeResult.stdout, `"applied": true`) {
		return fmt.Errorf("expected remove JSON output, got:\n%s", removeResult.combined())
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
		if r.strictRoundtrip {
			return fmt.Errorf("pbxproj did not roundtrip cleanly")
		}
		fmt.Fprintln(os.Stderr, "warning: pbxproj did not roundtrip cleanly")
	}

	if info.EntryFile != "" {
		entryAfter, err := os.ReadFile(info.EntryFile)
		if err != nil {
			return fmt.Errorf("read entry file after remove: %w", err)
		}
		if !bytes.Equal(entryBefore, entryAfter) {
			if r.strictRoundtrip {
				return fmt.Errorf("entry file did not roundtrip cleanly")
			}
			fmt.Fprintln(os.Stderr, "warning: entry file did not roundtrip cleanly")
		}
	}

	finalStatus, err := runCommand(invokerDir, r.apusBin, statusArgs...)
	if err != nil {
		return fmt.Errorf("final status failed: %w\n%s", err, finalStatus.combined())
	}
	if !strings.Contains(finalStatus.stdout, `"integrated": false`) {
		return fmt.Errorf("expected final status JSON to report not integrated, got:\n%s", finalStatus.combined())
	}

	return nil
}

func (r fixtureRunner) runUnsupportedFixture(workDir string, fixture fixturematrix.Fixture) error {
	invokerDir, err := os.MkdirTemp(r.workRoot, fixture.ID+"-invoker-")
	if err != nil {
		return fmt.Errorf("create invoker dir: %w", err)
	}

	doctorArgs := []string{"doctor", "--json", "--path", workDir}
	if fixture.Target != "" {
		doctorArgs = append(doctorArgs, "--target", fixture.Target)
	}
	doctorResult, err := runCommand(invokerDir, r.apusBin, doctorArgs...)
	if err == nil {
		return fmt.Errorf("expected doctor to fail for unsupported fixture, got:\n%s", doctorResult.combined())
	}
	if !strings.Contains(doctorResult.stdout, `"classification": "unsupported"`) {
		return fmt.Errorf("expected doctor unsupported JSON output, got:\n%s", doctorResult.combined())
	}

	before, err := hashRegularFiles(workDir)
	if err != nil {
		return fmt.Errorf("hash baseline files: %w", err)
	}

	args := appendPackagePath([]string{"init", "--path", workDir}, r.apusPackagePath)
	if fixture.Target != "" {
		args = append(args, "--target", fixture.Target)
	}
	result, err := runCommand(invokerDir, r.apusBin, args...)
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
	stdout   string
	stderr   string
	exitCode int
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
	result := commandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
	}
	return result, err
}

func appendPackagePath(args []string, packagePath string) []string {
	if strings.TrimSpace(packagePath) == "" {
		return args
	}
	return append(args, "--package-path", packagePath)
}

func checkoutExternalFixture(workDir string, fixture fixturematrix.Fixture) error {
	cloneResult, err := runCommand("", "git", "clone", fixture.Repo, workDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, cloneResult.combined())
	}

	checkoutResult, err := runCommand(workDir, "git", "checkout", fixture.Ref)
	if err != nil {
		return fmt.Errorf("git checkout %s failed: %w\n%s", fixture.Ref, err, checkoutResult.combined())
	}

	return nil
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
