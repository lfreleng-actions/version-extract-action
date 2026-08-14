// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Linux Foundation

package extractor

import (
	"fmt"
	"regexp"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

// detectDynamicVersioning checks if a file contains dynamic versioning indicators
func (e *VersionExtractor) detectDynamicVersioning(filePath string, indicators []config.DynamicVersionIndicator) (bool, error) {
	// Read full file content for dynamic versioning detection
	// This requires full content due to complex multi-line patterns and cross-references
	fileContent, err := fileReader.ReadFileContent(filePath, true)
	if err != nil {
		return false, err
	}

	for _, indicator := range indicators {
		matched, err := indicatorMatches(indicator, fileContent)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

// indicatorMatches reports whether a single indicator is satisfied by
// fileContent. An indicator can assert that a section exists, that a field
// contains one of several values, or both; either assertion is enough.
func indicatorMatches(indicator config.DynamicVersionIndicator,
	fileContent string) (bool, error) {
	if indicator.Exists && indicator.Path != "" {
		matched, err := sectionExists(indicator.Path, fileContent)
		if err != nil || matched {
			return matched, err
		}
	}

	if len(indicator.Contains) == 0 || indicator.Field == "" {
		return false, nil
	}

	for _, value := range indicator.Contains {
		for _, pattern := range dynamicFieldPatterns(indicator.Field, value) {
			compiledRegex, err := getCompiledRegex(pattern)
			if err != nil {
				return false, err
			}
			if compiledRegex.MatchString(fileContent) {
				return true, nil
			}
		}
	}

	return false, nil
}

// sectionExists reports whether path appears as a section header on a line of
// its own, such as the TOML table [tool.setuptools_scm].
func sectionExists(path, fileContent string) (bool, error) {
	sectionPattern := fmt.Sprintf(`(?m)^\s*%s\s*$`, regexp.QuoteMeta(path))
	compiledRegex, err := getCompiledRegex(sectionPattern)
	if err != nil {
		return false, err
	}

	return compiledRegex.MatchString(fileContent), nil
}

// dynamicFieldPatterns builds the regular expressions that indicate field is
// bound to value in one of the manifest formats we support. They are returned
// in probe order, from the most specific format-aware shapes to a generic
// same-line fallback.
func dynamicFieldPatterns(field, value string) []string {
	quotedField := regexp.QuoteMeta(field)
	quotedValue := regexp.QuoteMeta(value)

	patterns := []string{
		// TOML array format: dynamic = ["version"]
		fmt.Sprintf(`(?m)%s\s*=\s*\[.*?["']%s["'].*?\]`,
			quotedField, quotedValue),
		// JSON string format: "version": "0.0.0-development"
		fmt.Sprintf(`(?m)["']%s["']\s*:\s*["']%s["']`,
			quotedField, quotedValue),
		// JSON object/array pattern: "scripts": {..."semantic-release"...}
		fmt.Sprintf(`(?m)["']%s["']\s*:\s*\{[^}]*["']%s["']`,
			quotedField, quotedValue),
		// TOML string format: version = "0.0.0"
		fmt.Sprintf(`(?m)%s\s*=\s*["']%s["']`,
			quotedField, quotedValue),
		// Build script reference: build = "build.rs"
		fmt.Sprintf(`(?m)%s\s*=\s*["'][^"']*%s[^"']*["']`,
			quotedField, quotedValue),
		// XML tag format: <version>${revision}</version>
		fmt.Sprintf(`(?m)<%s[^>]*>.*?%s.*?</%s>`,
			quotedField, quotedValue, quotedField),
	}

	// Go module format: module github.com/...
	if field == "module" {
		patterns = append(patterns, fmt.Sprintf(`(?m)%s\s+[^\s]*%s[^\s]*`,
			quotedField, quotedValue))
	}

	// Generic pattern for SBT and other formats
	// Matches lines where the field name and value appear on the same line
	// Example: ThisBuild / version := dynverGitDescribeOutput.value
	// Note: Field requires word boundary, but value doesn't (can be part of identifier)
	return append(patterns, fmt.Sprintf(`(?m).*\b%s\b.*%s.*`,
		quotedField, quotedValue))
}
