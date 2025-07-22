// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

// BenchmarkExtractVersion benchmarks the main ExtractVersion function
func BenchmarkExtractVersion(b *testing.B) {
	// Create temporary test project
	tempDir := createTempJavaScriptProject(b)
	defer os.RemoveAll(tempDir)

	// Load configuration
	cfg, err := config.LoadConfig("../../configs/default-patterns.yaml")
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	extractor := New(cfg)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := extractor.Extract(tempDir)
		if err != nil {
			b.Fatalf("Extract failed: %v", err)
		}
		if !result.Success {
			b.Fatalf("Expected successful extraction")
		}
	}
}

// BenchmarkExtractVersionLargeProject benchmarks with a project containing many files
func BenchmarkExtractVersionLargeProject(b *testing.B) {
	tempDir := createLargeTestProject(b)
	defer os.RemoveAll(tempDir)

	cfg, err := config.LoadConfig("../../configs/default-patterns.yaml")
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	extractor := New(cfg)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := extractor.Extract(tempDir)
		if err != nil {
			b.Fatalf("Extract failed: %v", err)
		}
		if !result.Success {
			b.Fatalf("Expected successful extraction")
		}
	}
}

// BenchmarkMultipleProjectTypes benchmarks extraction across different project types
func BenchmarkMultipleProjectTypes(b *testing.B) {
	// Create different project types
	projects := map[string]func(*testing.B) string{
		"JavaScript": createTempJavaScriptProject,
		"Python":     createTempPythonProject,
		"Go":         createTempGoProject,
		"Rust":       createTempRustProject,
	}

	cfg, err := config.LoadConfig("../../configs/default-patterns.yaml")
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	extractor := New(cfg)

	for projectType, createFunc := range projects {
		b.Run(projectType, func(b *testing.B) {
			tempDir := createFunc(b)
			defer os.RemoveAll(tempDir)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := extractor.Extract(tempDir)
				if err != nil {
					b.Fatalf("Extract failed for %s: %v", projectType, err)
				}
				if !result.Success {
					b.Fatalf("Expected successful extraction for %s", projectType)
				}
			}
		})
	}
}

// BenchmarkConfigurationLoading benchmarks the configuration loading process
func BenchmarkConfigurationLoading(b *testing.B) {
	configPath := "../../configs/default-patterns.yaml"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
		if len(cfg.Projects) == 0 {
			b.Fatalf("Expected projects in configuration")
		}
	}
}

// BenchmarkRegexMatching benchmarks regex pattern matching performance
func BenchmarkRegexMatching(b *testing.B) {
	cfg, err := config.LoadConfig("../../configs/default-patterns.yaml")
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	extractor := New(cfg)

	// Sample content to match against
	testContent := `{
		"name": "benchmark-test-project",
		"version": "1.2.3-alpha.4+build.567",
		"description": "Performance test project",
		"dependencies": {
			"lodash": "^4.17.21",
			"express": "~4.18.2"
		}
	}`

	// Find JavaScript project configuration
	var jsProject *config.ProjectConfig
	for _, project := range cfg.Projects {
		if project.Type == "JavaScript" && project.Subtype == "npm" {
			jsProject = &project
			break
		}
	}

	if jsProject == nil {
		b.Fatalf("JavaScript project configuration not found")
	}

	b.ResetTimer()
	b.ReportAllocs()

	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "benchmark-regex-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		b.Fatalf("Failed to write test file: %v", err)
	}

	for i := 0; i < b.N; i++ {
		result, err := extractor.Extract(tempDir)
		if err != nil {
			b.Fatalf("Regex matching failed: %v", err)
		}
		if result.Version == "" {
			b.Fatalf("Expected version to be extracted")
		}
	}
}

// BenchmarkFileSystemOperations benchmarks file system scanning performance
func BenchmarkFileSystemOperations(b *testing.B) {
	tempDir := createDeepDirectoryStructure(b)
	defer os.RemoveAll(tempDir)

	cfg, err := config.LoadConfig("../../configs/default-patterns.yaml")
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Benchmark the file discovery process
		err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				// Simulate checking each file
				filename := filepath.Base(path)
				for _, project := range cfg.Projects {
					if filename == project.File {
						// Found a potential match
						break
					}
				}
			}
			return nil
		})

		if err != nil {
			b.Fatalf("File system walk failed: %v", err)
		}
	}
}

// Helper functions for creating test projects

func createTempJavaScriptProject(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-js-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	packageJSON := `{
		"name": "benchmark-test-project",
		"version": "1.2.3",
		"description": "Benchmark test project",
		"main": "index.js",
		"scripts": {
			"test": "jest",
			"start": "node index.js"
		},
		"dependencies": {
			"express": "^4.18.2",
			"lodash": "^4.17.21"
		},
		"devDependencies": {
			"jest": "^29.0.0"
		}
	}`

	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		b.Fatalf("Failed to write package.json: %v", err)
	}

	return tempDir
}

func createTempPythonProject(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-py-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	pyprojectToml := `[project]
name = "benchmark-test-project"
version = "2.1.0"
description = "Benchmark Python test project"
authors = [{name = "Test Suite", email = "test@example.com"}]
license = {text = "Apache-2.0"}
requires-python = ">=3.8"

[build-system]
requires = ["setuptools>=61.0"]
build-backend = "setuptools.build_meta"
`

	err = os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectToml), 0644)
	if err != nil {
		b.Fatalf("Failed to write pyproject.toml: %v", err)
	}

	return tempDir
}

func createTempGoProject(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-go-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	goMod := `module github.com/test/benchmark-project

go 1.24

require (
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.7 // indirect
)
`

	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644)
	if err != nil {
		b.Fatalf("Failed to write go.mod: %v", err)
	}

	return tempDir
}

func createTempRustProject(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-rust-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	cargoToml := `[package]
name = "benchmark-test-project"
version = "0.3.1"
edition = "2021"
description = "Benchmark Rust test project"
license = "Apache-2.0"
authors = ["Test Suite <test@example.com>"]

[dependencies]
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1.0", features = ["full"] }
`

	err = os.WriteFile(filepath.Join(tempDir, "Cargo.toml"), []byte(cargoToml), 0644)
	if err != nil {
		b.Fatalf("Failed to write Cargo.toml: %v", err)
	}

	return tempDir
}

func createLargeTestProject(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-large-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create main package.json
	packageJSON := `{
		"name": "large-benchmark-project",
		"version": "1.2.3",
		"description": "Large project with many files"
	}`

	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		b.Fatalf("Failed to write package.json: %v", err)
	}

	// Create many additional files to simulate a large project
	for i := 0; i < 100; i++ {
		subDir := filepath.Join(tempDir, "src", "components")
		err = os.MkdirAll(subDir, 0755)
		if err != nil {
			b.Fatalf("Failed to create subdirectory: %v", err)
		}

		filename := filepath.Join(subDir, fmt.Sprintf("component-%d.js", i))
		content := fmt.Sprintf("// Component %d\nmodule.exports = {};", i)
		err = os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to write component file: %v", err)
		}
	}

	return tempDir
}

func createDeepDirectoryStructure(b *testing.B) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "benchmark-deep-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a deep directory structure
	currentDir := tempDir
	for i := 0; i < 10; i++ {
		currentDir = filepath.Join(currentDir, fmt.Sprintf("level-%d", i))
		err = os.MkdirAll(currentDir, 0755)
		if err != nil {
			b.Fatalf("Failed to create deep directory: %v", err)
		}

		// Add some files at each level
		for j := 0; j < 5; j++ {
			filename := filepath.Join(currentDir, fmt.Sprintf("file-%d.txt", j))
			content := fmt.Sprintf("File %d at level %d", j, i)
			err = os.WriteFile(filename, []byte(content), 0644)
			if err != nil {
				b.Fatalf("Failed to write file: %v", err)
			}
		}
	}

	// Add a package.json at the root for actual version extraction
	packageJSON := `{"name": "deep-project", "version": "1.0.0"}`
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	if err != nil {
		b.Fatalf("Failed to write package.json: %v", err)
	}

	return tempDir
}
