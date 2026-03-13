package pbxproj

// sectionOrder defines the canonical ordering of isa-typed sections in a
// pbxproj file, matching Xcode's output. Objects with isa values not listed
// here are placed alphabetically at the end.
var sectionOrder = []string{
	"PBXBuildFile",
	"PBXContainerItemProxy",
	"PBXCopyFilesBuildPhase",
	"PBXFileReference",
	"PBXFrameworksBuildPhase",
	"PBXGroup",
	"PBXHeadersBuildPhase",
	"PBXNativeTarget",
	"PBXProject",
	"PBXResourcesBuildPhase",
	"PBXShellScriptBuildPhase",
	"PBXSourcesBuildPhase",
	"PBXTargetDependency",
	"PBXVariantGroup",
	"XCBuildConfiguration",
	"XCConfigurationList",
	"XCLocalSwiftPackageReference",
	"XCRemoteSwiftPackageReference",
	"XCSwiftPackageProductDependency",
}

// sectionRank maps isa names to their sort position.
// Unknown types get a rank of len(sectionOrder) and sort alphabetically.
var sectionRank map[string]int

func init() {
	sectionRank = make(map[string]int, len(sectionOrder))
	for i, name := range sectionOrder {
		sectionRank[name] = i
	}
}

func rankForISA(isa string) int {
	if rank, ok := sectionRank[isa]; ok {
		return rank
	}
	return len(sectionOrder)
}
