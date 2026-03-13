package pbxproj

import (
	"testing"
)

func TestParse_MinimalPBXProj(t *testing.T) {
	src := `// !$*UTF8*$!
{
	archiveVersion = 1;
	objects = {
	};
	rootObject = AAAA;
}
`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if root.GetString("archiveVersion") != "1" {
		t.Fatal("archiveVersion mismatch")
	}
	if root.GetString("rootObject") != "AAAA" {
		t.Fatal("rootObject mismatch")
	}
	if root.GetDict("objects") == nil {
		t.Fatal("expected objects dict")
	}
}

func TestParse_WithSections(t *testing.T) {
	src := `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
		AABBCCDD11223344 /* MyFile.swift in Sources */ = {isa = PBXBuildFile; fileRef = EEFF0011 /* MyFile.swift */; };
/* End PBXBuildFile section */

/* Begin PBXNativeTarget section */
		11223344AABBCCDD /* MyApp */ = {
			isa = PBXNativeTarget;
			name = MyApp;
			buildPhases = (
				DEADBEEF12345678 /* Sources */,
			);
		};
/* End PBXNativeTarget section */

	};
	rootObject = AAAA /* Project object */;
}
`

	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	objects := root.GetDict("objects")
	if objects == nil {
		t.Fatal("expected objects dict")
	}

	// Check PBXBuildFile
	bf := objects.GetDict("AABBCCDD11223344")
	if bf == nil {
		t.Fatal("expected build file object")
	}
	if bf.GetString("isa") != "PBXBuildFile" {
		t.Fatalf("expected isa=PBXBuildFile, got %q", bf.GetString("isa"))
	}
	if bf.GetString("fileRef") != "EEFF0011" {
		t.Fatalf("expected fileRef=EEFF0011, got %q", bf.GetString("fileRef"))
	}

	// Check PBXNativeTarget
	nt := objects.GetDict("11223344AABBCCDD")
	if nt == nil {
		t.Fatal("expected native target object")
	}
	if nt.GetString("name") != "MyApp" {
		t.Fatalf("expected name=MyApp, got %q", nt.GetString("name"))
	}

	phases := nt.GetArray("buildPhases")
	if phases == nil || len(phases.Items) != 1 {
		t.Fatal("expected 1 build phase")
	}
}

func TestParse_NestedDict(t *testing.T) {
	src := `{
	attributes = {
		BuildIndependentTargetsInParallel = YES;
		TargetAttributes = {
		};
	};
}`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	attrs := root.GetDict("attributes")
	if attrs == nil {
		t.Fatal("expected attributes dict")
	}
	if attrs.GetString("BuildIndependentTargetsInParallel") != "YES" {
		t.Fatal("expected YES")
	}
	if attrs.GetDict("TargetAttributes") == nil {
		t.Fatal("expected TargetAttributes dict")
	}
}

func TestParse_ArrayWithComments(t *testing.T) {
	src := `{
	targets = (
		AABB /* MyApp */,
		CCDD /* MyTests */,
	);
}`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	arr := root.GetArray("targets")
	if arr == nil || len(arr.Items) != 2 {
		t.Fatalf("expected 2 targets, got %v", arr)
	}

	if s, ok := arr.Items[0].Value.(*String); !ok || s.Value != "AABB" {
		t.Fatalf("expected AABB, got %v", arr.Items[0])
	}
	if arr.Items[0].Comment != "MyApp" {
		t.Fatalf("expected comment MyApp, got %q", arr.Items[0].Comment)
	}
}

func TestParse_QuotedStringValues(t *testing.T) {
	src := `{
	PRODUCT_BUNDLE_IDENTIFIER = "com.example.myapp";
	CODE_SIGN_IDENTITY = "-";
}`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if root.GetString("PRODUCT_BUNDLE_IDENTIFIER") != "com.example.myapp" {
		t.Fatal("bundle id mismatch")
	}
	if root.GetString("CODE_SIGN_IDENTITY") != "-" {
		t.Fatal("code sign identity mismatch")
	}
}

func TestParse_EmptyArray(t *testing.T) {
	src := `{
	buildRules = (
	);
}`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	arr := root.GetArray("buildRules")
	if arr == nil {
		t.Fatal("expected array")
	}
	if len(arr.Items) != 0 {
		t.Fatalf("expected empty array, got %d items", len(arr.Items))
	}
}

func TestParse_KeyWithComment(t *testing.T) {
	src := `{
	AABB /* MyTarget */ = {
		isa = PBXNativeTarget;
		name = MyTarget;
	};
}`
	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	found := false
	for _, e := range root.Entries {
		if e.Key == "AABB" {
			found = true
			if e.KeyComment != "MyTarget" {
				t.Fatalf("expected comment MyTarget, got %q", e.KeyComment)
			}
		}
	}
	if !found {
		t.Fatal("entry AABB not found")
	}
}

func TestParse_RealPBXProj(t *testing.T) {
	src := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 54;
	objects = {

/* Begin PBXBuildFile section */
		21ED1E497360F96C6FBAE449 /* App.swift in Sources */ = {isa = PBXBuildFile; fileRef = 7F4BE60494BE30847CEDD714 /* App.swift */; };
/* End PBXBuildFile section */

/* Begin PBXNativeTarget section */
		194CAFBE7427D6C66B6F1517 /* MyApp */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = 2EFA0295523BF253B8FB7985 /* Build configuration list for PBXNativeTarget "MyApp" */;
			buildPhases = (
				BA3A34A9D07B771D53E92471 /* Sources */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = MyApp;
			productName = MyApp;
			productReference = 863EE7ACE0E734392256509B /* MyApp.app */;
			productType = "com.apple.product-type.application";
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		04F96B1AA8E509BB1CED083E /* Project object */ = {
			isa = PBXProject;
			targets = (
				194CAFBE7427D6C66B6F1517 /* MyApp */,
			);
		};
/* End PBXProject section */

	};
	rootObject = 04F96B1AA8E509BB1CED083E /* Project object */;
}
`

	root, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	objects := root.GetDict("objects")
	targets := FindObjectsByISA(objects, "PBXNativeTarget")
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].Dict.GetString("name") != "MyApp" {
		t.Fatalf("expected MyApp, got %q", targets[0].Dict.GetString("name"))
	}
	if targets[0].Dict.GetString("productType") != "com.apple.product-type.application" {
		t.Fatal("expected productType")
	}
}
