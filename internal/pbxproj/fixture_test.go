package pbxproj

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_RealXcodeFixture(t *testing.T) {
	// Walk up to project root to find the fixture
	fixture := filepath.Join("..", "..", ".tmp", "compare", "swiftui-baseline-3VDjso",
		"FixtureSwiftUISingleTarget.xcodeproj", "project.pbxproj")

	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("fixture not found: %v", err)
	}

	root, err := Parse(string(data))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	objects := root.GetDict("objects")
	if objects == nil {
		t.Fatal("expected objects dict")
	}

	// Verify key object types are found
	targets := FindObjectsByISA(objects, "PBXNativeTarget")
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Dict.GetString("name") != "FixtureSwiftUISingleTarget" {
		t.Fatalf("unexpected target name: %s", targets[0].Dict.GetString("name"))
	}

	bfs := FindObjectsByISA(objects, "PBXBuildFile")
	if len(bfs) != 2 {
		t.Fatalf("expected 2 PBXBuildFile, got %d", len(bfs))
	}

	groups := FindObjectsByISA(objects, "PBXGroup")
	if len(groups) != 3 {
		t.Fatalf("expected 3 PBXGroup, got %d", len(groups))
	}

	configs := FindObjectsByISA(objects, "XCBuildConfiguration")
	if len(configs) != 4 {
		t.Fatalf("expected 4 XCBuildConfiguration, got %d", len(configs))
	}

	project := FindProjectObject(root)
	if project == nil {
		t.Fatal("expected PBXProject")
	}

	// Roundtrip: serialize → re-parse
	out := Serialize(root)
	root2, err := Parse(out)
	if err != nil {
		t.Fatalf("Roundtrip re-parse error: %v\nOutput (first 500 chars):\n%s", err, out[:min(500, len(out))])
	}

	objects2 := root2.GetDict("objects")
	targets2 := FindObjectsByISA(objects2, "PBXNativeTarget")
	if len(targets2) != 1 || targets2[0].Dict.GetString("name") != "FixtureSwiftUISingleTarget" {
		t.Fatal("target mismatch after roundtrip")
	}

	configs2 := FindObjectsByISA(objects2, "XCBuildConfiguration")
	if len(configs2) != 4 {
		t.Fatalf("config count mismatch after roundtrip: got %d", len(configs2))
	}
}
