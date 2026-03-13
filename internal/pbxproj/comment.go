package pbxproj

import "strings"

// GenerateComment produces the inline comment for an object reference,
// matching Xcode's convention: UUID /* Name */ = { ... }
//
// The comment is derived from the object's name, productName, path, or isa,
// in that priority order.
func GenerateComment(obj *Dict) string {
	if obj == nil {
		return ""
	}
	// Prefer name fields in priority order
	for _, key := range []string{"name", "productName", "path"} {
		if v := obj.GetString(key); v != "" {
			return v
		}
	}
	return obj.GetString("isa")
}

// BuildFileComment generates the comment for a PBXBuildFile entry.
// Format: "ProductName in PhaseName" (e.g. "Apus in Frameworks").
func BuildFileComment(productName, phaseName string) string {
	if productName == "" {
		return ""
	}
	if phaseName == "" {
		phaseName = "Frameworks"
	}
	return productName + " in " + phaseName
}

// PackageRefComment generates the comment for a package reference.
// Format: XCRemoteSwiftPackageReference "Name" or XCLocalSwiftPackageReference "Path".
func PackageRefComment(isa, nameOrPath string) string {
	return isa + ` "` + nameOrPath + `"`
}

// ResolveComment looks up a UUID in the objects dict and generates
// the appropriate inline comment for it.
func ResolveComment(objects *Dict, uuid string) string {
	obj := objects.GetDict(uuid)
	if obj == nil {
		return ""
	}

	isa := obj.GetString("isa")
	switch isa {
	case "XCRemoteSwiftPackageReference":
		url := obj.GetString("repositoryURL")
		name := lastPathComponent(url)
		return PackageRefComment(isa, name)
	case "XCLocalSwiftPackageReference":
		path := obj.GetString("relativePath")
		return PackageRefComment(isa, path)
	default:
		return GenerateComment(obj)
	}
}

func lastPathComponent(s string) string {
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}
