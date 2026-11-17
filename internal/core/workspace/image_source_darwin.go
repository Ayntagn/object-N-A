package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func readPickedImage(absolute string) ([]byte, error) {
	// A dropped screenshot can be readable even when opening its enclosing
	// TemporaryItems directory for reading is denied.
	file, err := os.OpenFile(absolute, os.O_RDONLY|unix.O_NOFOLLOW_ANY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", filepath.Base(absolute))
	}
	return io.ReadAll(io.LimitReader(file, maxImageBytes+1))
}
