#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2025 The Linux Foundation

# Consolidated Test Data Generator
# Creates consistent test project samples for all test scenarios

set -euo pipefail

# Default configuration
OUTPUT_DIR="${1:-test-samples}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Clean and create output directory
prepare_output_dir() {
    if [[ -d "$OUTPUT_DIR" ]]; then
        log_warning "Removing existing directory: $OUTPUT_DIR"
        rm -rf "$OUTPUT_DIR"
    fi

    mkdir -p "$OUTPUT_DIR"
    log_info "Created output directory: $OUTPUT_DIR"
}

# Generate JavaScript/Node.js test project
generate_javascript_project() {
    local project_dir="$OUTPUT_DIR/javascript"
    mkdir -p "$project_dir"

    cat > "$project_dir/package.json" << 'EOF'
{
  "name": "test-project",
  "version": "1.2.3",
  "description": "Test JavaScript project for version extraction",
  "main": "index.js",
  "scripts": {
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "keywords": ["test", "version-extraction"],
  "author": "Test Suite",
  "license": "Apache-2.0"
}
EOF

    # Create a simple index.js for completeness
    cat > "$project_dir/index.js" << 'EOF'
// Test JavaScript project
console.log('Version extraction test project');
EOF

    log_success "Generated JavaScript project (version: 1.2.3)"
}

# Generate Python (modern pyproject.toml) test project
generate_python_project() {
    local project_dir="$OUTPUT_DIR/python"
    mkdir -p "$project_dir"

    cat > "$project_dir/pyproject.toml" << 'EOF'
[project]
name = "test-project"
version = "2.1.0"
description = "Test Python project for version extraction"
authors = [
    {name = "Test Suite", email = "test@example.com"}
]
license = {text = "Apache-2.0"}
keywords = ["test", "version-extraction"]
classifiers = [
    "Development Status :: 4 - Beta",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: Apache Software License",
    "Programming Language :: Python :: 3.8",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
    "Programming Language :: Python :: 3.11",
]
requires-python = ">=3.8"

[build-system]
requires = ["setuptools>=61.0"]
build-backend = "setuptools.build_meta"
EOF

    # Create a simple Python module
    mkdir -p "$project_dir/src/test_project"
    cat > "$project_dir/src/test_project/__init__.py" << 'EOF'
"""Test Python project for version extraction."""
__version__ = "2.1.0"
EOF

    log_success "Generated Python project (version: 2.1.0)"
}

# Generate Go module test project
generate_go_project() {
    local project_dir="$OUTPUT_DIR/go"
    mkdir -p "$project_dir"

    cat > "$project_dir/go.mod" << 'EOF'
module github.com/test/project

go 1.24

require (
    github.com/spf13/cobra v1.8.0
)

require (
    github.com/inconshreveable/mousetrap v1.1.0 // indirect
    github.com/spf13/pflag v1.0.5 // indirect
)
EOF

    # Create a simple Go main file
    cat > "$project_dir/main.go" << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Test Go project for version extraction")
}
EOF

    log_success "Generated Go project (version: 1.24)"
}

# Generate Python setup.py (legacy) test project
generate_python_legacy_project() {
    local project_dir="$OUTPUT_DIR/python-legacy"
    mkdir -p "$project_dir"

    cat > "$project_dir/setup.py" << 'EOF'
from setuptools import setup, find_packages

setup(
    name="test-legacy-project",
    version="1.5.2",
    description="Test legacy Python project for version extraction",
    author="Test Suite",
    author_email="test@example.com",
    packages=find_packages(),
    install_requires=[],
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: Apache Software License",
        "Programming Language :: Python :: 3",
    ],
    python_requires=">=3.7",
)
EOF

    log_success "Generated Python legacy project (version: 1.5.2)"
}

# Generate Rust Cargo.toml test project
generate_rust_project() {
    local project_dir="$OUTPUT_DIR/rust"
    mkdir -p "$project_dir/src"

    cat > "$project_dir/Cargo.toml" << 'EOF'
[package]
name = "test-rust-project"
version = "0.3.1"
edition = "2021"
description = "Test Rust project for version extraction"
license = "Apache-2.0"
authors = ["Test Suite <test@example.com>"]
keywords = ["test", "version-extraction"]

[dependencies]
EOF

    cat > "$project_dir/src/main.rs" << 'EOF'
fn main() {
    println!("Test Rust project for version extraction");
}
EOF

    log_success "Generated Rust project (version: 0.3.1)"
}

# Generate Maven pom.xml test project
generate_maven_project() {
    local project_dir="$OUTPUT_DIR/maven"
    mkdir -p "$project_dir/src/main/java/com/test"

    cat > "$project_dir/pom.xml" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.test</groupId>
    <artifactId>test-maven-project</artifactId>
    <version>3.2.1</version>
    <packaging>jar</packaging>

    <name>test-maven-project</name>
    <description>Test Maven project for version extraction</description>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>
</project>
EOF

    cat > "$project_dir/src/main/java/com/test/Main.java" << 'EOF'
package com.test;

public class Main {
    public static void main(String[] args) {
        System.out.println("Test Maven project for version extraction");
    }
}
EOF

    log_success "Generated Maven project (version: 3.2.1)"
}

# Generate Gradle build.gradle test project
generate_gradle_project() {
    local project_dir="$OUTPUT_DIR/gradle"
    mkdir -p "$project_dir/src/main/java/com/test"

    cat > "$project_dir/build.gradle" << 'EOF'
plugins {
    id 'java'
    id 'application'
}

group = 'com.test'
version = '2.4.0'
description = 'Test Gradle project for version extraction'

java {
    sourceCompatibility = JavaVersion.VERSION_11
    targetCompatibility = JavaVersion.VERSION_11
}

application {
    mainClass = 'com.test.Main'
}

repositories {
    mavenCentral()
}

dependencies {
    testImplementation 'junit:junit:4.13.2'
}
EOF

    cat > "$project_dir/src/main/java/com/test/Main.java" << 'EOF'
package com.test;

public class Main {
    public static void main(String[] args) {
        System.out.println("Test Gradle project for version extraction");
    }
}
EOF

    log_success "Generated Gradle project (version: 2.4.0)"
}

# Generate empty project (for error testing)
generate_empty_project() {
    local project_dir="$OUTPUT_DIR/empty"
    mkdir -p "$project_dir"

    cat > "$project_dir/README.md" << 'EOF'
# Empty Test Project

This project contains no version information files.
Used for testing error handling scenarios.
EOF

    log_success "Generated empty project (no version)"
}

# Generate malformed project (for error testing)
generate_malformed_project() {
    local project_dir="$OUTPUT_DIR/malformed"
    mkdir -p "$project_dir"

    # Create malformed package.json
    cat > "$project_dir/package.json" << 'EOF'
{
  "name": "malformed-project",
  "version": "invalid-version-format-!@#$%",
  "description": "Project with malformed version for testing"
  // This JSON is intentionally malformed with comments
EOF

    log_success "Generated malformed project (invalid version)"
}

# Generate project with dynamic versioning indicators
generate_dynamic_version_project() {
    local project_dir="$OUTPUT_DIR/dynamic"
    mkdir -p "$project_dir"

    cat > "$project_dir/package.json" << 'EOF'
{
  "name": "dynamic-version-project",
  "version": "0.0.0-development",
  "description": "Project using dynamic versioning",
  "scripts": {
    "semantic-release": "semantic-release",
    "test": "echo \"Dynamic version test\""
  },
  "devDependencies": {
    "semantic-release": "^21.0.0"
  }
}
EOF

    log_success "Generated dynamic versioning project (0.0.0-development)"
}

# Generate comprehensive test manifest
generate_test_manifest() {
    cat > "$OUTPUT_DIR/TEST_MANIFEST.md" << 'EOF'
# Test Project Samples Manifest

Generated test projects for version extraction testing.

## Standard Projects

| Project Type | Directory | Expected Version | Expected Type |
|--------------|-----------|------------------|---------------|
| JavaScript   | javascript | 1.2.3 | JavaScript |
| Python (Modern) | python | 2.1.0 | Python |
| Python (Legacy) | python-legacy | 1.5.2 | Python |
| Go Module    | go | 1.24 | Go |
| Rust Cargo   | rust | 0.3.1 | Rust |
| Maven        | maven | 3.2.1 | Java |
| Gradle       | gradle | 2.4.0 | Java |

## Error Testing Projects

| Project Type | Directory | Purpose | Expected Behavior |
|--------------|-----------|---------|-------------------|
| Empty | empty | No version files | Should fail or return empty |
| Malformed | malformed | Invalid JSON | Should handle gracefully |
| Dynamic | dynamic | Dynamic versioning | Should detect dynamic indicators |

## Usage

### In Shell Scripts:
```bash
# Generate samples
./test/generate-samples.sh test-samples

# Test a specific project
./version-extract --path test-samples/javascript --format json

# Test all projects
for dir in test-samples/*/; do
  echo "Testing $(basename "$dir")"
  ./version-extract --path "$dir" --format json
done
```

### In GitHub Actions:
```yaml
- name: Generate test samples
  run: ./test/generate-samples.sh test-samples

- name: Test JavaScript project
  uses: ./
  with:
    path: test-samples/javascript
    format: json
```

### In Makefile:
```make
test-samples: build
	./test/generate-samples.sh test-samples
	@for dir in test-samples/*/; do \
		echo "Testing $$(basename "$$dir")"; \
		./version-extract --path "$$dir" --format json || exit 1; \
	done
```
EOF

    log_success "Generated test manifest"
}

# Main execution
main() {
    log_info "Starting consolidated test data generation"
    log_info "Output directory: $OUTPUT_DIR"

    prepare_output_dir

    # Generate all test projects
    generate_javascript_project
    generate_python_project
    generate_python_legacy_project
    generate_go_project
    generate_rust_project
    generate_maven_project
    generate_gradle_project
    generate_empty_project
    generate_malformed_project
    generate_dynamic_version_project

    # Generate documentation
    generate_test_manifest

    log_success "Test data generation completed!"
    log_info "Generated $(find "$OUTPUT_DIR" -maxdepth 1 -type d | wc -l | tr -d ' ') test projects"

    # Show summary
    echo ""
    echo "📋 Generated Projects:"
    for dir in "$OUTPUT_DIR"/*/; do
        if [[ -d "$dir" ]]; then
            project_name=$(basename "$dir")
            echo "  - $project_name"
        fi
    done
    echo ""
    echo "📖 See $OUTPUT_DIR/TEST_MANIFEST.md for usage instructions"
}

# Help function
show_help() {
    cat << 'EOF'
Consolidated Test Data Generator

USAGE:
    generate-samples.sh [OUTPUT_DIR]

ARGUMENTS:
    OUTPUT_DIR    Directory to create test samples in (default: test-samples)

EXAMPLES:
    # Generate in default directory
    ./generate-samples.sh

    # Generate in custom directory
    ./generate-samples.sh my-test-data

    # Clean existing and regenerate
    rm -rf test-samples && ./generate-samples.sh

GENERATED PROJECTS:
    - javascript     (package.json with version 1.2.3)
    - python         (pyproject.toml with version 2.1.0)
    - python-legacy  (setup.py with version 1.5.2)
    - go             (go.mod with Go version 1.24)
    - rust           (Cargo.toml with version 0.3.1)
    - maven          (pom.xml with version 3.2.1)
    - gradle         (build.gradle with version 2.4.0)
    - empty          (no version files - for error testing)
    - malformed      (invalid data - for error testing)
    - dynamic        (dynamic versioning indicators)

EOF
}

# Handle command line arguments
case "${1:-}" in
    -h|--help|help)
        show_help
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac
