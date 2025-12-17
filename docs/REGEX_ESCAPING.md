<!--
SPDX-License-Identifier: Apache-2.0
SPDX-FileCopyrightText: 2025 The Linux Foundation
-->

# Regex Pattern Escaping in Multi-Line Detection

## Overview

This document explains the critical escaping logic used in
`isMultiLinePattern()` for detecting patterns that require multi-line file
processing. Understanding this is essential to prevent regression bugs from
well-intentioned but incorrect "fixes".

## The Problem

The `isMultiLinePattern()` function needs to detect when a user's regex
pattern will match content that spans lines. A common idiom for this is
`[\s\S]`, which matches any character (whitespace OR non-whitespace =
everything, including newlines).

**The Challenge:** We need to detect the literal string `[\s\S]` (with
backslashes) within user-provided pattern strings from YAML configuration
files.

## How YAML Parsing Works

When YAML configuration files provide patterns:

```yaml
# In configs/default-patterns.yaml
regex:
  - '<project>[\s\S]*?<version>([^<]+)</version>'
```

**After YAML parsing**, the Go string contains:

- Characters: `<`, `p`, `r`, `o`, `j`, `e`, `c`, `t`, `>`, `[`, `\`, `s`,
  `\`, `S`, `]`, etc.
- The backslashes are **literal characters** (byte value 92), NOT escape
  sequences
- Length: 43 characters total
- The substring `[\s\S]` is 6 characters: `[`, `\`, `s`, `\`, `S`, `]`

## The Detector Pattern

In `extractor.go`, the detector pattern is:

```go
`\[\\s\\S\]`
```

### Breaking Down the Escaping

In a Go raw string (backticks), backslashes are literal. This pattern means:

- `\[` = Regex: match literal `[` character
- `\\s` = Regex: match literal `\` followed by literal `s`
- `\\S` = Regex: match literal `\` followed by literal `S`
- `\]` = Regex: match literal `]` character

**Combined:** This regex searches for the substring `[\s\S]` (with literal
backslashes) in the pattern string.

## Why This Is Correct

1. **YAML gives us single backslashes**: `[\s\S]` (6 characters)
2. **To detect a literal backslash in regex**, we need `\\` (escape the
   backslash)
3. **Thus `\[\\s\\S\]` finds `[\s\S]`** in the pattern string

## The Copilot Mistake

GitHub Copilot suggested changing the test pattern from:

```go
pattern: `version[\s\S]+?end`
```

to:

```go
pattern: `version[\\s\\S]+?end`  // WRONG!
```

### Why This Is Wrong

The double-backslash version (`[\\s\\S]`) contains **4 backslashes** in the raw
string:

- Characters: `[`, `\`, `\`, `s`, `\`, `\`, `S`, `]`
- This is NOT what YAML gives us
- The detector pattern `\[\\s\\S\]` would NOT match this (it looks for single backslashes)
- The test would FAIL

## Verification Examples

### Example 1: Real Pattern from YAML

```go
// What comes from YAML config
pattern := `<project>[\s\S]*?<version>([^<]+)</version>`

// What the detector looks for
detector := `\[\\s\\S\]`

// Result: MATCH ✓
// The pattern contains [\s\S] with single backslashes
```

### Example 2: Copilot's Incorrect Suggestion

```go
// What Copilot suggested (WRONG)
pattern := `version[\\s\\S]+?end`

// What the detector looks for
detector := `\[\\s\\S\]`

// Result: NO MATCH ✗
// The pattern contains [\\s\\S] with double backslashes
// This is not what YAML gives us
```

### Example 3: How It Works as a Regex

```go
// User's pattern from YAML
userPattern := `version[\s\S]+?end`

// When compiled as regex, [\s\S] means "any character"
re := regexp.MustCompile(userPattern)

// Matches multi-line content
text := "version\n\n\nend"
matches := re.MatchString(text) // true ✓
```

## Test Coverage

The following tests ensure this logic remains correct:

### 1. `TestIsMultiLinePattern`

Basic test of the detection logic with the correct single-backslash pattern.

### 2. `TestMultiLinePatternYAMLIntegration`

Validates the complete flow from YAML parsing to pattern detection, proving
that:

- YAML parsed patterns contain single backslashes
- The detector identifies them as expected
- The pattern works as a regex for multi-line matching
- Copilot's double-backslash suggestion would fail

### 3. `TestMultiLinePatternEscapingRegression`

Comprehensive regression suite with test scenarios:

- ✓ Real patterns from YAML (single backslashes)
- ✗ Incorrect double-backslash patterns
- ✓ Actual patterns from config files
- ✗ False positives that should not match

### 4. `TestMultiLinePatternWithActualYAMLParsing`

End-to-end test that:

- Creates a real YAML file with patterns
- Parses it with the actual YAML library
- Verifies pattern detection works as expected
- Validates the patterns work as regex for multi-line content

### 5. `TestMultiLinePatternImplementationCorrectness`

Tests the detector pattern itself to ensure:

- It compiles as valid regex
- It matches what we expect
- It rejects what we don't want

## Common Mistakes to Avoid

❌ **DO NOT** change `\[\\s\\S\]` to `\[\s\S\]` (single backslash before s/S)

- This would look for regex escape sequences instead of literal backslashes
- Would not match patterns from YAML

❌ **DO NOT** change test patterns to use double backslashes `[\\s\\S]`

- This doesn't match what YAML provides
- Would break the tests

❌ **DO NOT** assume the escaping is "too complex" and try to simplify it

- The double-escaping is necessary for the regex to work as intended
- Simplification will break YAML pattern detection

## Summary

**The key insight:** We're searching for literal backslashes in the pattern
string, not regex escape sequences. We need to escape the backslashes in our
detector regex.

**String contents at each stage:**

- **YAML file**: `[\s\S]` (YAML syntax)
- **After parsing**: `[\s\S]` (6 chars with literal backslashes)
- **Detector pattern**: `\[\\s\\S\]` (regex to find the above)
- **Test pattern**: `version[\s\S]+?end` (matches YAML output)

## References

- Implementation: `internal/extractor/extractor.go` (`isMultiLinePattern` function)
- Tests: `internal/extractor/extractor_test.go` (search for `TestMultiLinePattern`)
- Config example: `configs/default-patterns.yaml` (Java/Maven project type)

## Questions?

If you're considering changing this code, first:

1. Read this document in full
2. Run all the tests:
   `go test -v -run TestMultiLinePattern ./internal/extractor/`
3. Verify your changes work with actual YAML parsing
4. Update this document if you find a legitimate improvement

The escaping is correct as-is. Trust the tests.
