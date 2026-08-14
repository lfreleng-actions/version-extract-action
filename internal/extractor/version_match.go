// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Linux Foundation

package extractor

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Version validation patterns
const (
	// Official Semantic Versioning pattern from semver.org (used by tag-validate-action)
	// Matches: MAJOR.MINOR.PATCH with optional pre-release and build metadata
	// Examples: 1.2.3, 1.0.0-alpha, 1.0.0-alpha.1, 1.0.0+build.123
	semverPattern = `^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	// Python-style versions with dot separator (e.g., 3.2.0.dev, 1.0.0.alpha1)
	// Not strict semver but commonly used in Python ecosystem
	pythonStylePattern = `^[0-9]+\.[0-9]+\.[0-9]+\.[a-zA-Z][0-9a-zA-Z]*$`
	// Simple version patterns (numbers and dots, max 4 components)
	simplePattern = `^[0-9]+(\.[0-9]+){0,3}$`
	// Date-based versions (CalVer)
	datePattern = `^[0-9]{4}(\.[0-9]{2})*$`
)

// whitespaceRun collapses runs of whitespace so multi-line patterns can
// match across formatting. Compiled once: the pattern is a constant and
// the substitution runs against whole file contents.
var whitespaceRun = regexp.MustCompile(`\s+`)

// extractVersionFromFile attempts to extract version using regex patterns
func (e *VersionExtractor) extractVersionFromFile(filePath string,
	patterns []string) (string, string, error) {

	// Special handling for pyproject.toml files
	// The special handler is authoritative - don't fall back to regex patterns
	// because they would incorrectly match versions in wrong sections
	if strings.HasSuffix(filePath, "pyproject.toml") {
		return e.extractFromPyprojectToml(filePath)
	}

	return e.extractVersionWithPatterns(filePath, patterns)
}

// extractVersionWithPatterns extracts version from a file using regex patterns
// This is separated from extractVersionFromFile to avoid recursive issues when
// called from extractFromPyprojectToml for __version__.py files
func (e *VersionExtractor) extractVersionWithPatterns(filePath string,
	patterns []string) (string, string, error) {

	// Detect patterns that need multi-line processing
	needsMultiLine := false
	for _, pattern := range patterns {
		if e.isMultiLinePattern(pattern) {
			needsMultiLine = true
			break
		}
	}

	// Use different processing approaches based on pattern complexity
	if needsMultiLine {
		return e.extractWithMultiLineSupport(filePath, patterns)
	}
	return e.extractWithLineByLine(filePath, patterns)
}

// Check if a pattern likely needs multi-line matching
func (e *VersionExtractor) isMultiLinePattern(pattern string) bool {
	// Patterns that commonly span multiple lines
	//
	// IMPORTANT: Understanding the escaping in the [\s\S] detector:
	// - User patterns come from YAML config files like: '<project>[\s\S]*?<version>'
	// - YAML string parsing converts \s to literal backslash + s (not whitespace escape)
	// - So the Go string contains: [ \ s \ S ] (6 characters with literal backslashes)
	// - To detect this with regex, we need `\[\\s\\S\]` which means:
	//   - \[ = match literal [
	//   - \\s = match literal backslash followed by literal s
	//   - \\S = match literal backslash followed by literal S
	//   - \] = match literal ]
	// - This correctly identifies patterns that use the [\s\S] regex idiom for
	//   matching any character including newlines (whitespace OR non-whitespace)
	//
	// NOTE: Do NOT use `\[\s\S\]` (single backslash before s/S) as that would
	// look for regex escape sequences, not literal backslashes in the string.
	multiLineIndicators := []string{
		`\.package\(.*version`,  // Swift Package Manager dependencies
		`<[^>]*>.*<[^>]*>`,      // XML tags that might span lines
		`\([^)]*version[^)]*\)`, // Function calls with version parameters
		`\{[^}]*version[^}]*\}`, // JSON-like objects with version
		`\[\\s\\S\]`,            // Patterns using [\s\S] for any character including newlines
	}

	for _, indicator := range multiLineIndicators {
		if matched, _ := regexp.MatchString(indicator, pattern); matched {
			return true
		}
	}
	return false
}

// Extract using full file content (for multi-line patterns)
func (e *VersionExtractor) extractWithMultiLineSupport(filePath string, patterns []string) (string, string, error) {
	fileContent, err := fileReader.ReadFileContent(filePath, true)
	if err != nil {
		return "", "", err
	}

	// For multi-line patterns, whitespace and newlines need handling
	// flexibly, so match against a whitespace-collapsed copy as well as the
	// original. Normalise once rather than per pattern.
	normalizedContent := whitespaceRun.ReplaceAllString(fileContent, " ")

	// Try each regex pattern
	for _, pattern := range patterns {
		re, err := getCompiledRegex(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid regex pattern '%s': %v\n", pattern, err)
			continue
		}

		matches := re.FindStringSubmatch(normalizedContent)
		if len(matches) > 1 {
			version := strings.TrimSpace(matches[1])
			if version != "" {
				cleanVersion := e.cleanVersion(version)
				if e.isValidVersion(cleanVersion) {
					return cleanVersion, pattern, nil
				}
			}
		}

		// Also try matching against original content (preserving formatting)
		matches = re.FindStringSubmatch(fileContent)
		if len(matches) > 1 {
			version := strings.TrimSpace(matches[1])
			if version != "" {
				cleanVersion := e.cleanVersion(version)
				if e.isValidVersion(cleanVersion) {
					return cleanVersion, pattern, nil
				}
			}
		}
	}

	return "", "", nil
}

// Extract using line-by-line processing (for simple patterns)
func (e *VersionExtractor) extractWithLineByLine(filePath string, patterns []string) (string, string, error) {
	// Try each regex pattern and return first valid version
	for _, pattern := range patterns {
		re, err := getCompiledRegex(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid regex pattern '%s': %v\n", pattern, err)
			continue
		}

		// Use centralized line processing
		result, err := fileReader.ProcessFileLineByLine(filePath, func(line string) (string, bool) {
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				version := strings.TrimSpace(matches[1])
				if version != "" {
					cleanVersion := e.cleanVersion(version)
					if e.isValidVersion(cleanVersion) {
						return cleanVersion, true
					}
				}
			}
			return "", false
		})

		if err != nil {
			return "", "", err
		}

		if result != "" {
			return result, pattern, nil
		}
	}

	return "", "", nil
}

// cleanVersion removes common prefixes and cleans up version strings
func (e *VersionExtractor) cleanVersion(version string) string {
	// Trim whitespace first
	version = strings.TrimSpace(version)

	version = strings.Trim(version, `"'`)

	prefixes := []string{"version=", "Version=", "VERSION=", "v", "V"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(version, prefix) {
			version = strings.TrimPrefix(version, prefix)
			break
		}
	}

	version = strings.TrimRight(version, ";,")

	// Final trim
	version = strings.TrimSpace(version)

	return version
}

// isValidVersion performs basic validation on version strings
func (e *VersionExtractor) isValidVersion(version string) bool {
	if version == "" {
		return false
	}

	// Validate against official semantic version pattern (from semver.org)
	matched, _ := regexp.MatchString(semverPattern, version)
	if matched {
		return true
	}

	// Validate against Python-style versions (e.g., 3.2.0.dev)
	matched, _ = regexp.MatchString(pythonStylePattern, version)
	if matched {
		return true
	}

	matched, _ = regexp.MatchString(simplePattern, version)
	if matched {
		return true
	}

	// Validate against date-based version pattern (CalVer)
	matched, _ = regexp.MatchString(datePattern, version)
	return matched
}
