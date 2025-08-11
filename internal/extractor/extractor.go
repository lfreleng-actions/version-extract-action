// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
	"github.com/lfreleng-actions/version-extract-action/internal/git"
)

// Version validation patterns
const (
	// Basic semantic version pattern (max 4 components)
	semverPattern = `^v?[0-9]+(\.[0-9]+){0,3}(\-[0-9A-Za-z\-\.]+)?(\+[0-9A-Za-z\-\.]+)?$`
	// Simple version patterns (numbers and dots, max 4 components)
	simplePattern = `^[0-9]+(\.[0-9]+){0,3}$`
	// Date-based versions
	datePattern = `^[0-9]{4}(\.[0-9]{2})*$`
)

// File processing limits
const (
	// Maximum file size to process (10MB) to prevent memory exhaustion
	maxFileSizeLimit = 10 * 1024 * 1024
)

// defaultSkipDirectories defines common directories to skip during file search
// This is a package-level constant to prevent accidental modification
var defaultSkipDirectories = []string{"node_modules", "vendor", "target", "build", "dist"}

// Package-level regex cache to persist across multiple file operations
var (
	regexCache = make(map[string]*regexp.Regexp)
	cacheMutex sync.RWMutex
)

// getCompiledRegex gets or compiles a regex pattern with thread-safe caching
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	// Try to read from cache first
	cacheMutex.RLock()
	if compiledRegex, exists := regexCache[pattern]; exists {
		cacheMutex.RUnlock()
		return compiledRegex, nil
	}
	cacheMutex.RUnlock()

	// Compile the regex
	compiledRegex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex pattern '%s': %w", pattern, err)
	}

	// Store in cache
	cacheMutex.Lock()
	regexCache[pattern] = compiledRegex
	cacheMutex.Unlock()

	return compiledRegex, nil
}

// ExtractResult represents the result of version extraction
type ExtractResult struct {
	Version       string `json:"version"`
	ProjectType   string `json:"project_type"`
	Subtype       string `json:"subtype,omitempty"`
	File          string `json:"file"`
	MatchedBy     string `json:"matched_by"`
	Success       bool   `json:"success"`
	VersionSource string `json:"version_source,omitempty"` // "static" or "dynamic-git-tag"
	GitTag        string `json:"git_tag,omitempty"`        // Original git tag if dynamic
}

// VersionExtractor handles version extraction from project files
type VersionExtractor struct {
	config          *config.Config
	dynamicFallback bool
	skipDirectories []string
}

// New creates a new VersionExtractor instance
func New(cfg *config.Config) *VersionExtractor {
	return &VersionExtractor{
		config:          cfg,
		dynamicFallback: true,
		skipDirectories: defaultSkipDirectories,
	}
}

// NewWithOptions creates a new VersionExtractor instance with options
func NewWithOptions(cfg *config.Config, dynamicFallback bool) *VersionExtractor {
	return &VersionExtractor{
		config:          cfg,
		dynamicFallback: dynamicFallback,
		skipDirectories: defaultSkipDirectories,
	}
}

// Extract attempts to extract version from the given directory or file path
func (e *VersionExtractor) Extract(path string) (*ExtractResult, error) {
	// Validate path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", path)
	}

	// Check if this is a file or directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !fileInfo.IsDir() {
		// Handle specific file path
		return e.extractFromSpecificFile(path)
	}

	// Handle directory path (existing behavior)
	return e.extractFromDirectory(path)
}

// extractFromSpecificFile handles extraction from a specific file
func (e *VersionExtractor) extractFromSpecificFile(filePath string) (*ExtractResult, error) {
	fileName := filepath.Base(filePath)

	// Find matching project configuration for this file
	var matchingProject *config.ProjectConfig
	for _, project := range e.config.Projects {
		if e.fileMatchesPattern(fileName, project.File) {
			matchingProject = &project
			break
		}
	}

	if matchingProject == nil {
		return &ExtractResult{
			Success: false,
		}, fmt.Errorf("file '%s' is of an unsupported type", fileName)
	}

	// Try to extract version from the specific file
	version, matchedRegex, err := e.extractVersionFromFile(filePath, matchingProject.Regex)
	if err != nil {
		return &ExtractResult{
			Success: false,
		}, fmt.Errorf("error processing file %s: %w", filePath, err)
	}

	// If we found a version, use it (already cleaned and validated by extractVersionFromFile)
	if version != "" {
		return &ExtractResult{
			Version:     version,
			ProjectType: matchingProject.Type,
			Subtype:     matchingProject.Subtype,
			File:        filePath,
			MatchedBy:   matchedRegex,
			Success:     true,
		}, nil
	}

	return &ExtractResult{
		Success: false,
	}, fmt.Errorf("no valid version found in file: %s", filePath)
}

// extractFromDirectory handles extraction from a directory (existing behavior)
func (e *VersionExtractor) extractFromDirectory(searchPath string) (*ExtractResult, error) {
	// Try each project configuration in priority order
	for _, project := range e.config.Projects {
		result, err := e.tryExtractFromProject(searchPath, project)
		if err != nil {
			// Log error but continue to next project type
			fmt.Fprintf(os.Stderr, "Warning: Failed to extract from %s: %v\n",
				project.Type, err)
			continue
		}

		if result.Success {
			return result, nil
		}
	}

	return &ExtractResult{
		Success: false,
	}, fmt.Errorf("no version found in any supported project files")
}

// tryExtractFromProject attempts version extraction for a specific project
// type
func (e *VersionExtractor) tryExtractFromProject(searchPath string,
	project config.ProjectConfig) (*ExtractResult, error) {

	// Find matching files
	files, err := e.findProjectFiles(searchPath, project.File)
	if err != nil {
		return nil, fmt.Errorf("failed to find project files: %w", err)
	}

	if len(files) == 0 {
		return &ExtractResult{Success: false}, nil
	}

	// Try to extract version from each found file
	for _, file := range files {
		version, matchedRegex, err := e.extractVersionFromFile(file,
			project.Regex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error processing %s: %v\n", file, err)
			continue
		}

		// Check for dynamic versioning first if project supports it
		if e.dynamicFallback && project.SupportsDynamicVersioning && len(project.DynamicVersionIndicators) > 0 {
			if isDynamic, err := e.detectDynamicVersioning(file, project.DynamicVersionIndicators); err == nil && isDynamic {
				// Attempt Git fallback
				if gitResult := e.tryGitFallback(searchPath); gitResult != nil && gitResult.Success {
					return &ExtractResult{
						Version:       gitResult.Version,
						ProjectType:   project.Type,
						Subtype:       project.Subtype,
						File:          file,
						MatchedBy:     "dynamic-git-tag",
						Success:       true,
						VersionSource: "dynamic-git-tag",
						GitTag:        gitResult.Tag,
					}, nil
				}
			}
		}

		// If no dynamic versioning detected and we found a version, use it as static
		if version != "" {
			// Version is already cleaned and validated by extractVersionFromFile
			return &ExtractResult{
				Version:       version,
				ProjectType:   project.Type,
				Subtype:       project.Subtype,
				File:          file,
				MatchedBy:     matchedRegex,
				Success:       true,
				VersionSource: "static",
			}, nil
		}
	}

	return &ExtractResult{Success: false}, nil
}

// findProjectFiles searches for files matching the given pattern
func (e *VersionExtractor) findProjectFiles(searchPath,
	pattern string) ([]string, error) {

	var matchingFiles []string

	// Handle glob patterns
	if strings.Contains(pattern, "*") {
		matches, err := filepath.Glob(filepath.Join(searchPath, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob pattern error: %w", err)
		}
		matchingFiles = append(matchingFiles, matches...)
	} else {
		// Direct file path
		filePath := filepath.Join(searchPath, pattern)
		if _, err := os.Stat(filePath); err == nil {
			matchingFiles = append(matchingFiles, filePath)
		}
	}

	// Also search in subdirectories for common locations
	err := filepath.Walk(searchPath, func(path string,
		info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		// Skip hidden directories and common build/cache directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			for _, skipDir := range e.skipDirectories {
				if info.Name() == skipDir {
					return filepath.SkipDir
				}
			}
		}

		// Check if file matches pattern
		if !info.IsDir() {
			if strings.Contains(pattern, "*") {
				matched, _ := filepath.Match(pattern, info.Name())
				if matched {
					matchingFiles = append(matchingFiles, path)
				}
			} else if info.Name() == pattern {
				matchingFiles = append(matchingFiles, path)
			}
		}

		return nil
	})

	if err != nil {
		return matchingFiles, fmt.Errorf("error walking directory: %w", err)
	}

	return e.removeDuplicates(matchingFiles), nil
}

// extractVersionFromFile attempts to extract version using regex patterns
func (e *VersionExtractor) extractVersionFromFile(filePath string,
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
	multiLineIndicators := []string{
		`\.package\(.*version`,  // Swift Package Manager dependencies
		`<[^>]*>.*<[^>]*>`,      // XML tags that might span lines
		`\([^)]*version[^)]*\)`, // Function calls with version parameters
		`\{[^}]*version[^}]*\}`, // JSON-like objects with version
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

	// Try each regex pattern
	for _, pattern := range patterns {
		re, err := getCompiledRegex(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid regex pattern '%s': %v\n", pattern, err)
			continue
		}

		// For multi-line patterns, we need to handle whitespace and newlines flexibly
		// Remove excessive whitespace and newlines to improve matching
		normalizedContent := regexp.MustCompile(`\s+`).ReplaceAllString(fileContent, " ")

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

	// Remove quotes
	version = strings.Trim(version, `"'`)

	// Remove common prefixes
	prefixes := []string{"version=", "Version=", "VERSION=", "v", "V"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(version, prefix) {
			version = strings.TrimPrefix(version, prefix)
			break
		}
	}

	// Remove trailing semicolons or commas
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

	// Validate against semantic version pattern
	matched, _ := regexp.MatchString(semverPattern, version)
	if matched {
		return true
	}

	// Validate against simple version pattern
	matched, _ = regexp.MatchString(simplePattern, version)
	if matched {
		return true
	}

	// Validate against date-based version pattern
	matched, _ = regexp.MatchString(datePattern, version)
	return matched
}

// fileMatchesPattern checks if a filename matches a project file pattern
func (e *VersionExtractor) fileMatchesPattern(fileName, pattern string) bool {
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, fileName)
		return matched
	}
	return fileName == pattern
}

// removeDuplicates removes duplicate file paths
func (e *VersionExtractor) removeDuplicates(files []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, file := range files {
		if !seen[file] {
			seen[file] = true
			result = append(result, file)
		}
	}

	return result
}

// GetSupportedTypes returns list of supported project types from config
func (e *VersionExtractor) GetSupportedTypes() []string {
	return e.config.GetSupportedTypes()
}

// SetSkipDirectories allows customization of directories to skip during file search
func (e *VersionExtractor) SetSkipDirectories(dirs []string) {
	e.skipDirectories = dirs
}

// GetSkipDirectories returns the current list of directories to skip
func (e *VersionExtractor) GetSkipDirectories() []string {
	return e.skipDirectories
}

// detectDynamicVersioning checks if a file contains dynamic versioning indicators
func (e *VersionExtractor) detectDynamicVersioning(filePath string, indicators []config.DynamicVersionIndicator) (bool, error) {
	// Read full file content for dynamic versioning detection
	// This requires full content due to complex multi-line patterns and cross-references
	fileContent, err := fileReader.ReadFileContent(filePath, true)
	if err != nil {
		return false, err
	}

	for _, indicator := range indicators {
		if indicator.Exists {
			// Check if a section or field exists
			if indicator.Path != "" {
				// Look for TOML section like [tool.setuptools_scm]
				sectionPattern := fmt.Sprintf(`(?m)^\s*%s\s*$`, regexp.QuoteMeta(indicator.Path))
				compiledRegex, err := getCompiledRegex(sectionPattern)
				if err != nil {
					return false, err
				}
				if compiledRegex.MatchString(fileContent) {
					return true, nil
				}
			}
		}

		if len(indicator.Contains) > 0 && indicator.Field != "" {
			// Check if field contains specific values
			for _, value := range indicator.Contains {
				// Pattern 1: TOML array format: dynamic = ["version"]
				tomlArrayPattern := fmt.Sprintf(`(?m)%s\s*=\s*\[.*?["']%s["'].*?\]`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
				if compiledRegex, err := getCompiledRegex(tomlArrayPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 2: JSON string format: "version": "0.0.0-development"
				jsonStringPattern := fmt.Sprintf(`(?m)["']%s["']\s*:\s*["']%s["']`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
				if compiledRegex, err := getCompiledRegex(jsonStringPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 3: JSON object/array pattern: "scripts": {..."semantic-release"...}
				jsonObjectPattern := fmt.Sprintf(`(?m)["']%s["']\s*:\s*\{[^}]*["']%s["']`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
				if compiledRegex, err := getCompiledRegex(jsonObjectPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 4: TOML string format: version = "0.0.0"
				tomlStringPattern := fmt.Sprintf(`(?m)%s\s*=\s*["']%s["']`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
				if compiledRegex, err := getCompiledRegex(tomlStringPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 5: Build script reference: build = "build.rs"
				buildPattern := fmt.Sprintf(`(?m)%s\s*=\s*["'][^"']*%s[^"']*["']`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
				if compiledRegex, err := getCompiledRegex(buildPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 6: XML tag format: <version>${revision}</version>
				xmlTagPattern := fmt.Sprintf(`(?m)<%s[^>]*>.*?%s.*?</%s>`,
					regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value), regexp.QuoteMeta(indicator.Field))
				if compiledRegex, err := getCompiledRegex(xmlTagPattern); err != nil {
					return false, err
				} else if compiledRegex.MatchString(fileContent) {
					return true, nil
				}

				// Pattern 7: Go module format: module github.com/...
				if indicator.Field == "module" {
					modulePattern := fmt.Sprintf(`(?m)%s\s+[^\s]*%s[^\s]*`,
						regexp.QuoteMeta(indicator.Field), regexp.QuoteMeta(value))
					if compiledRegex, err := getCompiledRegex(modulePattern); err != nil {
						return false, err
					} else if compiledRegex.MatchString(fileContent) {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// tryGitFallback attempts to extract version from Git tags
func (e *VersionExtractor) tryGitFallback(searchPath string) *git.GitTagResult {
	gitExtractor := git.New(searchPath)

	// Try to fetch tags first (useful in CI environments)
	// Don't treat fetch failures as fatal
	gitExtractor.FetchTags()

	// Get the latest version tag
	result, err := gitExtractor.GetLatestVersionTag()
	if err != nil {
		return &git.GitTagResult{
			Success:   false,
			IsGitRepo: gitExtractor.IsGitRepository(),
		}
	}

	return result
}
