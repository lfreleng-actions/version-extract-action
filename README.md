<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: 2025 The Linux Foundation
-->

# 🔍 Version Extract Action

A lightweight GitHub Action and CLI tool that extracts version strings from
diverse software project types and files using configurable YAML patterns.
Features dynamic versioning detection with fallback to Git tags.

## version-extract-action

## Overview

This tool automatically detects and extracts version information from popular
project file formats including JavaScript/Node.js, Python, Java, C#/.NET, Go,
PHP, Ruby, Rust, Swift, Dart/Flutter, and others. It supports both
directory scanning and specific file analysis, with intelligent detection
of dynamic versioning patterns and automatic Git tag fallback. Can output as
either plain text or JSON, with pretty and minimised JSON formatting options.

## Usage Examples

### GitHub Action

```yaml
steps:
  - name: "Extract Project Version"
    id: version-extract
    uses: lfreleng-actions/version-extract-action@main
    with:
      path: "."
      format: "json"
      json_format: "pretty"
      fail-on-error: true
```

### CLI Tool

```bash
# Extract from current directory
./version-extract --path .

# Extract from specific file
./version-extract --path package.json --format json

# Pretty formatted JSON output
./version-extract --path . --format json --json-format pretty

# Minimised JSON output
./version-extract --path . --format json --json-format minimised

# List supported project types
./version-extract list --format json
```

## GitHub Action Inputs

<!-- markdownlint-disable MD013 -->

| Name             | Required | Default  | Description                                                 |
| ---------------- | -------- | -------- | ----------------------------------------------------------- |
| path             | false    | "."      | Path to search for project files or path to a specific file |
| config           | false    | ""       | Path to custom configuration file                           |
| format           | false    | "text"   | Output format (text or json)                                |
| verbose          | false    | "false"  | Enable verbose output                                       |
| fail-on-error.   | false    | "true"   | Fail the action if version extraction fails                 |
| json_format      | false    | "pretty" | JSON output format: pretty, minimised                       |
| dynamic-fallback | false    | "true"   | Enable dynamic versioning fallback to Git tags              |

<!-- markdownlint-enable MD013 -->

## GitHub Action Outputs

<!-- markdownlint-disable MD013 -->

| Name           | Description                                  |
| -------------- | -------------------------------------------- |
| version        | Extracted version string                     |
| project-type   | Detected project type                        |
| file           | File containing the extracted version        |
| success        | Whether version extraction was successful    |
| version-source | Source of version: static or dynamic-git-tag |
| git-tag        | Original Git tag when using dynamic fallback |

<!-- markdownlint-enable MD013 -->

## CLI Options

<!-- markdownlint-disable MD013 -->

| Flag               | Short | Default  | Description                                                 |
| ------------------ | ----- | -------- | ----------------------------------------------------------- |
| --path             | -p    | "."      | Path to search for project files or path to a specific file |
| --config           | -c    | ""       | Path to configuration file                                  |
| --format           | -f    | "text"   | Output format: text, json                                   |
| --verbose          | -v    | false    | Enable verbose output                                       |
| --fail-on-error    |       | true     | Exit with error code if version extraction fails            |
| --json-format      |       | "pretty" | JSON output format: pretty, minimised                       |
| --dynamic-fallback |       | true     | Enable dynamic versioning fallback to Git tags              |

<!-- markdownlint-enable MD013 -->

## Supported Project Types

The tool supports extraction from the following project types (in priority
order):

### Programming Languages & Frameworks

1. **JavaScript (npm)** - `package.json`
2. **Python (Modern)** - `pyproject.toml`
3. **Java (Maven)** - `pom.xml`
4. **Java (Gradle)** - `build.gradle`, `build.gradle.kts`
5. **C# (.NET)** - `*.csproj`
6. **Go** - `go.mod`
7. **PHP (Composer)** - `composer.json`
8. **Ruby (Gemspec)** - `*.gemspec`
9. **Python (Legacy)** - `setup.py`, `setup.cfg`, `__init__.py`
10. **Rust (Cargo)** - `Cargo.toml`
11. **Swift (Package Manager)** - `Package.swift`
12. **Dart/Flutter** - `pubspec.yaml`
13. **C/C++ (CMake)** - `CMakeLists.txt`
14. **Elixir (Mix)** - `mix.exs`
15. **Scala (SBT)** - `build.sbt`
16. **Haskell (Cabal)** - `*.cabal`
17. **Julia** - `Project.toml`, `Manifest.toml`
18. **R** - `DESCRIPTION`
19. **Perl** - `*.pm`, `*.pl`
20. **Lua (LuaRocks)** - `*.rockspec`

### Infrastructure & Deployment

1. **Helm Charts** - `Chart.yaml`
2. **Docker** - `Dockerfile`
3. **Terraform** - `versions.tf`
4. **Ansible (Galaxy)** - `galaxy.yml`
5. **Ansible (Role)** - `meta/main.yml`
6. **Kubernetes** - `*.yaml` (with version annotations)
7. **Docker Compose** - `docker-compose.yml`

### Application Packaging

1. **Snap Packages** - `snapcraft.yaml`
2. **Homebrew Formulas** - `*.rb`
3. **Flatpak** - `*.flatpak.yml`
4. **AppImage** - `*.desktop`

### Development Tools & Extensions

1. **OpenAPI/Swagger** - `openapi.yaml`, `swagger.yaml`
2. **VSCode Extensions** - `package.json`
3. **Web Extensions** - `manifest.json`
4. **GitHub Actions** - `action.yml`

### Build Systems

1. **Gradle Properties** - `gradle.properties`
2. **Meson** - `meson.build`
3. **Makefile** - `Makefile`
4. **Yarn Workspaces** - `yarn.lock`

## Path Handling

The tool supports two modes of operation:

### Directory Mode (Default)

When `path` points to a directory, the tool searches through the
directory structure looking for supported project files in priority order.

```yaml
with:
  path: "."  # Search current directory
```

### File Mode

When `path` points to a specific file, the tool validates that the file
is of a supported type and attempts version extraction directly.

```yaml
with:
  path: "package.json"  # Extract from specific file
```

If the specified file is not of a supported type, the action will fail unless
`fail-on-error` has a `false` value.

## JSON Output Format

### Pretty Format (default)

```json
{
  "success": true,
  "version": "1.2.3",
  "project_type": "JavaScript",
  "subtype": "npm",
  "file": "./package.json",
  "matched_by": "\"version\":\\s*\"([^\"]+)\"",
  "version_source": "static"
}
```

### Dynamic Versioning Example

```json
{
  "success": true,
  "version": "2.1.0",
  "project_type": "Python",
  "subtype": "Modern (pyproject.toml)",
  "file": "./pyproject.toml",
  "matched_by": "dynamic-git-tag",
  "version_source": "dynamic-git-tag",
  "git_tag": "v2.1.0"
}
```

### Minimised Format

```json
{"success":true,"version":"1.2.3","project_type":"JavaScript","subtype":"npm","file":"./package.json","matched_by":"\"version\":\\s*\"([^\"]+)\"","version_source":"static"}
```

## Error Handling

The tool provides comprehensive error handling:

- **File not found**: Reports when specified files don't exist
- **Unsupported file type**: Reports when a specific file is not supported
- **Invalid version format**: Reports when extracted versions don't match
  expected patterns
- **Configuration errors**: Reports configuration file parsing issues

When `fail-on-error` is `true` (default), the action will fail on errors. When
set to `false`, errors become warnings and the action continues.

## Dynamic Versioning Support

The tool intelligently detects projects using dynamic versioning and
automatically falls back to extracting version information from Git tags.

### Supported Dynamic Versioning Patterns

- **Python**: `setuptools_scm`, `versioneer`, `dynamic = ["version"]` in pyproject.toml
- **JavaScript**: `semantic-release`, development versions like `0.0.0-development`
- **Rust**: Development versions `0.0.0`, `build.rs` scripts
- **Java**: Maven `${revision}` variables, `SNAPSHOT` versions
- **Go**: GitHub/GitLab hosted modules relying on Git tags
- **C#**: Dynamic versioning with build-time resolution

### Dynamic Versioning Control

```yaml
# Enable dynamic versioning (default)
with:
  dynamic-fallback: "true"

# Disable dynamic versioning fallback
with:
  dynamic-fallback: "false"
```

When the tool finds dynamic versioning, it:

1. Identifies dynamic version indicators in project files
2. Attempts to extract the latest semantic version from Git tags
3. Returns the Git tag version with `version_source: "dynamic-git-tag"`
4. Includes the original Git tag in the `git_tag` field

### Git Tag Formats

The tool supports different Git tag formats:

- Semantic versioning: `v1.2.3`, `1.2.3`
- Release prefixes: `release-1.2.3`, `rel-1.2.3`
- Date-based: `2024.01.15`
- Pre-release: `1.2.3-beta.1`, `1.2.3-rc.1`

## Custom Configuration

You can provide a custom configuration file to extend or change the supported
project types:

```yaml
with:
  config: "path/to/custom-patterns.yaml"
```

Configuration files use YAML format with project definitions including file
patterns, regex patterns, dynamic versioning indicators, and metadata.

## Implementation Details

- Built with Go for fast, reliable performance
- Uses configurable regex patterns for version extraction
- Intelligent dynamic versioning detection with Git integration
- Supports semantic versioning and custom version formats
- Git tag extraction with fallback strategies
- Validates extracted versions against common patterns
- Provides detailed logging and error reporting
- Compatible with GitHub Actions and standalone CLI usage
- Backward compatible with existing static version extraction

## Development

For detailed development instructions, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

### Quick Start

```bash
# Install pre-commit hooks (runs Go fmt, vet, and other checks automatically)
pre-commit install

# Quick development cycle
make dev

# Full CI validation
make ci
```

### Pre-commit Hooks

Git hooks run automatically before each commit, matching CI requirements:

- **go fmt** - Code formatting (`gofmt -l .`)
- **go vet** - Static analysis (`go vet ./...`)
- **go mod verify** - Dependency verification
- **go mod tidy** - Dependency cleanup

These hooks prevent CI failures by catching issues locally before pushing.

### Common Commands

```bash
# Format code
make fmt

# Run tests
make test

# Run all linters
make lint-full

# Build binary
make build
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for complete documentation.
