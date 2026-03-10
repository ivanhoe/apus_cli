package preflight

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Scope string

const (
	ScopeInit   Scope = "init"
	ScopeNew    Scope = "new"
	ScopeDoctor Scope = "doctor"
)

type Check struct {
	Name string
	OK   bool
	Hint string
}

type Report struct {
	Checks []Check
}

func (r Report) HasFailures() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return true
		}
	}
	return false
}

func (r Report) Failures() []Check {
	failed := make([]Check, 0, len(r.Checks))
	for _, c := range r.Checks {
		if !c.OK {
			failed = append(failed, c)
		}
	}
	return failed
}

var lookPathFn = exec.LookPath
var runCombinedOutputFn = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func Run(scope Scope) Report {
	checks := []Check{
		checkBinary("xcodebuild", "Install Xcode and command line tools."),
		checkBinary("plutil", "Install Xcode command line tools."),
		checkXcodeSelect(),
	}

	if scope == ScopeNew || scope == ScopeDoctor {
		xcrunCheck := checkBinary("xcrun", "Install Xcode command line tools.")
		checks = append(checks,
			xcrunCheck,
			checkBinary("xcodegen", "Install xcodegen manually: brew install xcodegen."),
		)
		if xcrunCheck.OK {
			checks = append(checks, checkAvailableIPhoneSimulator())
		}
	}

	return Report{Checks: checks}
}

func Validate(scope Scope) error {
	report := Run(scope)
	if !report.HasFailures() {
		return nil
	}

	lines := []string{"preflight checks failed:"}
	for _, c := range report.Failures() {
		line := fmt.Sprintf("- %s", c.Name)
		if c.Hint != "" {
			line += " — " + c.Hint
		}
		lines = append(lines, line)
	}
	return fmt.Errorf(strings.Join(lines, "\n"))
}

func checkBinary(name, hint string) Check {
	_, err := lookPathFn(name)
	return Check{Name: "binary:" + name, OK: err == nil, Hint: hint}
}

func checkXcodeSelect() Check {
	out, err := runCombinedOutputFn("xcode-select", "-p")
	if err != nil || out == "" {
		return Check{
			Name: "xcode-select",
			OK:   false,
			Hint: "Run: sudo xcode-select -s /Applications/Xcode.app/Contents/Developer",
		}
	}
	return Check{Name: "xcode-select", OK: true}
}

func checkAvailableIPhoneSimulator() Check {
	out, err := runCombinedOutputFn("xcrun", "simctl", "list", "devices", "available", "--json")
	if err != nil {
		return Check{
			Name: "simulator:iphone",
			OK:   false,
			Hint: "Open Xcode > Settings > Platforms and install an iOS Simulator runtime.",
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
			Name: "simulator:iphone",
			OK:   false,
			Hint: "Could not parse `xcrun simctl list` output. Reinstall Xcode command line tools if this persists.",
		}
	}

	for _, devices := range result.Devices {
		for _, device := range devices {
			if device.IsAvailable && strings.Contains(device.Name, "iPhone") {
				return Check{Name: "simulator:iphone", OK: true}
			}
		}
	}

	return Check{
		Name: "simulator:iphone",
		OK:   false,
		Hint: "No iPhone simulator is available. Install one from Xcode > Settings > Platforms.",
	}
}
