// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FileReaderInterface defines the interface for file reading operations
type FileReaderInterface interface {
	ReadFileContent(filePath string, normalizeContent bool) (string, error)
	ProcessFileLineByLine(filePath string, processor func(string) (string, bool)) (string, error)
	ValidateFileSize(filePath string) error
	ReadFileContentWithFallback(filePath string, lineProcessor func(string) (string, bool), fullContentProcessor func(string) (string, error)) (string, error)
	GetFileSize(filePath string) (int64, error)
	IsFileSizeWithinLimit(filePath string) bool
}

// FileReader provides centralized file reading utilities
type FileReader struct{}

// NewFileReader creates a new FileReader instance
func NewFileReader() FileReaderInterface {
	return &FileReader{}
}

// Global instance for use throughout the package
var fileReader FileReaderInterface = NewFileReader()

// ValidateFileSize checks if file size is within acceptable limits
func (fr *FileReader) ValidateFileSize(filePath string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.Size() > maxFileSizeLimit {
		return fmt.Errorf("file size exceeds limit of 10MB: %s", filePath)
	}

	return nil
}

// ReadFileContent reads the entire file content with optional normalization
func (fr *FileReader) ReadFileContent(filePath string, normalizeContent bool) (string, error) {
	// Validate file size first
	if err := fr.ValidateFileSize(filePath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	fileContent := string(content)

	if normalizeContent {
		// Normalize line endings and excessive whitespace for better pattern matching
		fileContent = strings.ReplaceAll(fileContent, "\r\n", "\n")
		fileContent = strings.ReplaceAll(fileContent, "\r", "\n")
	}

	return fileContent, nil
}

// ProcessFileLineByLine processes a file line by line with a custom processor function
// The processor function receives each line and returns (result, shouldStop)
// If shouldStop is true, processing stops and the result is returned
func (fr *FileReader) ProcessFileLineByLine(filePath string, processor func(string) (string, bool)) (string, error) {
	// Validate file size first
	if err := fr.ValidateFileSize(filePath); err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		result, shouldStop := processor(line)
		if shouldStop {
			return result, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// No result found
	return "", nil
}

// ReadFileContentWithFallback attempts efficient line-by-line processing first,
// then falls back to full content reading if needed
func (fr *FileReader) ReadFileContentWithFallback(filePath string, lineProcessor func(string) (string, bool), fullContentProcessor func(string) (string, error)) (string, error) {
	// Try line-by-line processing first (more memory efficient)
	if lineProcessor != nil {
		result, err := fr.ProcessFileLineByLine(filePath, lineProcessor)
		if err != nil {
			return "", err
		}
		if result != "" {
			return result, nil
		}
	}

	// Fall back to full content processing
	if fullContentProcessor != nil {
		content, err := fr.ReadFileContent(filePath, true)
		if err != nil {
			return "", err
		}
		return fullContentProcessor(content)
	}

	return "", nil
}

// GetFileSize returns the size of the file in bytes
func (fr *FileReader) GetFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return fileInfo.Size(), nil
}

// IsFileSizeWithinLimit checks if file is within the size limit without reading it
func (fr *FileReader) IsFileSizeWithinLimit(filePath string) bool {
	size, err := fr.GetFileSize(filePath)
	if err != nil {
		return false
	}
	return size <= maxFileSizeLimit
}

// SetFileReader allows replacing the global file reader (useful for testing)
func SetFileReader(reader FileReaderInterface) {
	fileReader = reader
}

// GetFileReader returns the current global file reader
func GetFileReader() FileReaderInterface {
	return fileReader
}
