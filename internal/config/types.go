// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// DynamicVersionIndicator represents a condition to detect dynamic versioning
type DynamicVersionIndicator struct {
	Path     string   `yaml:"path,omitempty"`     // TOML section like "[project]"
	Field    string   `yaml:"field,omitempty"`    // Field name like "dynamic"
	Contains []string `yaml:"contains,omitempty"` // Values that indicate dynamic versioning
	Exists   bool     `yaml:"exists,omitempty"`   // True if section/field existence indicates dynamic
}

// ProjectConfig represents a single project type configuration
type ProjectConfig struct {
	Type    string `yaml:"type" validate:"required"`
	Subtype string `yaml:"subtype,omitempty"`
	File    string `yaml:"file" validate:"required"`
	// Regex patterns for version extraction. No struct validation tags because
	// empty arrays are allowed for projects with SupportsDynamicVersioning=true
	// (e.g., Go projects that use git tags). Runtime validation in validateConfig()
	// enforces that non-dynamic projects must have at least one regex pattern.
	Regex                     []string                  `yaml:"regex"`
	Samples                   []string                  `yaml:"samples" validate:"required,min=1"`
	Priority                  int                       `yaml:"priority,omitempty"`
	Notes                     string                    `yaml:"notes,omitempty"`
	SupportsDynamicVersioning bool                      `yaml:"supports_dynamic_versioning,omitempty"`
	DynamicVersionIndicators  []DynamicVersionIndicator `yaml:"dynamic_version_indicators,omitempty"`
	FallbackStrategy          string                    `yaml:"fallback_strategy,omitempty"`
}

// Config represents the complete configuration structure
type Config struct {
	Projects []ProjectConfig `yaml:"projects" validate:"required,min=1"`
}

// LoadConfig loads and validates configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Sort projects by priority
	sortProjectsByPriority(&config)

	return &config, nil
}

// validateConfig performs basic validation on the configuration
func validateConfig(config *Config) error {
	if len(config.Projects) == 0 {
		return fmt.Errorf("no projects defined in configuration")
	}

	seenTypes := make(map[string]bool)
	validProjects := []ProjectConfig{}

	for i, project := range config.Projects {
		// Basic field validation
		if project.Type == "" {
			fmt.Fprintf(os.Stderr, "Warning: Project at index %d missing type, "+
				"skipping\n", i)
			continue
		}
		if project.File == "" {
			fmt.Fprintf(os.Stderr, "Warning: Project %s missing file pattern, "+
				"skipping\n", project.Type)
			continue
		}
		if len(project.Regex) == 0 {
			// Allow empty regex for projects that support dynamic versioning
			// (e.g., Go projects that rely on git tags)
			if !project.SupportsDynamicVersioning {
				fmt.Fprintf(os.Stderr, "Warning: Project %s missing regex patterns, "+
					"skipping\n", project.Type)
				continue
			}
		}
		if len(project.Samples) == 0 {
			fmt.Fprintf(os.Stderr, "Warning: Project %s missing sample URLs, "+
				"skipping\n", project.Type)
			continue
		}

		// Create unique key for duplicate detection
		key := fmt.Sprintf("%s-%s-%s", project.Type, project.Subtype,
			project.File)
		if seenTypes[key] {
			fmt.Fprintf(os.Stderr, "Warning: Duplicate project config for %s, "+
				"skipping\n", key)
			continue
		}
		seenTypes[key] = true

		// Set default priority if not specified
		if project.Priority == 0 {
			project.Priority = i + 1
		}

		validProjects = append(validProjects, project)
	}

	// Update config with valid projects only
	config.Projects = validProjects

	if len(config.Projects) == 0 {
		return fmt.Errorf("no valid projects after validation")
	}

	return nil
}

// sortProjectsByPriority sorts projects by priority (lower number = higher
// priority)
func sortProjectsByPriority(config *Config) {
	sort.Slice(config.Projects, func(i, j int) bool {
		return config.Projects[i].Priority < config.Projects[j].Priority
	})
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	return filepath.Join("configs", "default-patterns.yaml")
}

// GetProjectByType finds a project configuration by type and subtype
func (c *Config) GetProjectByType(projectType,
	subtype string) *ProjectConfig {
	for _, project := range c.Projects {
		if project.Type == projectType {
			if subtype == "" || project.Subtype == subtype {
				return &project
			}
		}
	}
	return nil
}

// GetSupportedTypes returns a list of all supported project types
func (c *Config) GetSupportedTypes() []string {
	types := make(map[string]bool)
	var result []string

	for _, project := range c.Projects {
		key := project.Type
		if project.Subtype != "" {
			key = fmt.Sprintf("%s (%s)", project.Type, project.Subtype)
		}
		if !types[key] {
			types[key] = true
			result = append(result, key)
		}
	}

	sort.Strings(result)
	return result
}
