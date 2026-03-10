package fixturematrix

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Stage string

const (
	StagePlanned Stage = "planned"
	StageReady   Stage = "ready"
)

type SourceKind string

const (
	SourceSynthetic SourceKind = "synthetic"
	SourceExternal  SourceKind = "external"
)

type ExpectedOutcome string

const (
	OutcomeSupported           ExpectedOutcome = "supported"
	OutcomeSupportedWithTarget ExpectedOutcome = "supported-with-target"
	OutcomeUnsupportedCleanly  ExpectedOutcome = "unsupported-cleanly"
)

type Manifest struct {
	Version  int       `json:"version"`
	Fixtures []Fixture `json:"fixtures"`
}

type Fixture struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"display_name"`
	Stage           Stage           `json:"stage"`
	SourceKind      SourceKind      `json:"source_kind"`
	ExpectedOutcome ExpectedOutcome `json:"expected_outcome"`
	TargetRequired  bool            `json:"target_required,omitempty"`
	Target          string          `json:"target,omitempty"`
	Path            string          `json:"path,omitempty"`
	Repo            string          `json:"repo,omitempty"`
	Ref             string          `json:"ref,omitempty"`
	Subdir          string          `json:"subdir,omitempty"`
	Notes           string          `json:"notes,omitempty"`
}

func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (m *Manifest) Validate() error {
	var problems []string
	if m.Version <= 0 {
		problems = append(problems, "version must be greater than zero")
	}
	if len(m.Fixtures) == 0 {
		problems = append(problems, "fixtures must not be empty")
	}

	seen := make(map[string]struct{}, len(m.Fixtures))
	for i, fixture := range m.Fixtures {
		prefix := fmt.Sprintf("fixtures[%d]", i)
		if strings.TrimSpace(fixture.ID) == "" {
			problems = append(problems, prefix+": id is required")
		} else {
			if _, ok := seen[fixture.ID]; ok {
				problems = append(problems, prefix+": duplicate id "+fixture.ID)
			}
			seen[fixture.ID] = struct{}{}
		}
		if strings.TrimSpace(fixture.DisplayName) == "" {
			problems = append(problems, prefix+": display_name is required")
		}
		if err := validateStage(fixture.Stage); err != nil {
			problems = append(problems, prefix+": "+err.Error())
		}
		if err := validateSourceKind(fixture.SourceKind); err != nil {
			problems = append(problems, prefix+": "+err.Error())
		}
		if err := validateOutcome(fixture.ExpectedOutcome); err != nil {
			problems = append(problems, prefix+": "+err.Error())
		}
		if fixture.TargetRequired && strings.TrimSpace(fixture.Target) == "" && fixture.Stage == StageReady {
			problems = append(problems, prefix+": target is required when target_required is true for ready fixtures")
		}
		problems = append(problems, validateFixtureLocation(prefix, fixture)...)
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid fixture manifest:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func (m *Manifest) CountsByStage() map[Stage]int {
	counts := make(map[Stage]int, 2)
	for _, fixture := range m.Fixtures {
		counts[fixture.Stage]++
	}
	return counts
}

func (m *Manifest) PlannedFixtures() []Fixture {
	var fixtures []Fixture
	for _, fixture := range m.Fixtures {
		if fixture.Stage == StagePlanned {
			fixtures = append(fixtures, fixture)
		}
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures
}

func (m *Manifest) ReadyFixtures() []Fixture {
	var fixtures []Fixture
	for _, fixture := range m.Fixtures {
		if fixture.Stage == StageReady {
			fixtures = append(fixtures, fixture)
		}
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures
}

func (m *Manifest) ValidatePaths(baseDir string) error {
	var problems []string
	for _, fixture := range m.ReadyFixtures() {
		if fixture.SourceKind != SourceSynthetic {
			continue
		}

		path := filepath.Join(baseDir, fixture.Path)
		info, err := os.Stat(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: synthetic fixture path not found: %s", fixture.ID, fixture.Path))
			continue
		}
		if !info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: synthetic fixture path is not a directory: %s", fixture.ID, fixture.Path))
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid fixture paths:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func validateStage(stage Stage) error {
	switch stage {
	case StagePlanned, StageReady:
		return nil
	default:
		return fmt.Errorf("stage must be one of %q or %q", StagePlanned, StageReady)
	}
}

func validateSourceKind(kind SourceKind) error {
	switch kind {
	case SourceSynthetic, SourceExternal:
		return nil
	default:
		return fmt.Errorf("source_kind must be one of %q or %q", SourceSynthetic, SourceExternal)
	}
}

func validateOutcome(outcome ExpectedOutcome) error {
	switch outcome {
	case OutcomeSupported, OutcomeSupportedWithTarget, OutcomeUnsupportedCleanly:
		return nil
	default:
		return fmt.Errorf("expected_outcome is invalid")
	}
}

func validateFixtureLocation(prefix string, fixture Fixture) []string {
	var problems []string
	if fixture.Stage == StagePlanned {
		return problems
	}

	switch fixture.SourceKind {
	case SourceSynthetic:
		if strings.TrimSpace(fixture.Path) == "" {
			problems = append(problems, prefix+": path is required for ready synthetic fixtures")
		} else if err := validateRelativePath(fixture.Path); err != nil {
			problems = append(problems, prefix+": path "+err.Error())
		}
	case SourceExternal:
		if strings.TrimSpace(fixture.Repo) == "" {
			problems = append(problems, prefix+": repo is required for ready external fixtures")
		}
		if strings.TrimSpace(fixture.Ref) == "" {
			problems = append(problems, prefix+": ref is required for ready external fixtures")
		}
		if strings.TrimSpace(fixture.Subdir) != "" {
			if err := validateRelativePath(fixture.Subdir); err != nil {
				problems = append(problems, prefix+": subdir "+err.Error())
			}
		}
	}

	return problems
}

func validateRelativePath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("must be relative")
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.HasPrefix(clean, "..") {
		return fmt.Errorf("must not escape the repo root")
	}
	return nil
}
