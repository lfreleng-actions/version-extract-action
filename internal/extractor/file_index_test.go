// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"path/filepath"
	"testing"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

// TestMatchPathPattern guards against the basename-only lookup regression:
// a non-glob project-file pattern that includes a path component (e.g. the
// Ansible role type's "meta/main.yml") must still be found.
func TestMatchPathPattern(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "meta", "main.yml"),
		"galaxy_info:\n  version: \"2.3.4\"\n")

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "Ansible",
				Subtype:  "Role",
				File:     "meta/main.yml",
				Regex:    []string{`version:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?`},
				Priority: 1,
			},
		},
	}

	result, err := New(cfg).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success for meta/main.yml pattern, got: %v", err)
	}
	if result.Version != "2.3.4" {
		t.Errorf("expected version 2.3.4, got %q", result.Version)
	}
	if result.ProjectType != "Ansible" {
		t.Errorf("expected Ansible, got %q", result.ProjectType)
	}
}
