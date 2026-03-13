package pbxproj

import "testing"

func TestInsertObject_Idempotent(t *testing.T) {
	objects := &Dict{}

	InsertObject(objects, "AAAA", "First", BuildObject("PBXBuildFile"))
	InsertObject(objects, "AAAA", "Duplicate", BuildObject("PBXBuildFile"))

	if len(objects.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(objects.Entries))
	}
	if objects.Entries[0].KeyComment != "First" {
		t.Fatal("second insert should not overwrite")
	}
}

func TestRemoveObject(t *testing.T) {
	objects := &Dict{}
	InsertObject(objects, "AAAA", "Obj", BuildObject("PBXBuildFile"))

	if !RemoveObject(objects, "AAAA") {
		t.Fatal("expected removal to succeed")
	}
	if len(objects.Entries) != 0 {
		t.Fatal("expected empty after removal")
	}
	if RemoveObject(objects, "AAAA") {
		t.Fatal("expected second removal to return false")
	}
}

func TestEnsureArray_CreatesIfMissing(t *testing.T) {
	dict := &Dict{}
	arr := EnsureArray(dict, "files")

	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if dict.GetArray("files") != arr {
		t.Fatal("expected same array instance")
	}

	// calling again returns the same array
	arr2 := EnsureArray(dict, "files")
	if arr2 != arr {
		t.Fatal("expected same array on second call")
	}
}

func TestAppendToArrayIfAbsent(t *testing.T) {
	dict := &Dict{}

	AppendToArrayIfAbsent(dict, "targets", "AAAA", "MyApp")
	AppendToArrayIfAbsent(dict, "targets", "BBBB", "MyTests")
	AppendToArrayIfAbsent(dict, "targets", "AAAA", "DuplicateApp")

	arr := dict.GetArray("targets")
	if len(arr.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr.Items))
	}
}

func TestRemoveFromArray(t *testing.T) {
	dict := &Dict{}
	AppendToArrayIfAbsent(dict, "files", "AAAA", "File1")
	AppendToArrayIfAbsent(dict, "files", "BBBB", "File2")

	if !RemoveFromArray(dict, "files", "AAAA") {
		t.Fatal("expected removal to succeed")
	}

	arr := dict.GetArray("files")
	if len(arr.Items) != 1 {
		t.Fatalf("expected 1 item after removal, got %d", len(arr.Items))
	}

	if RemoveFromArray(dict, "nonexistent", "AAAA") {
		t.Fatal("expected false for missing key")
	}
}

func TestRemoveEmptyEntry(t *testing.T) {
	dict := &Dict{}
	dict.Set("emptyArr", &Array{})
	dict.Set("emptyDict", &Dict{})
	dict.SetString("str", "hello", false)

	RemoveEmptyEntry(dict, "emptyArr")
	RemoveEmptyEntry(dict, "emptyDict")
	RemoveEmptyEntry(dict, "str")

	if dict.Has("emptyArr") {
		t.Fatal("expected empty array to be removed")
	}
	if dict.Has("emptyDict") {
		t.Fatal("expected empty dict to be removed")
	}
	if !dict.Has("str") {
		t.Fatal("non-empty string should not be removed")
	}
}

func TestBuildObject(t *testing.T) {
	obj := BuildObject("PBXNativeTarget",
		"name", "MyApp",
		"productType", "com.apple.product-type.application",
	)

	if obj.GetString("isa") != "PBXNativeTarget" {
		t.Fatal("expected isa")
	}
	if obj.GetString("name") != "MyApp" {
		t.Fatal("expected name")
	}
	if obj.GetString("productType") != "com.apple.product-type.application" {
		t.Fatal("expected productType")
	}
}

func TestBuildObjectWithDict(t *testing.T) {
	attrs := &Dict{}
	attrs.SetString("key", "val", false)

	obj := BuildObjectWithDict("PBXProject", "attributes", attrs,
		"name", "MyProject",
	)

	if obj.GetString("isa") != "PBXProject" {
		t.Fatal("expected isa")
	}
	if obj.GetDict("attributes") != attrs {
		t.Fatal("expected nested dict")
	}
}
