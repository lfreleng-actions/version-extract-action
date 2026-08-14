// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
	"github.com/lfreleng-actions/version-extract-action/internal/git"
)

// File processing limits
const (
	// Maximum file size to process (10MB) to prevent memory exhaustion
	maxFileSizeLimit = 10 * 1024 * 1024
	// Maximum number of __version__.py files to check in fallback search
	maxVersionFilesToCheck = 10
)

// defaultSkipDirectories defines common directories to skip during file search
// This is a package-level constant to prevent accidental modification
var defaultSkipDirectories = []string{"node_modules", "vendor", "target", "build", "dist"}

// ExtractResult represents the result of version extraction
type ExtractResult struct {
	Version       string `json:"version"`
	ProjectType   string `json:"project_type"`
	Subtype       string `json:"subtype,omitempty"`
	File          string `json:"file"`
	MatchedBy     string `json:"matched_by"`
	Success       bool   `json:"success"`
	VersionSource string `json:"version_source,omitempty"` // "static", "static-constant", or "dynamic-git-tag"
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", path)
	}

	// Check if this is a file or directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !fileInfo.IsDir() {
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
			Version:       version,
			ProjectType:   matchingProject.Type,
			Subtype:       matchingProject.Subtype,
			File:          filePath,
			MatchedBy:     matchedRegex,
			Success:       true,
			VersionSource: "static",
		}, nil
	}

	// Fallback: the version may be assigned from a named Kotlin/Gradle constant
	// (e.g. `versionName = NEWPIPE_VERSION_NAME`). Resolve it relative to the
	// enclosing project root so buildSrc/build-logic definitions are found.
	root := e.projectRootForFile(filePath)
	if cv, matchedBy, cerr := e.resolveVersionConstant(filePath, root,
		matchingProject.Regex); cerr == nil && cv != "" {
		return &ExtractResult{
			Version:       cv,
			ProjectType:   matchingProject.Type,
			Subtype:       matchingProject.Subtype,
			File:          filePath,
			MatchedBy:     matchedBy,
			Success:       true,
			VersionSource: "static-constant",
		}, nil
	}

	return &ExtractResult{
		Success: false,
	}, fmt.Errorf("no valid version found in file: %s", filePath)
}

// projectRootForFile walks up from a file to find the enclosing project root,
// identified by a Gradle/VCS marker (settings.gradle[.kts], buildSrc,
// build-logic, or .git). The walk is bounded to 8 parent directories; if no
// marker is found within that limit (or at all) it falls back to the file's
// own directory, so a build script nested deeper than 8 levels may not
// resolve its buildSrc constants. This lets constant resolution locate
// buildSrc definitions when a specific build script (rather than a directory)
// is passed to Extract.
func (e *VersionExtractor) projectRootForFile(filePath string) string {
	dir := filepath.Dir(filePath)
	current := dir
	markers := []string{
		"settings.gradle", "settings.gradle.kts", "buildSrc",
		"build-logic", ".git",
	}
	for i := 0; i < 8; i++ {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return dir
}

// extractFromDirectory handles extraction from a directory (existing behavior)
func (e *VersionExtractor) extractFromDirectory(searchPath string) (*ExtractResult, error) {
	// Index the tree once, then match every project type against it, instead
	// of walking the whole repository separately for each project type.
	idx := e.buildFileIndex(searchPath)

	// Try each project configuration in priority order
	for _, project := range e.config.Projects {
		result, err := e.tryExtractFromProject(searchPath, project, idx)
		if err != nil {
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
	project config.ProjectConfig, idx *fileIndex) (*ExtractResult, error) {

	// Skip projects with empty regex patterns - they should use git tags
	if len(project.Regex) == 0 {
		// Early return if dynamic fallback is not enabled or project doesn't support it
		// This avoids unnecessary file system operations
		if !e.dynamicFallback || !project.SupportsDynamicVersioning {
			return &ExtractResult{Success: false}, nil
		}

		// Check if the project file exists (e.g., go.mod for Go projects)
		files := idx.match(project.File)
		if len(files) == 0 {
			return &ExtractResult{Success: false}, nil
		}

		// File exists but no regex patterns - use git fallback for version
		gitResult := e.tryGitFallback(searchPath)
		if gitResult == nil || !gitResult.Success {
			return &ExtractResult{Success: false}, nil
		}

		return &ExtractResult{
			Version:       gitResult.Version,
			ProjectType:   project.Type,
			Subtype:       project.Subtype,
			File:          files[0],
			MatchedBy:     "git-fallback",
			Success:       true,
			VersionSource: "dynamic-git-tag",
			GitTag:        gitResult.Tag,
		}, nil
	}

	// Find matching files
	files := idx.match(project.File)
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

		// Fallback: the version may be assigned from a named Kotlin/Gradle
		// constant (e.g. `versionName = NEWPIPE_VERSION_NAME`) rather than a
		// literal. Resolve it from buildSrc and similar locations.
		if cv, matchedBy, cerr := e.resolveVersionConstant(file,
			searchPath, project.Regex); cerr == nil && cv != "" {
			return &ExtractResult{
				Version:       cv,
				ProjectType:   project.Type,
				Subtype:       project.Subtype,
				File:          file,
				MatchedBy:     matchedBy,
				Success:       true,
				VersionSource: "static-constant",
			}, nil
		}
	}

	return &ExtractResult{Success: false}, nil
}

// findProjectFiles returns files matching the given pattern beneath searchPath.
// It builds a one-off fileIndex, so callers that match many patterns over the
// same tree should build a fileIndex once and call match directly (as
// extractFromDirectory does) rather than calling this per pattern.
func (e *VersionExtractor) findProjectFiles(searchPath,
	pattern string) ([]string, error) {
	return e.buildFileIndex(searchPath).match(pattern), nil
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

// tryGitFallback attempts to extract version from Git tags
func (e *VersionExtractor) tryGitFallback(searchPath string) *git.GitTagResult {
	gitExtractor := git.New(searchPath)

	// Get the latest version tag. Local tags are tried first; if none are
	// present (e.g. a shallow clone) the lookup falls back to `git ls-remote`,
	// which is far cheaper than fetching tag objects over the network.
	result, err := gitExtractor.GetLatestVersionTag()
	if err != nil {
		return &git.GitTagResult{
			Success:   false,
			IsGitRepo: gitExtractor.IsGitRepository(),
		}
	}

	return result
}
