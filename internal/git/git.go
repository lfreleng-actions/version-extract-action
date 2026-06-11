// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package git

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

// Timeouts bound git subprocess calls so version extraction can never hang on
// a slow or pathological repository. Local commands are quick; the remote
// lookup (ls-remote) contacts origin for refs only (no object download).
const (
	gitLocalTimeout  = 15 * time.Second
	gitRemoteTimeout = 45 * time.Second
)

// runGit runs a git command in the working directory, bounded by a timeout,
// and returns its standard output. On failure it surfaces the git arguments,
// the captured stderr, and distinguishes timeouts, so callers (and logs) get
// actionable diagnostics instead of a bare "exit status 128".
func (g *GitVersionExtractor) runGit(timeout time.Duration,
	args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workingDir
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("git %s timed out after %s",
			strings.Join(args, " "), timeout)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return out, fmt.Errorf("git %s: %w: %s",
				strings.Join(args, " "), err, stderr)
		}
	}
	return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// IsGitRepository checks if the working directory is a Git repository
func (g *GitVersionExtractor) IsGitRepository() bool {
	// Check if .git directory exists
	gitDir := filepath.Join(g.workingDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}

	// Alternative: try git rev-parse command
	_, err := g.runGit(gitLocalTimeout, "rev-parse", "--git-dir")
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

	// Strategy 6: no usable local tags (e.g. a shallow clone) — list the
	// remote's tags with ls-remote, which returns ref names only without
	// downloading objects. This avoids the very slow `git fetch --tags` on
	// large repositories.
	if version, tag, err := g.getTagFromRemote(); err == nil && version != "" {
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

	output, err := g.runGit(gitLocalTimeout, args...)
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
	output, err := g.runGit(gitLocalTimeout, "tag", "--list",
		"--sort=-version:refname")
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

// getTagFromRemote lists the origin's tags via `git ls-remote` (ref names
// only, no object download) and returns the highest-sorted valid version tag.
// This is dramatically cheaper than `git fetch --tags` on large repositories,
// where fetching tag objects onto a shallow clone can take many minutes.
func (g *GitVersionExtractor) getTagFromRemote() (string, string, error) {
	output, err := g.runGit(gitRemoteTimeout, "ls-remote", "--tags",
		"--sort=-version:refname", "origin")
	if err != nil {
		return "", "", err
	}

	const marker = "refs/tags/"
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		// Strip the peeled-tag suffix ls-remote emits for annotated tags.
		tag := strings.TrimSuffix(strings.TrimSpace(line[i+len(marker):]), "^{}")
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true

		version := g.cleanVersionFromTag(tag)
		if g.isValidVersionTag(version) {
			return version, tag, nil
		}
	}

	return "", "", fmt.Errorf("no valid version tags found on remote")
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

	// Try to fetch tags quietly, bounded by a timeout. The default extraction
	// path now prefers getTagFromRemote (ls-remote), which is far cheaper than
	// fetching tag objects on large repositories.
	_, err := g.runGit(gitRemoteTimeout, "fetch", "--tags", "--quiet")

	// Don't treat fetch failures as fatal - repository might be offline
	// or user might not have network access
	return err
}
