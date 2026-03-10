package xcode

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMigrateLegacyApusRequirement(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = upToNextMajorVersion;
				minimumVersion = 0.3.0;
			};`

	got := migrateLegacyApusRequirement(input)

	if strings.Contains(got, "upToNextMajorVersion") {
		t.Fatalf("legacy requirement should be removed, got:\n%s", got)
	}
	if strings.Contains(got, "minimumVersion = 0.3.0;") {
		t.Fatalf("legacy minimum version should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "kind = branch;") {
		t.Fatalf("expected branch requirement, got:\n%s", got)
	}
	if !strings.Contains(got, "branch = main;") {
		t.Fatalf("expected branch main requirement, got:\n%s", got)
	}
}

func TestMigrateLegacyApusRequirement_NoLegacy(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};`

	got := migrateLegacyApusRequirement(input)
	if got != input {
		t.Fatalf("expected no-op for non-legacy requirement")
	}
}

func TestMigrateLegacyApusRequirement_VariedFormatting(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
requirement = {
    kind = upToNextMajorVersion;
    minimumVersion = "0.3.0";
};`

	got := migrateLegacyApusRequirement(input)

	if strings.Contains(got, "upToNextMajorVersion") {
		t.Fatalf("legacy requirement should be removed, got:\n%s", got)
	}
	if strings.Contains(got, "minimumVersion") {
		t.Fatalf("legacy minimumVersion should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "kind = branch;") || !strings.Contains(got, "branch = main;") {
		t.Fatalf("expected branch main requirement, got:\n%s", got)
	}
}

func TestNormalizeLocalApusReference(t *testing.T) {
	input := `
		ABCDEFABCDEFABCDEFABCDEF /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */;
			productName = Apus;
		};
		EEEEEEEEEEEEEEEEEEEEEEEE /* XCRemoteSwiftPackageReference "Apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
		packageReferences = (
			AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */,
		);
		AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
`

	got, err := normalizeLocalApusReference(input)
	if err != nil {
		t.Fatalf("normalizeLocalApusReference() error: %v", err)
	}

	if strings.Contains(got, `XCLocalSwiftPackageReference "../apus"`) {
		t.Fatalf("local Apus package reference should be removed:\n%s", got)
	}
	if !strings.Contains(got, `package = EEEEEEEEEEEEEEEEEEEEEEEE /* XCRemoteSwiftPackageReference "Apus" */;`) {
		t.Fatalf("Apus dependency should point to remote package reference:\n%s", got)
	}
}

func TestNormalizeLocalApusReference_NoRemote(t *testing.T) {
	input := `
		ABCDEFABCDEFABCDEFABCDEF /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */;
			productName = Apus;
		};
		AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
`

	_, err := normalizeLocalApusReference(input)
	if err == nil {
		t.Fatalf("expected error when no Apus remote reference exists")
	}
}

func TestDetectApusDependencyInSource_LocalAndRemote(t *testing.T) {
	input := `
		AAAAAAAAAAAAAAAAAAAAAAAA /* XCRemoteSwiftPackageReference "Apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
		};
		BBBBBBBBBBBBBBBBBBBBBBBB /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
`

	state := detectApusDependencyInSource(input)
	if !state.Remote || !state.Local {
		t.Fatalf("expected both remote and local dependency states, got %+v", state)
	}
}

func TestAddApusDependencyWithLocalPath(t *testing.T) {
	projectUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	targetUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	frameworksUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"

	pbxproj := `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
/* End PBXBuildFile section */

/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			files = (
			);
		};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				` + frameworksUUID + ` /* Frameworks */,
			);
			packageProductDependencies = (
			);
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
			packageReferences = (
			);
		};
/* End PBXProject section */

/* Begin XCSwiftPackageProductDependency section */
/* End XCSwiftPackageProductDependency section */

	};
	rootObject = ` + projectUUID + ` /* Project object */;
}
`

	dir := t.TempDir()
	localApusDir := filepath.Join(dir, "vendor", "apus")
	if err := os.MkdirAll(localApusDir, 0o755); err != nil {
		t.Fatalf("mkdir local apus dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localApusDir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0o644); err != nil {
		t.Fatalf("write Package.swift: %v", err)
	}

	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	if err := os.WriteFile(pbxFile, []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	if err := AddApusDependencyWithLocalPath(projDir, "MyApp", localApusDir); err != nil {
		t.Fatalf("AddApusDependencyWithLocalPath() error: %v", err)
	}

	updated, err := os.ReadFile(pbxFile)
	if err != nil {
		t.Fatalf("read pbxproj: %v", err)
	}
	src := string(updated)
	if !strings.Contains(src, `relativePath = vendor/apus;`) {
		t.Fatalf("expected local package relative path, got:\n%s", src)
	}
	if strings.Contains(src, apusRepoURL) {
		t.Fatalf("did not expect remote Apus URL in local package setup:\n%s", src)
	}
}

func TestAddToPackageReferences_Idempotent(t *testing.T) {
	remoteUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	input := `
		packageReferences = (
			AAAAAAAAAAAAAAAAAAAAAAAA /* XCRemoteSwiftPackageReference "Apus" */,
		);
`

	got, err := addToPackageReferences(input, remoteUUID)
	if err != nil {
		t.Fatalf("addToPackageReferences() error: %v", err)
	}
	if got != input {
		t.Fatalf("expected addToPackageReferences() to be idempotent")
	}
}

func TestEnsureApusDependencyWiring_AddsMissingLinks(t *testing.T) {
	remoteUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	depUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	projectUUID := "EEEEEEEEEEEEEEEEEEEEEEEE"
	targetUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"
	frameworksUUID := "DDDDDDDDDDDDDDDDDDDDDDDD"

	input := `/* Begin PBXBuildFile section */
/* End PBXBuildFile section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
			packageReferences = (
			);
		};
/* End PBXProject section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				` + frameworksUUID + ` /* Frameworks */,
			);
			packageProductDependencies = (
			);
		};
/* End PBXNativeTarget section */

/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			files = (
			);
		};
/* End PBXFrameworksBuildPhase section */

/* Begin XCRemoteSwiftPackageReference section */
		` + remoteUUID + ` /* XCRemoteSwiftPackageReference "Apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
/* End XCRemoteSwiftPackageReference section */

/* Begin XCSwiftPackageProductDependency section */
		` + depUUID + ` /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = ` + remoteUUID + ` /* XCRemoteSwiftPackageReference "Apus" */;
			productName = Apus;
		};
/* End XCSwiftPackageProductDependency section */
`

	got, err := ensureApusDependencyWiring(input, "MyApp")
	if err != nil {
		t.Fatalf("ensureApusDependencyWiring() error: %v", err)
	}

	matched, err := regexp.MatchString(`(?s)packageReferences = \(\s*`+remoteUUID+` /\* XCRemoteSwiftPackageReference "Apus" \*/,`, got)
	if err != nil {
		t.Fatalf("packageReferences regexp error: %v", err)
	}
	if !matched {
		t.Fatalf("project should include Apus in packageReferences:\n%s", got)
	}
	if !strings.Contains(got, depUUID+` /* Apus */,`) {
		t.Fatalf("target should include Apus product dependency")
	}
	if !strings.Contains(got, ` /* Apus in Frameworks */ = {isa = PBXBuildFile; productRef = `+depUUID+` /* Apus */; };`) {
		t.Fatalf("missing Apus PBXBuildFile entry")
	}
	if !strings.Contains(got, `/* Apus in Frameworks */,`) {
		t.Fatalf("frameworks phase should include Apus build file")
	}
}

func TestNewUUID_Format(t *testing.T) {
	uuid, err := newUUID()
	if err != nil {
		t.Fatalf("newUUID() error: %v", err)
	}

	if len(uuid) != 24 {
		t.Fatalf("expected 24-char UUID, got %d: %s", len(uuid), uuid)
	}

	matched, _ := regexp.MatchString(`^[0-9A-F]{24}$`, uuid)
	if !matched {
		t.Fatalf("UUID should be uppercase hex, got: %s", uuid)
	}
}

func TestNewUUID_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		uuid, err := newUUID()
		if err != nil {
			t.Fatalf("newUUID() error on iteration %d: %v", i, err)
		}
		if _, ok := seen[uuid]; ok {
			t.Fatalf("duplicate UUID on iteration %d: %s", i, uuid)
		}
		seen[uuid] = struct{}{}
	}
}

func TestPbxprojPath_DirectXcodeproj(t *testing.T) {
	dir := t.TempDir()

	// Create a .xcodeproj directory with project.pbxproj inside
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	os.WriteFile(pbxFile, []byte("{}"), 0o644)

	got, err := pbxprojPath(projDir)
	if err != nil {
		t.Fatalf("pbxprojPath() error: %v", err)
	}
	if got != pbxFile {
		t.Fatalf("expected %s, got %s", pbxFile, got)
	}
}

func TestPbxprojPath_ParentDir(t *testing.T) {
	dir := t.TempDir()

	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	os.WriteFile(pbxFile, []byte("{}"), 0o644)

	// Pass the parent dir, not the .xcodeproj directly
	got, err := pbxprojPath(dir)
	if err != nil {
		t.Fatalf("pbxprojPath() error: %v", err)
	}
	if got != pbxFile {
		t.Fatalf("expected %s, got %s", pbxFile, got)
	}
}

func TestPbxprojPath_NoPbxproj(t *testing.T) {
	dir := t.TempDir()

	// Create .xcodeproj dir but no project.pbxproj
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)

	_, err := pbxprojPath(dir)
	if err == nil {
		t.Fatalf("expected error when project.pbxproj is missing")
	}
}

func TestRemoveApusDependency(t *testing.T) {
	remoteUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	depUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	buildUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"
	frameworksUUID := "DDDDDDDDDDDDDDDDDDDDDDDD"
	projectUUID := "EEEEEEEEEEEEEEEEEEEEEEEE"
	targetUUID := "FFFFFFFFFFFFFFFFFFFFFFFF"

	pbxproj := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 54;
	objects = {

/* Begin PBXBuildFile section */
		` + buildUUID + ` /* Apus in Frameworks */ = {isa = PBXBuildFile; productRef = ` + depUUID + ` /* Apus */; };
/* End PBXBuildFile section */

/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
				` + buildUUID + ` /* Apus in Frameworks */,
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				` + frameworksUUID + ` /* Frameworks */,
			);
			name = MyApp;
			packageProductDependencies = (
				` + depUUID + ` /* Apus */,
			);
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
			targets = (
				` + targetUUID + ` /* MyApp */,
			);
			packageReferences = (
				` + remoteUUID + ` /* XCRemoteSwiftPackageReference "Apus" */,
			);
		};
/* End PBXProject section */

/* Begin XCRemoteSwiftPackageReference section */
		` + remoteUUID + ` /* XCRemoteSwiftPackageReference "Apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
/* End XCRemoteSwiftPackageReference section */

/* Begin XCSwiftPackageProductDependency section */
		` + depUUID + ` /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = ` + remoteUUID + ` /* XCRemoteSwiftPackageReference "Apus" */;
			productName = Apus;
		};
/* End XCSwiftPackageProductDependency section */

		};
	rootObject = ` + projectUUID + ` /* Project object */;
}
`

	dir := t.TempDir()
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	os.WriteFile(pbxFile, []byte(pbxproj), 0o644)

	if err := RemoveApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	data, _ := os.ReadFile(pbxFile)
	result := string(data)

	// Verify all Apus references are gone
	for _, needle := range []string{
		"ivanhoe/apus",
		"XCRemoteSwiftPackageReference",
		"productName = Apus;",
		"Apus in Frameworks",
		remoteUUID,
		depUUID,
		buildUUID,
	} {
		if strings.Contains(result, needle) {
			t.Fatalf("expected %q to be removed from pbxproj:\n%s", needle, result)
		}
	}

	// Verify the file is structurally sound (has key markers)
	for _, marker := range []string{"rootObject", "objects = {"} {
		if !strings.Contains(result, marker) {
			t.Fatalf("expected %q to remain in pbxproj", marker)
		}
	}
}

func TestRemoveApusDependency_LocalOnly(t *testing.T) {
	localUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	depUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	buildUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"
	frameworksUUID := "DDDDDDDDDDDDDDDDDDDDDDDD"
	projectUUID := "EEEEEEEEEEEEEEEEEEEEEEEE"
	targetUUID := "FFFFFFFFFFFFFFFFFFFFFFFF"

	pbxproj := `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
		` + buildUUID + ` /* Apus in Frameworks */ = {isa = PBXBuildFile; productRef = ` + depUUID + ` /* Apus */; };
/* End PBXBuildFile section */

/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			files = (
				` + buildUUID + ` /* Apus in Frameworks */,
			);
		};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
			packageReferences = (
				` + localUUID + ` /* XCLocalSwiftPackageReference "../apus" */,
			);
		};
/* End PBXProject section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				` + frameworksUUID + ` /* Frameworks */,
			);
			packageProductDependencies = (
				` + depUUID + ` /* Apus */,
			);
		};
/* End PBXNativeTarget section */

/* Begin XCLocalSwiftPackageReference section */
		` + localUUID + ` /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
/* End XCLocalSwiftPackageReference section */

/* Begin XCSwiftPackageProductDependency section */
		` + depUUID + ` /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = ` + localUUID + ` /* XCLocalSwiftPackageReference "../apus" */;
			productName = Apus;
		};
/* End XCSwiftPackageProductDependency section */

	};
}
`

	dir := t.TempDir()
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	pbxPath := filepath.Join(projDir, "project.pbxproj")
	if err := os.WriteFile(pbxPath, []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	if err := RemoveApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	updated, err := os.ReadFile(pbxPath)
	if err != nil {
		t.Fatalf("read pbxproj: %v", err)
	}
	src := string(updated)
	if strings.Contains(src, "relativePath = ../apus") {
		t.Fatalf("expected local package reference to be removed:\n%s", src)
	}
	if strings.Contains(src, "productName = Apus;") {
		t.Fatalf("expected Apus product dependency to be removed:\n%s", src)
	}
	if strings.Contains(src, "Apus in Frameworks") {
		t.Fatalf("expected Apus build file to be removed:\n%s", src)
	}
}

func TestAddAndRemoveApusDependencyWithLocalPath_RoundTripsWithoutFrameworksPhase(t *testing.T) {
	projectUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	targetUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"

	pbxproj := `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
/* End PBXBuildFile section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				111111111111111111111111 /* Sources */,
				222222222222222222222222 /* Resources */,
			);
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
		};
/* End PBXProject section */

	};
	rootObject = ` + projectUUID + ` /* Project object */;
}
`

	dir := t.TempDir()
	localApusDir := filepath.Join(dir, "vendor", "apus")
	if err := os.MkdirAll(localApusDir, 0o755); err != nil {
		t.Fatalf("mkdir local apus dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localApusDir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0o644); err != nil {
		t.Fatalf("write Package.swift: %v", err)
	}

	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	pbxPath := filepath.Join(projDir, "project.pbxproj")
	if err := os.WriteFile(pbxPath, []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	if err := AddApusDependencyWithLocalPath(projDir, "MyApp", localApusDir); err != nil {
		t.Fatalf("AddApusDependencyWithLocalPath() error: %v", err)
	}
	if err := RemoveApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	updated, err := os.ReadFile(pbxPath)
	if err != nil {
		t.Fatalf("read pbxproj: %v", err)
	}
	if normalizePBXProjForComparison(string(updated)) != normalizePBXProjForComparison(pbxproj) {
		t.Fatalf("expected pbxproj to roundtrip cleanly.\nwant:\n%s\n\ngot:\n%s", pbxproj, string(updated))
	}
}

func TestAddAndRemoveApusDependency_RoundTripsWithExistingEmptyFrameworksPhase(t *testing.T) {
	projectUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	targetUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	frameworksUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"

	pbxproj := `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
/* End PBXBuildFile section */

/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				111111111111111111111111 /* Sources */,
				` + frameworksUUID + ` /* Frameworks */,
				222222222222222222222222 /* Resources */,
			);
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
		};
/* End PBXProject section */

	};
	rootObject = ` + projectUUID + ` /* Project object */;
}
`

	dir := t.TempDir()
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	pbxPath := filepath.Join(projDir, "project.pbxproj")
	if err := os.WriteFile(pbxPath, []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("write pbxproj: %v", err)
	}

	if err := AddApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("AddApusDependency() error: %v", err)
	}
	if err := RemoveApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	updated, err := os.ReadFile(pbxPath)
	if err != nil {
		t.Fatalf("read pbxproj: %v", err)
	}
	if normalizePBXProjForComparison(string(updated)) != normalizePBXProjForComparison(pbxproj) {
		t.Fatalf("expected existing empty frameworks phase to be preserved.\nwant:\n%s\n\ngot:\n%s", pbxproj, string(updated))
	}
}

func TestRemoveApusDependency_Idempotent(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	content := `// !$*UTF8*$!
{
	objects = {
	};
	rootObject = AAAA;
}
`
	os.WriteFile(pbxFile, []byte(content), 0o644)

	if err := RemoveApusDependency(projDir, "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() on non-Apus project error: %v", err)
	}

	data, _ := os.ReadFile(pbxFile)
	if string(data) != content {
		t.Fatalf("expected no changes on project without Apus")
	}
}

func TestPbxprojPath_NoXcodeproj(t *testing.T) {
	dir := t.TempDir()

	_, err := pbxprojPath(dir)
	if err == nil {
		t.Fatalf("expected error when no .xcodeproj exists")
	}
}

func normalizePBXProjForComparison(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = strings.TrimRight(line, " \t")
	}
	normalized := strings.Join(lines, "\n")
	normalized = regexp.MustCompile(`(/\* End [^\n]+ section \*/)\n(?:[ \t]*\n)+([ \t]*\};)`).ReplaceAllString(normalized, "$1\n$2")
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(normalized, "\n\n")
}
