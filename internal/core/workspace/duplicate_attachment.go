package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// DuplicateAttachment copies a regular Vault file to a uniquely named sibling.
func (v *Vault) DuplicateAttachment(relative string) (_ string, err error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return "", err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	dir, err := openVaultDirectory(v.root, filepath.Dir(native))
	if err != nil {
		return "", err
	}
	defer dir.Close()
	name := filepath.Base(native)
	info, err := dir.Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular Vault file", relative)
	}
	source, err := dir.Open(name)
	if err != nil {
		return "", err
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, opened) {
		return "", ErrChangedOnDisk
	}
	parent, err := dir.Open(".")
	if err != nil {
		return "", err
	}
	defer parent.Close()
	staged, err := stageLifecycleFile(dir, ".", source, opened.Mode().Perm(), opened.ModTime())
	if err != nil {
		return "", fmt.Errorf("duplicate %q: %w", relative, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, dir.Remove(staged))
		}
	}()
	current, err := source.Stat()
	if err != nil {
		return "", err
	}
	if current.Size() != opened.Size() || !current.ModTime().Equal(opened.ModTime()) || current.Mode() != opened.Mode() {
		return "", ErrChangedOnDisk
	}
	for number := 1; ; number++ {
		candidate := duplicateFileName(name, number)
		err := renameEntry(parent, staged, parent, candidate)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("duplicate %q: %w", relative, err)
		}
		return path.Join(path.Dir(relative), candidate), nil
	}
}
