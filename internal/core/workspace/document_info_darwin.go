package workspace

import (
	"io/fs"
	"syscall"
	"time"
)

func creationTime(info fs.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Birthtimespec.Sec != 0 {
		return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	}
	return time.Time{}
}
