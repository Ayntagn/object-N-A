//go:build !darwin && !linux

package workspace

import "io/fs"

func fileHasHardLinks(fs.FileInfo) bool { return false }

func dirEntryHasHardLinks(fs.DirEntry) bool { return false }
