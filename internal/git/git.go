// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package git

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Version tag regex patterns for Git tag validation
const (
	// Semantic version pattern (x.y.z with optional pre-release/build metadata)
	semanticVersionPattern = `^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z\-\.]+)?$`

	// Simple version pattern with optional third component and metadata
	simpleVersionPattern = `^[0-9]+\.[0-9]+(?:\.[0-9]+)?(?:[-+][0-9A-Za-z\-\.]+)?$`

	// Date-based version pattern (YYYY.MM.DD format)
	dateVersionPattern = `^[0-9]{4}\.[0-9]{2}(?:\.[0-9]{2})?$`

	// Maven SNAPSHOT version pattern
	snapshotVersionPattern = `^[0-9]+\.[0-9]+(?:\.[0-9]+)?-SNAPSHOT$`

	// Beta version pattern
	betaVersionPattern = `^[0-9]+\.[0-9]+(?:\.[0-9]+)?-beta\.[0-9]+$`

	// Alpha version pattern
	alphaVersionPattern = `^[0-9]+\.[0-9]+(?:\.[0-9]+)?-alpha\.[0-9]+$`

	// Release candidate version pattern
	rcVersionPattern = `^[0-9]+\.[0-9]+(?:\.[0-9]+)?-rc\.[0-9]+$`

	// Fallback pattern for basic semantic versions
	fallbackVersionPattern = `^[0-9]+\.[0-9]+\.[0-9]+$`
)

// versionTagPatterns holds pre-compiled regex patterns for version validation
var versionTagPatterns []*regexp.Regexp

// init initializes the version tag patterns with proper error handling
func init() {
	patterns := []string{
		semanticVersionPattern,
		simpleVersionPattern,
		dateVersionPattern,
		snapshotVersionPattern,
		betaVersionPattern,
		alphaVersionPattern,
		rcVersionPattern,
	}

	versionTagPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("warning: failed to compile version regex pattern '%s': %v", pattern, err)
			continue
		}
		versionTagPatterns = append(versionTagPatterns, re)
	}

	// Ensure we have at least one working pattern
	if len(versionTagPatterns) == 0 {
		log.Printf("error: no version regex patterns compiled successfully, using fallback pattern")
		// Use the fallback pattern constant
		if re, err := regexp.Compile(fallbackVersionPattern); err == nil {
			versionTagPatterns = append(versionTagPatterns, re)
		} else {
			panic("critical error: even fallback regex pattern failed to compile")
		}
	}
}

// GitTagResult represents the result of Git tag extraction
type GitTagResult struct {
	Version   string `json:"version"`
	Tag       string `json:"tag"`
	Success   bool   `json:"success"`
	IsGitRepo bool   `json:"is_git_repo"`
}

// GitVersionExtractor handles Git-based version extraction
type GitVersionExtractor struct {
	workingDir string
}

// New creates a new GitVersionExtractor
func New(workingDir string) *GitVersionExtractor {
	return &GitVersionExtractor{
		workingDir: workingDir,
	}
}

// IsGitRepository checks if the working directory is a Git repository
func (g *GitVersionExtractor) IsGitRepository() bool {
	// Check if .git directory exists
	gitDir := filepath.Join(g.workingDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}

	// Alternative: try git rev-parse command
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = g.workingDir
	err := cmd.Run()
	return err == nil
}

// GetLatestVersionTag extracts the latest version tag from Git
func (g *GitVersionExtractor) GetLatestVersionTag() (*GitTagResult, error) {
	result := &GitTagResult{
		IsGitRepo: g.IsGitRepository(),
	}

	if !result.IsGitRepo {
		return result, fmt.Errorf("not a git repository: %s", g.workingDir)
	}

	// Try different strategies to get version tags
	version, tag, err := g.tryGetLatestTag()
	if err != nil {
		return result, fmt.Errorf("failed to get git tags: %w", err)
	}

	if version == "" {
		return result, fmt.Errorf("no version tags found in repository")
	}

	result.Version = version
	result.Tag = tag
	result.Success = true

	return result, nil
}

// tryGetLatestTag attempts multiple strategies to get the latest version tag
func (g *GitVersionExtractor) tryGetLatestTag() (string, string, error) {
	// Strategy 1: git describe --tags --abbrev=0 --match="v*" (semantic versioning)
	if version, tag, err := g.getTagWithDescribe("v*"); err == nil && version != "" {
		return version, tag, nil
	}

	// Strategy 2: git describe --tags --abbrev=0 --match="*.*.*" (version patterns)
	if version, tag, err := g.getTagWithDescribe("*.*.*"); err == nil && version != "" {
		return version, tag, nil
	}

	// Strategy 3: git describe --tags --abbrev=0 --match="release-*" (release prefixes)
	if version, tag, err := g.getTagWithDescribe("release-*"); err == nil && version != "" {
		return version, tag, nil
	}

	// Strategy 4: git describe --tags --abbrev=0 (any tag)
	if version, tag, err := g.getTagWithDescribe(""); err == nil && version != "" {
		return version, tag, nil
	}

	// Strategy 5: git tag --list --sort=-version:refname
	if version, tag, err := g.getTagWithList(); err == nil && version != "" {
		return version, tag, nil
	}

	return "", "", fmt.Errorf("no tags found with any strategy")
}

// getTagWithDescribe uses git describe to get the latest tag
func (g *GitVersionExtractor) getTagWithDescribe(matchPattern string) (string, string, error) {
	args := []string{"describe", "--tags", "--abbrev=0"}
	if matchPattern != "" {
		args = append(args, fmt.Sprintf("--match=%s", matchPattern))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	tag := strings.TrimSpace(string(output))
	if tag == "" {
		return "", "", fmt.Errorf("empty tag output")
	}

	version := g.cleanVersionFromTag(tag)
	return version, tag, nil
}

// getTagWithList uses git tag --list with sorting to get the latest tag
func (g *GitVersionExtractor) getTagWithList() (string, string, error) {
	cmd := exec.Command("git", "tag", "--list", "--sort=-version:refname")
	cmd.Dir = g.workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", "", fmt.Errorf("no tags found")
	}

	// Find the first tag that looks like a version
	for _, tag := range lines {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		version := g.cleanVersionFromTag(tag)
		if g.isValidVersionTag(version) {
			return version, tag, nil
		}
	}

	return "", "", fmt.Errorf("no valid version tags found")
}

// cleanVersionFromTag extracts version from a git tag
func (g *GitVersionExtractor) cleanVersionFromTag(tag string) string {
	// Remove common prefixes
	version := strings.TrimSpace(tag)

	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	// Remove common prefixes
	prefixes := []string{"release-", "rel-", "release/", "rel/", "version-", "ver-", "v-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(version), strings.ToLower(prefix)) {
			version = version[len(prefix):]
			break
		}
	}

	return strings.TrimSpace(version)
}

// isValidVersionTag checks if a tag represents a valid version
func (g *GitVersionExtractor) isValidVersionTag(version string) bool {
	if version == "" {
		return false
	}

	// Use pre-compiled regex patterns
	for _, re := range versionTagPatterns {
		if re.MatchString(version) {
			return true
		}
	}

	return false
}

// FetchTags attempts to fetch remote tags (useful in CI environments)
func (g *GitVersionExtractor) FetchTags() error {
	if !g.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Try to fetch tags quietly
	cmd := exec.Command("git", "fetch", "--tags", "--quiet")
	cmd.Dir = g.workingDir
	err := cmd.Run()

	// Don't treat fetch failures as fatal - repository might be offline
	// or user might not have network access
	return err
}
