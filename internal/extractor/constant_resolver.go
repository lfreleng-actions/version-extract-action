// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxConstantScanFiles bounds the number of Kotlin/Gradle files scanned when
// resolving a version constant, to keep the cost reasonable on large repos.
const maxConstantScanFiles = 600

// errStopWalk is a sentinel used to halt filepath.Walk early once a match is
// found (filepath.Walk has no other early-exit mechanism).
var errStopWalk = errors.New("stop walk")

// resolveVersionConstant handles the common Kotlin/Gradle idiom where the
// version is assigned from a named constant instead of a literal:
//
//	// app/build.gradle.kts
//	versionName = NEWPIPE_VERSION_NAME
//	// buildSrc/src/main/kotlin/ProjectConfig.kt
//	const val NEWPIPE_VERSION_NAME = "0.28.8"
//
// The assignment key it looks for (versionName and/or version) is derived from
// the running project type's own regex patterns, so the resulting project_type
// label stays consistent with the type that matched. It scans refFile for such
// a reference and resolves the constant's literal value by searching
// conventional locations within searchPath (buildSrc, build-logic, the
// referencing file's directory) before falling back to a bounded walk of the
// wider tree. It returns the resolved version and a description of the match.
func (e *VersionExtractor) resolveVersionConstant(refFile, searchPath string,
	patterns []string) (string, string, error) {

	// Only Gradle build scripts use this assignment idiom.
	if !isGradleScript(refFile) {
		return "", "", nil
	}

	keys := versionAssignmentKeys(patterns)
	if len(keys) == 0 {
		return "", "", nil
	}
	refRe, err := constRefPattern(keys)
	if err != nil {
		return "", "", err
	}

	content, err := fileReader.ReadFileContent(refFile, true)
	if err != nil {
		return "", "", err
	}

	refs := refRe.FindAllStringSubmatch(stripCommentLines(content), -1)
	if len(refs) == 0 {
		return "", "", nil
	}

	for _, ref := range refs {
		ident := ref[1]
		value, defFile := e.lookupConstantValue(ident, searchPath,
			filepath.Dir(refFile))
		if value == "" {
			continue
		}
		clean := e.cleanVersion(value)
		if e.isValidVersion(clean) {
			matchedBy := fmt.Sprintf("constant %s defined in %s",
				ident, filepath.Base(defFile))
			return clean, matchedBy, nil
		}
	}

	return "", "", nil
}

// versionAssignmentKeys inspects a project type's regex patterns and returns
// the version assignment key(s) they target — "versionName" (Android) and/or
// "version" (generic Gradle/Kotlin). versionCode and other keys are ignored so
// that, e.g., a Java/Kotlin type (which targets `version`) does not claim an
// Android app whose version lives in `versionName`.
func versionAssignmentKeys(patterns []string) []string {
	var keys []string
	seen := map[string]bool{}
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, p := range patterns {
		switch {
		case strings.Contains(p, "versionName"):
			add("versionName")
		case strings.Contains(p, "versionCode"):
			// integer code, not a resolvable version name
		case strings.Contains(p, "version"):
			add("version")
		}
	}
	return keys
}

// constRefPattern builds (and caches) a regex matching `<key> = IDENTIFIER` for
// the given assignment keys, where the right-hand side is a bare constant
// reference rather than a quoted literal, numeric literal, or function call.
func constRefPattern(keys []string) (*regexp.Regexp, error) {
	alt := strings.Join(keys, "|")
	return getCompiledRegex(
		`(?m)(?:^|[^A-Za-z0-9_])(?:` + alt + `)[ \t]*=[ \t]*` +
			`([A-Za-z_][A-Za-z0-9_]*)[ \t]*(?://.*)?$`)
}

// lookupConstantValue searches the project for a Kotlin constant definition of
// the form `const val IDENT = "value"` (or `val IDENT = "value"`, optionally
// typed) and returns its literal value and the file it was found in.
// Conventional constant locations are searched first; a bounded walk of
// searchPath is the fallback.
func (e *VersionExtractor) lookupConstantValue(ident, searchPath,
	refDir string) (string, string) {

	defRe, err := getCompiledRegex(
		`(?m)(?:^|[^A-Za-z0-9_])(?:const[ \t]+)?val[ \t]+` +
			regexp.QuoteMeta(ident) +
			`[ \t]*(?::[^=\n]+)?=[ \t]*"([^"]+)"`)
	if err != nil {
		return "", ""
	}

	var value, valueFile string
	scanned := 0

	scan := func(root string) {
		info, statErr := os.Stat(root)
		if statErr != nil || !info.IsDir() {
			return
		}
		_ = filepath.Walk(root, func(path string, fi os.FileInfo,
			werr error) error {
			if werr != nil {
				return nil
			}
			if fi.IsDir() {
				if strings.HasPrefix(fi.Name(), ".") {
					return filepath.SkipDir
				}
				for _, skip := range e.skipDirectories {
					if fi.Name() == skip {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if !isKotlinSource(fi.Name()) {
				return nil
			}
			if scanned >= maxConstantScanFiles {
				return errStopWalk
			}
			scanned++
			fileContent, readErr := fileReader.ReadFileContent(path, true)
			if readErr != nil {
				return nil
			}
			if m := defRe.FindStringSubmatch(stripCommentLines(fileContent)); len(m) == 2 {
				value, valueFile = m[1], path
				return errStopWalk
			}
			return nil
		})
	}

	for _, root := range []string{
		filepath.Join(searchPath, "buildSrc"),
		filepath.Join(searchPath, "build-logic"),
		refDir,
		searchPath,
	} {
		scan(root)
		if value != "" || scanned >= maxConstantScanFiles {
			break
		}
	}

	return value, valueFile
}

// isGradleScript reports whether the file is a Gradle build script where the
// constant-reference idiom can appear.
func isGradleScript(name string) bool {
	return strings.HasSuffix(name, ".gradle.kts") ||
		strings.HasSuffix(name, ".gradle") ||
		strings.HasSuffix(name, ".kts")
}

// isKotlinSource reports whether the file may contain a Kotlin/Gradle constant
// definition. Only Kotlin sources are scanned: `const val`/`val` is Kotlin
// syntax, so Groovy `*.gradle` files (which use `def`) would never match and
// would only consume the scan budget.
func isKotlinSource(name string) bool {
	return strings.HasSuffix(name, ".kt") ||
		strings.HasSuffix(name, ".kts")
}

// stripCommentLines removes whole-line comments (lines whose first
// non-whitespace characters are //, /*, or *) before pattern matching, so a
// commented-out example like `// versionName = SOME_CONST` or a doc snippet
// like `// const val FOO = "9.9.9"` is not mistaken for a real assignment or
// definition. Trailing comments after real code (e.g. `versionName = X // c`)
// are unaffected because such lines do not start with a comment marker.
func stripCommentLines(content string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
