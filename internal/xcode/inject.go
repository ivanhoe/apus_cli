package xcode

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	apusImportBlock = "#if DEBUG\nimport Apus\n#endif\n"
	apusStartLine   = "\n        #if DEBUG\n        Apus.shared.start(interceptNetwork: true)\n        #endif"
)

// HasApusIntegration returns true when the project already contains both
// `import Apus` and `Apus.shared.start(...)` anywhere in Swift sources.
func HasApusIntegration(dir string) (bool, error) {
	hasImport := false
	hasStart := false

	err := filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		base := filepath.Base(p)
		if info.IsDir() {
			if base == ".build" || base == "DerivedData" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".swift") {
			return nil
		}

		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		src := string(raw)
		if strings.Contains(src, "import Apus") {
			hasImport = true
		}
		if strings.Contains(src, "Apus.shared.start(") {
			hasStart = true
		}
		if hasImport && hasStart {
			return filepath.SkipAll
		}
		return nil
	})
	if err == filepath.SkipAll {
		err = nil
	}
	if err != nil {
		return false, err
	}
	return hasImport && hasStart, nil
}

// InjectApus modifies the Swift @main file to import and start Apus.
// It is idempotent — calling it twice produces the same result.
func InjectApus(filePath string) error {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read entry file: %w", err)
	}
	src := string(raw)

	// ── Idempotency check ──
	if strings.Contains(src, "import Apus") {
		return nil // already injected
	}

	// ── 1. Add import block after the last `import` line ──
	src, err = injectImport(src)
	if err != nil {
		return err
	}

	// ── 2. Inject Apus.shared.start() ──
	src, err = injectStart(src)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(src), 0o644)
}

// injectImport adds the `#if DEBUG / import Apus / #endif` block after the last import statement.
func injectImport(src string) (string, error) {
	lines := strings.Split(src, "\n")
	lastImport := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") {
			lastImport = i
		}
	}
	if lastImport == -1 {
		return "", fmt.Errorf("no import statement found in Swift file — cannot determine where to add `import Apus`")
	}

	// Insert the block after the last import line
	insertLines := strings.Split("\n"+apusImportBlock, "\n")
	newLines := make([]string, 0, len(lines)+len(insertLines))
	newLines = append(newLines, lines[:lastImport+1]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[lastImport+1:]...)
	return strings.Join(newLines, "\n"), nil
}

// injectStart inserts Apus.shared.start() inside init() or synthesizes init() before var body.
func injectStart(src string) (string, error) {
	// Case A: already has init() — insert as first statement
	if idx := findInit(src); idx != -1 {
		return insertAfterInitBrace(src, idx)
	}

	// Case B: no init() — synthesize one before `var body`
	bodyIdx := strings.Index(src, "var body")
	if bodyIdx == -1 {
		// SwiftUI App using @main — try to find struct/class body opening brace
		bodyIdx = findStructBodyBrace(src)
		if bodyIdx == -1 {
			return "", fmt.Errorf("cannot find `var body` or struct body in Swift file — please add `Apus.shared.start()` manually in your App init()")
		}
	}

	initBlock := "\n    init() {" + apusStartLine + "\n    }\n\n    "
	return src[:bodyIdx] + initBlock + src[bodyIdx:], nil
}

// initPattern matches `init()` declarations with any indentation (spaces or tabs),
// optional `override` keyword, and optional spacing before the opening brace.
// It avoids matching `init(param:)` (init with parameters) or `deinit`.
var initPattern = regexp.MustCompile(`(?m)^[ \t]+(override[ \t]+)?init\(\)[ \t]*\{`)

// findInit returns the byte index of an `init()` declaration in src, or -1.
func findInit(src string) int {
	loc := initPattern.FindStringIndex(src)
	if loc == nil {
		return -1
	}
	return loc[0]
}

// insertAfterInitBrace inserts the Apus start call as first line inside the init() body.
func insertAfterInitBrace(src string, initIdx int) (string, error) {
	// Find the opening brace of init
	braceIdx := strings.Index(src[initIdx:], "{")
	if braceIdx == -1 {
		return "", fmt.Errorf("malformed init() — no opening brace found")
	}
	insertAt := initIdx + braceIdx + 1 // position right after "{"
	return src[:insertAt] + apusStartLine + src[insertAt:], nil
}

// findStructBodyBrace finds the opening brace of a @main struct/class.
func findStructBodyBrace(src string) int {
	mainIdx := findMainAttributeIndex(src)
	if mainIdx == -1 {
		return -1
	}
	// Find the struct/class declaration after @main
	rest := src[mainIdx:]
	structIdx := strings.Index(rest, "struct ")
	classIdx := strings.Index(rest, "class ")

	var declIdx int
	switch {
	case structIdx == -1 && classIdx == -1:
		return -1
	case structIdx == -1:
		declIdx = classIdx
	case classIdx == -1:
		declIdx = structIdx
	default:
		if structIdx < classIdx {
			declIdx = structIdx
		} else {
			declIdx = classIdx
		}
	}

	braceIdx := strings.Index(rest[declIdx:], "{")
	if braceIdx == -1 {
		return -1
	}
	return mainIdx + declIdx + braceIdx + 1
}

func findMainAttributeIndex(src string) int {
	mainIdx := strings.Index(src, "@main")
	uiAppMainIdx := strings.Index(src, "@UIApplicationMain")
	switch {
	case mainIdx == -1:
		return uiAppMainIdx
	case uiAppMainIdx == -1:
		return mainIdx
	case mainIdx < uiAppMainIdx:
		return mainIdx
	default:
		return uiAppMainIdx
	}
}
