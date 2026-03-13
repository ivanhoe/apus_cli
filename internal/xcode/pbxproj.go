package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ivanhoe/apus_cli/internal/pbxproj"
)

const (
	apusRepoURL         = "https://github.com/ivanhoe/apus"
	apusBranch          = "main"
	apusProduct         = "Apus"
	apusFrameworksPhase = "Apus Frameworks"
)

// DependencyState describes whether the project references Apus remotely, locally, or both.
type DependencyState struct {
	Remote bool
	Local  bool
}

// Any reports whether any Apus package reference is present in the project.
func (s DependencyState) Any() bool {
	return s.Remote || s.Local
}

// DetectApusDependency returns whether the project references Apus via remote and/or local SPM packages.
func DetectApusDependency(projPath string) (DependencyState, error) {
	root, _, err := readAndParsePBXProj(projPath)
	if err != nil {
		return DependencyState{}, err
	}

	objects := root.GetDict("objects")
	return detectState(objects), nil
}

func detectState(objects *pbxproj.Dict) DependencyState {
	remote := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	locals := findLocalApusRefs(objects)
	return DependencyState{
		Remote: remote != nil,
		Local:  len(locals) > 0,
	}
}

// AddApusDependency inserts Apus as a Swift Package dependency in the .pbxproj file.
// It is idempotent — returns nil without modifying the file if Apus is already present.
func AddApusDependency(projPath string, target string) error {
	return AddApusDependencyWithLocalPath(projPath, target, "")
}

// AddApusDependencyWithLocalPath inserts Apus as either a remote or local Swift Package dependency.
// When localPath is empty, the remote GitHub dependency is used.
func AddApusDependencyWithLocalPath(projPath string, target string, localPath string) error {
	root, pbxPath, err := readAndParsePBXProj(projPath)
	if err != nil {
		return err
	}

	objects := root.GetDict("objects")
	localPath = strings.TrimSpace(localPath)
	state := detectState(objects)

	if localPath != "" {
		if state.Any() {
			return nil
		}
		return addLocalApusDependency(root, objects, pbxPath, target, localPath)
	}

	if state.Remote {
		// Already has remote ref — migrate legacy requirement, normalize local refs, ensure wiring
		migrateLegacyRequirement(objects)
		normalizeLocalRefs(objects)
		ensureWiring(objects, target)
		return writePBXProj(pbxPath, root)
	}

	// Fresh remote installation
	return addRemoteApusDependency(root, objects, pbxPath, target)
}

// RemoveApusDependency removes all Apus Swift Package references from the .pbxproj file.
// It is idempotent — returns nil without modifying the file if Apus is not present.
func RemoveApusDependency(projPath string, target string) error {
	root, pbxPath, err := readAndParsePBXProj(projPath)
	if err != nil {
		return err
	}

	objects := root.GetDict("objects")
	state := detectState(objects)
	if !state.Any() {
		return nil
	}

	removeApusObjects(objects, root, target)
	return writePBXProj(pbxPath, root)
}

// ── Remote dependency addition ─────────────────────────────────────────────

func addRemoteApusDependency(root *pbxproj.Dict, objects *pbxproj.Dict, pbxPath, target string) error {
	refUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	depUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	buildUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}

	insertRemotePackageRef(objects, refUUID)
	insertProductDependency(objects, depUUID, refUUID, "XCRemoteSwiftPackageReference")
	insertBuildFile(objects, buildUUID, depUUID)
	addToProjectPackageRefs(root, objects, refUUID)
	addToTarget(objects, target, depUUID, buildUUID)

	return writePBXProj(pbxPath, root)
}

// ── Local dependency addition ──────────────────────────────────────────────

func addLocalApusDependency(root *pbxproj.Dict, objects *pbxproj.Dict, pbxPath, target, localPath string) error {
	projectDir := filepath.Dir(filepath.Dir(pbxPath))
	relativePath, err := filepath.Rel(projectDir, localPath)
	if err != nil {
		return fmt.Errorf("compute local package path: %w", err)
	}
	relativePath = filepath.ToSlash(relativePath)

	refUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	depUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	buildUUID, err := pbxproj.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}

	insertLocalPackageRef(objects, refUUID, relativePath)
	insertProductDependency(objects, depUUID, refUUID, "XCLocalSwiftPackageReference")
	insertBuildFile(objects, buildUUID, depUUID)
	addToProjectPackageRefs(root, objects, refUUID)
	addToTarget(objects, target, depUUID, buildUUID)

	return writePBXProj(pbxPath, root)
}

// ── Object insertion helpers ───────────────────────────────────────────────

func insertRemotePackageRef(objects *pbxproj.Dict, refUUID string) {
	requirement := &pbxproj.Dict{}
	requirement.SetString("kind", "branch", false)
	requirement.SetString("branch", apusBranch, false)

	obj := pbxproj.BuildObject("XCRemoteSwiftPackageReference",
		"repositoryURL", apusRepoURL,
	)
	obj.Set("requirement", requirement)

	comment := pbxproj.PackageRefComment("XCRemoteSwiftPackageReference", apusProduct)
	pbxproj.InsertObject(objects, refUUID, comment, obj)
}

func insertLocalPackageRef(objects *pbxproj.Dict, refUUID, relativePath string) {
	obj := pbxproj.BuildObject("XCLocalSwiftPackageReference",
		"relativePath", relativePath,
	)
	comment := pbxproj.PackageRefComment("XCLocalSwiftPackageReference", relativePath)
	pbxproj.InsertObject(objects, refUUID, comment, obj)
}

func insertProductDependency(objects *pbxproj.Dict, depUUID, refUUID, refISA string) {
	obj := pbxproj.BuildObject("XCSwiftPackageProductDependency",
		"package", refUUID,
		"productName", apusProduct,
	)
	pbxproj.InsertObject(objects, depUUID, apusProduct, obj)
}

func insertBuildFile(objects *pbxproj.Dict, buildUUID, depUUID string) {
	obj := pbxproj.BuildObject("PBXBuildFile",
		"productRef", depUUID,
	)
	comment := pbxproj.BuildFileComment(apusProduct, "Frameworks")
	pbxproj.InsertObject(objects, buildUUID, comment, obj)
}

func addToProjectPackageRefs(root *pbxproj.Dict, objects *pbxproj.Dict, refUUID string) {
	project := pbxproj.FindProjectObject(root)
	if project == nil {
		return
	}
	pbxproj.AppendToArrayIfAbsent(project.Dict, "packageReferences", refUUID, "")
}

func addToTarget(objects *pbxproj.Dict, targetName, depUUID, buildUUID string) {
	targetRef := pbxproj.FindNativeTarget(objects, targetName)
	if targetRef == nil {
		return
	}

	// Add to packageProductDependencies
	pbxproj.AppendToArrayIfAbsent(targetRef.Dict, "packageProductDependencies", depUUID, apusProduct)

	// Add to Frameworks build phase
	fwPhase := pbxproj.FindFrameworksBuildPhase(objects, targetRef)
	if fwPhase == nil {
		// Create a new PBXFrameworksBuildPhase
		phaseUUID, err := pbxproj.NewUUID()
		if err != nil {
			return
		}

		filesArr := &pbxproj.Array{}
		filesArr.Append(&pbxproj.String{Value: buildUUID}, pbxproj.BuildFileComment(apusProduct, "Frameworks"))

		phaseObj := pbxproj.BuildObject("PBXFrameworksBuildPhase",
			"buildActionMask", "2147483647",
			"runOnlyForDeploymentPostprocessing", "0",
		)
		phaseObj.Set("files", filesArr)
		pbxproj.InsertObject(objects, phaseUUID, apusFrameworksPhase, phaseObj)

		// Add phase UUID to target's buildPhases
		pbxproj.AppendToArrayIfAbsent(targetRef.Dict, "buildPhases", phaseUUID, apusFrameworksPhase)
	} else {
		// Append to existing phase's files list
		pbxproj.AppendToArrayIfAbsent(fwPhase.Dict, "files", buildUUID, pbxproj.BuildFileComment(apusProduct, "Frameworks"))
	}
}

// ── Migration / normalization ──────────────────────────────────────────────

func migrateLegacyRequirement(objects *pbxproj.Dict) {
	ref := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if ref == nil {
		return
	}

	requirement := ref.Dict.GetDict("requirement")
	if requirement == nil {
		return
	}

	kind := requirement.GetString("kind")
	if kind == "upToNextMajorVersion" || kind == "upToNextMinorVersion" {
		requirement.SetString("kind", "branch", false)
		requirement.Remove("minimumVersion")
		requirement.SetString("branch", apusBranch, false)
	}
}

func normalizeLocalRefs(objects *pbxproj.Dict) {
	remoteRef := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if remoteRef == nil {
		return
	}

	locals := findLocalApusRefs(objects)
	for _, local := range locals {
		// Re-point any product dependency from local to remote
		for _, dep := range pbxproj.FindObjectsByISA(objects, "XCSwiftPackageProductDependency") {
			if dep.Dict.GetString("package") == local.UUID {
				dep.Dict.SetString("package", remoteRef.UUID, false)
			}
		}
		// Remove local ref from project packageReferences and objects
		removeFromAllArrays(objects, local.UUID)
		pbxproj.RemoveObject(objects, local.UUID)
	}
}

func ensureWiring(objects *pbxproj.Dict, target string) {
	remoteRef := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if remoteRef == nil {
		return
	}

	dep := pbxproj.FindProductDependency(objects, remoteRef.UUID, apusProduct)
	if dep == nil {
		depUUID, err := pbxproj.NewUUID()
		if err != nil {
			return
		}
		insertProductDependency(objects, depUUID, remoteRef.UUID, "XCRemoteSwiftPackageReference")
		dep = pbxproj.FindObject(objects, depUUID)
	}

	bf := pbxproj.FindBuildFile(objects, dep.UUID)
	if bf == nil {
		buildUUID, err := pbxproj.NewUUID()
		if err != nil {
			return
		}
		insertBuildFile(objects, buildUUID, dep.UUID)
		bf = pbxproj.FindObject(objects, buildUUID)
	}

	addToTarget(objects, target, dep.UUID, bf.UUID)
}

// ── Removal ────────────────────────────────────────────────────────────────

func removeApusObjects(objects *pbxproj.Dict, root *pbxproj.Dict, target string) {
	// Collect all package UUIDs (remote + local)
	var packageUUIDs []string

	remoteRef := pbxproj.FindObjectByISAAndFieldContains(
		objects, "XCRemoteSwiftPackageReference", "repositoryURL", apusRepoURL,
	)
	if remoteRef != nil {
		packageUUIDs = append(packageUUIDs, remoteRef.UUID)
	}

	for _, local := range findLocalApusRefs(objects) {
		packageUUIDs = append(packageUUIDs, local.UUID)
	}

	// Find dependent objects
	var depUUIDs, buildUUIDs []string
	for _, pkgUUID := range packageUUIDs {
		dep := pbxproj.FindProductDependency(objects, pkgUUID, apusProduct)
		if dep != nil {
			depUUIDs = append(depUUIDs, dep.UUID)
			bf := pbxproj.FindBuildFile(objects, dep.UUID)
			if bf != nil {
				buildUUIDs = append(buildUUIDs, bf.UUID)
			}
		}
	}

	// Also find deps/builds that match by product name alone (catches orphaned refs)
	for _, dep := range pbxproj.FindObjectsByISA(objects, "XCSwiftPackageProductDependency") {
		if dep.Dict.GetString("productName") == apusProduct && !containsStr(depUUIDs, dep.UUID) {
			depUUIDs = append(depUUIDs, dep.UUID)
			bf := pbxproj.FindBuildFile(objects, dep.UUID)
			if bf != nil && !containsStr(buildUUIDs, bf.UUID) {
				buildUUIDs = append(buildUUIDs, bf.UUID)
			}
		}
	}

	// Remove build file references from framework phases, then remove the objects
	for _, buildUUID := range buildUUIDs {
		removeFromAllArrays(objects, buildUUID)
		pbxproj.RemoveObject(objects, buildUUID)
	}

	// Remove dependency references from target, then remove the objects
	for _, depUUID := range depUUIDs {
		removeFromAllArrays(objects, depUUID)
		pbxproj.RemoveObject(objects, depUUID)
	}

	// Remove package references from project, then remove the objects
	for _, pkgUUID := range packageUUIDs {
		removeFromAllArrays(objects, pkgUUID)
		pbxproj.RemoveObject(objects, pkgUUID)
	}

	// Clean up empty "Apus Frameworks" phases
	removeEmptyApusFrameworksPhases(objects, target)

	// Clean up empty packageReferences / packageProductDependencies on project & target
	project := pbxproj.FindProjectObject(root)
	if project != nil {
		pbxproj.RemoveEmptyEntry(project.Dict, "packageReferences")
	}
	targetRef := pbxproj.FindNativeTarget(objects, target)
	if targetRef != nil {
		pbxproj.RemoveEmptyEntry(targetRef.Dict, "packageProductDependencies")
	}
}

func removeEmptyApusFrameworksPhases(objects *pbxproj.Dict, targetName string) {
	targetRef := pbxproj.FindNativeTarget(objects, targetName)
	if targetRef == nil {
		return
	}

	phases := targetRef.Dict.GetArray("buildPhases")
	if phases == nil {
		return
	}

	var phasesToRemove []string
	for _, item := range phases.Items {
		s, ok := item.Value.(*pbxproj.String)
		if !ok {
			continue
		}
		phaseObj := pbxproj.FindObject(objects, s.Value)
		if phaseObj == nil {
			continue
		}
		if phaseObj.Dict.GetString("isa") != "PBXFrameworksBuildPhase" {
			continue
		}
		// Only remove if it's our named phase and it's empty
		if item.Comment == apusFrameworksPhase || phaseObj.Comment == apusFrameworksPhase {
			filesArr := phaseObj.Dict.GetArray("files")
			if filesArr == nil || len(filesArr.Items) == 0 {
				phasesToRemove = append(phasesToRemove, s.Value)
			}
		}
	}

	for _, uuid := range phasesToRemove {
		phases.RemoveByValue(uuid)
		pbxproj.RemoveObject(objects, uuid)
	}
}

// removeFromAllArrays removes a UUID string value from all arrays in all objects.
func removeFromAllArrays(objects *pbxproj.Dict, uuid string) {
	for _, entry := range objects.Entries {
		dict, ok := entry.Value.(*pbxproj.Dict)
		if !ok {
			continue
		}
		for _, sub := range dict.Entries {
			arr, ok := sub.Value.(*pbxproj.Array)
			if !ok {
				continue
			}
			arr.RemoveByValue(uuid)
		}
	}
}

// ── Query helpers ──────────────────────────────────────────────────────────

func findLocalApusRefs(objects *pbxproj.Dict) []pbxproj.ObjectRef {
	var refs []pbxproj.ObjectRef
	for _, ref := range pbxproj.FindObjectsByISA(objects, "XCLocalSwiftPackageReference") {
		path := strings.ToLower(ref.Dict.GetString("relativePath"))
		comment := strings.ToLower(ref.Comment)
		if strings.Contains(path, "apus") || strings.Contains(comment, "apus") {
			refs = append(refs, ref)
		}
	}
	return refs
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// ── I/O helpers ────────────────────────────────────────────────────────────

func readAndParsePBXProj(projPath string) (*pbxproj.Dict, string, error) {
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return nil, "", err
	}

	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return nil, "", fmt.Errorf("read pbxproj: %w", err)
	}

	root, err := pbxproj.Parse(string(raw))
	if err != nil {
		return nil, "", fmt.Errorf("parse pbxproj: %w", err)
	}

	return root, pbxPath, nil
}

func writePBXProj(pbxPath string, root *pbxproj.Dict) error {
	out := pbxproj.Serialize(root)
	return os.WriteFile(pbxPath, []byte(out), 0o644)
}

// pbxprojPath returns the path to project.pbxproj inside the .xcodeproj bundle.
func pbxprojPath(projPath string) (string, error) {
	p := projPath
	if !strings.HasSuffix(p, ".xcodeproj") {
		entries, err := os.ReadDir(p)
		if err != nil {
			return "", fmt.Errorf("read dir for pbxproj: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() && strings.HasSuffix(e.Name(), ".xcodeproj") {
				p = filepath.Join(projPath, e.Name())
				break
			}
		}
	}
	path := filepath.Join(p, "project.pbxproj")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("project.pbxproj not found at %s", path)
	}
	return path, nil
}
