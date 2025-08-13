// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestHandleErrorJSONOutput(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat string
		jsonFormat   string
		expectedJSON bool
	}{
		{
			name:         "JSON format with pretty output",
			outputFormat: "json",
			jsonFormat:   "pretty",
			expectedJSON: true,
		},
		{
			name:         "JSON format with minimized output",
			outputFormat: "json",
			jsonFormat:   "minimised",
			expectedJSON: true,
		},
		{
			name:         "Text format",
			outputFormat: "text",
			jsonFormat:   "pretty",
			expectedJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set global variables to simulate command line flags
			originalOutputFormat := outputFormat
			originalJsonFormat := jsonFormat

			outputFormat = tt.outputFormat
			jsonFormat = tt.jsonFormat

			// Capture stdout
			originalStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Capture stderr
			originalStderr := os.Stderr
			rErr, wErr, _ := os.Pipe()
			os.Stderr = wErr

			// Test handleError function
			err := handleError(testError("test error message"))

			// Close writers and restore
			w.Close()
			wErr.Close()
			os.Stdout = originalStdout
			os.Stderr = originalStderr

			// Read captured output
			stdoutData := make([]byte, 1024)
			n, _ := r.Read(stdoutData)
			stdoutOutput := string(stdoutData[:n])

			stderrData := make([]byte, 1024)
			nErr, _ := rErr.Read(stderrData)
			stderrOutput := string(stderrData[:nErr])

			// Verify that handleError returns the original error
			if err == nil {
				t.Errorf("Expected handleError to return an error")
			}
			if err.Error() != "test error message" {
				t.Errorf("Expected returned error message 'test error message', got: %v", err.Error())
			}

			if tt.expectedJSON {
				// Verify JSON output
				var result map[string]interface{}
				unmarshalErr := json.Unmarshal([]byte(stdoutOutput), &result)
				if unmarshalErr != nil {
					t.Errorf("Expected valid JSON output, got error: %v\nOutput: %s", unmarshalErr, stdoutOutput)
				}

				if success, ok := result["success"].(bool); !ok || success {
					t.Errorf("Expected success=false in JSON output, got: %v", result["success"])
				}

				if errorMsg, ok := result["error"].(string); !ok || errorMsg != "test error message" {
					t.Errorf("Expected error message in JSON output, got: %v", result["error"])
				}
			} else {
				// Verify text output goes to stderr
				if stderrOutput == "" {
					t.Errorf("Expected error message in stderr for text format")
				}
			}

			// Restore original values
			outputFormat = originalOutputFormat
			jsonFormat = originalJsonFormat
		})
	}
}

// testError creates a simple error for testing
func testError(msg string) error {
	return &simpleError{msg}
}

type simpleError struct {
	message string
}

func (e *simpleError) Error() string {
	return e.message
}
