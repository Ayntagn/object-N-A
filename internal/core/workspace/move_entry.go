package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// StatEntry reports the type of a regular file or directory in the Vault.
func (v *Vault) StatEntry(relative string) (fs.FileInfo, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return nil, err
	}
	return v.statEntry(native)
}

func (v *Vault) statEntry(native string) (fs.FileInfo, error) {
	if err := v.checkLifecycleParent(native); err != nil {
		return nil, err
	}
	info, err := v.root.Lstat(native)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular Vault file or directory", native)
	}
	return info, nil
}

// MoveEntry moves a file or directory inside the Vault without replacing the
// destination. Missing destination parents are created before the rename.
func (v *Vault) MoveEntry(from, to string) error {
	return v.moveEntry(from, to, true, nil)
}

// AttachmentMove binds an open editor to the destination of a completed move.
type AttachmentMove struct {
	from, to string
	vault    *Vault
}

// RelocateDocument preserves the saved revision used to detect external edits.
// An individually opened file may have been read through its parent directory.
func (move AttachmentMove) RelocateDocument(doc Document) Document {
	if move.vault != nil && doc.vault != nil && doc.Path == doc.readPath &&
		filepath.Join(doc.vault.root.Name(), filepath.FromSlash(doc.readPath)) ==
			filepath.Join(move.vault.root.Name(), filepath.FromSlash(move.from)) {
		doc.Path, doc.readPath, doc.vault = move.to, move.to, move.vault
	}
	return doc
}

// MoveAttachment moves a regular file between existing Vault directories
// and checks that the selected file still has the same identity.
func (v *Vault) MoveAttachment(from, to string, expected fs.FileInfo) (AttachmentMove, error) {
	if err := v.moveEntry(from, to, false, expected); err != nil {
		return AttachmentMove{}, err
	}
	return AttachmentMove{from: from, to: to, vault: v}, nil
}

func (v *Vault) moveEntry(from, to string, createParents bool, expected fs.FileInfo) error {
	source, err := lifecyclePath(from)
	if err != nil {
		return err
	}
	dest, err := lifecyclePath(to)
	if err != nil {
		return err
	}
	v.writes.Lock()
	defer v.writes.Unlock()

	info, err := v.statEntry(source)
	if err != nil {
		return err
	}
	if !createParents && !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular Vault file", from)
	}
	if !createParents && (expected == nil || !os.SameFile(info, expected)) {
		return ErrChangedOnDisk
	}
	if info.IsDir() && strings.HasPrefix(to, from+"/") {
		return fmt.Errorf("cannot move %q into itself", from)
	}
	if !createParents && source == dest {
		return nil
	}
	if _, err := v.root.Lstat(dest); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(dest)
	if err := v.lifecycleDirectory(parent, createParents); err != nil {
		return err
	}
	fromDir, err := v.root.Open(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer fromDir.Close()
	toDir, err := v.root.Open(parent)
	if err != nil {
		return err
	}
	defer toDir.Close()
	if err := renameEntry(fromDir, filepath.Base(source), toDir, filepath.Base(dest)); err != nil {
		return &os.LinkError{Op: "move", Old: from, New: to, Err: err}
	}
	return nil
}
