#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The Linux Foundation

# Runs the "Extract project version" step of action.yaml against a stub
# extractor that writes its JSON result to stdout, and plain and
# JSON-shaped diagnostics to stderr both before and after it, then
# checks the step's outputs, log, step summary and exit status, and
# that the extractor runs once. Needs bash, jq and mikefarah yq.
#
# Usage: bash test/action-step-test.sh [path/to/action.yaml]

set -euo pipefail

ACTION_FILE="${1:-$(dirname "$0")/../action.yaml}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "${WORK_DIR}"' EXIT

STEP_SCRIPT="${WORK_DIR}/step.sh"
yq -r '.runs.steps[] | select(.id == "extract") | .run' "${ACTION_FILE}" \
  > "${STEP_SCRIPT}"
if [ ! -s "${STEP_SCRIPT}" ]; then
  echo "❌ No step with id 'extract' in ${ACTION_FILE}"
  exit 1
fi

# The step runs ./version-extract from ACTION_PATH, so a stub placed
# there stands in for the real binary.
STUB_DIR="${WORK_DIR}/action"
mkdir -p "${STUB_DIR}"
cat > "${STUB_DIR}/version-extract" << 'EOF'
#!/usr/bin/env bash
echo "run" >> "${STUB_CALLS}"
echo "Warning: early diagnostic" >&2
echo '{"level":"info","msg":"early structured log"}' >&2
if [ "${STUB_EXIT:-0}" -ne 0 ]; then
  echo "Error: stub extraction failed" >&2
  exit "${STUB_EXIT}"
fi
minimised=false
for arg in "$@"; do
  [ "${arg}" = "--json-format=minimised" ] && minimised=true
done
if [ "${minimised}" = "true" ]; then
  echo '{"success":true,"version":"1.2.3","project_type":"JavaScript","file":"package.json"}'
else
  cat << 'JSON'
{
  "success": true,
  "version": "1.2.3",
  "project_type": "JavaScript",
  "file": "package.json"
}
JSON
fi
echo "Warning: late diagnostic" >&2
echo '{"level":"warn","msg":"late structured log"}' >&2
EOF
chmod +x "${STUB_DIR}/version-extract"

failures=0

fail() {
  echo "  ❌ $1"
  failures=$((failures + 1))
}

output_value() {
  sed -n "s/^$1=//p" "${CASE_DIR}/output" | head -n 1
}

# run_case <name> <verbose> <json-format> <stub-exit> <fail-on-error>
run_case() {
  local name="$1"
  CASE_DIR="${WORK_DIR}/${name}"
  mkdir -p "${CASE_DIR}/tmp"
  : > "${CASE_DIR}/output"
  : > "${CASE_DIR}/summary"
  : > "${CASE_DIR}/calls"

  set +e
  env \
    INPUT_PATH="." \
    INPUT_CONFIG="" \
    INPUT_FORMAT="json" \
    INPUT_VERBOSE="$2" \
    INPUT_FAIL_ON_ERROR="$5" \
    INPUT_JSON_FORMAT="$3" \
    INPUT_DYNAMIC_FALLBACK="true" \
    ACTION_PATH="${STUB_DIR}" \
    GITHUB_WORKSPACE="${WORK_DIR}" \
    GITHUB_OUTPUT="${CASE_DIR}/output" \
    GITHUB_STEP_SUMMARY="${CASE_DIR}/summary" \
    RUNNER_TEMP="${CASE_DIR}/tmp" \
    STUB_CALLS="${CASE_DIR}/calls" \
    STUB_EXIT="$4" \
    bash --noprofile --norc -eo pipefail "${STEP_SCRIPT}" \
    > "${CASE_DIR}/log" 2>&1
  CASE_EXIT=$?
  set -e

  echo "Case: ${name}"
  local calls
  calls=$(wc -l < "${CASE_DIR}/calls")
  if [ "${calls}" -ne 1 ]; then
    fail "extractor ran ${calls} times, expected once"
  fi
  if [ -n "$(ls -A "${CASE_DIR}/tmp")" ]; then
    fail "temporary files left in RUNNER_TEMP"
  fi
}

# Prints the case's log when it added failures since <before>
report() {
  if [ "${failures}" -eq "$1" ]; then
    echo "  ✅ passed"
  else
    sed 's/^/    | /' "${CASE_DIR}/log"
  fi
}

expect_success() {
  local verbose="$1"
  if [ "${CASE_EXIT}" -ne 0 ]; then
    fail "step exited ${CASE_EXIT}, expected 0"
  fi
  if [ "$(output_value success)" != "true" ]; then
    fail "success='$(output_value success)', expected 'true'"
  fi
  if [ "$(output_value version)" != "1.2.3" ]; then
    fail "version='$(output_value version)', expected '1.2.3'"
  fi
  if [ "$(output_value project-type)" != "JavaScript" ]; then
    fail "project-type='$(output_value project-type)'"
  fi
  if [ "$(output_value file)" != "package.json" ]; then
    fail "file='$(output_value file)', expected 'package.json'"
  fi
  if [ "${verbose}" = "true" ]; then
    if ! grep -q "late structured log" "${CASE_DIR}/log"; then
      fail "verbose log lacks the extractor's stderr"
    fi
  elif grep -q "diagnostic\|structured log" "${CASE_DIR}/log"; then
    fail "non-verbose log shows stderr from a successful run"
  fi
}

expect_failure() {
  local expected_exit="$1"
  if [ "${CASE_EXIT}" -ne "${expected_exit}" ]; then
    fail "step exited ${CASE_EXIT}, expected ${expected_exit}"
  fi
  if [ "$(output_value success)" != "false" ]; then
    fail "success='$(output_value success)', expected 'false'"
  fi
  if ! grep -q "stub extraction failed" "${CASE_DIR}/log"; then
    fail "log lacks the extractor's error"
  fi
  if ! grep -q "stub extraction failed" "${CASE_DIR}/summary"; then
    fail "step summary lacks the extractor's error"
  fi
}

for verbose in false true; do
  for json_format in pretty minimised; do
    before=${failures}
    run_case "success-verbose-${verbose}-${json_format}" \
      "${verbose}" "${json_format}" 0 true
    expect_success "${verbose}"
    report "${before}"
  done
done

for verbose in false true; do
  before=${failures}
  run_case "failure-verbose-${verbose}" "${verbose}" pretty 1 true
  expect_failure 1
  report "${before}"
done

before=${failures}
run_case "failure-no-fail-on-error" false pretty 1 false
expect_failure 0
report "${before}"

if [ "${failures}" -ne 0 ]; then
  echo "❌ ${failures} check(s) failed"
  exit 1
fi
echo "✅ All action step tests passed"
