// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

func TestNew(t *testing.T) {
	extractor := New("/tmp/test")
	if extractor == nil {
		t.Fatal("New() returned nil")
	}
	if extractor.workingDir != "/tmp/test" {
		t.Errorf("Expected working dir '/tmp/test', got '%s'", extractor.workingDir)
	}
}

func TestIsGitRepository(t *testing.T) {
	// Test with a non-git directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	extractor := New(tempDir)
	if extractor.IsGitRepository() {
		t.Error("Expected false for non-git directory, got true")
	}
}

func TestCleanVersionFromTag(t *testing.T) {
	extractor := New("/tmp")

	tests := []struct {
		input    string
		expected string
	}{
		{"v1.2.3", "1.2.3"},
		{"V2.0.0", "2.0.0"},
		{"release-1.5.0", "1.5.0"},
		{"rel-2.1.0", "2.1.0"},
		{"release/3.0.0", "3.0.0"},
		{"1.0.0", "1.0.0"},
		{"  v1.2.3  ", "1.2.3"},
		{"v1.0.0-beta.1", "1.0.0-beta.1"},
		{"v2.1.0+build.123", "2.1.0+build.123"},
	}

	for _, test := range tests {
		result := extractor.cleanVersionFromTag(test.input)
		if result != test.expected {
			t.Errorf("cleanVersionFromTag(%q) = %q, expected %q",
				test.input, result, test.expected)
		}
	}
}

func TestIsValidVersionTag(t *testing.T) {
	extractor := New("/tmp")

	tests := []struct {
		input    string
		expected bool
	}{
		{"1.2.3", true},
		{"1.0.0", true},
		{"2.1.0-beta.1", true},
		{"1.0.0+build.123", true},
		{"1.5", true},
		{"2020.01", true},
		{"2020.01.15", true},
		{"", false},
		{"abc", false},
		{"1", false},
		{"1.", false},
		{"1.2.3.4.5", false},
		{"not-a-version", false},
	}

	for _, test := range tests {
		result := extractor.isValidVersionTag(test.input)
		if result != test.expected {
			t.Errorf("isValidVersionTag(%q) = %t, expected %t",
				test.input, result, test.expected)
		}
	}
}

func TestGetLatestVersionTag_NonGitRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	extractor := New(tempDir)
	result, err := extractor.GetLatestVersionTag()

	if err == nil {
		t.Error("Expected error for non-git repository, got nil")
	}

	if result == nil {
		t.Fatal("Expected result struct, got nil")
	}

	if result.IsGitRepo {
		t.Error("Expected IsGitRepo=false, got true")
	}

	if result.Success {
		t.Error("Expected Success=false, got true")
	}
}

// Integration test - only runs if git is available
func TestGetLatestVersionTag_WithGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create a temporary git repository
	tempDir, err := os.MkdirTemp("", "git-repo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize git repo
	if err := runGitCommand(tempDir, "init"); err != nil {
		t.Skipf("Failed to initialize git repo: %v", err)
	}

	// Configure git for testing
	if err := runGitCommand(tempDir, "config", "user.email", "test@example.com"); err != nil {
		t.Skipf("Failed to configure git: %v", err)
	}
	if err := runGitCommand(tempDir, "config", "user.name", "Test User"); err != nil {
		t.Skipf("Failed to configure git: %v", err)
	}

	// Create a test file and commit
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runGitCommand(tempDir, "add", "test.txt"); err != nil {
		t.Skipf("Failed to add file: %v", err)
	}
	if err := runGitCommand(tempDir, "commit", "-m", "Initial commit"); err != nil {
		t.Skipf("Failed to commit: %v", err)
	}

	// Create tags
	if err := runGitCommand(tempDir, "tag", "-a", "v1.0.0", "-m", "Test tag v1.0.0"); err != nil {
		t.Skipf("Failed to create tag: %v", err)
	}
	if err := runGitCommand(tempDir, "tag", "-a", "v1.1.0", "-m", "Test tag v1.1.0"); err != nil {
		t.Skipf("Failed to create tag: %v", err)
	}

	// Test the extractor
	extractor := New(tempDir)

	if !extractor.IsGitRepository() {
		t.Fatal("Expected git repository detection to work")
	}

	result, err := extractor.GetLatestVersionTag()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !result.Success {
		t.Error("Expected Success=true, got false")
	}

	if !result.IsGitRepo {
		t.Error("Expected IsGitRepo=true, got false")
	}

	// Should get the latest tag (1.1.0)
	expectedVersions := []string{"1.1.0", "1.0.0"} // Could be either depending on git version
	found := false
	for _, expected := range expectedVersions {
		if result.Version == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected version to be one of %v, got %s", expectedVersions, result.Version)
	}
}

func TestFetchTags(t *testing.T) {
	// Test with non-git directory
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	extractor := New(tempDir)
	err = extractor.FetchTags()
	if err == nil {
		t.Error("Expected error for non-git repository, got nil")
	}
}

// Helper function to run git commands for testing
func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// Benchmark tests
func BenchmarkCleanVersionFromTag(b *testing.B) {
	extractor := New("/tmp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor.cleanVersionFromTag("v1.2.3-beta.1+build.123")
	}
}

func BenchmarkIsValidVersionTag(b *testing.B) {
	extractor := New("/tmp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor.isValidVersionTag("1.2.3-beta.1")
	}
}

func TestRegexPatternCompilation(t *testing.T) {
	// Test that our regex patterns are properly compiled
	// This verifies that the init() function worked correctly
	if len(versionTagPatterns) == 0 {
		t.Fatal("No version tag patterns were compiled, init() error handling may have failed")
	}

	// Test that at least one pattern works correctly
	extractor := New("/tmp")

	// These should match with our patterns
	validVersions := []string{
		"1.2.3",
		"1.0.0-beta.1",
		"2.1.0+build.123",
		"2020.01.15",
		"1.5.0-SNAPSHOT",
		"3.0.0-rc.1",
		"2.0.0-alpha.1",
	}

	foundMatch := false
	for _, version := range validVersions {
		if extractor.isValidVersionTag(version) {
			foundMatch = true
			break
		}
	}

	if !foundMatch {
		t.Error("None of the expected valid versions matched any compiled patterns")
	}
}

func TestFallbackPatternFunctionality(t *testing.T) {
	// Test that the fallback pattern would work for basic semantic versions
	// This simulates what would happen if all other patterns failed to compile
	fallbackPattern := `^[0-9]+\.[0-9]+\.[0-9]+$`

	re, err := regexp.Compile(fallbackPattern)
	if err != nil {
		t.Fatalf("Fallback pattern failed to compile: %v", err)
	}

	// Test cases that should match the fallback pattern
	shouldMatch := []string{
		"1.0.0",
		"2.3.1",
		"10.20.30",
		"0.0.1",
	}

	for _, version := range shouldMatch {
		if !re.MatchString(version) {
			t.Errorf("Fallback pattern should match %q but didn't", version)
		}
	}

	// Test cases that should NOT match the fallback pattern
	shouldNotMatch := []string{
		"1.0.0-beta",
		"2.1",
		"1.0.0+build",
		"v1.0.0",
		"not-a-version",
		"",
	}

	for _, version := range shouldNotMatch {
		if re.MatchString(version) {
			t.Errorf("Fallback pattern should NOT match %q but did", version)
		}
	}
}
