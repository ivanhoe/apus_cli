package fixturematrix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matrix.json")
	content := `{
  "version": 1,
  "fixtures": [
    {
      "id": "swiftui-single-target",
      "display_name": "SwiftUI single target",
      "stage": "planned",
      "source_kind": "synthetic",
      "expected_outcome": "supported"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if manifest.Version != 1 {
		t.Fatalf("expected version 1, got %d", manifest.Version)
	}
	if len(manifest.Fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(manifest.Fixtures))
	}
}

func TestValidateReadyExternalFixtureRequiresRepoAndRef(t *testing.T) {
	manifest := &Manifest{
		Version: 1,
		Fixtures: []Fixture{
			{
				ID:              "external-open-source",
				DisplayName:     "External fixture",
				Stage:           StageReady,
				SourceKind:      SourceExternal,
				ExpectedOutcome: OutcomeSupported,
			},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatalf("expected Validate() to fail")
	}
	if !strings.Contains(err.Error(), "repo is required") || !strings.Contains(err.Error(), "ref is required") {
		t.Fatalf("expected missing repo/ref errors, got: %v", err)
	}
}

func TestValidateReadySyntheticFixtureRequiresRelativePath(t *testing.T) {
	manifest := &Manifest{
		Version: 1,
		Fixtures: []Fixture{
			{
				ID:              "synthetic-local",
				DisplayName:     "Synthetic fixture",
				Stage:           StageReady,
				SourceKind:      SourceSynthetic,
				ExpectedOutcome: OutcomeSupported,
				Path:            "/tmp/fixture",
			},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatalf("expected Validate() to fail")
	}
	if !strings.Contains(err.Error(), "must be relative") {
		t.Fatalf("expected relative path validation, got: %v", err)
	}
}

func TestPlannedFixturesCanOmitLocation(t *testing.T) {
	manifest := &Manifest{
		Version: 1,
		Fixtures: []Fixture{
			{
				ID:              "planned-widget",
				DisplayName:     "Planned widget fixture",
				Stage:           StagePlanned,
				SourceKind:      SourceExternal,
				ExpectedOutcome: OutcomeSupportedWithTarget,
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected planned fixture to validate without location info, got: %v", err)
	}
}

func TestReadyFixtureWithRequiredTargetNeedsTargetName(t *testing.T) {
	manifest := &Manifest{
		Version: 1,
		Fixtures: []Fixture{
			{
				ID:              "multi-target",
				DisplayName:     "Multi-target",
				Stage:           StageReady,
				SourceKind:      SourceSynthetic,
				ExpectedOutcome: OutcomeSupportedWithTarget,
				TargetRequired:  true,
				Path:            "fixtures/synthetic/multi-target",
			},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatalf("expected Validate() to fail")
	}
	if !strings.Contains(err.Error(), "target is required") {
		t.Fatalf("expected target validation error, got: %v", err)
	}
}

func TestValidatePathsChecksReadySyntheticFixtures(t *testing.T) {
	dir := t.TempDir()
	manifest := &Manifest{
		Version: 1,
		Fixtures: []Fixture{
			{
				ID:              "swiftui-single-target",
				DisplayName:     "SwiftUI single target",
				Stage:           StageReady,
				SourceKind:      SourceSynthetic,
				ExpectedOutcome: OutcomeSupported,
				Path:            "fixtures/synthetic/swiftui-single-target",
			},
		},
	}

	if err := manifest.ValidatePaths(dir); err == nil {
		t.Fatalf("expected ValidatePaths() to fail when the fixture directory is missing")
	}

	fixtureDir := filepath.Join(dir, "fixtures", "synthetic", "swiftui-single-target")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}

	if err := manifest.ValidatePaths(dir); err != nil {
		t.Fatalf("expected ValidatePaths() to succeed, got: %v", err)
	}
}
