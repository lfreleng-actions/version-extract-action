// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReader_ReadFileContent(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with test content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "line1\r\nline2\nline3\r"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with normalization
	content, err := fr.ReadFileContent(testFile, true)
	if err != nil {
		t.Fatalf("ReadFileContent failed: %v", err)
	}

	expected := "line1\nline2\nline3\n"
	if content != expected {
		t.Errorf("Expected normalized content %q, got %q", expected, content)
	}

	// Test without normalization
	content, err = fr.ReadFileContent(testFile, false)
	if err != nil {
		t.Fatalf("ReadFileContent failed: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected original content %q, got %q", testContent, content)
	}
}

func TestFileReader_ProcessFileLineByLine(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with test content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "version: 1.0.0\nname: test\nversion: 2.0.0\n"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test finding first version
	result, err := fr.ProcessFileLineByLine(testFile, func(line string) (string, bool) {
		if strings.Contains(line, "version:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), true
			}
		}
		return "", false
	})

	if err != nil {
		t.Fatalf("ProcessFileLineByLine failed: %v", err)
	}

	if result != "1.0.0" {
		t.Errorf("Expected '1.0.0', got %q", result)
	}
}

func TestFileReader_ProcessFileLineByLine_NoMatch(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with test content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "name: test\ndescription: test package\n"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test finding version (should not find any)
	result, err := fr.ProcessFileLineByLine(testFile, func(line string) (string, bool) {
		if strings.Contains(line, "version:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), true
			}
		}
		return "", false
	})

	if err != nil {
		t.Fatalf("ProcessFileLineByLine failed: %v", err)
	}

	if result != "" {
		t.Errorf("Expected empty result, got %q", result)
	}
}

func TestFileReader_ValidateFileSize(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with small content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "small content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test valid size
	err = fr.ValidateFileSize(testFile)
	if err != nil {
		t.Errorf("ValidateFileSize should pass for small file: %v", err)
	}

	// Test non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")
	err = fr.ValidateFileSize(nonExistentFile)
	if err == nil {
		t.Error("ValidateFileSize should fail for non-existent file")
	}
}

func TestFileReader_GetFileSize(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with known content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "test content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	size, err := fr.GetFileSize(testFile)
	if err != nil {
		t.Fatalf("GetFileSize failed: %v", err)
	}

	expectedSize := int64(len(testContent))
	if size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, size)
	}
}

func TestFileReader_IsFileSizeWithinLimit(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with small content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "small content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test small file
	if !fr.IsFileSizeWithinLimit(testFile) {
		t.Error("Small file should be within limit")
	}

	// Test non-existent file
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")
	if fr.IsFileSizeWithinLimit(nonExistentFile) {
		t.Error("Non-existent file should not be within limit")
	}
}

func TestFileReader_ReadFileContentWithFallback(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with test content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "version: 1.0.0\nother content\nmore content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful line processing
	result, err := fr.ReadFileContentWithFallback(testFile,
		func(line string) (string, bool) {
			if strings.Contains(line, "version:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1]), true
				}
			}
			return "", false
		},
		func(content string) (string, error) {
			t.Error("Should not reach full content processor")
			return "", nil
		},
	)

	if err != nil {
		t.Fatalf("ReadFileContentWithFallback failed: %v", err)
	}

	if result != "1.0.0" {
		t.Errorf("Expected '1.0.0', got %q", result)
	}
}

func TestFileReader_ReadFileContentWithFallback_FallbackCase(t *testing.T) {
	fr := NewFileReader()

	// Create a temporary file with test content
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "no version here\nother content\nmore content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test fallback to full content processing
	result, err := fr.ReadFileContentWithFallback(testFile,
		func(line string) (string, bool) {
			if strings.Contains(line, "version:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1]), true
				}
			}
			return "", false
		},
		func(content string) (string, error) {
			if strings.Contains(content, "other content") {
				return "found from full content", nil
			}
			return "", nil
		},
	)

	if err != nil {
		t.Fatalf("ReadFileContentWithFallback failed: %v", err)
	}

	if result != "found from full content" {
		t.Errorf("Expected 'found from full content', got %q", result)
	}
}

func TestSetGetFileReader(t *testing.T) {
	original := GetFileReader()

	// Create a mock file reader
	mockReader := &FileReader{}

	SetFileReader(mockReader)

	if GetFileReader() != mockReader {
		t.Error("SetFileReader/GetFileReader did not work correctly")
	}

	// Restore original
	SetFileReader(original)

	if GetFileReader() != original {
		t.Error("Failed to restore original file reader")
	}
}

func TestFileReader_ErrorHandling(t *testing.T) {
	fr := NewFileReader()

	// Test reading non-existent file
	_, err := fr.ReadFileContent("/nonexistent/file.txt", true)
	if err == nil {
		t.Error("ReadFileContent should fail for non-existent file")
	}

	// Test processing non-existent file
	_, err = fr.ProcessFileLineByLine("/nonexistent/file.txt", func(line string) (string, bool) {
		return "", false
	})
	if err == nil {
		t.Error("ProcessFileLineByLine should fail for non-existent file")
	}
}
