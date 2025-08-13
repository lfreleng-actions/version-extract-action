// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test-config.yaml")

	validConfig := `---
projects:
  - type: JavaScript
    subtype: npm
    file: package.json
    regex:
      - '"version":\s*"([^"]+)"'
    samples:
      - https://github.com/facebook/react
    priority: 1
    notes: "Test config"

  - type: Python
    subtype: Modern
    file: pyproject.toml
    regex:
      - 'version\s*=\s*["'']([^"'']+)["'']'
    samples:
      - https://github.com/python/cpython
      - https://github.com/pallets/flask
    priority: 2
`

	err := os.WriteFile(configFile, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test successful loading
	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Expected successful load, got error: %v", err)
	}

	if len(cfg.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(cfg.Projects))
	}

	// Verify first project
	project := cfg.Projects[0]
	if project.Type != "JavaScript" {
		t.Errorf("Expected JavaScript, got %s", project.Type)
	}
	if project.Subtype != "npm" {
		t.Errorf("Expected npm, got %s", project.Subtype)
	}
	if len(project.Regex) != 1 {
		t.Errorf("Expected 1 regex pattern, got %d", len(project.Regex))
	}
	if len(project.Samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(project.Samples))
	}
	if project.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", project.Priority)
	}
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	_, err := LoadConfig("nonexistent-file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	invalidConfig := `---
projects:
  - type: JavaScript
    subtype: npm
    invalid_yaml: [
`

	err := os.WriteFile(configFile, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		expectCount int
	}{
		{
			name: "valid config",
			config: Config{
				Projects: []ProjectConfig{
					{
						Type:     "JavaScript",
						File:     "package.json",
						Regex:    []string{`"version":\s*"([^"]+)"`},
						Samples:  []string{"https://github.com/test/repo"},
						Priority: 1,
					},
				},
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:        "empty projects",
			config:      Config{Projects: []ProjectConfig{}},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "missing type",
			config: Config{
				Projects: []ProjectConfig{
					{
						File:    "package.json",
						Regex:   []string{`"version":\s*"([^"]+)"`},
						Samples: []string{"https://github.com/test/repo"},
					},
				},
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "missing file",
			config: Config{
				Projects: []ProjectConfig{
					{
						Type:    "JavaScript",
						Regex:   []string{`"version":\s*"([^"]+)"`},
						Samples: []string{"https://github.com/test/repo"},
					},
				},
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "missing regex",
			config: Config{
				Projects: []ProjectConfig{
					{
						Type:    "JavaScript",
						File:    "package.json",
						Samples: []string{"https://github.com/test/repo"},
					},
				},
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "missing samples",
			config: Config{
				Projects: []ProjectConfig{
					{
						Type:  "JavaScript",
						File:  "package.json",
						Regex: []string{`"version":\s*"([^"]+)"`},
					},
				},
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(&tt.config)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if len(tt.config.Projects) != tt.expectCount {
				t.Errorf("Expected %d projects after validation, got %d",
					tt.expectCount, len(tt.config.Projects))
			}
		})
	}
}

func TestSortProjectsByPriority(t *testing.T) {
	config := Config{
		Projects: []ProjectConfig{
			{Type: "C", Priority: 3},
			{Type: "A", Priority: 1},
			{Type: "B", Priority: 2},
		},
	}

	sortProjectsByPriority(&config)

	expected := []string{"A", "B", "C"}
	for i, project := range config.Projects {
		if project.Type != expected[i] {
			t.Errorf("Expected project %d to be %s, got %s",
				i, expected[i], project.Type)
		}
	}
}

func TestGetProjectByType(t *testing.T) {
	config := Config{
		Projects: []ProjectConfig{
			{
				Type:    "JavaScript",
				Subtype: "npm",
				File:    "package.json",
			},
			{
				Type:    "Python",
				Subtype: "Modern",
				File:    "pyproject.toml",
			},
			{
				Type: "Python",
				File: "setup.py",
			},
		},
	}

	// Test exact match with subtype
	project := config.GetProjectByType("JavaScript", "npm")
	if project == nil {
		t.Error("Expected to find JavaScript npm project")
	} else if project.File != "package.json" {
		t.Errorf("Expected package.json, got %s", project.File)
	}

	// Test match without subtype (should return first match)
	project = config.GetProjectByType("Python", "")
	if project == nil {
		t.Error("Expected to find Python project")
	} else if project.Subtype != "Modern" {
		t.Errorf("Expected Modern subtype, got %s", project.Subtype)
	}

	// Test no match
	project = config.GetProjectByType("NonExistent", "")
	if project != nil {
		t.Error("Expected nil for non-existent project type")
	}
}

func TestGetSupportedTypes(t *testing.T) {
	config := Config{
		Projects: []ProjectConfig{
			{Type: "JavaScript", Subtype: "npm"},
			{Type: "Python", Subtype: "Modern"},
			{Type: "Python", Subtype: "Legacy"},
			{Type: "Java"},
		},
	}

	types := config.GetSupportedTypes()

	expected := []string{
		"Java",
		"JavaScript (npm)",
		"Python (Legacy)",
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

func TestGetDefaultConfigPath(t *testing.T) {
	path := GetDefaultConfigPath()
	expected := filepath.Join("configs", "default-patterns.yaml")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}
