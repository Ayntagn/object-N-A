//go:build !darwin

package workspace

import "os"

func indexedNoteCacheDevice(*os.Root) uint64 { return 0 }

func indexedNoteIdentity(os.FileInfo) (device, inode uint64, changed int64) {
	return 0, 0, 0
}
