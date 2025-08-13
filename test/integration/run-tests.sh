#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2025 The Linux Foundation

# Simplified Integration Test Runner
# Replaces the complex Go-based runner with straightforward shell script

# Basic error handling - will be refined after CI detection
set -uo pipefail

# Default configuration
BINARY="${1:-./version-extract}"
CONFIG="${2:-configs/default-patterns.yaml}"
WORK_DIR="${3:-./test-workspace}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Global variable for clone duration
CLONE_DURATION=0

# Arrays to store results with timing
declare -a PASSED_REPOS=()
declare -a FAILED_REPOS=()
declare -a SKIPPED_REPOS=()

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️ $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Detect CI environment and set appropriate error handling
CI_MODE="${CI:-false}"
if [[ "${GITHUB_ACTIONS:-false}" == "true" || "${GITLAB_CI:-false}" == "true" || "${CIRCLECI:-false}" == "true" || "${TRAVIS:-false}" == "true" ]]; then
    CI_MODE="true"
fi

# Check for --ci flag to explicitly enable CI mode
for arg in "$@"; do
    if [[ "$arg" == "--ci" ]]; then
        CI_MODE="true"
        # Rebuild argument list without --ci
        new_args=()
        for current_arg in "$@"; do
            if [[ "$current_arg" != "--ci" ]]; then
                new_args+=("$current_arg")
            fi
        done
        set -- "${new_args[@]}"
        break
    fi
done

# Refine error handling based on environment
if [[ "$CI_MODE" == "true" ]]; then
    # In CI: continue on errors but track them
    log_info "Running in CI mode - will continue on individual test failures"
else
    # Locally: fail fast for immediate feedback
    set -e
fi



# Extract org/repo name from GitHub URL
get_org_repo_name() {
    local url="$1"
    echo "$url" | sed -E 's|https://github\.com/([^/]+/[^/]+).*|\1|'
}

# Check prerequisites
check_prerequisites() {
    local missing=0

    # Check for required tools
    if ! command -v git >/dev/null 2>&1; then
        log_error "git is required but not installed"
        missing=$((missing + 1))
    fi

    if ! command -v jq >/dev/null 2>&1; then
        log_error "jq is required but not installed"
        missing=$((missing + 1))
    fi

    # Check if binary exists
    if [[ ! -f "$BINARY" ]]; then
        log_error "Binary not found: $BINARY"
        missing=$((missing + 1))
    fi

    # Check if config exists
    if [[ ! -f "$CONFIG" ]]; then
        log_error "Config file not found: $CONFIG"
        missing=$((missing + 1))
    fi

    if [[ $missing -gt 0 ]]; then
        log_error "Missing $missing required dependencies"
        return 1
    fi

    log_success "All prerequisites satisfied"
    return 0
}

# Extract sample repositories from config file with detailed error handling
extract_sample_repos() {
    # Check if config file exists and is readable
    if [[ ! -f "$CONFIG" ]]; then
        echo "ERROR: Config file not found: $CONFIG" >&2
        return 1
    fi

    if [[ ! -r "$CONFIG" ]]; then
        echo "ERROR: Config file not readable: $CONFIG" >&2
        return 1
    fi

    local repos_output
    local extraction_error=""

    if command -v yq >/dev/null 2>&1; then
        # Use yq if available (more reliable)
        local yq_output

        # Capture both stdout and stderr
        if ! yq_output=$(yq '.projects[].samples[]' "$CONFIG" 2>&1); then
            extraction_error="yq command failed: $yq_output"
            echo "ERROR: Failed to parse YAML with yq: $extraction_error" >&2
            return 1
        fi

        # Check if yq returned valid data
        if [[ -z "$yq_output" ]]; then
            echo "ERROR: No sample repositories found in config (yq returned empty result)" >&2
            return 1
        fi

        # Process yq output and filter for GitHub URLs
        repos_output=$(echo "$yq_output" | sed 's/^"//; s/"$//' | grep -E '^https://github\.com/' || true)

    else
        # Fallback to grep-based extraction
        echo "WARNING: yq not available, using grep fallback (less reliable)" >&2

        # Check if the file appears to be valid YAML
        if ! grep -q "projects:" "$CONFIG"; then
            echo "ERROR: Config file does not appear to contain valid project structure" >&2
            return 1
        fi

        repos_output=$(grep -oP '- https://github\.com/[^/]+/[^/]+' "$CONFIG" | sed 's/^- //' || true)
    fi

    # Final validation of extracted repositories
    if [[ -z "$repos_output" ]]; then
        echo "ERROR: No GitHub repository URLs found in config file" >&2
        return 1
    fi

    # Validate that we have proper GitHub URLs
    local invalid_urls=0
    while IFS= read -r repo_url; do
        if [[ ! "$repo_url" =~ ^https://github\.com/[^/]+/[^/]+$ ]]; then
            echo "WARNING: Invalid GitHub URL format: $repo_url" >&2
            invalid_urls=$((invalid_urls + 1))
        fi
    done <<< "$repos_output"

    if [[ $invalid_urls -gt 0 ]]; then
        echo "WARNING: Found $invalid_urls invalid repository URLs" >&2
    fi

    # Output the valid repositories
    echo "$repos_output"
    return 0
}

# Clone a repository with timeout and retry
clone_repository() {
    local repo_url="$1"
    local target_dir="$2"
    local current_test="$3"
    local total_repos="$4"
    local repo_name
    local timeout=120
    local max_attempts=3
    local clone_start_time
    local clone_end_time

    repo_name=$(basename "$repo_url")

    echo "💬 Cloning repository: $(get_org_repo_name "$repo_url")"

    clone_start_time=$(date +%s)

    for attempt in $(seq 1 $max_attempts); do
        # Remove existing directory
        [[ -d "$target_dir" ]] && rm -rf "$target_dir"

        # Clone with timeout - handle errors gracefully in CI mode
        if timeout $timeout git clone --depth=1 --quiet "$repo_url" "$target_dir" 2>/dev/null; then
            clone_end_time=$(date +%s)
            CLONE_DURATION=$((clone_end_time - clone_start_time))
            return 0
        else
            local clone_exit_code=$?
            if [[ $attempt -lt $max_attempts ]]; then
                # Backoff calculated as 15 * (2^attempt): attempt=1: 30s, attempt=2: 60s, attempt=3: 120s
                local backoff_time=$((15 * (2 ** attempt)))
                echo "⏪ Retry cloning: $(get_org_repo_name "$repo_url") (attempt $((attempt + 1))/$max_attempts, waiting ${backoff_time}s)"
                sleep $backoff_time
            elif [[ "$CI_MODE" == "true" ]]; then
                # In CI mode, log error but don't exit
                log_error "Failed to clone after $max_attempts attempts: $(get_org_repo_name "$repo_url")"
                clone_end_time=$(date +%s)
                CLONE_DURATION=$((clone_end_time - clone_start_time))
                return $clone_exit_code
            fi
        fi
    done

    clone_end_time=$(date +%s)
    CLONE_DURATION=$((clone_end_time - clone_start_time))
    return 1
}

# Test version extraction on a repository
test_repository() {
    local repo_url="$1"
    local current_test="$2"
    local total_repos="$3"
    local repo_name
    local org_repo
    local target_dir
    local start_time
    local end_time
    local duration
    local extraction_start_time
    local extraction_end_time
    local extraction_duration_ms

    repo_name=$(basename "$repo_url")
    org_repo=$(get_org_repo_name "$repo_url")
    target_dir="$WORK_DIR/$repo_name"
    start_time=$(date +%s)

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo "🧪 Testing repository: $repo_url #${current_test}/${total_repos}"

    # Clone repository - handle failures gracefully in CI mode
    if ! clone_repository "$repo_url" "$target_dir" "$current_test" "$total_repos"; then
        end_time=$(date +%s)
        duration=$((end_time - start_time))

        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        SKIPPED_REPOS+=("$repo_url (clone failed, ${duration}s)")

        log_warning "$org_repo: clone failed"
        echo "    📁 Repository: $repo_url"
        echo "    ⏱️ Repository clone: ${CLONE_DURATION}s  Test/extract: 0ms"
        echo ""

        # In CI mode, continue to next test; locally, this would exit due to set -e
        if [[ "$CI_MODE" == "true" ]]; then
            return 0
        else
            return 1  # Will cause script to exit in local mode due to set -e
        fi
    fi

    # Run version extraction - capture errors gracefully with timing
    local extraction_result
    local extraction_exit_code

    extraction_start_time=$(date +%s%3N)  # milliseconds
    if extraction_result=$("$BINARY" --path "$target_dir" --format json 2>/dev/null); then
        extraction_exit_code=0
    else
        extraction_exit_code=$?
        # In CI mode, don't let extraction failures stop the script
        if [[ "$CI_MODE" != "true" ]] && [[ $extraction_exit_code -ne 0 ]]; then
            rm -rf "$target_dir" 2>/dev/null || true
            return $extraction_exit_code
        fi
    fi
    extraction_end_time=$(date +%s%3N)  # milliseconds
    extraction_duration_ms=$((extraction_end_time - extraction_start_time))

    # Clean up repository
    rm -rf "$target_dir" 2>/dev/null || true

    end_time=$(date +%s)
    duration=$((end_time - start_time))

    # Analyze result
    if [[ $extraction_exit_code -eq 0 ]]; then
        # Validate JSON output - handle jq failures gracefully in CI mode
        local jq_success=false
        if echo "$extraction_result" | jq -e '.success == true' >/dev/null 2>&1; then
            jq_success=true
        elif [[ "$CI_MODE" != "true" ]]; then
            # In local mode, jq failures should be treated as errors
            FAILED_TESTS=$((FAILED_TESTS + 1))
            FAILED_REPOS+=("$repo_url (JSON parse error, ${duration}s)")
            log_error "$org_repo: Failed to parse JSON output"
            echo "    📁 Repository: $repo_url"
            echo "    ⏱️ Duration: ${duration}s"
            echo ""
            return 1
        fi

        if [[ "$jq_success" == "true" ]]; then
            local version
            local file_analyzed
            local project_type

            version=$(echo "$extraction_result" | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")
            file_analyzed=$(echo "$extraction_result" | jq -r '.file // "unknown"' 2>/dev/null || echo "unknown")
            project_type=$(echo "$extraction_result" | jq -r '.project_type // "unknown"' 2>/dev/null || echo "unknown")

            PASSED_TESTS=$((PASSED_TESTS + 1))
            PASSED_REPOS+=("$repo_url (v$version, ${duration}s)")

            # Show success message without timing
            log_success "$org_repo: extracted version $version"

            echo "    📁 Repository: $repo_url"
            echo "    📄 File analyzed: $file_analyzed"
            echo "    🏷️ Project type: $project_type"
            echo "    ⏱️ Repository clone: ${CLONE_DURATION}s  Test/extract: ${extraction_duration_ms}ms"
            echo ""
        else
            FAILED_TESTS=$((FAILED_TESTS + 1))
            local error_msg
            error_msg=$(echo "$extraction_result" | jq -r '.error // "extraction reported failure"' 2>/dev/null || echo "JSON parse error")
            FAILED_REPOS+=("$repo_url ($error_msg, ${duration}s)")

            log_error "$org_repo: $error_msg"
            echo "    📁 Repository: $repo_url"
            echo "    ⏱️ Repository clone: ${CLONE_DURATION}s  Test/extract: ${extraction_duration_ms}ms"
            echo ""
        fi
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        FAILED_REPOS+=("$repo_url (binary exit code: $extraction_exit_code, ${duration}s)")

        log_error "$org_repo: binary failed with exit code $extraction_exit_code"
        echo "    📁 Repository: $repo_url"
        echo "    ⏱️ Repository clone: ${CLONE_DURATION}s  Test/extract: ${extraction_duration_ms}ms"
        echo ""
    fi

    # In CI mode, always return 0 to continue with next test
    if [[ "$CI_MODE" == "true" ]]; then
        return 0
    fi
}

# Print summary of test results
print_summary() {
    local success_rate

    if [[ $TOTAL_TESTS -eq 0 ]]; then
        success_rate="N/A"
    else
        success_rate=$(awk "BEGIN {printf \"%.1f\", ($PASSED_TESTS/$TOTAL_TESTS)*100}")
    fi

    echo ""
    echo "============================================================"
    echo "🏁 INTEGRATION TESTING RESULTS/SUMMARY"
    echo "============================================================"
    echo "Total repositories tested: $TOTAL_TESTS"
    echo "Passed: $PASSED_TESTS"
    echo "Failed: $FAILED_TESTS"
    echo "Skipped: $SKIPPED_TESTS"
    echo "Success rate: $success_rate%"
    echo "============================================================"

    # Show failed tests if any with timing
    if [[ $FAILED_TESTS -gt 0 ]]; then
        echo ""
        log_error "FAILED REPOSITORIES:"
        for repo in "${FAILED_REPOS[@]}"; do
            echo "  ❌ $repo"
        done
    fi

    # Show skipped tests if any
    if [[ $SKIPPED_TESTS -gt 0 ]]; then
        echo ""
        log_warning "SKIPPED REPOSITORIES:"
        for repo in "${SKIPPED_REPOS[@]}"; do
            echo "  ⏭️  $repo"
        done
    fi

    # Show passed tests with timing
    if [[ $PASSED_TESTS -gt 0 ]]; then
        echo ""
        log_success "PASSED REPOSITORIES:"
        for repo in "${PASSED_REPOS[@]}"; do
            echo "  ✅ $repo"
        done
    fi
}

# Generate simple JSON report
generate_report() {
    local report_file="integration-test-report.json"
    local timestamp
    timestamp=$(date -Iseconds)

    cat > "$report_file" << EOF
{
  "timestamp": "$timestamp",
  "summary": {
    "total": $TOTAL_TESTS,
    "passed": $PASSED_TESTS,
    "failed": $FAILED_TESTS,
    "skipped": $SKIPPED_TESTS,
    "success_rate": $(awk "BEGIN {printf \"%.1f\", ($TOTAL_TESTS > 0) ? ($PASSED_TESTS/$TOTAL_TESTS)*100 : 0}")
  },
  "configuration": {
    "binary": "$BINARY",
    "config": "$CONFIG",
    "work_dir": "$WORK_DIR"
  },
  "results": {
    "passed": [$(printf '"%s",' "${PASSED_REPOS[@]}" | sed 's/,$//')],
    "failed": [$(printf '"%s",' "${FAILED_REPOS[@]}" | sed 's/,$//')],
    "skipped": [$(printf '"%s",' "${SKIPPED_REPOS[@]}" | sed 's/,$//')]
  }
}
EOF

    log_info "Generated report: $report_file"
}

# Show help
show_help() {
    cat << 'EOF'
Simplified Integration Test Runner

USAGE:
    run-tests.sh [BINARY] [CONFIG] [WORK_DIR] [--ci]

ARGUMENTS:
    BINARY     Path to version-extract binary (default: ./version-extract)
    CONFIG     Path to configuration file (default: configs/default-patterns.yaml)
    WORK_DIR   Temporary directory for cloning repos (default: ./test-workspace)

FLAGS:
    --ci       Force CI mode (continue on individual test failures)

ENVIRONMENT VARIABLES:
    VERBOSE    Set to 'true' for detailed output (default: false)
    CI         Auto-detected CI environments: GITHUB_ACTIONS, GITLAB_CI, CIRCLECI, TRAVIS

EXAMPLES:
    # Basic usage
    ./test/integration/run-tests.sh

    # With custom binary
    ./test/integration/run-tests.sh ./build/version-extract

    # Force CI mode locally
    ./test/integration/run-tests.sh --ci

    # Verbose output
    VERBOSE=true ./test/integration/run-tests.sh

    # Custom configuration
    ./test/integration/run-tests.sh ./version-extract configs/custom.yaml

REQUIREMENTS:
    - git (for cloning repositories)
    - jq (for JSON processing)
    - yq (optional, for better config parsing)
    - timeout (for repository clone timeouts)

EOF
}

# Cleanup function
# shellcheck disable=SC2317  # Function is called via trap
cleanup() {
    if [[ -d "$WORK_DIR" ]]; then
        echo "🔍 Cleaning up work directory: $WORK_DIR"
        rm -rf "$WORK_DIR"
    fi
}

# Set up cleanup trap
trap cleanup EXIT

# Main execution
main() {
    log_info "Starting simplified integration test runner"
    log_info "Binary: $BINARY"
    log_info "Config: $CONFIG"
    log_info "Work directory: $WORK_DIR"
    log_info "Clone timeout: 120s with 3 retries and exponential backoff (30s, 60s, 120s)"

    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi

    # Create work directory
    mkdir -p "$WORK_DIR" || {
        log_error "Failed to create work directory: $WORK_DIR"
        exit 1
    }

    # Extract sample repositories with detailed error handling
    local repos_list
    local extract_output
    local extract_exit_code

    # Capture both stdout and stderr from extract_sample_repos
    extract_output=$(extract_sample_repos 2>&1)
    extract_exit_code=$?

    if [[ $extract_exit_code -ne 0 ]]; then
        log_error "Failed to extract repository list from config:"
        # Print the specific error messages from extract_sample_repos
        echo "$extract_output" | while IFS= read -r line; do
            if [[ "$line" =~ ^ERROR: ]]; then
                echo "  🔴 ${line#ERROR: }"
            elif [[ "$line" =~ ^WARNING: ]]; then
                echo "  🟡 ${line#WARNING: }"
            else
                echo "  📝 $line"
            fi
        done
        exit 1
    fi

    # Extract just the repository URLs (filter out warning messages)
    repos_list=$(echo "$extract_output" | grep -E '^https://github\.com/' || true)

    if [[ -z "$repos_list" ]]; then
        log_error "No repositories to test"
        exit 1
    fi

    local repo_count
    repo_count=$(echo "$repos_list" | wc -l)
    log_info "Found $repo_count sample repositories to test"

    echo ""
    echo "🚀 STARTING COMPREHENSIVE REPOSITORY TESTING"
    echo "============================================================"
    echo "Configuration file: $CONFIG"
    echo "Binary path: $BINARY"
    echo "Work directory: $WORK_DIR"
    echo "Expected repositories: $repo_count"
    echo "============================================================"
    echo ""

    # Run tests
    local current_test=0
    while IFS= read -r repo; do
        if [[ -n "$repo" && "$repo" != "null" && "$repo" =~ ^https://github\.com/ ]]; then
            current_test=$((current_test + 1))
            if [[ "$CI_MODE" == "true" ]]; then
                # In CI mode, continue on individual test failures
                test_repository "$repo" "$current_test" "$repo_count" || true
            else
                # In local mode, let failures propagate due to set -e
                test_repository "$repo" "$current_test" "$repo_count"
            fi
        fi
    done <<< "$repos_list"

    # Generate report and summary
    generate_report
    print_summary

    # Always generate summary in CI mode, then exit with appropriate code
    if [[ $FAILED_TESTS -gt 0 ]]; then
        if [[ "$CI_MODE" == "true" ]]; then
            log_error "Integration tests completed with $FAILED_TESTS failures (CI mode)"
        fi
        exit 1
    else
        if [[ "$CI_MODE" == "true" ]]; then
            log_success "All integration tests passed (CI mode)"
        fi
        exit 0
    fi
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
