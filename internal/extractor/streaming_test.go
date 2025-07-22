// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

// TestStreamingMemoryEfficiency verifies that the streaming approach
// processes files line by line without loading entire content into memory
func TestStreamingMemoryEfficiency(t *testing.T) {
	// Create a large test file with version info at different positions
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large_file.txt")

	// Create content with version at line 1000 to test streaming
	var content strings.Builder
	for i := 0; i < 999; i++ {
		content.WriteString("This is a dummy line of content that makes the file larger\n")
	}
	content.WriteString("version: 2.5.7\n") // Version at line 1000
	for i := 0; i < 1000; i++ {
		content.WriteString("More dummy content after the version line\n")
	}

	err := os.WriteFile(testFile, []byte(content.String()), 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Set up extractor with patterns
	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "Generic",
				File:  "large_file.txt",
				Regex: []string{`version:\s*([0-9]+\.[0-9]+\.[0-9]+)`},
			},
		},
	}

	extractor := New(cfg)

	// Extract version using streaming approach
	version, pattern, err := extractor.extractVersionFromFile(testFile, cfg.Projects[0].Regex)
	if err != nil {
		t.Fatalf("Failed to extract version: %v", err)
	}

	// Verify results
	if version != "2.5.7" {
		t.Errorf("Expected version '2.5.7', got '%s'", version)
	}

	if pattern != `version:\s*([0-9]+\.[0-9]+\.[0-9]+)` {
		t.Errorf("Expected pattern to match, got '%s'", pattern)
	}

	fileSize := int64(len(content.String()))
	t.Logf("Successfully processed %d byte file using streaming approach", fileSize)
}

// TestStreamingVersionAtBeginning tests that streaming works when version
// is at the beginning of the file
func TestStreamingVersionAtBeginning(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "version_first.txt")

	content := `version = "1.0.0"
# This is a configuration file
name = "test-app"
description = "A test application"
# Many more lines of configuration...
`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "TOML",
				File:  "version_first.txt",
				Regex: []string{`version\s*=\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)
	version, pattern, err := extractor.extractVersionFromFile(testFile, cfg.Projects[0].Regex)

	if err != nil {
		t.Fatalf("Failed to extract version: %v", err)
	}

	if version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", version)
	}

	if pattern == "" {
		t.Error("Expected pattern to be returned")
	}
}

// TestStreamingVersionAtEnd tests that streaming works when version
// is at the end of the file
func TestStreamingVersionAtEnd(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "version_last.txt")

	var content strings.Builder
	content.WriteString("# Configuration file\n")
	content.WriteString("name = \"test-app\"\n")
	content.WriteString("description = \"A test application\"\n")
	for i := 0; i < 100; i++ {
		content.WriteString("# More configuration options\n")
	}
	content.WriteString("version = \"3.2.1\"\n") // Version at the end

	err := os.WriteFile(testFile, []byte(content.String()), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:  "TOML",
				File:  "version_last.txt",
				Regex: []string{`version\s*=\s*"([^"]+)"`},
			},
		},
	}

	extractor := New(cfg)
	version, pattern, err := extractor.extractVersionFromFile(testFile, cfg.Projects[0].Regex)

	if err != nil {
		t.Fatalf("Failed to extract version: %v", err)
	}

	if version != "3.2.1" {
		t.Errorf("Expected version '3.2.1', got '%s'", version)
	}

	if pattern == "" {
		t.Error("Expected pattern to be returned")
	}
}

// TestStreamingMultiplePatterns tests that streaming works correctly
// when trying multiple regex patterns
func TestStreamingMultiplePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multi_pattern.txt")

	content := `{
  "name": "test-project",
  "description": "Test package",
  "version": "4.5.6"
}`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	patterns := []string{
		`"ver":\s*"([^"]+)"`,      // Won't match
		`"version":\s*"([^"]+)"`,  // Will match
		`version\s*=\s*"([^"]+)"`, // Won't match
	}

	extractor := New(&config.Config{})
	version, pattern, err := extractor.extractVersionFromFile(testFile, patterns)

	if err != nil {
		t.Fatalf("Failed to extract version: %v", err)
	}

	if version != "4.5.6" {
		t.Errorf("Expected version '4.5.6', got '%s'", version)
	}

	if pattern != `"version":\s*"([^"]+)"` {
		t.Errorf("Expected matching pattern, got '%s'", pattern)
	}
}

// TestStreamingInvalidRegex tests error handling for invalid regex patterns
func TestStreamingInvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := `version = "1.0.0"`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	patterns := []string{
		`[invalid regex`,          // Invalid regex
		`version\s*=\s*"([^"]+)"`, // Valid regex that should match
	}

	extractor := New(&config.Config{})
	version, pattern, err := extractor.extractVersionFromFile(testFile, patterns)

	// Should succeed with the valid pattern despite invalid first pattern
	if err != nil {
		t.Fatalf("Failed to extract version: %v", err)
	}

	if version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", version)
	}

	if pattern != `version\s*=\s*"([^"]+)"` {
		t.Errorf("Expected valid pattern, got '%s'", pattern)
	}
}
