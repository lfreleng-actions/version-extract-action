// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lfreleng-actions/version-extract-action/internal/config"
)

// kotlinAndroidConfig returns a config mirroring the real
// "Android - build.gradle.kts" project type (literal versionName only).
func kotlinAndroidConfig() *config.Config {
	return &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:    "Android",
				Subtype: "Gradle (Kotlin)",
				File:    "build.gradle.kts",
				Regex: []string{
					`versionName\s*=\s*"([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:[-\.][a-zA-Z0-9]+)?)"`,
				},
				Priority: 1,
			},
		},
	}
}

// writeFile is a small test helper that creates parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestResolveVersionConstantFromBuildSrc reproduces the NewPipe idiom: an
// app build script that sets versionName from a constant defined in buildSrc.
func TestResolveVersionConstantFromBuildSrc(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "app", "build.gradle.kts"), `android {
    defaultConfig {
        applicationId = "org.example.app"
        versionCode = 1013
        versionName = NEWPIPE_VERSION_NAME
    }
}`)
	writeFile(t, filepath.Join(tmpDir, "buildSrc", "src", "main", "kotlin", "ProjectConfig.kt"), `const val NEWPIPE_VERSION_CODE = 1013
const val NEWPIPE_VERSION_NAME = "0.28.8"`)

	result, err := New(kotlinAndroidConfig()).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful extraction")
	}
	if result.Version != "0.28.8" {
		t.Errorf("expected version 0.28.8, got %q", result.Version)
	}
	if result.VersionSource != "static-constant" {
		t.Errorf("expected version_source static-constant, got %q", result.VersionSource)
	}
	if result.ProjectType != "Android" {
		t.Errorf("expected Android, got %q", result.ProjectType)
	}
}

// TestResolveVersionConstantTypedVal covers a typed `const val NAME: String`.
func TestResolveVersionConstantTypedVal(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "build.gradle.kts"),
		"version = APP_VERSION\n")
	writeFile(t, filepath.Join(tmpDir, "buildSrc", "Versions.kt"),
		`const val APP_VERSION: String = "2.5.1"`)

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "Kotlin",
				Subtype:  "Gradle",
				File:     "build.gradle.kts",
				Regex:    []string{`version\s*=\s*"([0-9]+\.[0-9]+(?:\.[0-9]+)?)"`},
				Priority: 1,
			},
		},
	}

	result, err := New(cfg).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.Version != "2.5.1" {
		t.Errorf("expected version 2.5.1, got %q", result.Version)
	}
}

// TestLiteralVersionStillPreferred ensures a literal versionName is used
// directly and the constant fallback does not interfere.
func TestLiteralVersionStillPreferred(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "app", "build.gradle.kts"), `android {
    defaultConfig {
        versionName = "1.4.2"
    }
}`)

	result, err := New(kotlinAndroidConfig()).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.Version != "1.4.2" {
		t.Errorf("expected version 1.4.2, got %q", result.Version)
	}
	if result.VersionSource != "static" {
		t.Errorf("expected version_source static, got %q", result.VersionSource)
	}
}

// TestUnresolvedConstantFails ensures a reference with no resolvable definition
// does not produce a (wrong) version.
func TestUnresolvedConstantFails(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "app", "build.gradle.kts"), `android {
    defaultConfig {
        versionName = MISSING_VERSION_CONSTANT
    }
}`)

	result, err := New(kotlinAndroidConfig()).Extract(tmpDir)
	if err == nil && result != nil && result.Success {
		t.Errorf("expected no extraction, got version %q", result.Version)
	}
}

// TestResolveVersionConstantQualifiedAssignment covers a qualified left-hand
// side such as `project.version = APP_VERSION` (Copilot review feedback).
func TestResolveVersionConstantQualifiedAssignment(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "build.gradle.kts"),
		"project.version = APP_VERSION\n")
	writeFile(t, filepath.Join(tmpDir, "buildSrc", "Versions.kt"),
		`const val APP_VERSION = "3.1.4"`)

	cfg := &config.Config{
		Projects: []config.ProjectConfig{
			{
				Type:     "Kotlin",
				Subtype:  "Gradle",
				File:     "build.gradle.kts",
				Regex:    []string{`version\s*=\s*"([0-9]+\.[0-9]+(?:\.[0-9]+)?)"`},
				Priority: 1,
			},
		},
	}

	result, err := New(cfg).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.Version != "3.1.4" {
		t.Errorf("expected version 3.1.4, got %q", result.Version)
	}
}

// TestResolveVersionConstantSpecificFile covers passing a build script path
// directly (not its directory), resolving via the discovered project root
// (Copilot review feedback).
func TestResolveVersionConstantSpecificFile(t *testing.T) {
	tmpDir := t.TempDir()

	// A root marker so buildSrc is discoverable when walking up from app/.
	writeFile(t, filepath.Join(tmpDir, "settings.gradle.kts"), "")
	appScript := filepath.Join(tmpDir, "app", "build.gradle.kts")
	writeFile(t, appScript, `android {
    defaultConfig {
        versionName = NEWPIPE_VERSION_NAME
    }
}`)
	writeFile(t, filepath.Join(tmpDir, "buildSrc", "ProjectConfig.kt"),
		`const val NEWPIPE_VERSION_NAME = "0.28.8"`)

	// Pass the specific file, not the directory.
	result, err := New(kotlinAndroidConfig()).Extract(appScript)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.Version != "0.28.8" {
		t.Errorf("expected version 0.28.8, got %q", result.Version)
	}
	if result.VersionSource != "static-constant" {
		t.Errorf("expected version_source static-constant, got %q", result.VersionSource)
	}
}

// TestResolveVersionConstantIgnoresComments ensures commented-out assignments
// and definitions are not mistaken for real ones (Copilot review feedback).
func TestResolveVersionConstantIgnoresComments(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "app", "build.gradle.kts"), `android {
    defaultConfig {
        // versionName = FAKE_VERSION  (example in a comment, must be ignored)
        versionName = APP_VERSION
    }
}`)
	writeFile(t, filepath.Join(tmpDir, "buildSrc", "ProjectConfig.kt"),
		"// const val FAKE_VERSION = \"9.9.9\"\nconst val APP_VERSION = \"1.2.3\"\n")

	result, err := New(kotlinAndroidConfig()).Extract(tmpDir)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	// Without comment stripping this would wrongly resolve FAKE_VERSION=9.9.9.
	if result.Version != "1.2.3" {
		t.Errorf("expected 1.2.3 (commented assignment/definition ignored), got %q", result.Version)
	}
}
