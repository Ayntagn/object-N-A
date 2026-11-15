package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const terminalTrash = ".cwtrash"

// TrashEntry moves an exact Vault path to the system trash or the local
// recovery directory. The callback must leave the source in place on failure.
func (v *Vault) TrashEntry(relative string, trash func(string) error) (string, error) {
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
	return v.trashEntry(native, current, trash)
}

// TrashAttachment moves a regular file to the system Trash.
func (v *Vault) TrashAttachment(relative string, expected fs.FileInfo, trash func(string) error) error {
	native, err := lifecyclePath(relative)
	if err != nil {
		return err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	current, err := v.statEntry(native)
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || expected == nil || !os.SameFile(current, expected) {
		return ErrChangedOnDisk
	}
	_, err = v.trashInSystem(native, current, trash)
	return err
}

func (v *Vault) trashEntry(native string, expected fs.FileInfo, trash func(string) error) (string, error) {
	attempted, err := v.trashInSystem(native, expected, trash)
	if err == nil {
		return "Trash", nil
	}
	if !attempted {
		return "", err
	}
	if recoveryErr := v.recoverTrashEntry(native, expected); recoveryErr != nil {
		return "", errors.Join(err, recoveryErr)
	}
	return terminalTrash + "/", nil
}

func (v *Vault) trashInSystem(native string, expected fs.FileInfo, trash func(string) error) (bool, error) {
	if trash == nil {
		return false, errors.New("system Trash is unavailable")
	}
	root, err := v.root.Stat(".")
	if err != nil {
		return false, err
	}
	visibleRoot, err := os.Stat(v.root.Name())
	if err != nil {
		return false, err
	}
	if !os.SameFile(root, visibleRoot) {
		return false, ErrChangedOnDisk
	}
	// The OS API takes an absolute path. Check that it names the same item
	// as the confined Vault handle before delegating the move.
	absolute := filepath.Join(v.root.Name(), native)
	visible, err := os.Lstat(absolute)
	if err != nil {
		return false, err
	}
	if !os.SameFile(visible, expected) {
		return false, ErrChangedOnDisk
	}
	return true, trash(absolute)
}

func (v *Vault) recoverTrashEntry(native string, expected fs.FileInfo) error {
	if strings.EqualFold(native, terminalTrash) {
		return errors.New("the Vault trash cannot be moved into itself")
	}
	if err := v.lifecycleDirectory(terminalTrash, true); err != nil {
		return err
	}
	from, err := v.root.Open(filepath.Dir(native))
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := v.root.Open(terminalTrash)
	if err != nil {
		return err
	}
	defer to.Close()
	base := filepath.Base(native)
	sourceInTrash, err := from.Stat()
	if err != nil {
		return err
	}
	recovery, err := to.Stat()
	if err != nil {
		return err
	}
	sameDirectory := os.SameFile(sourceInTrash, recovery)
	for target := base; ; {
		current, err := v.statEntry(native)
		if err != nil {
			return err
		}
		if !os.SameFile(current, expected) {
			return ErrChangedOnDisk
		}
		// Some platforms treat renaming a file to itself as success.
		if !sameDirectory || target != base {
			err = renameEntry(from, base, to, target)
			if !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
		var suffix [4]byte
		rand.Read(suffix[:])
		target = strings.ToUpper(hex.EncodeToString(suffix[:])) + "-" + base
	}
}
