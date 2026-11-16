package workspace

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// The cache uses APFS's change timestamps to detect a write even when the
// editor restored the modification time. Other volumes may quantize or cache
// their stat results, so notes from them are read again.
func indexedNoteCacheDevice(root *os.Root) uint64 {
	directory, err := root.Open(".")
	if err != nil {
		return 0
	}
	defer directory.Close()
	var volume unix.Statfs_t
	if err := unix.Fstatfs(int(directory.Fd()), &volume); err != nil || unix.ByteSliceToString(volume.Fstypename[:]) != "apfs" {
		return 0
	}
	info, err := directory.Stat()
	if err != nil {
		return 0
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev)
	}
	return 0
}

func indexedNoteIdentity(info os.FileInfo) (device, inode uint64, changed int64) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ctimespec.Nsec == 0 {
		return 0, 0, 0
	}
	return uint64(stat.Dev), stat.Ino, stat.Ctimespec.Nano()
}
