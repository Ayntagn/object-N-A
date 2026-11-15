package exportdoc

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes a completed export to a destination confirmed by the save picker.
func WriteFile(path string, data []byte) error {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	name := filepath.Base(path)
	info, err := dir.Lstat(name)
	newFile := errors.Is(err, fs.ErrNotExist)
	if err != nil && !newFile {
		return err
	}
	if !newFile && (!info.Mode().IsRegular() || info.Mode().Perm()&0o222 == 0) {
		return fmt.Errorf("%q is not a writable regular file", name)
	}

	staged := ".cyberwriter-export-" + rand.Text()
	file, err := dir.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dir.Remove(staged)
	writeErr := writeFile(file, data)
	if writeErr == nil && !newFile {
		writeErr = file.Chmod(info.Mode().Perm())
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if newFile {
		// A hard link publishes the completed file without replacing a name
		// created since the save dialog closed. Some volumes cannot link files.
		if err := dir.Link(staged, name); err == nil || errors.Is(err, fs.ErrExist) {
			return err
		}
		_ = dir.Remove(staged)
		return writeNewFile(dir, name, data)
	}
	latest, err := dir.Lstat(name)
	if err != nil || !os.SameFile(info, latest) || latest.Mode() != info.Mode() ||
		latest.Size() != info.Size() || !latest.ModTime().Equal(info.ModTime()) {
		return fmt.Errorf("%q changed while the export was being written", name)
	}
	return dir.Rename(staged, name)
}

func writeNewFile(dir *os.Root, name string, data []byte) error {
	file, err := dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	info, _ := file.Stat()
	err = errors.Join(writeFile(file, data), file.Close())
	if err != nil && info != nil {
		if latest, checkErr := dir.Lstat(name); checkErr == nil && os.SameFile(info, latest) {
			_ = dir.Remove(name)
		}
	}
	return err
}

func writeFile(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}
