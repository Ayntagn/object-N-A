//go:build !darwin && !windows

package workspace

import (
	"io/fs"
	"time"
)

func creationTime(fs.FileInfo) time.Time { return time.Time{} }
