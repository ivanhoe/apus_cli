package xcode

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	apusRepoURL          = "https://github.com/ivanhoe/apus"
	apusBranch           = "main"
	apusProduct          = "Apus"
	apusLegacyMinVersion = "0.3.0"
)

// AddApusDependency inserts Apus as a Swift Package dependency in the .pbxproj file.
// It is idempotent — returns nil without modifying the file if Apus is already present.
func AddApusDependency(projPath string, target string) error {
	pbxPath, err := pbxprojPath(projPath)
	if err != nil {
		return err
	}

	raw, err := os.ReadFile(pbxPath)
	if err != nil {
		return fmt.Errorf("read pbxproj: %w", err)
	}
	src := string(raw)

	// Idempotency: already has Apus
	if strings.Contains(src, apusRepoURL) {
		updated := migrateLegacyApusRequirement(src)
		normalized, err := normalizeLocalApusReference(updated)
		if err != nil {
			return fmt.Errorf("normalize local Apus package reference: %w", err)
		}
		updated = normalized
		if updated == src {
			return nil
		}
		return os.WriteFile(pbxPath, []byte(updated), 0o644)
	}

	// Generate UUIDs for the 4 new objects
	refUUID := newUUID()   // XCRemoteSwiftPackageReference
	depUUID := newUUID()   // XCSwiftPackageProductDependency
	buildUUID := newUUID() // PBXBuildFile (framework phase entry)
	_ = buildUUID          // used below

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

	return os.WriteFile(pbxPath, []byte(src), 0o644)
}

func migrateLegacyApusRequirement(src string) string {
	re := regexp.MustCompile(`(?s)(repositoryURL = "` + regexp.QuoteMeta(apusRepoURL) + `";\s*requirement = \{\s*)kind = upToNextMajorVersion;\s*minimumVersion = [^;]+;(\s*\};)`)
	return re.ReplaceAllString(src, fmt.Sprintf("${1}kind = branch;\n\t\t\t\tbranch = %s;${2}", apusBranch))
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
	// Example block:
	// AAAAAA /* XCLocalSwiftPackageReference "../apus" */ = {
	//     isa = XCLocalSwiftPackageReference;
	//     relativePath = ../apus;
	// };
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

func replaceLocalApusPackageLine(src, localUUID, remoteUUID string) string {
	re := regexp.MustCompile(`package = ` + localUUID + ` /\* XCLocalSwiftPackageReference "[^"]*" \*/;`)
	return re.ReplaceAllString(src, fmt.Sprintf(`package = %s /* XCRemoteSwiftPackageReference "%s" */;`, remoteUUID, apusProduct))
}

func removeLocalApusPackageReferenceLine(src, localUUID string) string {
	re := regexp.MustCompile(`\n\s*` + localUUID + ` /\* XCLocalSwiftPackageReference "[^"]*" \*/,\s*`)
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
				p = strings.Join([]string{projPath, e.Name()}, "/")
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
func newUUID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return strings.ToUpper(hex.EncodeToString(b))
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

	return insertBeforeSectionEnd(src, "/* End XCRemoteSwiftPackageReference section */", entry)
}

func insertProductDependency(src, depUUID, refUUID string) (string, error) {
	entry := fmt.Sprintf(`		%s /* %s */ = {
			isa = XCSwiftPackageProductDependency;
			package = %s /* XCRemoteSwiftPackageReference "%s" */;
			productName = %s;
		};`, depUUID, apusProduct, refUUID, apusProduct, apusProduct)

	// Section may not exist yet if there are no SPM deps — create it
	const sectionEnd = "/* End XCSwiftPackageProductDependency section */"
	if !strings.Contains(src, sectionEnd) {
		// Insert a new section before the final closing brace of the objects block
		section := "\n/* Begin XCSwiftPackageProductDependency section */\n" +
			entry + "\n" +
			"/* End XCSwiftPackageProductDependency section */\n"
		return insertBeforeSectionEnd(src, "/* End XCRemoteSwiftPackageReference section */", section)
	}
	return insertBeforeSectionEnd(src, sectionEnd, entry)
}

func insertBuildFile(src, buildUUID, depUUID string) (string, error) {
	entry := fmt.Sprintf(`		%s /* %s in Frameworks */ = {isa = PBXBuildFile; productRef = %s /* %s */; };`,
		buildUUID, apusProduct, depUUID, apusProduct)
	return insertBeforeSectionEnd(src, "/* End PBXBuildFile section */", entry)
}

func addToPackageReferences(src, refUUID string) (string, error) {
	// Find packageReferences = ( ... ); in PBXProject
	re := regexp.MustCompile(`(packageReferences\s*=\s*\()`)
	if !re.MatchString(src) {
		// No packageReferences key yet — insert it before the closing of PBXProject object
		// Find `/* End PBXProject section */` and work backwards
		target := "/* End PBXProject section */"
		idx := strings.Index(src, target)
		if idx == -1 {
			return "", fmt.Errorf("cannot find PBXProject section to add packageReferences")
		}
		// Find the last `};` before the section end
		beforeSection := src[:idx]
		lastBrace := strings.LastIndex(beforeSection, "};")
		if lastBrace == -1 {
			return "", fmt.Errorf("cannot find PBXProject object closing brace")
		}
		injection := fmt.Sprintf("\t\t\tpackageReferences = (\n\t\t\t\t%s /* XCRemoteSwiftPackageReference \"%s\" */,\n\t\t\t);\n\t\t", refUUID, apusProduct)
		return src[:lastBrace] + injection + src[lastBrace:], nil
	}

	// Append to existing packageReferences list
	loc := re.FindStringIndex(src)
	insertPoint := loc[1] // right after `packageReferences = (`
	entry := fmt.Sprintf("\n\t\t\t\t%s /* XCRemoteSwiftPackageReference \"%s\" */,", refUUID, apusProduct)
	return src[:insertPoint] + entry + src[insertPoint:], nil
}

func addToTarget(src, targetName, depUUID, buildUUID string) (string, error) {
	// ── packageProductDependencies ──
	// Find the target's object by looking for `name = <targetName>;` inside a PBXNativeTarget block
	targetObjStart, targetObjEnd, err := findTargetObject(src, targetName)
	if err != nil {
		return "", err
	}
	targetObj := src[targetObjStart:targetObjEnd]

	// Add to packageProductDependencies
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

	// ── PBXFrameworksBuildPhase ──
	// Find the frameworks build phase UUID referenced by this target, then inject the build file
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
	// Pattern: `<UUID> /* <targetName> */ = {\n\t\t\tisa = PBXNativeTarget;`
	pattern := fmt.Sprintf("/* %s */ = {\n\t\t\tisa = PBXNativeTarget", targetName)
	idx := strings.Index(src, pattern)
	if idx == -1 {
		return 0, 0, fmt.Errorf("PBXNativeTarget for %q not found in pbxproj", targetName)
	}
	// Walk back to find UUID start
	start := idx
	for start > 0 && src[start-1] != '\n' {
		start--
	}

	// Find the closing `};` of this object by counting braces from `= {`
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
				// consume the trailing `;`
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
func addToBuildPhase(src, targetName, buildUUID string) (string, error) {
	// Find the frameworks phase UUID from the target's buildPhases list
	targetObjStart, targetObjEnd, err := findTargetObject(src, targetName)
	if err != nil {
		return "", err
	}
	targetObj := src[targetObjStart:targetObjEnd]

	// Extract the frameworks phase UUID (first UUID in buildPhases that maps to PBXFrameworksBuildPhase)
	reBuildPhases := regexp.MustCompile(`buildPhases\s*=\s*\(([^)]*)\)`)
	match := reBuildPhases.FindStringSubmatch(targetObj)
	if match == nil {
		return "", fmt.Errorf("no buildPhases found in target %s", targetName)
	}

	reUUID := regexp.MustCompile(`([0-9A-F]{24})\s*/\* Frameworks \*/`)
	uuidMatch := reUUID.FindStringSubmatch(match[1])
	if uuidMatch == nil {
		// No Frameworks phase found in the target's phase list — that's unusual; skip
		return src, nil
	}
	frameworksPhaseUUID := uuidMatch[1]

	// Find that phase object in the PBXFrameworksBuildPhase section
	phasePattern := fmt.Sprintf("%s /* Frameworks */ = {\n\t\t\tisa = PBXFrameworksBuildPhase", frameworksPhaseUUID)
	phaseIdx := strings.Index(src, phasePattern)
	if phaseIdx == -1 {
		return src, nil // phase not found — skip gracefully
	}

	// Find the `files = (` list in this phase
	phaseEnd := strings.Index(src[phaseIdx:], "};")
	if phaseEnd == -1 {
		return src, nil
	}
	phaseSection := src[phaseIdx : phaseIdx+phaseEnd+2]

	entry := fmt.Sprintf("\n\t\t\t\t%s /* %s in Frameworks */,", buildUUID, apusProduct)
	reFiles := regexp.MustCompile(`(files\s*=\s*\()`)
	if !reFiles.MatchString(phaseSection) {
		return src, nil // no files list — skip
	}
	newPhaseSection := reFiles.ReplaceAllStringFunc(phaseSection, func(s string) string {
		return s + entry
	})

	return src[:phaseIdx] + newPhaseSection + src[phaseIdx+phaseEnd+2:], nil
}
