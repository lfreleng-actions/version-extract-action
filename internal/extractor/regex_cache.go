// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Linux Foundation

package extractor

import (
	"fmt"
	"regexp"
	"sync"
)

// Package-level regex cache to persist across multiple file operations
var (
	regexCache = make(map[string]*regexp.Regexp)
	cacheMutex sync.RWMutex
)

// getCompiledRegex gets or compiles a regex pattern with thread-safe caching
func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	// Try to read from cache first
	cacheMutex.RLock()
	if compiledRegex, exists := regexCache[pattern]; exists {
		cacheMutex.RUnlock()
		return compiledRegex, nil
	}
	cacheMutex.RUnlock()

	// Compile the regex
	compiledRegex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex pattern '%s': %w", pattern, err)
	}

	// Store in cache
	cacheMutex.Lock()
	regexCache[pattern] = compiledRegex
	cacheMutex.Unlock()

	return compiledRegex, nil
}
