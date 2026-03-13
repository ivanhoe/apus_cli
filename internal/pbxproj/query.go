package pbxproj

// ObjectRef pairs a UUID with its object dict.
type ObjectRef struct {
	UUID    string
	Comment string
	Dict    *Dict
}

// FindObjectsByISA returns all objects in the objects dict where isa == isaType.
func FindObjectsByISA(objects *Dict, isaType string) []ObjectRef {
	var refs []ObjectRef
	for _, entry := range objects.Entries {
		dict, ok := entry.Value.(*Dict)
		if !ok {
			continue
		}
		if dict.GetString("isa") == isaType {
			refs = append(refs, ObjectRef{
				UUID:    entry.Key,
				Comment: entry.KeyComment,
				Dict:    dict,
			})
		}
	}
	return refs
}

// FindObject returns the ObjectRef for a given UUID, or nil if not found.
func FindObject(objects *Dict, uuid string) *ObjectRef {
	for _, entry := range objects.Entries {
		if entry.Key != uuid {
			continue
		}
		dict, ok := entry.Value.(*Dict)
		if !ok {
			continue
		}
		return &ObjectRef{UUID: uuid, Comment: entry.KeyComment, Dict: dict}
	}
	return nil
}

// FindObjectByISAAndField finds the first object where isa matches and
// the given field equals the expected value.
func FindObjectByISAAndField(objects *Dict, isa, field, value string) *ObjectRef {
	for _, ref := range FindObjectsByISA(objects, isa) {
		if ref.Dict.GetString(field) == value {
			return &ref
		}
	}
	return nil
}

// FindObjectByISAAndFieldContains finds the first object where isa matches
// and the given field contains the substring.
func FindObjectByISAAndFieldContains(objects *Dict, isa, field, substr string) *ObjectRef {
	for _, ref := range FindObjectsByISA(objects, isa) {
		v := ref.Dict.GetString(field)
		if v != "" && contains(v, substr) {
			return &ref
		}
	}
	return nil
}

// FindProductDependency finds an XCSwiftPackageProductDependency that references
// the given package UUID and product name.
func FindProductDependency(objects *Dict, packageUUID, productName string) *ObjectRef {
	for _, ref := range FindObjectsByISA(objects, "XCSwiftPackageProductDependency") {
		if ref.Dict.GetString("productName") != productName {
			continue
		}
		if packageUUID == "" || ref.Dict.GetString("package") == packageUUID {
			return &ref
		}
	}
	return nil
}

// FindBuildFile finds a PBXBuildFile that references the given product dependency UUID.
func FindBuildFile(objects *Dict, productRefUUID string) *ObjectRef {
	for _, ref := range FindObjectsByISA(objects, "PBXBuildFile") {
		if ref.Dict.GetString("productRef") == productRefUUID {
			return &ref
		}
	}
	return nil
}

// FindNativeTarget finds a PBXNativeTarget by name.
func FindNativeTarget(objects *Dict, name string) *ObjectRef {
	return FindObjectByISAAndField(objects, "PBXNativeTarget", "name", name)
}

// FindFrameworksBuildPhase finds the PBXFrameworksBuildPhase for a target by
// looking at the target's buildPhases array and finding the phase with isa
// PBXFrameworksBuildPhase.
func FindFrameworksBuildPhase(objects *Dict, targetRef *ObjectRef) *ObjectRef {
	phases := targetRef.Dict.GetArray("buildPhases")
	if phases == nil {
		return nil
	}
	for _, item := range phases.Items {
		s, ok := item.Value.(*String)
		if !ok {
			continue
		}
		phaseObj := FindObject(objects, s.Value)
		if phaseObj != nil && phaseObj.Dict.GetString("isa") == "PBXFrameworksBuildPhase" {
			return phaseObj
		}
	}
	return nil
}

// FindProjectObject finds the PBXProject root object.
func FindProjectObject(root *Dict) *ObjectRef {
	rootUUID := root.GetString("rootObject")
	if rootUUID == "" {
		return nil
	}
	objects := root.GetDict("objects")
	if objects == nil {
		return nil
	}
	return FindObject(objects, rootUUID)
}

// ListNativeTargetNames returns the names of all PBXNativeTarget objects.
func ListNativeTargetNames(objects *Dict) []string {
	refs := FindObjectsByISA(objects, "PBXNativeTarget")
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if name := ref.Dict.GetString("name"); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
