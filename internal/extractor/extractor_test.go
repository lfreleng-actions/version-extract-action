// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:    "JavaScript",
				File:    "package.json",
				Regex:   []string{`"version":\s*"([^"]+)"`},
				Samples: []string{"https://github.com/test/repo"},
			},
		},
	}

	extractor := New(cfg)
	if extractor == nil {
		t.Fatal("Expected non-nil extractor")
	}
	if extractor.config != cfg {
		t.Error("Expected config to be set correctly")
	}
}

func TestExtractFromPackageJSON(t *testing.T) {
	// Create test directory and file
	tmpDir := t.TempDir()
	packageJSON := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test-project",
  "version": "1.2.3",
  "description": "Test package"
}`

	err := os.WriteFile(packageJSON, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "JavaScript",
				Subtype:  "npm",
				File:     "package.json",
				Regex:    []string{`"version":\s*"([^"]+)"`},
				Samples:  []string{"https://github.com/test/repo"},
				Priority: 1,
			},
		},
	}

	// Test extraction
	extractor := New(cfg)
	result, err := extractor.Extract(tmpDir)

	if err != nil {
		t.Fatalf("Expected successful extraction, got error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	if result.Version != "1.2.3" {
		t.Errorf("Expected version 1.2.3, got %s", result.Version)
	}

	if result.ProjectType != "JavaScript" {
		t.Errorf("Expected JavaScript, got %s", result.ProjectType)
	}

	if result.Subtype != "npm" {
		t.Errorf("Expected npm subtype, got %s", result.Subtype)
	}
}

func TestExtractFromPyprojectToml(t *testing.T) {
	// Create test directory and file
	tmpDir := t.TempDir()
	pyprojectFile := filepath.Join(tmpDir, "pyproject.toml")

	content := `[build-system]
requires = ["setuptools", "wheel"]

[project]
name = "test-project"
version = "2.1.0"
description = "Test Python project"`

	err := os.WriteFile(pyprojectFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "Python",
				Subtype:  "Modern",
				File:     "pyproject.toml",
				Regex:    []string{`version\s*=\s*["']([^"']+)["']`},
				Samples:  []string{"https://github.com/test/repo"},
				Priority: 1,
			},
		},
	}

	// Test extraction
	extractor := New(cfg)
	result, err := extractor.Extract(tmpDir)

	if err != nil {
		t.Fatalf("Expected successful extraction, got error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	if result.Version != "2.1.0" {
		t.Errorf("Expected version 2.1.0, got %s", result.Version)
	}

	if result.ProjectType != "Python" {
		t.Errorf("Expected Python, got %s", result.ProjectType)
	}
}

func TestExtractFromGoMod(t *testing.T) {
	// Create test directory and file
	tmpDir := t.TempDir()
	goModFile := filepath.Join(tmpDir, "go.mod")

	content := `module github.com/test/project

go 1.24

require (
    github.com/spf13/cobra v1.9.1
)`

	err := os.WriteFile(goModFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "Go",
				File:     "go.mod",
				Regex:    []string{`go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)`},
				Samples:  []string{"https://github.com/test/repo"},
				Priority: 1,
			},
		},
	}

	// Test extraction
	extractor := New(cfg)
	result, err := extractor.Extract(tmpDir)

	if err != nil {
		t.Fatalf("Expected successful extraction, got error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	if result.Version != "1.24" {
		t.Errorf("Expected version 1.24, got %s", result.Version)
	}
}

func TestExtractNoMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "JavaScript",
				File:     "package.json",
				Regex:    []string{`"version":\s*"([^"]+)"`},
				Samples:  []string{"https://github.com/test/repo"},
				Priority: 1,
			},
		},
	}

	extractor := New(cfg)
	result, err := extractor.Extract(tmpDir)

	if err == nil {
		t.Error("Expected error for no matching files")
	}

	if result.Success {
		t.Error("Expected unsuccessful result")
	}
}

func TestExtractNonExistentPath(t *testing.T) {
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:    "JavaScript",
				File:    "package.json",
				Regex:   []string{`"version":\s*"([^"]+)"`},
				Samples: []string{"https://github.com/test/repo"},
			},
		},
	}

	extractor := New(cfg)
	_, err := extractor.Extract("/nonexistent/path")

	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestCleanVersion(t *testing.T) {
	extractor := &VersionExtractor{}

	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"V1.2.3", "1.2.3"},
		{`"1.2.3"`, "1.2.3"},
		{"'1.2.3'", "1.2.3"},
		{"version=1.2.3", "1.2.3"},
		{"1.2.3;", "1.2.3"},
		{"1.2.3,", "1.2.3"},
		{"  1.2.3  ", "1.2.3"},
		{`"v1.2.3-alpha"`, "1.2.3-alpha"},
	}

	for _, test := range tests {
		result := extractor.cleanVersion(test.input)
		if result != test.expected {
			t.Errorf("cleanVersion(%s) = %s, expected %s",
				test.input, result, test.expected)
		}
	}
}

func TestIsValidVersion(t *testing.T) {
	extractor := &VersionExtractor{}

	validVersions := []string{
		"1.2.3",
		"1.0.0",
		"10.20.30",
		"1.2.3-alpha",
		"1.2.3-beta.1",
		"1.2.3+build.1",
		"v1.2.3",
		"2021.12",
		"2021.12.01",
	}

	invalidVersions := []string{
		"",
		"not-a-version",
		"1.2.3.4.5",
		"abc",
		"1.2.3..4",
	}

	for _, version := range validVersions {
		if !extractor.isValidVersion(version) {
			t.Errorf("Expected %s to be valid", version)
		}
	}

	for _, version := range invalidVersions {
		if extractor.isValidVersion(version) {
			t.Errorf("Expected %s to be invalid", version)
		}
	}
}

func TestFindProjectFiles(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		"package.json",
		"src/package.json",
		"test.txt",
		"subdir/another.json",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	extractor := &VersionExtractor{}

	// Test exact file matching
	matches, err := extractor.findProjectFiles(tmpDir, "package.json")
	if err != nil {
		t.Fatalf("Error finding files: %v", err)
	}

	if len(matches) < 1 {
		t.Error("Expected at least 1 match for package.json")
	}

	// Test glob pattern matching
	matches, err = extractor.findProjectFiles(tmpDir, "*.json")
	if err != nil {
		t.Fatalf("Error finding files with glob: %v", err)
	}

	if len(matches) < 1 {
		t.Error("Expected at least 1 match for *.json")
	}
}

func TestRemoveDuplicates(t *testing.T) {
	extractor := &VersionExtractor{}

	input := []string{
		"/path/to/file1",
		"/path/to/file2",
		"/path/to/file1", // duplicate
		"/path/to/file3",
		"/path/to/file2", // duplicate
	}

	result := extractor.removeDuplicates(input)

	if len(result) != 3 {
		t.Errorf("Expected 3 unique files, got %d", len(result))
	}

	// Check that all expected files are present
	expected := map[string]bool{
		"/path/to/file1": true,
		"/path/to/file2": true,
		"/path/to/file3": true,
	}

	for _, file := range result {
		if !expected[file] {
			t.Errorf("Unexpected file in result: %s", file)
		}
		delete(expected, file)
	}

	if len(expected) > 0 {
		t.Errorf("Missing expected files: %v", expected)
	}
}

func TestGetSupportedTypes(t *testing.T) {
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{Type: "JavaScript", Subtype: "npm"},
			{Type: "Python", Subtype: "Modern"},
			{Type: "Java"},
		},
	}

	extractor := New(cfg)
	types := extractor.GetSupportedTypes()

	expected := []string{
		"Java",
		"JavaScript (npm)",
		"Python (Modern)",
	}

	if len(types) != len(expected) {
		t.Errorf("Expected %d types, got %d", len(expected), len(types))
	}

	for i, expectedType := range expected {
		if i >= len(types) || types[i] != expectedType {
			t.Errorf("Expected type %d to be %s, got %s",
				i, expectedType, types[i])
		}
	}
}

func TestExtractVersionFromFile(t *testing.T) {
	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	content := `{
  "name": "test-project",
  "version": "1.2.3",
  "other": "data"
}`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extractor := &VersionExtractor{}
	patterns := []string{`"version":\s*"([^"]+)"`}

	version, matchedPattern, err := extractor.extractVersionFromFile(testFile,
		patterns)
	if err != nil {
		t.Fatalf("Error extracting version: %v", err)
	}

	if version != "1.2.3" {
		t.Errorf("Expected version 1.2.3, got %s", version)
	}

	if matchedPattern != patterns[0] {
		t.Errorf("Expected matched pattern %s, got %s",
			patterns[0], matchedPattern)
	}
}

func TestExtractVersionFromFileNoMatch(t *testing.T) {
	// Create test file without version
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	content := `{
  "name": "test-project",
  "description": "No version here"
}`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extractor := &VersionExtractor{}
	patterns := []string{`"version":\s*"([^"]+)"`}

	version, matchedPattern, err := extractor.extractVersionFromFile(testFile,
		patterns)
	if err != nil {
		t.Fatalf("Error extracting version: %v", err)
	}

	if version != "" {
		t.Errorf("Expected empty version, got %s", version)
	}

	if matchedPattern != "" {
		t.Errorf("Expected empty matched pattern, got %s", matchedPattern)
	}
}

func TestExtractVersionFromFileFileSizeLimit(t *testing.T) {
	// Create test file that exceeds 10MB limit
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large_test.json")

	// Create content larger than the max file size limit (10MB + some extra)
	largeContent := strings.Repeat("x", maxFileSizeLimit+1000)

	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "Test",
				File:  "large_test.json",
				Regex: []string{`"version":\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)
	patterns := []string{`"version":\s*"([^"]+)"`}

	// Should fail due to file size limit
	version, matchedPattern, err := extractor.extractVersionFromFile(testFile,
		patterns)
	if err == nil {
		t.Fatal("Expected error due to file size limit, got none")
	}

	if !strings.Contains(err.Error(), "file size exceeds limit of 10MB") {
		t.Errorf("Expected file size limit error, got: %v", err)
	}

	if version != "" {
		t.Errorf("Expected empty version, got %s", version)
	}

	if matchedPattern != "" {
		t.Errorf("Expected empty matched pattern, got %s", matchedPattern)
	}
}

func TestExtractVersionFromFileStreamingApproach(t *testing.T) {
	// Create test file with normal size to verify streaming approach works
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "normal_test.json")

	// Create content that's well under the 10MB limit but uses multiple lines
	content := `{
  "name": "test-project",
  "description": "A test project for streaming file reading",
  "version": "2.5.7",
  "dependencies": {
    "test-dep": "^1.0.0"
  },
  "scripts": {
    "test": "echo test",
    "build": "echo build"
  }
}`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "JavaScript",
				File:  "normal_test.json",
				Regex: []string{`"version":\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)
	patterns := []string{`"version":\s*"([^"]+)"`}

	// Should successfully extract version using streaming approach
	version, matchedPattern, err := extractor.extractVersionFromFile(testFile,
		patterns)
	if err != nil {
		t.Fatalf("Error extracting version: %v", err)
	}

	expectedVersion := "2.5.7"
	if version != expectedVersion {
		t.Errorf("Expected version %s, got %s", expectedVersion, version)
	}

	expectedPattern := `"version":\s*"([^"]+)"`
	if matchedPattern != expectedPattern {
		t.Errorf("Expected pattern %s, got %s", expectedPattern,
			matchedPattern)
	}
}

func TestNewWithOptions(t *testing.T) {
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:    "Python",
				File:    "pyproject.toml",
				Regex:   []string{`version\s*=\s*["']([^"']+)["']`},
				Samples: []string{"https://github.com/test/repo"},
			},
		},
	}

	// Test with dynamic fallback enabled
	extractor1 := NewWithOptions(cfg, true)
	if extractor1 == nil {
		t.Fatal("Expected non-nil extractor")
	}
	if extractor1.dynamicFallback != true {
		t.Error("Expected dynamicFallback to be true")
	}

	// Test with dynamic fallback disabled
	extractor2 := NewWithOptions(cfg, false)
	if extractor2 == nil {
		t.Fatal("Expected non-nil extractor")
	}
	if extractor2.dynamicFallback != false {
		t.Error("Expected dynamicFallback to be false")
	}
}

func TestDetectDynamicVersioning(t *testing.T) {
	extractor := &VersionExtractor{}

	tests := []struct {
		name       string
		content    string
		indicators []config.DynamicVersionIndicator
		expected   bool
	}{
		{
			name: "setuptools_scm section exists",
			content: `[build-system]
requires = ["setuptools", "setuptools_scm"]

[tool.setuptools_scm]
version_scheme = "post-release"`,
			indicators: []config.DynamicVersionIndicator{
				{Path: "[tool.setuptools_scm]", Exists: true},
			},
			expected: true,
		},
		{
			name: "dynamic field contains version",
			content: `[project]
name = "test-project"
dynamic = ["version", "description"]`,
			indicators: []config.DynamicVersionIndicator{
				{Field: "dynamic", Contains: []string{"version"}},
			},
			expected: true,
		},
		{
			name: "versioneer section exists",
			content: `[tool.versioneer]
VCS = "git"
style = "pep440"`,
			indicators: []config.DynamicVersionIndicator{
				{Path: "[tool.versioneer]", Exists: true},
			},
			expected: true,
		},
		{
			name: "no dynamic versioning indicators",
			content: `[project]
name = "test-project"
version = "1.0.0"`,
			indicators: []config.DynamicVersionIndicator{
				{Path: "[tool.setuptools_scm]", Exists: true},
			},
			expected: false,
		},
		{
			name: "dynamic field exists but doesn't contain version",
			content: `[project]
name = "test-project"
dynamic = ["description", "readme"]`,
			indicators: []config.DynamicVersionIndicator{
				{Field: "dynamic", Contains: []string{"version"}},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "pyproject.toml")
			err := os.WriteFile(testFile, []byte(test.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test detection
			result, err := extractor.detectDynamicVersioning(testFile, test.indicators)
			if err != nil {
				t.Fatalf("Error detecting dynamic versioning: %v", err)
			}

			if result != test.expected {
				t.Errorf("Expected %t, got %t", test.expected, result)
			}
		})
	}
}

func TestDetectDynamicVersioningFileSizeLimit(t *testing.T) {
	// Create test file that exceeds 10MB limit
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large_pyproject.toml")

	// Create content larger than the max file size limit (10MB + some extra)
	largeContent := strings.Repeat("x", maxFileSizeLimit+1000)

	err := os.WriteFile(testFile, []byte(largeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extractor := &VersionExtractor{}
	indicators := []config.DynamicVersionIndicator{
		{Path: "[tool.setuptools_scm]", Exists: true},
	}

	// Should fail due to file size limit
	result, err := extractor.detectDynamicVersioning(testFile, indicators)
	if err == nil {
		t.Fatal("Expected error due to file size limit, got none")
	}

	if !strings.Contains(err.Error(), "file size exceeds limit of 10MB") {
		t.Errorf("Expected file size limit error, got: %v", err)
	}

	// Result should be false when there's an error
	if result {
		t.Errorf("Expected result to be false when there's an error, got true")
	}
}

func TestTryGitFallback(t *testing.T) {
	extractor := &VersionExtractor{}

	// Test with non-git directory
	tmpDir := t.TempDir()
	result := extractor.tryGitFallback(tmpDir)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Success {
		t.Error("Expected Success=false for non-git directory")
	}

	if result.IsGitRepo {
		t.Error("Expected IsGitRepo=false for non-git directory")
	}
}

func TestExtractWithDynamicVersioning(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()
	pyprojectFile := filepath.Join(tmpDir, "pyproject.toml")

	// Create pyproject.toml with dynamic versioning
	content := `[build-system]
requires = ["setuptools", "setuptools_scm"]

[project]
name = "test-project"
dynamic = ["version"]
description = "Test project with dynamic versioning"

[tool.setuptools_scm]
version_scheme = "post-release"`

	err := os.WriteFile(pyprojectFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config with dynamic versioning support
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:                      "Python",
				Subtype:                   "Modern (pyproject.toml)",
				File:                      "pyproject.toml",
				Regex:                     []string{`version\s*=\s*["']([^"']+)["']`},
				Samples:                   []string{"https://github.com/test/repo"},
				Priority:                  1,
				SupportsDynamicVersioning: true,
				DynamicVersionIndicators: []config.DynamicVersionIndicator{
					{Field: "dynamic", Contains: []string{"version"}},
					{Path: "[tool.setuptools_scm]", Exists: true},
				},
				FallbackStrategy: "git-tags",
			},
		},
	}

	// Test with dynamic versioning enabled (should not find git repo)
	extractor := NewWithOptions(cfg, false)
	result, err := extractor.Extract(tmpDir)

	if err == nil {
		t.Fatal("Expected error for non-git repository with dynamic versioning")
	}

	// Test with dynamic versioning disabled (should not try git fallback)
	extractorDisabled := NewWithOptions(cfg, true)
	resultDisabled, errDisabled := extractorDisabled.Extract(tmpDir)

	// Use result variable to avoid unused variable error
	_ = result

	if errDisabled == nil {
		t.Fatal("Expected error when no static version found and dynamic disabled")
	}

	if resultDisabled.Success {
		t.Error("Expected failure when no static version available")
	}
}

func TestVersionSourceField(t *testing.T) {
	// Test static version extraction includes version_source
	tmpDir := t.TempDir()
	packageJSON := filepath.Join(tmpDir, "package.json")

	content := `{
  "name": "test-project",
  "version": "1.2.3"
}`

	err := os.WriteFile(packageJSON, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "JavaScript",
				Subtype:  "npm",
				File:     "package.json",
				Regex:    []string{`"version":\s*"([^"]+)"`},
				Samples:  []string{"https://github.com/test/repo"},
				Priority: 1,
			},
		},
	}

	extractor := New(cfg)
	result, err := extractor.Extract(tmpDir)

	if err != nil {
		t.Fatalf("Expected successful extraction, got error: %v", err)
	}

	if result.VersionSource != "static" {
		t.Errorf("Expected VersionSource 'static', got '%s'", result.VersionSource)
	}

	if result.GitTag != "" {
		t.Errorf("Expected empty GitTag for static version, got '%s'", result.GitTag)
	}
}

func TestMultiLanguageDynamicVersioning(t *testing.T) {
	tests := []struct {
		name         string
		language     string
		subtype      string
		filename     string
		content      string
		expectedType string
		shouldDetect bool
		hasStaticVer bool
	}{
		{
			name:     "JavaScript semantic-release",
			language: "JavaScript",
			subtype:  "npm",
			filename: "package.json",
			content: `{
  "name": "test-project",
  "version": "0.0.0-development",
  "scripts": {
    "semantic-release": "semantic-release"
  }
}`,
			expectedType: "JavaScript",
			shouldDetect: true,
			hasStaticVer: true, // Has static version, but marked for dynamic versioning
		},
		{
			name:     "JavaScript static version",
			language: "JavaScript",
			subtype:  "npm",
			filename: "package.json",
			content: `{
  "name": "test-project",
  "version": "1.2.3"
}`,
			expectedType: "JavaScript",
			shouldDetect: false,
			hasStaticVer: true,
		},
		{
			name:     "Rust with build script",
			language: "Rust",
			subtype:  "Cargo",
			filename: "Cargo.toml",
			content: `[package]
name = "test-project"
version = "0.0.0"
build = "build.rs"

[dependencies]
serde = "1.0"`,
			expectedType: "Rust",
			shouldDetect: true,
			hasStaticVer: true,
		},
		{
			name:     "Go module with git hosting",
			language: "Go",
			subtype:  "Go Module",
			filename: "go.mod",
			content: `module github.com/user/test-project

go 1.24

require (
    github.com/spf13/cobra v1.9.1
)`,
			expectedType: "Go",
			shouldDetect: true,
			hasStaticVer: true, // Has go version
		},
		{
			name:     "Java Maven with SNAPSHOT",
			language: "Java",
			subtype:  "Maven",
			filename: "pom.xml",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test-project</artifactId>
    <version>1.0.0-SNAPSHOT</version>
    <properties>
        <maven.compiler.source>11</maven.compiler.source>
    </properties>
</project>`,
			expectedType: "Java",
			shouldDetect: true,
			hasStaticVer: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create temporary directory and file
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, test.filename)
			err := os.WriteFile(testFile, []byte(test.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Create configuration
			cfg := createTestConfigForLanguage(test.language, test.subtype, test.filename)

			// Test with dynamic versioning enabled
			extractor := NewWithOptions(cfg, false)
			result, err := extractor.Extract(tmpDir)

			// All projects have static versions, so they should succeed
			if err != nil {
				t.Fatalf("Expected successful extraction for static version, got error: %v", err)
			}
			if !result.Success {
				t.Error("Expected successful result")
			}
			if result.ProjectType != test.expectedType {
				t.Errorf("Expected project type %s, got %s", test.expectedType, result.ProjectType)
			}
			if result.VersionSource != "static" {
				t.Errorf("Expected static version source, got %s", result.VersionSource)
			}

			// Test with dynamic versioning disabled - should still work for static versions
			extractorDisabled := NewWithOptions(cfg, true)
			resultDisabled, errDisabled := extractorDisabled.Extract(tmpDir)

			if errDisabled != nil {
				t.Errorf("Expected success for static version with dynamic disabled: %v", errDisabled)
			}

			if resultDisabled != nil && resultDisabled.Success {
				if resultDisabled.VersionSource != "static" {
					t.Errorf("Expected static version source with dynamic disabled, got %s", resultDisabled.VersionSource)
				}
			}
		})
	}
}

func TestDynamicVersioningWithGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping git integration test")
	}

	// Create a temporary git repository
	tmpDir := t.TempDir()

	// Initialize git repo
	if err := runGitCommand(tmpDir, "init"); err != nil {
		t.Skipf("Failed to initialize git repo: %v", err)
	}

	// Configure git for testing
	if err := runGitCommand(tmpDir, "config", "user.email", "test@example.com"); err != nil {
		t.Skipf("Failed to configure git: %v", err)
	}
	if err := runGitCommand(tmpDir, "config", "user.name", "Test User"); err != nil {
		t.Skipf("Failed to configure git: %v", err)
	}

	// Create a JavaScript project with semantic-release
	packageJSON := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-dynamic-js",
  "version": "0.0.0-development",
  "scripts": {
    "semantic-release": "semantic-release"
  }
}`
	if err := os.WriteFile(packageJSON, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit and tag
	if err := runGitCommand(tmpDir, "add", "package.json"); err != nil {
		t.Skipf("Failed to add file: %v", err)
	}
	if err := runGitCommand(tmpDir, "commit", "-m", "Initial commit"); err != nil {
		t.Skipf("Failed to commit: %v", err)
	}
	if err := runGitCommand(tmpDir, "tag", "-a", "v2.1.4", "-m", "Test tag"); err != nil {
		t.Skipf("Failed to create tag: %v", err)
	}

	// Test extraction
	cfg := createTestConfigForLanguage("JavaScript", "npm", "package.json")
	extractor := NewWithOptions(cfg, true)

	result, err := extractor.Extract(tmpDir)
	if err != nil {
		t.Fatalf("Expected successful extraction from git tags: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful result")
	}

	if result.Version != "2.1.4" {
		t.Errorf("Expected version 2.1.4, got %s", result.Version)
	}

	if result.VersionSource != "dynamic-git-tag" {
		t.Errorf("Expected dynamic-git-tag version source, got %s", result.VersionSource)
	}

	if result.GitTag != "v2.1.4" {
		t.Errorf("Expected git tag v2.1.4, got %s", result.GitTag)
	}

	if result.ProjectType != "JavaScript" {
		t.Errorf("Expected JavaScript project type, got %s", result.ProjectType)
	}
}

func createTestConfigForLanguage(language, subtype, filename string) *config.Config {
	var dynamicIndicators []config.DynamicVersionIndicator
	var supportsDynamic bool

	switch language {
	case "JavaScript":
		supportsDynamic = true
		dynamicIndicators = []config.DynamicVersionIndicator{
			{Field: "version", Contains: []string{"0.0.0-development", "0.0.0-semantic-release"}},
			{Field: "scripts", Contains: []string{"semantic-release", "auto-release"}},
		}
	case "Rust":
		supportsDynamic = true
		dynamicIndicators = []config.DynamicVersionIndicator{
			{Field: "version", Contains: []string{"0.0.0", "0.1.0-dev"}},
			{Path: "[package.metadata.release]", Exists: true},
			{Field: "build", Contains: []string{"build.rs"}},
		}
	case "Go":
		supportsDynamic = true
		dynamicIndicators = []config.DynamicVersionIndicator{
			{Field: "version", Contains: []string{"v0.0.0", "v0.1.0"}},
			{Path: "go.mod", Field: "module", Contains: []string{"github.com", "gitlab.com"}},
		}
	case "Java":
		supportsDynamic = true
		dynamicIndicators = []config.DynamicVersionIndicator{
			{Field: "version", Contains: []string{"${revision}", "${project.version}", "SNAPSHOT"}},
			{Path: "<properties>", Exists: true},
			{Field: "plugin", Contains: []string{"git-commit-id", "buildnumber-maven", "versions-maven"}},
		}
	}

	// Create appropriate regex patterns
	var regexPatterns []string
	switch language {
	case "JavaScript":
		regexPatterns = []string{`"version":\s*"([^"]+)"`}
	case "Rust":
		regexPatterns = []string{`version\s*=\s*"([^"]+)"`}
	case "Go":
		regexPatterns = []string{`go\s+([0-9]+\.[0-9]+(?:\.[0-9]+)?)`}
	case "Java":
		regexPatterns = []string{`<version>([^<]+)</version>`}
	default:
		regexPatterns = []string{`version.*?([0-9]+\.[0-9]+\.[0-9]+)`}
	}

	return &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:                      language,
				Subtype:                   subtype,
				File:                      filename,
				Regex:                     regexPatterns,
				Samples:                   []string{"https://github.com/test/repo"},
				Priority:                  1,
				SupportsDynamicVersioning: supportsDynamic,
				DynamicVersionIndicators:  dynamicIndicators,
				FallbackStrategy:          "git-tags",
			},
		},
	}
}

func TestDynamicVersioningEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		indicators  []config.DynamicVersionIndicator
		expected    bool
		description string
	}{
		{
			name: "complex JSON scripts object",
			content: `{
  "name": "test-project",
  "version": "1.0.0",
  "scripts": {
    "build": "webpack",
    "test": "jest",
    "release": "semantic-release"
  }
}`,
			indicators: []config.DynamicVersionIndicator{
				{Field: "scripts", Contains: []string{"semantic-release"}},
			},
			expected:    true,
			description: "Should detect semantic-release in scripts object",
		},
		{
			name: "TOML metadata section",
			content: `[package]
name = "test-project"
version = "0.1.0"

[package.metadata.release]
sign-commit = true
sign-tag = true`,
			indicators: []config.DynamicVersionIndicator{
				{Path: "[package.metadata.release]", Exists: true},
			},
			expected:    true,
			description: "Should detect cargo-release metadata section",
		},
		{
			name: "Maven properties with variables",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <version>${revision}</version>
    <properties>
        <revision>1.0.0-SNAPSHOT</revision>
    </properties>
</project>`,
			indicators: []config.DynamicVersionIndicator{
				{Field: "version", Contains: []string{"${revision}"}},
			},
			expected:    true,
			description: "Should detect Maven variable versioning",
		},
		{
			name: "false positive avoidance",
			content: `{
  "name": "test-project",
  "version": "1.0.0",
  "description": "This project does not use semantic-release"
}`,
			indicators: []config.DynamicVersionIndicator{
				{Field: "scripts", Contains: []string{"semantic-release"}},
			},
			expected:    false,
			description: "Should not detect when semantic-release is only in description",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			extractor := &VersionExtractor{}

			// Create temporary file
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test-file")
			err := os.WriteFile(testFile, []byte(test.content), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test detection
			result, err := extractor.detectDynamicVersioning(testFile, test.indicators)
			if err != nil {
				t.Fatalf("Error detecting dynamic versioning: %v", err)
			}

			if result != test.expected {
				t.Errorf("%s: Expected %t, got %t", test.description, test.expected, result)
			}
		})
	}
}

// TestSetGetSkipDirectories tests the skip directories configuration functionality
func TestSetGetSkipDirectories(t *testing.T) {
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "JavaScript",
				File:  "package.json",
				Regex: []string{`"version":\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)

	// Test default skip directories
	defaultDirs := extractor.GetSkipDirectories()
	expectedDefault := []string{"node_modules", "vendor", "target", "build", "dist"}
	if len(defaultDirs) != len(expectedDefault) {
		t.Errorf("Expected %d default skip directories, got %d", len(expectedDefault), len(defaultDirs))
	}
	for i, dir := range expectedDefault {
		if defaultDirs[i] != dir {
			t.Errorf("Expected default skip directory %s at index %d, got %s", dir, i, defaultDirs[i])
		}
	}

	// Test setting custom skip directories
	customDirs := []string{"custom1", "custom2", "temp"}
	extractor.SetSkipDirectories(customDirs)

	retrievedDirs := extractor.GetSkipDirectories()
	if len(retrievedDirs) != len(customDirs) {
		t.Errorf("Expected %d custom skip directories, got %d", len(customDirs), len(retrievedDirs))
	}
	for i, dir := range customDirs {
		if retrievedDirs[i] != dir {
			t.Errorf("Expected custom skip directory %s at index %d, got %s", dir, i, retrievedDirs[i])
		}
	}
}

// TestSkipDirectoriesInFileSearch tests that skip directories are actually used during file search
func TestSkipDirectoriesInFileSearch(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create subdirectories including ones that should be skipped
	testDirs := []string{"src", "node_modules", "vendor", "custom_skip"}
	for _, dir := range testDirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}

		// Create a package.json in each directory
		packageJSON := filepath.Join(tmpDir, dir, "package.json")
		content := `{
  "name": "test-project",
  "version": "1.0.0"
}`
		err = os.WriteFile(packageJSON, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create package.json in %s: %v", dir, err)
		}
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "JavaScript",
				File:  "package.json",
				Regex: []string{`"version":\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)

	// First test with default skip directories
	files, err := extractor.findProjectFiles(tmpDir, "package.json")
	if err != nil {
		t.Fatalf("Failed to find project files: %v", err)
	}

	// Should find files in src and custom_skip, but not in node_modules or vendor
	expectedFiles := []string{
		filepath.Join(tmpDir, "src", "package.json"),
		filepath.Join(tmpDir, "custom_skip", "package.json"),
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files with default skip dirs, got %d: %v", len(expectedFiles), len(files), files)
	}

	// Now test with custom skip directories that include custom_skip
	customSkipDirs := []string{"custom_skip", "temp"}
	extractor.SetSkipDirectories(customSkipDirs)

	files, err = extractor.findProjectFiles(tmpDir, "package.json")
	if err != nil {
		t.Fatalf("Failed to find project files with custom skip dirs: %v", err)
	}

	// Should now find files in src, node_modules, and vendor, but not in custom_skip
	expectedFilesCustom := []string{
		filepath.Join(tmpDir, "src", "package.json"),
		filepath.Join(tmpDir, "node_modules", "package.json"),
		filepath.Join(tmpDir, "vendor", "package.json"),
	}

	if len(files) != len(expectedFilesCustom) {
		t.Errorf("Expected %d files with custom skip dirs, got %d: %v", len(expectedFilesCustom), len(files), files)
	}
}

// Helper function to run git commands for testing
func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
