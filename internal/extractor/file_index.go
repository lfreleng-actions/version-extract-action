// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Linux Foundation

package extractor

import (
	"os"
	"path/filepath"
	"strings"
)

// fileIndex is a one-time snapshot of the files beneath a search path. It lets
// many project-file patterns be matched without re-walking the tree for each
// one. The previous approach walked the entire repository once per project
// type (50+ types), which dominated extraction time on large repos (the walk
// is I/O-bound on Lstat/Open syscalls).
type fileIndex struct {
	root   string
	byName map[string][]string // base name -> full paths, in walk order
	all    []string            // every file path, in walk order
}

// buildFileIndex walks searchPath exactly once, honouring the same
// directory-skip rules as the original per-type search: hidden directories and
// the configured skipDirectories (e.g. node_modules, vendor, build).
func (e *VersionExtractor) buildFileIndex(searchPath string) *fileIndex {
	// Normalise the root so root-level files (filepath.Dir(p) == root) are
	// classified correctly even if the caller passes a trailing slash or an
	// otherwise unclean path.
	searchPath = filepath.Clean(searchPath)
	idx := &fileIndex{root: searchPath, byName: make(map[string][]string)}
	_ = filepath.Walk(searchPath, func(path string, info os.FileInfo,
		err error) error {
		if err != nil {
			return nil // continue despite errors, matching prior behaviour
		}
		if info.IsDir() {
			// Never skip the search root itself, even if its name is dotted.
			if path != searchPath && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			for _, skip := range e.skipDirectories {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		idx.all = append(idx.all, path)
		idx.byName[info.Name()] = append(idx.byName[info.Name()], path)
		return nil
	})
	return idx
}

// match returns the files matching a project-file pattern (an exact file name
// or a glob such as "*.csproj"). Matches directly under the search root are
// returned first, then deeper matches in walk order, preserving the original
// search's preference for a root-level manifest.
func (idx *fileIndex) match(pattern string) []string {
	isGlob := strings.Contains(pattern, "*")

	// A non-glob pattern that includes a path component (e.g. Ansible's
	// "meta/main.yml") is matched relative to the search root, mirroring the
	// original search. byName is keyed by base name, so look it up there and
	// confirm the full root-relative path.
	if !isGlob && strings.ContainsAny(pattern, `/\`) {
		target := filepath.Join(idx.root, pattern)
		for _, p := range idx.byName[filepath.Base(pattern)] {
			if p == target {
				return []string{p}
			}
		}
		return nil
	}

	candidates := idx.all
	if !isGlob {
		candidates = idx.byName[pattern]
	}

	var rootMatches, deepMatches []string
	for _, p := range candidates {
		if isGlob {
			if ok, _ := filepath.Match(pattern, filepath.Base(p)); !ok {
				continue
			}
		}
		if filepath.Dir(p) == idx.root {
			rootMatches = append(rootMatches, p)
		} else {
			deepMatches = append(deepMatches, p)
		}
	}
	return append(rootMatches, deepMatches...)
}
