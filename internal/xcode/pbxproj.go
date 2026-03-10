package xcode

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	apusRepoURL          = "https://github.com/ivanhoe/apus"
	apusBranch           = "main"
	apusProduct          = "Apus"
	apusLegacyMinVersion = "0.3.0"
	apusFrameworksPhase  = "Apus Frameworks"
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
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return DependencyState{}, err
	}

	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return DependencyState{}, fmt.Errorf("read pbxproj: %w", err)
	}

	return detectApusDependencyInSource(string(raw)), nil
}

func detectApusDependencyInSource(src string) DependencyState {
	return DependencyState{
		Remote: strings.Contains(src, apusRepoURL),
		Local:  len(findLocalApusRefUUIDs(src)) > 0,
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
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return fmt.Errorf("read pbxproj: %w", err)
	}
	src := string(raw)
	localPath = strings.TrimSpace(localPath)
	state := detectApusDependencyInSource(src)

	if localPath != "" {
		if state.Any() {
			return nil
		}
		return addLocalApusDependency(pbxPath, src, target, localPath)
	}

	// Idempotency: already has Apus remote reference. Migrate legacy requirement,
	// normalize old local references, and ensure target wiring is complete.
	if strings.Contains(src, apusRepoURL) {
		updated := migrateLegacyApusRequirement(src)
		normalized, err := normalizeLocalApusReference(updated)
		if err != nil {
			return fmt.Errorf("normalize local Apus package reference: %w", err)
		}
		updated, err = ensureApusDependencyWiring(normalized, target)
		if err != nil {
			return fmt.Errorf("ensure Apus dependency wiring: %w", err)
		}
		if updated == src {
			return nil
		}
		return os.WriteFile(pbxPath, []byte(updated), 0o644)
	}

	// Generate UUIDs for the 4 new objects
	refUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	depUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	buildUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}

	// ── 1. Insert XCRemoteSwiftPackageReference section entry ──
	src, err = insertRemotePackageRef(src, refUUID)
	if err != nil {
		return fmt.Errorf("insert XCRemoteSwiftPackageReference: %w", err)
	}

	// ── 2. Insert XCSwiftPackageProductDependency section entry ──
	src, err = insertProductDependency(src, depUUID, refUUID)
	if err != nil {
		return fmt.Errorf("insert XCSwiftPackageProductDependency: %w", err)
	}

	// ── 3. Insert PBXBuildFile entry ──
	src, err = insertBuildFile(src, buildUUID, depUUID)
	if err != nil {
		return fmt.Errorf("insert PBXBuildFile: %w", err)
	}

	// ── 4. Add to packageReferences in PBXProject ──
	src, err = addToPackageReferences(src, refUUID)
	if err != nil {
		return fmt.Errorf("add to packageReferences: %w", err)
	}

	// ── 5. Add to target's packageProductDependencies + frameworks phase ──
	src, err = addToTarget(src, target, depUUID, buildUUID)
	if err != nil {
		return fmt.Errorf("add to target %s: %w", target, err)
	}

	src, err = normalizeLocalApusReference(src)
	if err != nil {
		return fmt.Errorf("normalize local Apus package reference: %w", err)
	}

	src, err = ensureApusDependencyWiring(src, target)
	if err != nil {
		return fmt.Errorf("ensure Apus dependency wiring: %w", err)
	}

	return os.WriteFile(pbxPath, []byte(src), 0o644)
}

func addLocalApusDependency(pbxPath, src, target, localPath string) error {
	projectDir := filepath.Dir(filepath.Dir(pbxPath))
	relativePath, err := filepath.Rel(projectDir, localPath)
	if err != nil {
		return fmt.Errorf("compute local package path: %w", err)
	}
	relativePath = filepath.ToSlash(relativePath)

	refUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	depUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	buildUUID, err := newUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}

	src, err = insertLocalPackageRef(src, refUUID, relativePath)
	if err != nil {
		return fmt.Errorf("insert XCLocalSwiftPackageReference: %w", err)
	}
	src, err = insertProductDependencyAfterSection(src, depUUID, refUUID, localPackageReferenceComment(relativePath), "/* End XCLocalSwiftPackageReference section */")
	if err != nil {
		return fmt.Errorf("insert XCSwiftPackageProductDependency: %w", err)
	}
	src, err = insertBuildFile(src, buildUUID, depUUID)
	if err != nil {
		return fmt.Errorf("insert PBXBuildFile: %w", err)
	}
	src, err = addToPackageReferencesWithComment(src, refUUID, localPackageReferenceComment(relativePath))
	if err != nil {
		return fmt.Errorf("add to packageReferences: %w", err)
	}
	src, err = addToTarget(src, target, depUUID, buildUUID)
	if err != nil {
		return fmt.Errorf("add to target %s: %w", target, err)
	}

	return os.WriteFile(pbxPath, []byte(src), 0o644)
}

func migrateLegacyApusRequirement(src string) string {
	re := regexp.MustCompile(`(?s)(repositoryURL = "` + regexp.QuoteMeta(apusRepoURL) + `";\s*requirement = \{\s*)kind = upToNextMajorVersion;\s*minimumVersion = [^;]+;(\s*\};)`)
	return re.ReplaceAllString(src, fmt.Sprintf("${1}kind = branch;\n\t\t\t\tbranch = %s;${2}", apusBranch))
}

func ensureApusDependencyWiring(src, target string) (string, error) {
	remoteUUID, err := findApusRemoteRefUUID(src)
	if err != nil {
		return "", err
	}

	depUUID := findApusProductDependencyUUID(src, remoteUUID)
	if depUUID == "" {
		depUUID, err = newUUID()
		if err != nil {
			return "", fmt.Errorf("generate UUID: %w", err)
		}
		src, err = insertProductDependency(src, depUUID, remoteUUID)
		if err != nil {
			return "", err
		}
	}

	buildUUID := findApusBuildFileUUID(src, depUUID)
	if buildUUID == "" {
		buildUUID, err = newUUID()
		if err != nil {
			return "", fmt.Errorf("generate UUID: %w", err)
		}
		src, err = insertBuildFile(src, buildUUID, depUUID)
		if err != nil {
			return "", err
		}
	}

	src, err = addToPackageReferences(src, remoteUUID)
	if err != nil {
		return "", err
	}

	src, err = addToTarget(src, target, depUUID, buildUUID)
	if err != nil {
		return "", err
	}

	return src, nil
}

func normalizeLocalApusReference(src string) (string, error) {
	localRefs := findLocalApusRefUUIDs(src)
	if len(localRefs) == 0 {
		return src, nil
	}

	remoteUUID, err := findApusRemoteRefUUID(src)
	if err != nil {
		return "", err
	}

	for _, localUUID := range localRefs {
		src = replaceLocalApusPackageLine(src, localUUID, remoteUUID)
		src = removeLocalApusPackageReferenceLine(src, localUUID)
		src = removeLocalApusReferenceObject(src, localUUID)
	}

	return src, nil
}

func findLocalApusRefUUIDs(src string) []string {
	// Match local package reference objects and keep those that clearly point to Apus.
	re := regexp.MustCompile(`(?s)([0-9A-F]{24}) /\* XCLocalSwiftPackageReference "([^"]*)" \*/ = \{\s*isa = XCLocalSwiftPackageReference;\s*(.*?)\s*\};`)
	matches := re.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var uuids []string
	for _, m := range matches {
		uuid := m[1]
		commentPath := strings.ToLower(m[2])
		body := strings.ToLower(m[3])
		if strings.Contains(commentPath, "apus") || strings.Contains(body, "relativepath = ../apus") || strings.Contains(body, "/apus") {
			if _, ok := seen[uuid]; ok {
				continue
			}
			seen[uuid] = struct{}{}
			uuids = append(uuids, uuid)
		}
	}
	return uuids
}

func findApusRemoteRefUUID(src string) (string, error) {
	re := regexp.MustCompile(`(?s)([0-9A-F]{24}) /\* XCRemoteSwiftPackageReference "[^"]*" \*/ = \{\s*isa = XCRemoteSwiftPackageReference;\s*repositoryURL = "` + regexp.QuoteMeta(apusRepoURL) + `";`)
	m := re.FindStringSubmatch(src)
	if len(m) < 2 {
		return "", fmt.Errorf("Apus remote package reference not found")
	}
	return m[1], nil
}

func findApusProductDependencyUUID(src, remoteUUID string) string {
	re := regexp.MustCompile(`(?s)([0-9A-F]{24}) /\* ` + regexp.QuoteMeta(apusProduct) + ` \*/ = \{\s*isa = XCSwiftPackageProductDependency;(.*?)\};`)
	matches := re.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return ""
	}

	fallback := ""
	for _, m := range matches {
		uuid := m[1]
		body := m[2]
		if fallback == "" {
			fallback = uuid
		}
		if strings.Contains(body, "package = "+remoteUUID+" ") {
			return uuid
		}
	}
	return fallback
}

func findApusBuildFileUUID(src, depUUID string) string {
	re := regexp.MustCompile(`(?s)([0-9A-F]{24}) /\* ` + regexp.QuoteMeta(apusProduct) + ` in Frameworks \*/ = \{\s*isa = PBXBuildFile;\s*productRef = ` + depUUID + ` /\* ` + regexp.QuoteMeta(apusProduct) + ` \*/;\s*\};`)
	m := re.FindStringSubmatch(src)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func findApusProductDependencyUUIDs(src string, packageUUIDs []string) []string {
	if len(packageUUIDs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(packageUUIDs))
	var depUUIDs []string
	for _, packageUUID := range packageUUIDs {
		if packageUUID == "" {
			continue
		}
		depUUID := findApusProductDependencyUUID(src, packageUUID)
		if depUUID == "" {
			continue
		}
		if _, ok := seen[depUUID]; ok {
			continue
		}
		seen[depUUID] = struct{}{}
		depUUIDs = append(depUUIDs, depUUID)
	}
	return depUUIDs
}

func findApusBuildFileUUIDs(src string, depUUIDs []string) []string {
	if len(depUUIDs) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(depUUIDs))
	var buildUUIDs []string
	for _, depUUID := range depUUIDs {
		if depUUID == "" {
			continue
		}
		buildUUID := findApusBuildFileUUID(src, depUUID)
		if buildUUID == "" {
			continue
		}
		if _, ok := seen[buildUUID]; ok {
			continue
		}
		seen[buildUUID] = struct{}{}
		buildUUIDs = append(buildUUIDs, buildUUID)
	}
	return buildUUIDs
}

func replaceLocalApusPackageLine(src, localUUID, remoteUUID string) string {
	re := regexp.MustCompile(`package = ` + localUUID + ` /\* XCLocalSwiftPackageReference "[^"]*" \*/;`)
	return re.ReplaceAllString(src, fmt.Sprintf(`package = %s /* XCRemoteSwiftPackageReference "%s" */;`, remoteUUID, apusProduct))
}

func removeLocalApusPackageReferenceLine(src, localUUID string) string {
	re := regexp.MustCompile(`(?m)\n[ \t]*` + localUUID + ` /\* XCLocalSwiftPackageReference "[^"]*" \*/,[ \t]*\n`)
	return re.ReplaceAllString(src, "\n")
}

func removeLocalApusReferenceObject(src, localUUID string) string {
	re := regexp.MustCompile(`(?s)\n?\s*` + localUUID + ` /\* XCLocalSwiftPackageReference "[^"]*" \*/ = \{\s*isa = XCLocalSwiftPackageReference;\s*relativePath = [^;]+;\s*\};\n?`)
	return re.ReplaceAllString(src, "\n")
}

// pbxprojPath returns the path to project.pbxproj inside the .xcodeproj bundle.
func pbxprojPath(projPath string) (string, error) {
	p := projPath
	if !strings.HasSuffix(p, ".xcodeproj") {
		// projPath might be the directory containing .xcodeproj
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
	path := p + "/project.pbxproj"
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("project.pbxproj not found at %s", path)
	}
	return path, nil
}

// newUUID generates a 24-character uppercase hex string (Xcode PBX UUID format).
func newUUID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PBX UUID: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

// ── Section insertion helpers ──────────────────────────────────────────────

func insertRemotePackageRef(src, refUUID string) (string, error) {
	entry := fmt.Sprintf(`		%s /* XCRemoteSwiftPackageReference "%s" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "%s";
			requirement = {
				kind = branch;
				branch = %s;
			};
		};`, refUUID, apusProduct, apusRepoURL, apusBranch)

	// Section may not exist if the project has zero SPM dependencies — create it.
	const sectionEnd = "/* End XCRemoteSwiftPackageReference section */"
	if !strings.Contains(src, sectionEnd) {
		section := "\n/* Begin XCRemoteSwiftPackageReference section */\n" +
			entry + "\n" +
			sectionEnd + "\n"
		// Insert inside the objects dictionary — before the `};` that closes it.
		// That `};` is the last one before `rootObject`.
		rootIdx := strings.Index(src, "rootObject")
		if rootIdx == -1 {
			return "", fmt.Errorf("cannot find rootObject in pbxproj to insert XCRemoteSwiftPackageReference section")
		}
		closingIdx := strings.LastIndex(src[:rootIdx], "};")
		if closingIdx == -1 {
			return "", fmt.Errorf("cannot find objects closing brace in pbxproj")
		}
		return src[:closingIdx] + section + src[closingIdx:], nil
	}
	return insertBeforeSectionEnd(src, sectionEnd, entry)
}

func insertProductDependency(src, depUUID, refUUID string) (string, error) {
	return insertProductDependencyAfterSection(src, depUUID, refUUID, fmt.Sprintf(`XCRemoteSwiftPackageReference "%s"`, apusProduct), "/* End XCRemoteSwiftPackageReference section */")
}

func insertLocalPackageRef(src, refUUID, relativePath string) (string, error) {
	entry := fmt.Sprintf(`		%s /* %s */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = %s;
		};`, refUUID, localPackageReferenceComment(relativePath), relativePath)

	const sectionEnd = "/* End XCLocalSwiftPackageReference section */"
	if !strings.Contains(src, sectionEnd) {
		section := "\n/* Begin XCLocalSwiftPackageReference section */\n" +
			entry + "\n" +
			sectionEnd + "\n"
		rootIdx := strings.Index(src, "rootObject")
		if rootIdx == -1 {
			return "", fmt.Errorf("cannot find rootObject in pbxproj to insert XCLocalSwiftPackageReference section")
		}
		closingIdx := strings.LastIndex(src[:rootIdx], "};")
		if closingIdx == -1 {
			return "", fmt.Errorf("cannot find objects closing brace in pbxproj")
		}
		return src[:closingIdx] + section + src[closingIdx:], nil
	}
	return insertBeforeSectionEnd(src, sectionEnd, entry)
}

func localPackageReferenceComment(relativePath string) string {
	return fmt.Sprintf(`XCLocalSwiftPackageReference "%s"`, relativePath)
}

func insertProductDependencyWithComment(src, depUUID, refUUID, packageComment string) (string, error) {
	return insertProductDependencyAfterSection(src, depUUID, refUUID, packageComment, "/* End XCRemoteSwiftPackageReference section */")
}

func insertProductDependencyAfterSection(src, depUUID, refUUID, packageComment, sectionEndMarker string) (string, error) {
	entry := fmt.Sprintf(`		%s /* %s */ = {
			isa = XCSwiftPackageProductDependency;
			package = %s /* %s */;
			productName = %s;
		};`, depUUID, apusProduct, refUUID, packageComment, apusProduct)

	// Section may not exist yet if there are no SPM deps — create it
	const sectionEnd = "/* End XCSwiftPackageProductDependency section */"
	if !strings.Contains(src, sectionEnd) {
		section := "\n/* Begin XCSwiftPackageProductDependency section */\n" +
			entry + "\n" +
			sectionEnd + "\n"
		// Insert after the package reference section (not inside it).
		idx := strings.Index(src, sectionEndMarker)
		if idx == -1 {
			return "", fmt.Errorf("cannot find package reference section end marker %q", sectionEndMarker)
		}
		insertPoint := idx + len(sectionEndMarker)
		return src[:insertPoint] + section + src[insertPoint:], nil
	}
	return insertBeforeSectionEnd(src, sectionEnd, entry)
}

func insertBuildFile(src, buildUUID, depUUID string) (string, error) {
	entry := fmt.Sprintf(`		%s /* %s in Frameworks */ = {isa = PBXBuildFile; productRef = %s /* %s */; };`,
		buildUUID, apusProduct, depUUID, apusProduct)
	return insertBeforeSectionEnd(src, "/* End PBXBuildFile section */", entry)
}

func packageReferencesContain(src, refUUID string) bool {
	re := regexp.MustCompile(`(?s)packageReferences\s*=\s*\(([^)]*)\)`)
	match := re.FindStringSubmatch(src)
	if match == nil {
		return false
	}
	return strings.Contains(match[1], refUUID+" /*")
}

func addToPackageReferences(src, refUUID string) (string, error) {
	return addToPackageReferencesWithComment(src, refUUID, fmt.Sprintf(`XCRemoteSwiftPackageReference "%s"`, apusProduct))
}

func addToPackageReferencesWithComment(src, refUUID, comment string) (string, error) {
	if packageReferencesContain(src, refUUID) {
		return src, nil
	}

	// Find packageReferences = ( ... ); in PBXProject
	re := regexp.MustCompile(`(packageReferences\s*=\s*\()`)
	if !re.MatchString(src) {
		// No packageReferences key yet — insert it before the closing of PBXProject object
		target := "/* End PBXProject section */"
		idx := strings.Index(src, target)
		if idx == -1 {
			return "", fmt.Errorf("cannot find PBXProject section to add packageReferences")
		}
		beforeSection := src[:idx]
		lastBrace := strings.LastIndex(beforeSection, "};")
		if lastBrace == -1 {
			return "", fmt.Errorf("cannot find PBXProject object closing brace")
		}
		injection := fmt.Sprintf("\t\t\tpackageReferences = (\n\t\t\t\t%s /* %s */,\n\t\t\t);\n", refUUID, comment)
		return src[:lastBrace] + injection + src[lastBrace:], nil
	}

	// Append to existing packageReferences list
	loc := re.FindStringIndex(src)
	insertPoint := loc[1] // right after `packageReferences = (`
	entry := fmt.Sprintf("\n\t\t\t\t%s /* %s */,", refUUID, comment)
	return src[:insertPoint] + entry + src[insertPoint:], nil
}

func addToTarget(src, targetName, depUUID, buildUUID string) (string, error) {
	// ── packageProductDependencies ──
	targetObjStart, targetObjEnd, err := findTargetObject(src, targetName)
	if err != nil {
		return "", err
	}
	targetObj := src[targetObjStart:targetObjEnd]

	depToken := depUUID + " /* " + apusProduct + " */"
	if !strings.Contains(targetObj, depToken) {
		depEntry := fmt.Sprintf("\n\t\t\t\t%s /* %s */,", depUUID, apusProduct)
		reDeps := regexp.MustCompile(`(packageProductDependencies\s*=\s*\()`)
		if reDeps.MatchString(targetObj) {
			loc := reDeps.FindStringIndex(targetObj)
			targetObj = targetObj[:loc[1]] + depEntry + targetObj[loc[1]:]
		} else {
			// Insert packageProductDependencies before closing `};`
			lastBrace := strings.LastIndex(targetObj, "};")
			if lastBrace == -1 {
				return "", fmt.Errorf("cannot find target object closing brace for target %s", targetName)
			}
			injection := fmt.Sprintf("\t\t\tpackageProductDependencies = (%s\n\t\t\t);\n\t\t", depEntry)
			targetObj = targetObj[:lastBrace] + injection + targetObj[lastBrace:]
		}
		src = src[:targetObjStart] + targetObj + src[targetObjEnd:]
	}

	// ── PBXFrameworksBuildPhase ──
	src, err = addToBuildPhase(src, targetName, buildUUID)
	if err != nil {
		return "", err
	}

	return src, nil
}

// insertBeforeSectionEnd inserts `entry` on its own line before `sectionEnd` marker.
func insertBeforeSectionEnd(src, sectionEnd, entry string) (string, error) {
	idx := strings.Index(src, sectionEnd)
	if idx == -1 {
		return "", fmt.Errorf("section marker %q not found in pbxproj", sectionEnd)
	}
	return src[:idx] + entry + "\n" + src[idx:], nil
}

// findTargetObject returns the byte range [start, end) of the PBXNativeTarget object for targetName.
func findTargetObject(src, targetName string) (int, int, error) {
	targetPattern := regexp.MustCompile(`(?s)/\* ` + regexp.QuoteMeta(targetName) + ` \*/ = \{\s*isa = PBXNativeTarget`)
	loc := targetPattern.FindStringIndex(src)
	if loc == nil {
		return 0, 0, fmt.Errorf("PBXNativeTarget for %q not found in pbxproj", targetName)
	}
	idx := loc[0]
	start := idx
	for start > 0 && src[start-1] != '\n' {
		start--
	}

	braceStart := strings.Index(src[idx:], "{")
	if braceStart == -1 {
		return 0, 0, fmt.Errorf("no opening brace for PBXNativeTarget %s", targetName)
	}
	absStart := idx + braceStart
	depth := 0
	end := absStart
	for end < len(src) {
		switch src[end] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				if end+1 < len(src) && src[end+1] == ';' {
					end += 2
				}
				return start, end, nil
			}
		}
		end++
	}
	return 0, 0, fmt.Errorf("unmatched braces in PBXNativeTarget %s", targetName)
}

// addToBuildPhase appends a build file reference to the PBXFrameworksBuildPhase for the target.
// If the target has no Frameworks build phase, one is created.
func addToBuildPhase(src, targetName, buildUUID string) (string, error) {
	targetObjStart, targetObjEnd, err := findTargetObject(src, targetName)
	if err != nil {
		return "", err
	}
	targetObj := src[targetObjStart:targetObjEnd]

	reBuildPhases := regexp.MustCompile(`(buildPhases\s*=\s*\()([^)]*)\)`)
	match := reBuildPhases.FindStringSubmatch(targetObj)
	if match == nil {
		return "", fmt.Errorf("no buildPhases found in target %s", targetName)
	}

	reUUID := regexp.MustCompile(`([0-9A-F]{24})\s*/\* (?:Frameworks|` + regexp.QuoteMeta(apusFrameworksPhase) + `) \*/`)
	uuidMatch := reUUID.FindStringSubmatch(match[2])

	if uuidMatch == nil {
		// No Frameworks build phase exists — create one
		phaseUUID, err := newUUID()
		if err != nil {
			return "", fmt.Errorf("generate UUID for Frameworks phase: %w", err)
		}

		// Insert PBXFrameworksBuildPhase object into its section
		phaseObj := fmt.Sprintf(`		%s /* %s */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
				%s /* %s in Frameworks */,
			);
			runOnlyForDeploymentPostprocessing = 0;
		};`, phaseUUID, apusFrameworksPhase, buildUUID, apusProduct)

		const fwSectionEnd = "/* End PBXFrameworksBuildPhase section */"
		if strings.Contains(src, fwSectionEnd) {
			src, err = insertBeforeSectionEnd(src, fwSectionEnd, phaseObj)
			if err != nil {
				return "", err
			}
		} else {
			// Create the entire PBXFrameworksBuildPhase section
			section := "\n/* Begin PBXFrameworksBuildPhase section */\n" +
				phaseObj + "\n" +
				fwSectionEnd + "\n"
			// Insert before PBXGroup or PBXNativeTarget section
			for _, marker := range []string{"/* Begin PBXGroup section */", "/* Begin PBXNativeTarget section */"} {
				idx := strings.Index(src, marker)
				if idx != -1 {
					src = src[:idx] + section + src[idx:]
					break
				}
			}
		}

		// Add the phase UUID to the target's buildPhases list
		// Re-find target since src changed
		targetObjStart, targetObjEnd, err = findTargetObject(src, targetName)
		if err != nil {
			return "", err
		}
		targetObj = src[targetObjStart:targetObjEnd]
		phaseEntry := fmt.Sprintf("\n\t\t\t\t%s /* %s */,", phaseUUID, apusFrameworksPhase)
		reBP := regexp.MustCompile(`(buildPhases\s*=\s*\()`)
		if loc := reBP.FindStringIndex(targetObj); loc != nil {
			newTarget := targetObj[:loc[1]] + phaseEntry + targetObj[loc[1]:]
			src = src[:targetObjStart] + newTarget + src[targetObjEnd:]
		}
		return src, nil
	}

	// Frameworks phase already exists — append build file to it
	frameworksPhaseUUID := uuidMatch[1]

	phasePattern := regexp.MustCompile(`(?s)` + frameworksPhaseUUID + ` /\* (?:Frameworks|` + regexp.QuoteMeta(apusFrameworksPhase) + `) \*/ = \{\s*isa = PBXFrameworksBuildPhase`)
	phaseLoc := phasePattern.FindStringIndex(src)
	if phaseLoc == nil {
		return src, nil
	}
	phaseIdx := phaseLoc[0]

	phaseEnd := strings.Index(src[phaseIdx:], "};")
	if phaseEnd == -1 {
		return src, nil
	}
	phaseSection := src[phaseIdx : phaseIdx+phaseEnd+2]

	buildToken := buildUUID + " /* " + apusProduct + " in Frameworks */"
	if strings.Contains(phaseSection, buildToken) {
		return src, nil
	}

	entry := fmt.Sprintf("\n\t\t\t\t%s /* %s in Frameworks */,", buildUUID, apusProduct)
	reFiles := regexp.MustCompile(`(files\s*=\s*\()`)
	if !reFiles.MatchString(phaseSection) {
		return src, nil
	}
	newPhaseSection := reFiles.ReplaceAllStringFunc(phaseSection, func(s string) string {
		return s + entry
	})

	return src[:phaseIdx] + newPhaseSection + src[phaseIdx+phaseEnd+2:], nil
}

// ── Removal helpers ───────────────────────────────────────────────────────

// RemoveApusDependency removes all Apus Swift Package references from the .pbxproj file.
// It is idempotent — returns nil without modifying the file if Apus is not present.
func RemoveApusDependency(projPath string, target string) error {
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return fmt.Errorf("read pbxproj: %w", err)
	}
	src := string(raw)

	state := detectApusDependencyInSource(src)
	if !state.Any() {
		return nil
	}

	remoteUUID := ""
	if state.Remote {
		remoteUUID, err = findApusRemoteRefUUID(src)
		if err != nil {
			return fmt.Errorf("find Apus remote ref: %w", err)
		}
	}

	localUUIDs := findLocalApusRefUUIDs(src)
	packageUUIDs := make([]string, 0, len(localUUIDs)+1)
	if remoteUUID != "" {
		packageUUIDs = append(packageUUIDs, remoteUUID)
	}
	packageUUIDs = append(packageUUIDs, localUUIDs...)

	depUUIDs := findApusProductDependencyUUIDs(src, packageUUIDs)
	buildUUIDs := findApusBuildFileUUIDs(src, depUUIDs)

	// Remove in reverse order of insertion
	for _, buildUUID := range buildUUIDs {
		src = removeFromBuildPhaseFiles(src, buildUUID)
		src = removeBuildFileEntry(src, buildUUID)
	}

	for _, depUUID := range depUUIDs {
		src = removeFromTargetProductDeps(src, target, depUUID)
		src = removeProductDependencyEntry(src, depUUID)
	}

	if remoteUUID != "" {
		src = removeFromPackageRefsList(src, remoteUUID)
		src = removeRemotePackageRefEntry(src, remoteUUID)
	}

	for _, localUUID := range localUUIDs {
		src = removeLocalApusPackageReferenceLine(src, localUUID)
		src = removeLocalApusReferenceObject(src, localUUID)
	}

	src = removeEmptyFrameworksPhases(src, target)
	src = cleanupEmptySections(src)
	src = cleanupEmptyLists(src)

	return os.WriteFile(pbxPath, []byte(src), 0o644)
}

func removeFromBuildPhaseFiles(src, buildUUID string) string {
	re := regexp.MustCompile(`(?m)\n[ \t]*` + buildUUID + ` /\* ` + regexp.QuoteMeta(apusProduct) + ` in Frameworks \*/,[ \t]*\n`)
	return re.ReplaceAllString(src, "\n")
}

func removeBuildFileEntry(src, buildUUID string) string {
	re := regexp.MustCompile(`(?m)\n[ \t]*` + buildUUID + ` /\* ` + regexp.QuoteMeta(apusProduct) + ` in Frameworks \*/ = \{[^}]+\};[ \t]*\n`)
	return re.ReplaceAllString(src, "\n")
}

func removeFromTargetProductDeps(src, targetName, depUUID string) string {
	re := regexp.MustCompile(`(?m)\n[ \t]*` + depUUID + ` /\* ` + regexp.QuoteMeta(apusProduct) + ` \*/,[ \t]*\n`)
	return re.ReplaceAllString(src, "\n")
}

func removeProductDependencyEntry(src, depUUID string) string {
	// Use \n\t\t}; to match the 2-tab-indented closing brace (not nested ones)
	re := regexp.MustCompile(`(?s)\n?\s*` + depUUID + ` /\* ` + regexp.QuoteMeta(apusProduct) + ` \*/ = \{\s*isa = XCSwiftPackageProductDependency;.*?\n\t\t\};[ \t]*`)
	return re.ReplaceAllString(src, "\n")
}

func removeFromPackageRefsList(src, remoteUUID string) string {
	re := regexp.MustCompile(`(?m)\n[ \t]*` + remoteUUID + ` /\* XCRemoteSwiftPackageReference "` + regexp.QuoteMeta(apusProduct) + `" \*/,[ \t]*\n`)
	return re.ReplaceAllString(src, "\n")
}

func removeRemotePackageRefEntry(src, remoteUUID string) string {
	// Use \n\t\t}; to match the 2-tab-indented closing brace (not nested ones like requirement = {...};)
	re := regexp.MustCompile(`(?s)\n?\s*` + remoteUUID + ` /\* XCRemoteSwiftPackageReference "` + regexp.QuoteMeta(apusProduct) + `" \*/ = \{\s*isa = XCRemoteSwiftPackageReference;.*?\n\t\t\};[ \t]*`)
	return re.ReplaceAllString(src, "\n")
}

func removeEmptyFrameworksPhases(src, targetName string) string {
	targetObjStart, targetObjEnd, err := findTargetObject(src, targetName)
	if err != nil {
		return src
	}

	targetObj := src[targetObjStart:targetObjEnd]
	reUUID := regexp.MustCompile(`([0-9A-F]{24}) /\* ` + regexp.QuoteMeta(apusFrameworksPhase) + ` \*/`)
	matches := reUUID.FindAllStringSubmatch(targetObj, -1)
	if len(matches) == 0 {
		return src
	}

	var emptyPhaseUUIDs []string
	for _, match := range matches {
		phaseUUID := match[1]
		phasePattern := regexp.MustCompile(`(?s)\n?\s*` + phaseUUID + ` /\* ` + regexp.QuoteMeta(apusFrameworksPhase) + ` \*/ = \{\s*isa = PBXFrameworksBuildPhase;(.*?)\n\t\t\};[ \t]*`)
		phaseMatch := phasePattern.FindStringSubmatch(src)
		if len(phaseMatch) == 0 {
			continue
		}

		if !regexp.MustCompile(`files\s*=\s*\(\s*\);`).MatchString(phaseMatch[1]) {
			continue
		}

		refPattern := regexp.MustCompile(`\n[ \t]*` + phaseUUID + ` /\* ` + regexp.QuoteMeta(apusFrameworksPhase) + ` \*/,\n`)
		targetObj = refPattern.ReplaceAllString(targetObj, "\n")
		emptyPhaseUUIDs = append(emptyPhaseUUIDs, phaseUUID)
	}

	if len(emptyPhaseUUIDs) == 0 {
		return src
	}

	src = src[:targetObjStart] + targetObj + src[targetObjEnd:]
	for _, phaseUUID := range emptyPhaseUUIDs {
		phasePattern := regexp.MustCompile(`(?s)\n?\s*` + phaseUUID + ` /\* ` + regexp.QuoteMeta(apusFrameworksPhase) + ` \*/ = \{\s*isa = PBXFrameworksBuildPhase;.*?\n\t\t\};[ \t]*`)
		src = phasePattern.ReplaceAllString(src, "\n")
	}

	return src
}

func cleanupEmptySections(src string) string {
	for _, section := range []string{"PBXFrameworksBuildPhase", "XCRemoteSwiftPackageReference", "XCLocalSwiftPackageReference", "XCSwiftPackageProductDependency"} {
		re := regexp.MustCompile(`(?s)\n?/\* Begin ` + section + ` section \*/\s*/\* End ` + section + ` section \*/\n?`)
		src = re.ReplaceAllString(src, "\n")
	}
	return src
}

func cleanupEmptyLists(src string) string {
	for _, key := range []string{"packageReferences", "packageProductDependencies"} {
		re := regexp.MustCompile(`(?ms)\n[ \t]*` + key + ` = \(\s*\n[ \t]*\);\n`)
		src = re.ReplaceAllString(src, "\n")
	}
	src = regexp.MustCompile(`\(\n(?:[ \t]*\n)+([ \t]*\);)`).ReplaceAllString(src, "(\n$1")
	src = regexp.MustCompile(`\n{2,}(/\* End [^\n]+ section \*/)`).ReplaceAllString(src, "\n$1")
	src = regexp.MustCompile(`(/\* End [^\n]+ section \*/)\n[ \t]*\n(\t\};)`).ReplaceAllString(src, "$1\n$2")
	src = regexp.MustCompile(`(?m)\n[ \t]*\n([ \t]*\};)`).ReplaceAllString(src, "\n$1")
	src = regexp.MustCompile(`(?m)\n[ \t]*\n(\t\};\n\trootObject = )`).ReplaceAllString(src, "\n$1")
	src = regexp.MustCompile(`\n{3,}`).ReplaceAllString(src, "\n\n")
	src = regexp.MustCompile(`(?m)^};\n(/\* End [^\n]+ section \*/)`).ReplaceAllString(src, "\t\t};\n$1")
	src = strings.ReplaceAll(src, "\n\n\t};\n\trootObject = ", "\n\t};\n\trootObject = ")
	src = strings.ReplaceAll(src, "\n};\n\trootObject = ", "\n\t};\n\trootObject = ")
	return src
}
