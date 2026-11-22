// Package vaultdelta coalesces directory hints for incremental tree refreshes.
package vaultdelta

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

// Scope is an immutable set of dirty subtrees. Its zero value has no hints.
type Scope struct {
	directories []string
	full        bool
}

// Hints accepts directories relative to the watched root, with slash
// separators. A root, invalid, or oversized batch requests a full refresh.
func Hints(directories ...string) Scope {
	const maxDirectories = 128
	if len(directories) == 0 {
		return Scope{}
	}
	set := make(map[string]struct{}, min(len(directories), maxDirectories))
	for _, dir := range directories {
		if dir == "." || !fs.ValidPath(dir) || strings.ContainsRune(dir, 0) {
			return Scope{full: true}
		}
		set[dir] = struct{}{}
		if len(set) > maxDirectories {
			return Scope{full: true}
		}
	}
	var roots []string
	for dir := range set {
		if !coveredByParent(dir, set) {
			roots = append(roots, dir)
		}
	}
	slices.Sort(roots)
	return Scope{directories: roots}
}

func coveredByParent(dir string, directories map[string]struct{}) bool {
	for parent := path.Dir(dir); parent != "."; parent = path.Dir(parent) {
		if _, covered := directories[parent]; covered {
			return true
		}
	}
	return false
}

// Merge combines pending hints. A full refresh covers every named hint.
func (s Scope) Merge(other Scope) Scope {
	if s.full || other.Empty() {
		return s
	}
	if other.full || s.Empty() {
		return other
	}
	return Hints(slices.Concat(s.directories, other.directories)...)
}

// Directories gives the ordered, non-overlapping directories of a named scope.
func (s Scope) Directories() []string { return slices.Clone(s.directories) }

// Full reports whether the entire tree should be read.
func (s Scope) Full() bool { return s.full }

// Empty reports whether there were any directory hints.
func (s Scope) Empty() bool { return !s.full && len(s.directories) == 0 }
