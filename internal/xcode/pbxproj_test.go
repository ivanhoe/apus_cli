package xcode

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ivanhoe/apus_cli/internal/pbxproj"
)

// ── Migration tests ────────────────────────────────────────────────────────

func TestMigrateLegacyRequirement(t *testing.T) {
	src := `// !$*UTF8*$!
{
	objects = {

/* Begin XCRemoteSwiftPackageReference section */
		AAAAAAAAAAAAAAAAAAAAAAAA /* XCRemoteSwiftPackageReference "apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = upToNextMajorVersion;
				minimumVersion = 0.3.0;
			};
		};
/* End XCRemoteSwiftPackageReference section */

	};
	rootObject = BBBB;
}
`
	root, err := pbxproj.Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	objects := root.GetDict("objects")
	migrateLegacyRequirement(objects)

	ref := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if ref == nil {
		t.Fatal("expected to find remote ref after migration")
	}

	req := ref.Dict.GetDict("requirement")
	if req == nil {
		t.Fatal("expected requirement dict")
	}
	if req.GetString("kind") != "branch" {
		t.Fatalf("expected kind=branch, got %q", req.GetString("kind"))
	}
	if req.GetString("branch") != "main" {
		t.Fatalf("expected branch=main, got %q", req.GetString("branch"))
	}
	if req.Has("minimumVersion") {
		t.Fatal("minimumVersion should be removed after migration")
	}
}

func TestMigrateLegacyRequirement_NoOp(t *testing.T) {
	src := `// !$*UTF8*$!
{
	objects = {

/* Begin XCRemoteSwiftPackageReference section */
		AAAAAAAAAAAAAAAAAAAAAAAA /* XCRemoteSwiftPackageReference "apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
/* End XCRemoteSwiftPackageReference section */

	};
	rootObject = BBBB;
}
`
	root, err := pbxproj.Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	objects := root.GetDict("objects")
	migrateLegacyRequirement(objects)

	ref := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	req := ref.Dict.GetDict("requirement")
	if req.GetString("kind") != "branch" {
		t.Fatal("should remain branch")
	}
}

// ── Detection tests ────────────────────────────────────────────────────────

func TestDetectState_RemoteAndLocal(t *testing.T) {
	src := `// !$*UTF8*$!
{
	objects = {

/* Begin XCLocalSwiftPackageReference section */
		BBBBBBBBBBBBBBBBBBBBBBBB /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
/* End XCLocalSwiftPackageReference section */

/* Begin XCRemoteSwiftPackageReference section */
		AAAAAAAAAAAAAAAAAAAAAAAA /* XCRemoteSwiftPackageReference "apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
/* End XCRemoteSwiftPackageReference section */

	};
	rootObject = CCCC;
}
`
	root, err := pbxproj.Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	state := detectState(root.GetDict("objects"))
	if !state.Remote || !state.Local {
		t.Fatalf("expected both remote and local, got %+v", state)
	}
}

// ── Add dependency tests ───────────────────────────────────────────────────

func TestAddApusDependencyWithLocalPath(t *testing.T) {
	pbx := buildTestPBXProj(testPBXProjOpts{
		withFrameworks: true,
	})

	dir := t.TempDir()
	localApusDir := filepath.Join(dir, "vendor", "apus")
	os.MkdirAll(localApusDir, 0o755)
	os.WriteFile(filepath.Join(localApusDir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0o644)

	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := AddApusDependencyWithLocalPath(filepath.Dir(pbxPath), "MyApp", localApusDir); err != nil {
		t.Fatalf("AddApusDependencyWithLocalPath() error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	// Should have local ref, not remote
	locals := findLocalApusRefs(objects)
	if len(locals) == 0 {
		t.Fatal("expected local package reference")
	}
	remote := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if remote != nil {
		t.Fatal("did not expect remote Apus URL in local package setup")
	}
}

func TestAddApusDependency_Remote(t *testing.T) {
	pbx := buildTestPBXProj(testPBXProjOpts{
		withFrameworks: true,
	})

	dir := t.TempDir()
	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := AddApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("AddApusDependency() error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	remote := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if remote == nil {
		t.Fatal("expected remote package reference")
	}

	dep := pbxproj.FindProductDependency(objects, remote.UUID, apusProduct)
	if dep == nil {
		t.Fatal("expected product dependency")
	}

	bf := pbxproj.FindBuildFile(objects, dep.UUID)
	if bf == nil {
		t.Fatal("expected build file")
	}
}

// ── Remove dependency tests ────────────────────────────────────────────────

func TestRemoveApusDependency(t *testing.T) {
	remoteUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	depUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	buildUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"
	frameworksUUID := "DDDDDDDDDDDDDDDDDDDDDDDD"
	projectUUID := "EEEEEEEEEEEEEEEEEEEEEEEE"
	targetUUID := "FFFFFFFFFFFFFFFFFFFFFFFF"

	pbx := `// !$*UTF8*$!
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
	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := RemoveApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	// Verify all Apus-related objects are gone
	if ref := pbxproj.FindObjectByISAAndFieldContains(objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL); ref != nil {
		t.Fatal("remote package reference should be removed")
	}
	if dep := pbxproj.FindProductDependency(objects, "", apusProduct); dep != nil {
		t.Fatal("product dependency should be removed")
	}
	bfs := pbxproj.FindObjectsByISA(objects, "PBXBuildFile")
	for _, bf := range bfs {
		if bf.Dict.GetString("productRef") == depUUID {
			t.Fatal("build file should be removed")
		}
	}

	// Project should still be structurally sound
	project := pbxproj.FindProjectObject(root)
	if project == nil {
		t.Fatal("PBXProject should survive removal")
	}
}

func TestRemoveApusDependency_LocalOnly(t *testing.T) {
	localUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	depUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	buildUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"
	frameworksUUID := "DDDDDDDDDDDDDDDDDDDDDDDD"
	projectUUID := "EEEEEEEEEEEEEEEEEEEEEEEE"
	targetUUID := "FFFFFFFFFFFFFFFFFFFFFFFF"

	pbx := `// !$*UTF8*$!
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
			packageReferences = (
				` + localUUID + ` /* XCLocalSwiftPackageReference "../apus" */,
			);
		};
/* End PBXProject section */

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
	rootObject = ` + projectUUID + `;
}
`

	dir := t.TempDir()
	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := RemoveApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("RemoveApusDependency() error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	if len(findLocalApusRefs(objects)) > 0 {
		t.Fatal("local package reference should be removed")
	}
	if dep := pbxproj.FindProductDependency(objects, "", apusProduct); dep != nil {
		t.Fatal("product dependency should be removed")
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

	// File should not be modified when no Apus dependency exists
	data, _ := os.ReadFile(pbxFile)
	if string(data) != content {
		t.Fatalf("expected no changes on project without Apus")
	}
}

// ── Roundtrip tests (add + remove) ─────────────────────────────────────────

func TestAddAndRemove_RoundTrip_NoFrameworks(t *testing.T) {
	pbx := buildTestPBXProj(testPBXProjOpts{})

	dir := t.TempDir()
	localApusDir := filepath.Join(dir, "vendor", "apus")
	os.MkdirAll(localApusDir, 0o755)
	os.WriteFile(filepath.Join(localApusDir, "Package.swift"), []byte("// swift-tools-version: 5.9"), 0o644)

	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := AddApusDependencyWithLocalPath(filepath.Dir(pbxPath), "MyApp", localApusDir); err != nil {
		t.Fatalf("Add error: %v", err)
	}
	if err := RemoveApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	// Structurally equivalent: no Apus objects, target intact
	if dep := pbxproj.FindProductDependency(objects, "", apusProduct); dep != nil {
		t.Fatal("Apus should be fully removed after roundtrip")
	}

	target := pbxproj.FindNativeTarget(objects, "MyApp")
	if target == nil {
		t.Fatal("MyApp target should survive roundtrip")
	}
}

func TestAddAndRemove_RoundTrip_WithExistingFrameworks(t *testing.T) {
	pbx := buildTestPBXProj(testPBXProjOpts{
		withFrameworks: true,
	})

	dir := t.TempDir()
	pbxPath := writeTempPBXProj(t, dir, pbx)

	if err := AddApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("Add error: %v", err)
	}
	if err := RemoveApusDependency(filepath.Dir(pbxPath), "MyApp"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	root := readAndVerify(t, pbxPath)
	objects := root.GetDict("objects")

	if dep := pbxproj.FindProductDependency(objects, "", apusProduct); dep != nil {
		t.Fatal("Apus should be fully removed after roundtrip")
	}

	// The existing Frameworks phase should survive
	target := pbxproj.FindNativeTarget(objects, "MyApp")
	if target == nil {
		t.Fatal("target should survive roundtrip")
	}
	fwPhase := pbxproj.FindFrameworksBuildPhase(objects, target)
	if fwPhase == nil {
		t.Fatal("existing Frameworks phase should survive roundtrip")
	}
}

// ── pbxprojPath tests ──────────────────────────────────────────────────────

func TestPbxprojPath_DirectXcodeproj(t *testing.T) {
	dir := t.TempDir()
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
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	os.MkdirAll(projDir, 0o755)

	_, err := pbxprojPath(dir)
	if err == nil {
		t.Fatal("expected error when project.pbxproj is missing")
	}
}

func TestPbxprojPath_NoXcodeproj(t *testing.T) {
	dir := t.TempDir()

	_, err := pbxprojPath(dir)
	if err == nil {
		t.Fatal("expected error when no .xcodeproj exists")
	}
}

// ── UUID tests (via pbxproj package) ───────────────────────────────────────

func TestNewUUID_Format(t *testing.T) {
	uuid, err := pbxproj.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID() error: %v", err)
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
		uuid, err := pbxproj.NewUUID()
		if err != nil {
			t.Fatalf("NewUUID() error on iteration %d: %v", i, err)
		}
		if _, ok := seen[uuid]; ok {
			t.Fatalf("duplicate UUID on iteration %d: %s", i, uuid)
		}
		seen[uuid] = struct{}{}
	}
}

// ── Test helpers ───────────────────────────────────────────────────────────

type testPBXProjOpts struct {
	withFrameworks bool
}

func buildTestPBXProj(opts testPBXProjOpts) string {
	projectUUID := "AAAAAAAAAAAAAAAAAAAAAAAA"
	targetUUID := "BBBBBBBBBBBBBBBBBBBBBBBB"
	frameworksUUID := "CCCCCCCCCCCCCCCCCCCCCCCC"

	var frameworksSection, frameworksPhaseRef string
	if opts.withFrameworks {
		frameworksSection = `
/* Begin PBXFrameworksBuildPhase section */
		` + frameworksUUID + ` /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXFrameworksBuildPhase section */
`
		frameworksPhaseRef = "\n\t\t\t\t" + frameworksUUID + ` /* Frameworks */,`
	}

	return `// !$*UTF8*$!
{
	objects = {

/* Begin PBXBuildFile section */
/* End PBXBuildFile section */
` + frameworksSection + `
/* Begin PBXNativeTarget section */
		` + targetUUID + ` /* MyApp */ = {
			isa = PBXNativeTarget;
			buildPhases = (
				111111111111111111111111 /* Sources */,` + frameworksPhaseRef + `
			);
			name = MyApp;
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		` + projectUUID + ` /* Project object */ = {
			isa = PBXProject;
			targets = (
				` + targetUUID + ` /* MyApp */,
			);
		};
/* End PBXProject section */

	};
	rootObject = ` + projectUUID + ` /* Project object */;
}
`
}

func writeTempPBXProj(t *testing.T, dir, content string) string {
	t.Helper()
	projDir := filepath.Join(dir, "MyApp.xcodeproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pbxFile := filepath.Join(projDir, "project.pbxproj")
	if err := os.WriteFile(pbxFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return pbxFile
}

func readAndVerify(t *testing.T, pbxPath string) *pbxproj.Dict {
	t.Helper()
	data, err := os.ReadFile(pbxPath)
	if err != nil {
		t.Fatalf("read pbxproj: %v", err)
	}
	root, err := pbxproj.Parse(string(data))
	if err != nil {
		t.Fatalf("parse pbxproj: %v\ncontent:\n%s", err, string(data))
	}
	if root.GetDict("objects") == nil {
		t.Fatal("expected objects dict")
	}
	return root
}

// normalizePBXProjForComparison normalizes whitespace for rough comparison.
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
	normalized = regexp.MustCompile(`\n{3,}`).ReplaceAllString(normalized, "\n\n")
	return normalized
}
