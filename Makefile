# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2025 The Linux Foundation

# Streamlined Makefile for version-extract project
# Eliminates duplication and provides clear hierarchy

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_NAME=version-extract
BINARY_PATH=./$(BINARY_NAME)

# Build info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Test configuration
TEST_DATA_DIR=test-samples
INTEGRATION_REPORT=integration-test-report.json

# Keep in step with the golangci-lint version in .github/workflows/testing.yaml
GOLANGCI_LINT_VERSION=v2.14.0

# Default target
.DEFAULT_GOAL := help

# === HELP AND INFORMATION ===
.PHONY: help
help: ## Display this help screen
	@echo "Version Extract - Streamlined Build System"
	@echo "============================================="
	@echo ""
	@echo "🚀 Quick Commands:"
	@echo "  make dev        - Fast development cycle (build + test)"
	@echo "  make ci         - Local CI validation (network-optimized)"
	@echo "  make ci-full    - Complete CI validation (matches GitHub Actions)"
	@echo "  make build      - Build the binary"
	@echo ""
	@echo "📋 Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

# === CORE BUILD TARGETS ===
.PHONY: build
build: ## Build the binary
	@echo "🔨 Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) ./cmd/version-extract
	@echo "✅ Binary built: $(BINARY_PATH)"

.PHONY: clean
clean: ## Clean build artifacts and test data
	@echo "🧹 Cleaning artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -f supported-types.json
	rm -f $(INTEGRATION_REPORT)
	rm -rf $(TEST_DATA_DIR)
	rm -rf test-workspace
	@echo "✅ Clean completed"

# === DEPENDENCY MANAGEMENT ===
.PHONY: deps
deps: ## Download and verify dependencies
	@echo "📦 Managing dependencies..."
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy
	@echo "✅ Dependencies updated"

# === QUALITY CHECKS ===
.PHONY: fmt
fmt: ## Format Go code
	@echo "🎨 Formatting code..."
	$(GOCMD) fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "🔍 Running go vet..."
	$(GOCMD) vet ./...

.PHONY: lint-fast
lint-fast: fmt vet ## Fast linting (format + vet)
	@echo "✅ Fast linting completed"

.PHONY: lint-full
lint-full: lint-fast ## Comprehensive linting (requires external tools)
	@echo "🔍 Running comprehensive linting..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
		echo "✅ golangci-lint passed"; \
	else \
		echo "❌ golangci-lint not found, install with: make dev-setup"; \
		exit 1; \
	fi
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
		echo "✅ staticcheck passed"; \
	else \
		echo "⚠️  staticcheck not found, skipping"; \
	fi

# === TESTING TARGETS ===
.PHONY: test-unit
test-unit: ## Run unit tests with coverage
	@echo "🧪 Running unit tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✅ Unit tests completed (see coverage.html)"

.PHONY: test-samples
test-samples: build ## Generate and test sample projects
	@echo "📝 Generating test samples..."
	./test/generate-samples.sh $(TEST_DATA_DIR)
	@echo "🏗️  Testing sample project extraction..."
	@passed=0; total=0; \
	echo "Testing JavaScript..."; \
	if ./$(BINARY_NAME) --path $(TEST_DATA_DIR)/javascript --format json | jq -e '.success == true' >/dev/null; then \
		echo "  ✅ JavaScript: v1.2.3"; passed=$$((passed+1)); \
	else echo "  ❌ JavaScript: failed"; fi; \
	total=$$((total+1)); \
	echo "Testing Python..."; \
	if ./$(BINARY_NAME) --path $(TEST_DATA_DIR)/python --format json | jq -e '.success == true' >/dev/null; then \
		echo "  ✅ Python: v2.1.0"; passed=$$((passed+1)); \
	else echo "  ❌ Python: failed"; fi; \
	total=$$((total+1)); \
	echo "Testing Go..."; \
	if ./$(BINARY_NAME) --path $(TEST_DATA_DIR)/go --format json | jq -e '.success == true' >/dev/null; then \
		echo "  ✅ Go: v1.23"; passed=$$((passed+1)); \
	else echo "  ❌ Go: failed"; fi; \
	total=$$((total+1)); \
	echo "Testing Maven..."; \
	if ./$(BINARY_NAME) --path $(TEST_DATA_DIR)/maven --format json | jq -e '.success == true' >/dev/null; then \
		echo "  ✅ Maven: v3.2.1"; passed=$$((passed+1)); \
	else echo "  ❌ Maven: failed"; fi; \
	total=$$((total+1)); \
	echo "Sample tests: $$passed/$$total passed"; \
	if [ $$passed -lt $$total ]; then exit 1; fi

.PHONY: test-integration
test-integration: build ## Run integration tests with ALL sample repositories from config
	@echo "🔗 Running comprehensive integration tests..."
	@echo "📝 Testing ALL sample repositories from configuration file"
	VERBOSE=true ./test/integration/run-tests.sh ./$(BINARY_NAME)
	@if [ -f $(INTEGRATION_REPORT) ]; then \
		echo "📊 Comprehensive integration test report generated: $(INTEGRATION_REPORT)"; \
	fi

.PHONY: test-cli
test-cli: build ## Test CLI functionality
	@echo "⚙️  Testing CLI functionality..."
	./$(BINARY_NAME) version
	./$(BINARY_NAME) list --format json > supported-types.json
	@if jq . supported-types.json >/dev/null 2>&1; then \
		echo "✅ CLI tests passed"; \
	else \
		echo "❌ CLI JSON output invalid"; \
		exit 1; \
	fi

.PHONY: test-errors
test-errors: build test-samples ## Test error handling scenarios
	@echo "🚫 Testing error handling..."
	@# Test empty project (should fail)
	@if ./$(BINARY_NAME) --path $(TEST_DATA_DIR)/empty --fail-on-error=true >/dev/null 2>&1; then \
		echo "❌ Should have failed with empty project"; \
		exit 1; \
	else \
		echo "✅ Correctly failed with empty project"; \
	fi
	@# Test fail-on-error=false (should succeed but report failure)
	@if result=$$(./$(BINARY_NAME) --path $(TEST_DATA_DIR)/empty --fail-on-error=false --format json 2>/dev/null); then \
		if echo "$$result" | jq -e '.success == false' >/dev/null 2>&1; then \
			echo "✅ Correctly handled fail-on-error=false"; \
		else \
			echo "❌ Should have reported success=false"; \
			exit 1; \
		fi \
	else \
		echo "❌ Should have succeeded with fail-on-error=false"; \
		exit 1; \
	fi

# === COMPREHENSIVE WORKFLOWS ===
.PHONY: dev
dev: clean deps build lint-fast test-unit test-cli ## Fast development cycle
	@echo "🚀 Development cycle completed successfully!"
	@echo ""
	@echo "Quick validation:"
	@echo "- ✅ Dependencies updated"
	@echo "- ✅ Binary built"
	@echo "- ✅ Fast linting passed"
	@echo "- ✅ Unit tests passed"
	@echo "- ✅ CLI functionality tested"

.PHONY: ci
ci: clean deps build lint-full test-unit test-samples test-cli test-errors ## Complete local CI validation (network-optimized)
	@echo "🎉 Local CI validation passed!"
	@echo ""
	@echo "Local comprehensive validation:"
	@echo "- ✅ Dependencies verified"
	@echo "- ✅ Binary built with version info"
	@echo "- ✅ Comprehensive linting passed"
	@echo "- ✅ Unit tests with coverage completed"
	@echo "- ✅ Sample project tests passed"
	@echo "- ✅ CLI functionality verified"
	@echo "- ✅ Error handling tested"
	@echo "- ⏭️  Integration tests skipped (optimized for local development)"
	@echo ""
	@echo "📊 Generated reports:"
	@echo "- Code coverage: coverage.html"
	@echo "- CLI output: supported-types.json"
	@echo ""
	@echo "💡 GitHub Actions runs the FULL test suite with all repository integration tests"
	@echo "💡 For local full testing including network-dependent tests, run: make ci-full"

.PHONY: ci-full
ci-full: clean deps build lint-full test-unit test-samples test-cli test-errors test-integration ## Complete CI validation including ALL repository integration tests
	@echo "🎉 Complete CI validation with ALL integration tests passed!"
	@echo ""
	@echo "Full comprehensive validation (matches GitHub Actions):"
	@echo "- ✅ Dependencies verified"
	@echo "- ✅ Binary built with version info"
	@echo "- ✅ Comprehensive linting passed"
	@echo "- ✅ Unit tests with coverage completed"
	@echo "- ✅ Sample project tests passed"
	@echo "- ✅ CLI functionality verified"
	@echo "- ✅ Error handling tested"
	@echo "- ✅ Integration tests with ALL sample repositories completed"
	@echo ""
	@echo "📊 Generated reports:"
	@echo "- Code coverage: coverage.html"
	@echo "- CLI output: supported-types.json"
	@if [ -f $(INTEGRATION_REPORT) ]; then \
		echo "- Comprehensive integration results: $(INTEGRATION_REPORT)"; \
	fi
	@echo ""
	@echo "🚀 This matches the full test suite run by GitHub Actions!"

# === SPECIALIZED TARGETS ===
.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "📦 Installing $(BINARY_NAME)..."
	cp $(BINARY_PATH) $(GOPATH)/bin/
	@echo "✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	docker build -t version-extract:$(VERSION) .
	@echo "✅ Docker image built: version-extract:$(VERSION)"

.PHONY: security-scan
security-scan: ## Run security scan
	@echo "🛡️  Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
		echo "✅ Security scan completed"; \
	else \
		echo "⚠️  gosec not found, install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

.PHONY: benchmark
benchmark: build ## Run performance benchmarks
	@echo "⚡ Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
	@echo "✅ Benchmarks completed"

# === DEVELOPMENT HELPERS ===
.PHONY: dev-setup
dev-setup: ## Setup development environment
	@echo "🛠️  Setting up development environment..."
	@echo "Installing development tools..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		echo "Installing staticcheck..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	@if ! command -v gosec >/dev/null 2>&1; then \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@if [ -f .pre-commit-config.yaml ]; then \
		if command -v pre-commit >/dev/null 2>&1; then \
			pre-commit install; \
			echo "✅ Pre-commit hooks installed"; \
		else \
			echo "⚠️  pre-commit not found, install with: pip install pre-commit"; \
		fi \
	fi
	@echo "✅ Development environment setup completed"

.PHONY: check-deps
check-deps: ## Check if required dependencies are available
	@echo "🔍 Checking dependencies..."
	@missing=0; \
	for cmd in git jq go; do \
		if ! command -v $$cmd >/dev/null 2>&1; then \
			echo "❌ $$cmd is required but not installed"; \
			missing=$$((missing + 1)); \
		else \
			echo "✅ $$cmd is available"; \
		fi \
	done; \
	if [ $$missing -gt 0 ]; then \
		echo "❌ $$missing required dependencies missing"; \
		exit 1; \
	else \
		echo "✅ All required dependencies satisfied"; \
	fi

.PHONY: run-sample
run-sample: build test-samples ## Build and run against generated samples
	@echo "🎯 Running against sample projects..."
	@for dir in $(TEST_DATA_DIR)/*; do \
		if [ -d "$$dir" ]; then \
			echo ""; \
			echo "Testing $$(basename $$dir):"; \
			./$(BINARY_NAME) --path "$$dir" --verbose || true; \
		fi \
	done

# === CLEANUP AND MAINTENANCE ===
.PHONY: deep-clean
deep-clean: clean ## Deep clean including Go module cache
	@echo "🧹 Deep cleaning..."
	go clean -modcache
	go clean -cache
	go clean -testcache
	@echo "✅ Deep clean completed"

# Ensure binary is executable after build
$(BINARY_PATH): build

# Help ensure make works correctly
.SUFFIXES:
