package pbxproj

import (
	"strings"
	"testing"
)

func TestSerialize_MinimalProject(t *testing.T) {
	root := &Dict{}
	root.SetString("archiveVersion", "1", false)
	root.Set("classes", &Dict{})
	root.SetString("objectVersion", "54", false)
	root.Set("objects", &Dict{})
	root.SetString("rootObject", "AAAA", false)

	out := Serialize(root)

	if !strings.Contains(out, "// !$*UTF8*$!") {
		t.Fatal("expected UTF8 header")
	}
	if !strings.Contains(out, "archiveVersion = 1;") {
		t.Fatal("expected archiveVersion")
	}
	if !strings.Contains(out, "rootObject = AAAA;") {
		t.Fatal("expected rootObject")
	}
}

func TestSerialize_WithSections(t *testing.T) {
	objects := &Dict{}
	objects.Entries = append(objects.Entries, DictEntry{
		Key:        "AABBCCDDAABBCCDDAABBCCDD",
		KeyComment: "Apus in Frameworks",
		Value: BuildObject("PBXBuildFile",
			"productRef", "1122334411223344AABBCCDD",
		),
	})
	objects.Entries = append(objects.Entries, DictEntry{
		Key:        "1122334411223344AABBCCDD",
		KeyComment: "Apus",
		Value: BuildObject("XCSwiftPackageProductDependency",
			"productName", "Apus",
			"package", "EEFFEEFFEEFFEEFFEEFFEE11",
		),
	})

	root := &Dict{}
	root.SetString("archiveVersion", "1", false)
	root.Set("objects", objects)
	root.SetString("rootObject", "AAAA", false)

	out := Serialize(root)

	if !strings.Contains(out, "/* Begin PBXBuildFile section */") {
		t.Fatal("expected PBXBuildFile section header")
	}
	if !strings.Contains(out, "/* End PBXBuildFile section */") {
		t.Fatal("expected PBXBuildFile section end")
	}
	if !strings.Contains(out, "/* Begin XCSwiftPackageProductDependency section */") {
		t.Fatal("expected XCSwiftPackageProductDependency section")
	}

	// PBXBuildFile should come before XCSwiftPackageProductDependency
	bfIdx := strings.Index(out, "PBXBuildFile section")
	depIdx := strings.Index(out, "XCSwiftPackageProductDependency section")
	if bfIdx >= depIdx {
		t.Fatal("PBXBuildFile section should come before XCSwiftPackageProductDependency")
	}
}

func TestSerialize_PBXBuildFileSingleLine(t *testing.T) {
	objects := &Dict{}
	objects.Entries = append(objects.Entries, DictEntry{
		Key:        "AABBCCDDAABBCCDDAABBCCDD",
		KeyComment: "Apus in Frameworks",
		Value: BuildObject("PBXBuildFile",
			"productRef", "1122334411223344AABBCCDD",
		),
	})

	root := &Dict{}
	root.Set("objects", objects)
	root.SetString("rootObject", "AAAA", false)

	out := Serialize(root)

	// PBXBuildFile should be on a single line
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "PBXBuildFile") && strings.Contains(line, "= {") {
			if !strings.Contains(line, "};") {
				t.Fatalf("PBXBuildFile should be single-line, got: %s", line)
			}
			return
		}
	}
}

func TestSerialize_QuotedStrings(t *testing.T) {
	root := &Dict{}
	root.SetString("simple", "hello", false)
	root.SetString("quoted", "hello world", true)
	root.SetString("path", "com.apple.product-type.application", true)

	out := Serialize(root)
	if !strings.Contains(out, `simple = hello;`) {
		t.Fatal("simple string should not be quoted")
	}
	if !strings.Contains(out, `quoted = "hello world";`) {
		t.Fatal("quoted string should be quoted")
	}
}

func TestSerialize_ArrayWithComments(t *testing.T) {
	arr := &Array{}
	arr.Append(&String{Value: "AABB"}, "Sources")
	arr.Append(&String{Value: "CCDD"}, "Frameworks")

	root := &Dict{}
	root.Set("phases", arr)

	out := Serialize(root)
	if !strings.Contains(out, "AABB /* Sources */,") {
		t.Fatalf("expected array item with comment, got:\n%s", out)
	}
}

func TestRoundtrip_ParseAndSerialize(t *testing.T) {
	src := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 54;
	objects = {

/* Begin PBXBuildFile section */
		AABBCCDDAABBCCDDAABBCCDD /* App.swift in Sources */ = {isa = PBXBuildFile; fileRef = 1122334455667788AABBCCDD /* App.swift */; };
/* End PBXBuildFile section */

/* Begin PBXNativeTarget section */
		1111111111111111111111AA /* MyApp */ = {
			isa = PBXNativeTarget;
			name = MyApp;
			buildPhases = (
				2222222222222222222222BB /* Sources */,
			);
		};
/* End PBXNativeTarget section */

	};
	rootObject = AAAA;
}
`

	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	out := Serialize(root)

	// Re-parse the output to verify structural equivalence
	root2, err := Parse(out)
	if err != nil {
		t.Fatalf("Re-parse error: %v\nOutput:\n%s", err, out)
	}

	objects := root2.GetDict("objects")
	if objects == nil {
		t.Fatal("expected objects in re-parsed output")
	}

	targets := FindObjectsByISA(objects, "PBXNativeTarget")
	if len(targets) != 1 {
		t.Fatalf("expected 1 target after roundtrip, got %d", len(targets))
	}
	if targets[0].Dict.GetString("name") != "MyApp" {
		t.Fatal("target name mismatch after roundtrip")
	}
}

func TestRoundtrip_RealFixture(t *testing.T) {
	// Use a realistic pbxproj structure
	src := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 54;
	objects = {

/* Begin PBXBuildFile section */
		21ED1E497360F96C6FBAE449 /* FixtureApp.swift in Sources */ = {isa = PBXBuildFile; fileRef = 7F4BE60494BE30847CEDD714 /* FixtureApp.swift */; };
		758FA229799E3ACEE28C9B8E /* ContentView.swift in Sources */ = {isa = PBXBuildFile; fileRef = 4A06463A13E73E2FE0B6E6DD /* ContentView.swift */; };
/* End PBXBuildFile section */

/* Begin PBXFileReference section */
		4A06463A13E73E2FE0B6E6DD /* ContentView.swift */ = {
			isa = PBXFileReference;
			lastKnownFileType = sourcecode.swift;
			path = ContentView.swift;
			sourceTree = "<group>";
		};
		7F4BE60494BE30847CEDD714 /* FixtureApp.swift */ = {
			isa = PBXFileReference;
			lastKnownFileType = sourcecode.swift;
			path = FixtureApp.swift;
			sourceTree = "<group>";
		};
/* End PBXFileReference section */

/* Begin PBXNativeTarget section */
		194CAFBE7427D6C66B6F1517 /* MyFixture */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				BA3A34A9D07B771D53E92471 /* Sources */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = MyFixture;
			productName = MyFixture;
			productType = "com.apple.product-type.application";
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		04F96B1AA8E509BB1CED083E /* Project object */ = {
			isa = PBXProject;
			attributes = {
				BuildIndependentTargetsInParallel = YES;
			};
			targets = (
				194CAFBE7427D6C66B6F1517 /* MyFixture */,
			);
		};
/* End PBXProject section */

/* Begin PBXSourcesBuildPhase section */
		BA3A34A9D07B771D53E92471 /* Sources */ = {
			isa = PBXSourcesBuildPhase;
			buildActionMask = 2147483647;
			files = (
				758FA229799E3ACEE28C9B8E /* ContentView.swift in Sources */,
				21ED1E497360F96C6FBAE449 /* FixtureApp.swift in Sources */,
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXSourcesBuildPhase section */

	};
	rootObject = 04F96B1AA8E509BB1CED083E /* Project object */;
}
`

	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	out := Serialize(root)

	// Re-parse to validate structural integrity
	root2, err := Parse(out)
	if err != nil {
		t.Fatalf("Roundtrip re-parse error: %v\nOutput:\n%s", err, out)
	}

	objects := root2.GetDict("objects")

	// Verify all sections survived
	bfs := FindObjectsByISA(objects, "PBXBuildFile")
	if len(bfs) != 2 {
		t.Fatalf("expected 2 PBXBuildFile, got %d", len(bfs))
	}

	frs := FindObjectsByISA(objects, "PBXFileReference")
	if len(frs) != 2 {
		t.Fatalf("expected 2 PBXFileReference, got %d", len(frs))
	}

	targets := FindObjectsByISA(objects, "PBXNativeTarget")
	if len(targets) != 1 || targets[0].Dict.GetString("name") != "MyFixture" {
		t.Fatal("target mismatch after roundtrip")
	}

	project := FindProjectObject(root2)
	if project == nil {
		t.Fatal("expected PBXProject after roundtrip")
	}
}
