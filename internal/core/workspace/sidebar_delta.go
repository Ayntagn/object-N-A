package workspace

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"object-na.local/vaultdelta"
)

// RefreshSidebarEntries replaces the hinted subtrees of a previous snapshot.
// The caller supplies the last snapshot of this Vault with the same all flag.
// The returned scope records whether a full reread was needed.
func (v *Vault) RefreshSidebarEntries(previous []Entry, all bool, scope vaultdelta.Scope) ([]Entry, vaultdelta.Scope, error) {
	if len(previous) > 0 && !scope.Full() && !scope.Empty() {
		entries := previous
		vaultFS, include := v.root.FS(), sidebarEntryFilter(all)
		for _, directory := range scope.Directories() {
			entries = v.refreshSidebarBranch(vaultFS, entries, directory, include)
			if entries == nil {
				break
			}
		}
		if entries != nil {
			return entries, scope, nil
		}
	}
	entries, err := v.SidebarEntries(all)
	return entries, vaultdelta.Hints("."), err
}

// A nil result means the directory hint is insufficient to refresh the snapshot.
func (v *Vault) refreshSidebarBranch(vaultFS fs.FS, entries []Entry, relative string, include func(fs.DirEntry) bool) []Entry {
	name, child, _ := strings.Cut(relative, "/")
	at := slices.IndexFunc(entries, func(entry Entry) bool { return entry.IsDir && entry.Name == name })
	if at < 0 {
		return nil
	}
	branch := entries[at]
	native, err := filepath.Localize(branch.Path)
	if err != nil {
		return nil
	}
	before, err := v.root.Lstat(native)
	if err != nil || !before.IsDir() {
		return nil
	}
	if child == "" {
		// A write through a hard link may only notify its own directory.
		// The previous listing also catches a name removed after the write.
		if branch.hasHardLinks {
			return nil
		}
		var linked bool
		branch.Children, linked, err = readEntriesWithLinks(vaultFS, branch.Path, -1, include)
		if err != nil || linked {
			return nil
		}
	} else {
		// A changed ancestor may have gained or lost an entry alongside the hint.
		if !branch.modified.Equal(before.ModTime()) || branch.size != before.Size() {
			return nil
		}
		branch.Children = v.refreshSidebarBranch(vaultFS, branch.Children, child, include)
		if branch.Children == nil {
			return nil
		}
	}
	after, err := v.root.Lstat(native)
	if err != nil || !after.IsDir() || !os.SameFile(before, after) ||
		!before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		return nil
	}
	// A case-insensitive lookup can still stat a directory under its old spelling.
	siblings, err := fs.ReadDir(vaultFS, path.Dir(branch.Path))
	if err != nil {
		return nil
	}
	i, found := slices.BinarySearchFunc(siblings, branch.Name, func(entry fs.DirEntry, name string) int {
		return strings.Compare(entry.Name(), name)
	})
	if !found || !siblings[i].IsDir() {
		return nil
	}
	branch.modified, branch.size = after.ModTime(), after.Size()
	next := slices.Clone(entries)
	next[at] = branch
	return next
}
