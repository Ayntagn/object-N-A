package workspace

import (
	"io/fs"
	"syscall"
	"time"
)

func creationTime(info fs.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && stat.CreationTime != (syscall.Filetime{}) {
		return time.Unix(0, stat.CreationTime.Nanoseconds())
	}
	return time.Time{}
}
