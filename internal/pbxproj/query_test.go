package pbxproj

import "testing"

func buildTestObjects() *Dict {
	objects := &Dict{}

	InsertObject(objects, "AAAA", "MyApp", BuildObject("PBXNativeTarget",
		"name", "MyApp",
		"productType", "com.apple.product-type.application",
	))
	InsertObject(objects, "BBBB", "MyTests", BuildObject("PBXNativeTarget",
		"name", "MyTests",
		"productType", "com.apple.product-type.bundle.unit-test",
	))
	InsertObject(objects, "CCCC", "Apus", BuildObject("XCSwiftPackageProductDependency",
		"productName", "Apus",
		"package", "DDDD",
	))
	InsertObject(objects, "DDDD", `XCRemoteSwiftPackageReference "apus"`,
		BuildObject("XCRemoteSwiftPackageReference",
			"repositoryURL", "https://github.com/ivanhoe/apus",
		))
	InsertObject(objects, "EEEE", "Apus in Frameworks", BuildObject("PBXBuildFile",
		"productRef", "CCCC",
	))

	return objects
}

func TestFindObjectsByISA(t *testing.T) {
	objects := buildTestObjects()

	targets := FindObjectsByISA(objects, "PBXNativeTarget")
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	refs := FindObjectsByISA(objects, "XCRemoteSwiftPackageReference")
	if len(refs) != 1 {
		t.Fatalf("expected 1 remote ref, got %d", len(refs))
	}
}

func TestFindObject(t *testing.T) {
	objects := buildTestObjects()

	ref := FindObject(objects, "AAAA")
	if ref == nil {
		t.Fatal("expected to find AAAA")
	}
	if ref.Dict.GetString("name") != "MyApp" {
		t.Fatal("expected MyApp")
	}

	if FindObject(objects, "ZZZZ") != nil {
		t.Fatal("expected nil for unknown UUID")
	}
}

func TestFindNativeTarget(t *testing.T) {
	objects := buildTestObjects()

	ref := FindNativeTarget(objects, "MyApp")
	if ref == nil {
		t.Fatal("expected to find MyApp target")
	}
	if ref.UUID != "AAAA" {
		t.Fatalf("expected UUID AAAA, got %s", ref.UUID)
	}

	if FindNativeTarget(objects, "NonExistent") != nil {
		t.Fatal("expected nil for nonexistent target")
	}
}

func TestFindProductDependency(t *testing.T) {
	objects := buildTestObjects()

	dep := FindProductDependency(objects, "DDDD", "Apus")
	if dep == nil {
		t.Fatal("expected to find Apus product dependency")
	}
	if dep.UUID != "CCCC" {
		t.Fatalf("expected UUID CCCC, got %s", dep.UUID)
	}
}

func TestFindBuildFile(t *testing.T) {
	objects := buildTestObjects()

	bf := FindBuildFile(objects, "CCCC")
	if bf == nil {
		t.Fatal("expected to find build file for CCCC")
	}
	if bf.UUID != "EEEE" {
		t.Fatalf("expected UUID EEEE, got %s", bf.UUID)
	}
}

func TestFindObjectByISAAndFieldContains(t *testing.T) {
	objects := buildTestObjects()

	ref := FindObjectByISAAndFieldContains(objects, "XCRemoteSwiftPackageReference", "repositoryURL", "ivanhoe/apus")
	if ref == nil {
		t.Fatal("expected to find remote ref by URL substring")
	}
	if ref.UUID != "DDDD" {
		t.Fatalf("expected UUID DDDD, got %s", ref.UUID)
	}
}

func TestListNativeTargetNames(t *testing.T) {
	objects := buildTestObjects()

	names := ListNativeTargetNames(objects)
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}
