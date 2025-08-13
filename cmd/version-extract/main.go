// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

// Version extractor CLI tool with fixed verbose output handling
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
	"github.com/lfreleng-actions/version-extract-action/internal/extractor"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// CLI flags
var (
	path            string
	configPath      string
	outputFormat    string
	verbose         bool
	failOnError     bool
	jsonFormat      string
	dynamicFallback bool
)

// verboseLog outputs message to appropriate stream based on output format
func verboseLog(message string) {
	if !verbose {
		return
	}
	if outputFormat == "json" {
		fmt.Fprintf(os.Stderr, "%s\n", message)
	} else {
		fmt.Printf("%s\n", message)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "version-extract",
	Short: "Extract version strings from various project types",
	Long: `A lightweight Go tool that extracts version strings from various
software project types using configurable YAML patterns.

Supports popular project types including JavaScript/Node.js, Python, Java,
C#/.NET, Go, PHP, Ruby, Rust, Swift, Dart/Flutter, and many more.

The tool searches for project metadata files in order of popularity and
uses regular expressions to extract version information.`,
	RunE: runExtractor,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("version-extract %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built: %s\n", date)
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported project types",
	Long: `List all supported project types and their configuration details.

This command loads the configuration file and displays all supported
project types in priority order.`,
	RunE: listSupportedTypes,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Don't use log.Fatal as it interferes with JSON output format
		os.Exit(1)
	}
}

func init() {
	// Root command flags
	rootCmd.Flags().StringVarP(&path, "path", "p", ".",
		"Path to search for project files or path to a specific file")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "",
		"Path to configuration file (default: configs/default-patterns.yaml)")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "text",
		"Output format: text, json")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose output")
	rootCmd.Flags().BoolVar(&failOnError, "fail-on-error", true,
		"Exit with error code if version cannot be extracted")
	rootCmd.Flags().StringVar(&jsonFormat, "json-format", "pretty",
		"JSON output format: pretty, minimised")
	rootCmd.Flags().BoolVar(&dynamicFallback, "dynamic-fallback", true,
		"Enable dynamic versioning fallback to Git tags")

	// List command flags
	listCmd.Flags().StringVarP(&configPath, "config", "c", "",
		"Path to configuration file (default: configs/default-patterns.yaml)")
	listCmd.Flags().StringVarP(&outputFormat, "format", "f", "text",
		"Output format: text, json")
	listCmd.Flags().StringVar(&jsonFormat, "json-format", "pretty",
		"JSON output format: pretty, minimised")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
}

// runExtractor is the main extraction function
func runExtractor(cmd *cobra.Command, args []string) error {
	// Set default config path if not provided
	if configPath == "" {
		configPath = config.GetDefaultConfigPath()
	}

	// Make config path absolute if relative
	if !filepath.IsAbs(configPath) {
		if wd, err := os.Getwd(); err == nil {
			configPath = filepath.Join(wd, configPath)
		}
	}

	verboseLog(fmt.Sprintf("Loading configuration from: %s", configPath))
	verboseLog(fmt.Sprintf("Searching in path: %s", path))

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// Handle config loading error with proper output format
		return handleError(fmt.Errorf("failed to load configuration: %w", err))
	}

	verboseLog(fmt.Sprintf("Loaded %d project configurations", len(cfg.Projects)))

	// Create extractor
	ext := extractor.NewWithOptions(cfg, dynamicFallback)

	// Extract version
	result, err := ext.Extract(path)
	if err != nil {
		if failOnError {
			return handleError(fmt.Errorf("version extraction failed: %w", err))
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	// Output result
	return outputResult(result, err)
}

// handleError outputs error in the appropriate format and returns the error
func handleError(err error) error {
	if outputFormat == "json" {
		// Output JSON error format
		output := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}

		var data []byte
		var jsonErr error
		if jsonFormat == "pretty" {
			data, jsonErr = json.MarshalIndent(output, "", "  ")
		} else {
			data, jsonErr = json.Marshal(output)
		}
		if jsonErr != nil {
			fallbackOutput := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("JSON marshal error: %s", jsonErr.Error()),
			}
			fallbackData, _ := json.Marshal(fallbackOutput)
			fmt.Fprintln(os.Stderr, string(fallbackData))
		} else {
			fmt.Println(string(data))
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	return err
}

// listSupportedTypes lists all supported project types
func listSupportedTypes(cmd *cobra.Command, args []string) error {
	// Set default config path if not provided
	if configPath == "" {
		configPath = config.GetDefaultConfigPath()
	}

	// Make config path absolute if relative
	if !filepath.IsAbs(configPath) {
		if wd, err := os.Getwd(); err == nil {
			configPath = filepath.Join(wd, configPath)
		}
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Output supported types
	if outputFormat == "json" {
		var data []byte
		var err error
		if jsonFormat == "pretty" {
			data, err = json.MarshalIndent(cfg.Projects, "", "  ")
		} else {
			data, err = json.Marshal(cfg.Projects)
		}
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Supported Project Types (%d total):\n\n",
			len(cfg.Projects))
		for i, project := range cfg.Projects {
			fmt.Printf("%d. %s", i+1, project.Type)
			if project.Subtype != "" {
				fmt.Printf(" (%s)", project.Subtype)
			}
			fmt.Printf("\n   File: %s\n", project.File)
			fmt.Printf("   Priority: %d\n", project.Priority)
			if project.Notes != "" {
				fmt.Printf("   Notes: %s\n", project.Notes)
			}
			fmt.Printf("   Regex patterns: %d\n", len(project.Regex))
			fmt.Printf("   Sample repositories: %d\n\n",
				len(project.Samples))
		}
	}

	return nil
}

// outputResult formats and outputs the extraction result
func outputResult(result *extractor.ExtractResult, extractErr error) error {
	if outputFormat == "json" {
		// Create JSON output structure
		output := map[string]interface{}{
			"success": result != nil && result.Success,
		}

		if result != nil {
			output["version"] = result.Version
			output["project_type"] = result.ProjectType
			output["subtype"] = result.Subtype
			output["file"] = result.File
			output["matched_by"] = result.MatchedBy
			output["version_source"] = result.VersionSource
			if result.GitTag != "" {
				output["git_tag"] = result.GitTag
			}
		}

		if extractErr != nil {
			output["error"] = extractErr.Error()
		}

		var data []byte
		var err error
		if jsonFormat == "pretty" {
			data, err = json.MarshalIndent(output, "", "  ")
		} else {
			data, err = json.Marshal(output)
		}
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Text output
		if result != nil && result.Success {
			fmt.Printf("✅ Version extracted successfully\n")
			fmt.Printf("Version: %s\n", result.Version)
			fmt.Printf("Project Type: %s", result.ProjectType)
			if result.Subtype != "" {
				fmt.Printf(" (%s)", result.Subtype)
			}
			fmt.Printf("\nFile: %s\n", result.File)
			if verbose {
				fmt.Printf("Matched by regex: %s\n", result.MatchedBy)
			}

		} else {
			fmt.Printf("❌ No version found\n")
			if extractErr != nil {
				fmt.Printf("Error: %v\n", extractErr)
			}
		}
	}

	return nil
}
