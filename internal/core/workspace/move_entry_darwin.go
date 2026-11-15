package workspace

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameEntry(fromDir *os.File, from string, toDir *os.File, to string) error {
	return unix.RenameatxNp(int(fromDir.Fd()), from, int(toDir.Fd()), to, unix.RENAME_EXCL)
}
