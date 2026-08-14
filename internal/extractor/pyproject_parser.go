// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Linux Foundation

package extractor

import (
	"path/filepath"
	"strings"
)

// Patterns used when parsing pyproject.toml. Both are constants, so they
// are resolved once rather than per line or per candidate file.
const projectVersionPattern = `^version\s*=\s*["']([^"']+)["']`

var dunderVersionPatterns = []string{`__version__\s*=\s*["']([^"']+)["']`}

// extractFromPyprojectToml handles pyproject.toml with section-aware parsing
func (e *VersionExtractor) extractFromPyprojectToml(filePath string) (string, string, error) {
	fileContent, err := fileReader.ReadFileContent(filePath, false)
	if err != nil {
		return "", "", err
	}

	// Matches lines like: version = "1.2.3" or version = '1.2.3'. Using a
	// regex avoids false matches such as "version_info" or "versioning".
	versionRe, err := getCompiledRegex(projectVersionPattern)
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(fileContent, "\n")
	inProjectSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") {
			// Only the direct [project] section should be processed, not subtables
			// [project] = true (we want this)
			// [project.dependencies] = false (subtable, skip)
			// [tool.something] = false (different section, skip)
			if trimmed == "[project]" {
				inProjectSection = true
			} else {
				inProjectSection = false
			}
			continue
		}

		// If we're in [project] section, look for version
		if inProjectSection && !strings.HasPrefix(trimmed, "#") {
			matches := versionRe.FindStringSubmatch(trimmed)
			if len(matches) == 2 {
				version := matches[1]
				if version != "" && e.isValidVersion(version) {
					return version, "[project] section version", nil
				}
			}
		}
	}

	// If no version found in [project] section, try to find __version__.py files
	// Limit search to prevent performance issues in large projects
	projectDir := filepath.Dir(filePath)
	versionFiles := []string{
		filepath.Join(projectDir, "__version__.py"),
		filepath.Join(projectDir, "src", "*", "__version__.py"),
		filepath.Join(projectDir, "*", "__version__.py"),
	}

	filesChecked := 0
	for _, pattern := range versionFiles {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, versionFile := range matches {
			// Enforce maximum files to check to prevent performance issues
			if filesChecked >= maxVersionFilesToCheck {
				break
			}
			filesChecked++

			// extractVersionWithPatterns rather than extractVersionFromFile:
			// the latter routes anything whose path ends in "pyproject.toml"
			// back into the section-aware parser above.
			if version, _, err := e.extractVersionWithPatterns(versionFile, dunderVersionPatterns); err == nil && version != "" {
				return version, "__version__.py", nil
			}
		}
		// Break outer loop if limit reached
		if filesChecked >= maxVersionFilesToCheck {
			break
		}
	}

	return "", "", nil
}
