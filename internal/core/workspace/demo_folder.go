package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// TrashGeneratedFolder moves an unchanged demo folder and its sole document
// through the Vault's recoverable trash workflow.
func (v *Vault) TrashGeneratedFolder(relative string, created fs.FileInfo, document Document, trash func(string) error) (string, error) {
	if created == nil || document.vault != v || path.Dir(document.Path) != relative || trash == nil {
		return "", errors.New("the demo folder is unavailable")
	}
	native, err := lifecyclePath(relative)
	if err != nil {
		return "", err
	}
	v.writes.Lock()
	defer v.writes.Unlock()

	current, err := v.statEntry(native)
	if err != nil {
		return "", err
	}
	items, err := fs.ReadDir(v.root.FS(), relative)
	if err != nil {
		return "", err
	}
	if !os.SameFile(current, created) || len(items) != 1 || items[0].Name() != path.Base(document.Path) {
		return "", ErrChangedOnDisk
	}
	data, _, err := readRegularFile(v.root, filepath.FromSlash(document.Path))
	if err != nil {
		return "", err
	}
	if string(data) != document.Text {
		return "", ErrChangedOnDisk
	}
	return v.trashEntry(native, current, trash)
}
