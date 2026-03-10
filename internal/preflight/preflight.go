package preflight

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ivanhoe/apus_cli/internal/xcode"
)

type Scope string

const (
	ScopeInit   Scope = "init"
	ScopeNew    Scope = "new"
	ScopeDoctor Scope = "doctor"
)

type Classification string

const (
	ClassificationSupported   Classification = "supported"
	ClassificationRisky       Classification = "risky"
	ClassificationUnsupported Classification = "unsupported"
)

type CheckStatus string

const (
	CheckStatusPass CheckStatus = "pass"
	CheckStatusWarn CheckStatus = "warn"
	CheckStatusFail CheckStatus = "fail"
)

type Options struct {
	Scope      Scope
	ProjectDir string
	Target     string
}

type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
	Hint   string      `json:"hint,omitempty"`
}

type ProjectAssessment struct {
	Directory   string `json:"directory"`
	ProjectPath string `json:"project_path,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Target      string `json:"target,omitempty"`
	EntryFile   string `json:"entry_file,omitempty"`
	IsSwiftUI   bool   `json:"is_swiftui,omitempty"`

	Info *xcode.ProjectInfo `json:"-"`
}

type Report struct {
	Scope          Scope              `json:"scope"`
	Classification Classification     `json:"classification"`
	Checks         []Check            `json:"checks"`
	Project        *ProjectAssessment `json:"project,omitempty"`
}

func (r Report) HasFailures() bool {
	for _, c := range r.Checks {
		if c.Status == CheckStatusFail {
			return true
		}
	}
	return false
}

func (r Report) HasWarnings() bool {
	for _, c := range r.Checks {
		if c.Status == CheckStatusWarn {
			return true
		}
	}
	return false
}

func (r Report) Failures() []Check {
	failed := make([]Check, 0, len(r.Checks))
	for _, c := range r.Checks {
		if c.Status == CheckStatusFail {
			failed = append(failed, c)
		}
	}
	return failed
}

func (r Report) Warnings() []Check {
	warnings := make([]Check, 0, len(r.Checks))
	for _, c := range r.Checks {
		if c.Status == CheckStatusWarn {
			warnings = append(warnings, c)
		}
	}
	return warnings
}

func (r Report) Validate() error {
	if !r.HasFailures() {
		return nil
	}

	lines := []string{"preflight checks failed:"}
	for _, c := range r.Failures() {
		line := fmt.Sprintf("- %s", c.Name)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		if c.Hint != "" {
			line += " (" + c.Hint + ")"
		}
		lines = append(lines, line)
	}
	return fmt.Errorf(strings.Join(lines, "\n"))
}

var lookPathFn = exec.LookPath
var runCombinedOutputFn = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
var detectProjectFn = xcode.DetectProjectWithTarget

func Run(scope Scope) Report {
	return RunWithOptions(Options{Scope: scope})
}

func RunWithOptions(opts Options) Report {
	scope := opts.Scope
	if scope == "" {
		scope = ScopeDoctor
	}

	checks := []Check{
		checkBinary("xcodebuild", "Install Xcode and command line tools."),
		checkBinary("plutil", "Install Xcode and command line tools."),
		checkXcodeSelect(),
	}

	if scope == ScopeNew || scope == ScopeDoctor {
		xcrunCheck := checkBinary("xcrun", "Install Xcode command line tools.")
		checks = append(checks, xcrunCheck)

		xcodegenStatus := CheckStatusFail
		if scope == ScopeDoctor {
			xcodegenStatus = CheckStatusWarn
		}
		checks = append(checks,
			checkBinaryWithStatus("xcodegen", "Install xcodegen manually: brew install xcodegen.", xcodegenStatus),
		)
		if xcrunCheck.Status == CheckStatusPass {
			checks = append(checks, checkAvailableIPhoneSimulator())
		}
	}

	var project *ProjectAssessment
	if strings.TrimSpace(opts.ProjectDir) != "" {
		projectChecks, assessment := checkProject(opts.ProjectDir, opts.Target)
		checks = append(checks, projectChecks...)
		project = assessment
	}

	report := Report{
		Scope:   scope,
		Checks:  checks,
		Project: project,
	}
	report.Classification = classifyReport(checks)
	return report
}

func Validate(scope Scope) error {
	return Run(scope).Validate()
}

func classifyReport(checks []Check) Classification {
	hasWarnings := false
	for _, c := range checks {
		switch c.Status {
		case CheckStatusFail:
			return ClassificationUnsupported
		case CheckStatusWarn:
			hasWarnings = true
		}
	}
	if hasWarnings {
		return ClassificationRisky
	}
	return ClassificationSupported
}

func checkBinary(name, hint string) Check {
	return checkBinaryWithStatus(name, hint, CheckStatusFail)
}

func checkBinaryWithStatus(name, hint string, failureStatus CheckStatus) Check {
	_, err := lookPathFn(name)
	if err != nil {
		return Check{
			Name:   "binary:" + name,
			Status: failureStatus,
			Detail: name + " is not available in PATH",
			Hint:   hint,
		}
	}
	return Check{Name: "binary:" + name, Status: CheckStatusPass}
}

func checkXcodeSelect() Check {
	out, err := runCombinedOutputFn("xcode-select", "-p")
	if err != nil || out == "" {
		return Check{
			Name:   "xcode-select",
			Status: CheckStatusFail,
			Detail: "active developer directory is not configured",
			Hint:   "Run: sudo xcode-select -s /Applications/Xcode.app/Contents/Developer",
		}
	}
	return Check{
		Name:   "xcode-select",
		Status: CheckStatusPass,
		Detail: out,
	}
}

func checkAvailableIPhoneSimulator() Check {
	out, err := runCombinedOutputFn("xcrun", "simctl", "list", "devices", "available", "--json")
	if err != nil {
		return Check{
			Name:   "simulator:iphone",
			Status: CheckStatusFail,
			Detail: "failed to list available simulator devices",
			Hint:   "Open Xcode > Settings > Platforms and install an iOS Simulator runtime.",
		}
	}

	var result struct {
		Devices map[string][]struct {
			Name        string `json:"name"`
			IsAvailable bool   `json:"isAvailable"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return Check{
			Name:   "simulator:iphone",
			Status: CheckStatusFail,
			Detail: "could not parse xcrun simctl list output",
			Hint:   "Reinstall Xcode command line tools if this persists.",
		}
	}

	for _, devices := range result.Devices {
		for _, device := range devices {
			if device.IsAvailable && strings.Contains(device.Name, "iPhone") {
				return Check{
					Name:   "simulator:iphone",
					Status: CheckStatusPass,
					Detail: "at least one iPhone simulator runtime is installed",
				}
			}
		}
	}

	return Check{
		Name:   "simulator:iphone",
		Status: CheckStatusFail,
		Detail: "no available iPhone simulator was found",
		Hint:   "Install one from Xcode > Settings > Platforms.",
	}
}

func checkProject(dir, target string) ([]Check, *ProjectAssessment) {
	info, err := detectProjectFn(dir, target)
	if err != nil {
		return []Check{{
			Name:   "project:detect",
			Status: CheckStatusFail,
			Detail: err.Error(),
			Hint:   projectHint(err),
		}}, &ProjectAssessment{Directory: dir}
	}

	assessment := &ProjectAssessment{
		Directory:   dir,
		ProjectPath: info.ProjectPath,
		ProjectName: info.ProjectName,
		Target:      info.Target,
		EntryFile:   info.EntryFile,
		IsSwiftUI:   info.IsSwiftUI,
		Info:        info,
	}

	checks := []Check{
		{
			Name:   "project:xcodeproj",
			Status: CheckStatusPass,
			Detail: filepath.Base(info.ProjectPath),
		},
		{
			Name:   "project:target",
			Status: CheckStatusPass,
			Detail: info.Target,
		},
	}

	if info.EntryFile == "" {
		checks = append(checks, Check{
			Name:   "project:entrypoint",
			Status: CheckStatusWarn,
			Detail: "no Swift entry point was detected for the selected target",
			Hint:   "Apus can add the package dependency, but you may need to add Apus.shared.start() manually.",
		})
	} else {
		entryDetail := filepath.Base(info.EntryFile)
		if info.IsSwiftUI {
			entryDetail += " (SwiftUI)"
		} else {
			entryDetail += " (non-SwiftUI)"
		}
		checks = append(checks, Check{
			Name:   "project:entrypoint",
			Status: CheckStatusPass,
			Detail: entryDetail,
		})
	}

	return checks, assessment
}

func projectHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "multiple app targets found"):
		return "Rerun with --target <name> to choose the app target explicitly."
	case strings.Contains(msg, "not found — app targets"):
		return "Use --target with one of the listed app targets."
	case strings.Contains(msg, "no .xcodeproj found"):
		return "Point --path at the project root that contains the .xcodeproj."
	default:
		return ""
	}
}
