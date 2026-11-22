//go:build darwin || linux

package workspace

import (
	"io/fs"
	"syscall"
)

func fileHasHardLinks(info fs.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink > 1
}

func dirEntryHasHardLinks(entry fs.DirEntry) bool {
	if !entry.Type().IsRegular() {
		return false
	}
	info, err := entry.Info()
	return err == nil && fileHasHardLinks(info)
}
