package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveCommandPath(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		return cwd, nil
	}

	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve path: %w", err)
		}
		path = absolute
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("invalid --path %q: %w", rawPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("invalid --path %q: not a directory", rawPath)
	}

	return path, nil
}
