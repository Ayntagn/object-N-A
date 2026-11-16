package workspace

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// TrashDirectory holds recoverable notes in unique directories inside the Vault.
const TrashDirectory = ".cyberwriter-trash"
const moveStagePrefix = ".cyberwriter-move-"

// Relocated updates a draft's path from a completed move while keeping the
// draft's previous saved revision for conflict detection.
func (d Document) Relocated(to Document) Document {
	if d.vault != nil && d.vault == to.vault && to.Path == to.readPath {
		d.Path, d.readPath = to.Path, to.readPath
	}
	return d
}

// NeedsRecreation reports whether saving must create the file exclusively.
func (d Document) NeedsRecreation() bool { return d.missing }

// PrepareDocumentRecreation binds an open draft to a note removed by a Vault restore.
func (v *Vault) PrepareDocumentRecreation(relative string) (Document, error) {
	native, err := lifecycleNotePath(relative)
	if err != nil {
		return Document{}, err
	}
	if err := v.checkLifecycleParent(native); err != nil {
		return Document{}, err
	}
	if _, err := v.root.Lstat(native); err == nil {
		return Document{}, ErrChangedOnDisk
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Document{}, err
	}
	return Document{Path: relative, vault: v, readPath: relative, missing: true}, nil
}

// CreateDocument writes a new UTF-8 Markdown note at a Vault-relative path.
func (v *Vault) CreateDocument(relative, content string) (Document, error) {
	native, err := lifecycleNotePath(relative)
	if err != nil {
		return Document{}, err
	}
	return v.createDocument(relative, native, content, false)
}

// CreateTextDocument writes a new UTF-8 document and creates missing parent folders.
func (v *Vault) CreateTextDocument(relative, content string) (Document, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return Document{}, err
	}
	return v.createDocument(relative, native, content, true)
}

func (v *Vault) createDocument(relative, native, content string, createParents bool) (Document, error) {
	if !utf8.ValidString(content) {
		return Document{}, errors.New("the source contains invalid UTF-8")
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	if err := v.lifecycleDirectory(filepath.Dir(native), createParents); err != nil {
		return Document{}, err
	}
	if err := v.writeNewLifecycleFile(native, content, 0o600); err != nil {
		return Document{}, fmt.Errorf("create %q: %w", relative, err)
	}
	return lifecycleDocument(v, relative, content), nil
}

// CreateFolder creates one folder within an existing Vault directory.
func (v *Vault) CreateFolder(relative string) error {
	native, err := lifecyclePath(relative)
	if err != nil {
		return err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	if err := v.checkLifecycleParent(native); err != nil {
		return err
	}
	return v.root.Mkdir(native, 0o755)
}

// ErrFolderNameSeparators identifies the characters reserved by the naming prompt.
var ErrFolderNameSeparators = errors.New("folder names can't contain “/” or “:”")

// CreateNamedFolder applies the New Folder naming rules to one child of parent.
// A blank submission reports false without making a filesystem change.
func (v *Vault) CreateNamedFolder(parent, input string) (bool, error) {
	name := strings.TrimSpace(input)
	switch {
	case name == "":
		return false, nil
	case strings.ContainsAny(name, "/:"):
		return false, ErrFolderNameSeparators
	case name == "." || name == "..":
		return false, fs.ErrExist
	}
	if !fs.ValidPath(parent) {
		return false, fmt.Errorf("invalid Vault parent path %q", parent)
	}
	err := v.CreateFolder(path.Join(parent, name))
	return err == nil, err
}

// EnsureFolder creates a Vault directory and its missing parents.
func (v *Vault) EnsureFolder(relative string) error {
	native, err := lifecyclePath(relative)
	if err != nil {
		return err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	return v.lifecycleDirectory(native, true)
}

// DuplicateDocument creates a uniquely named sibling from the saved note bytes.
func (v *Vault) DuplicateDocument(relative string) (Document, error) {
	native, err := lifecycleNotePath(relative)
	if err != nil {
		return Document{}, err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	if err := v.checkLifecycleParent(native); err != nil {
		return Document{}, err
	}
	data, info, err := readRegularFile(v.root, native)
	if err != nil {
		return Document{}, err
	}
	if !utf8.Valid(data) {
		return Document{}, fmt.Errorf("%q is not UTF-8 Markdown", relative)
	}
	content := string(data)
	for number := 1; ; number++ {
		candidate := duplicateFileName(relative, number)
		dest := filepath.FromSlash(candidate)
		if _, err := v.root.Lstat(dest); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return Document{}, err
		}
		err := v.writeNewLifecycleFile(dest, content, info.Mode().Perm())
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return Document{}, fmt.Errorf("duplicate %q: %w", relative, err)
		}
		return lifecycleDocument(v, candidate, content), nil
	}
}

func duplicateFileName(name string, number int) string {
	extension := path.Ext(name)
	stem := strings.TrimSuffix(name, extension) + " copy"
	if number > 1 {
		return fmt.Sprintf("%s %d%s", stem, number, extension)
	}
	return stem + extension
}

// MoveDocument renames or moves a Markdown note within the Vault.
func (v *Vault) MoveDocument(from, to string) (Document, error) {
	source, err := lifecycleNotePath(from)
	if err != nil {
		return Document{}, err
	}
	dest, err := lifecycleNotePath(to)
	if err != nil {
		return Document{}, err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	if err := v.checkLifecycleParent(source); err != nil {
		return Document{}, err
	}
	if err := v.checkLifecycleParent(dest); err != nil {
		return Document{}, err
	}
	data, info, err := readRegularFile(v.root, source)
	if err != nil {
		return Document{}, err
	}
	if !utf8.Valid(data) {
		return Document{}, fmt.Errorf("%q is not UTF-8 Markdown", from)
	}
	if source != dest {
		current, err := v.root.Lstat(dest)
		if err == nil && !os.SameFile(info, current) {
			return Document{}, fmt.Errorf("move to %q: %w", to, fs.ErrExist)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Document{}, err
		}
		if err := v.moveLifecycleFile(source, dest, info); err != nil {
			return Document{}, fmt.Errorf("move %q to %q: %w", from, to, err)
		}
	}
	return lifecycleDocument(v, to, string(data)), nil
}

// TrashDocument moves a Markdown file into the Vault recovery directory. The
// file keeps its original relative path within a unique recovery directory.
func (v *Vault) TrashDocument(relative string) error {
	native, err := lifecycleNotePath(relative)
	if err != nil {
		return err
	}
	v.writes.Lock()
	defer v.writes.Unlock()
	if err := v.checkLifecycleParent(native); err != nil {
		return err
	}
	info, err := v.root.Lstat(native)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular Markdown file", relative)
	}
	if err := v.root.Mkdir(TrashDirectory, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	item := filepath.Join(TrashDirectory, rand.Text())
	if err := v.checkLifecycleParent(item); err != nil {
		return err
	}
	if err := v.root.Mkdir(item, 0o700); err != nil {
		return err
	}
	dest := filepath.Join(item, native)
	if err := v.root.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return errors.Join(err, v.removeEmptyRecoveryDirs(filepath.Dir(dest)))
	}
	if err := v.moveLifecycleFile(native, dest, info); err != nil {
		return fmt.Errorf("move %q to trash: %w", relative, errors.Join(err, v.removeEmptyRecoveryDirs(filepath.Dir(dest))))
	}
	return nil
}

func lifecyclePath(relative string) (string, error) {
	native, err := filepath.Localize(relative)
	first, _, _ := strings.Cut(relative, "/")
	if err != nil || relative == "." || strings.EqualFold(first, TrashDirectory) {
		return "", fmt.Errorf("invalid Vault path %q", relative)
	}
	return native, nil
}

func lifecycleNotePath(relative string) (string, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return "", err
	}
	if !IsMarkdownPath(relative) {
		return "", fmt.Errorf("invalid Vault note path %q", relative)
	}
	return native, nil
}

func lifecycleDocument(v *Vault, relative, content string) Document {
	return Document{Path: relative, Text: content, vault: v, readPath: relative, revision: sha256.Sum256([]byte(content))}
}

func (v *Vault) checkLifecycleParent(native string) error {
	return v.lifecycleDirectory(filepath.Dir(native), false)
}

func (v *Vault) lifecycleDirectory(native string, create bool) error {
	if native == "." {
		return nil
	}
	var prefix string
	for part := range strings.SplitSeq(filepath.ToSlash(native), "/") {
		prefix = filepath.Join(prefix, part)
		info, err := v.root.Lstat(prefix)
		if create && errors.Is(err, fs.ErrNotExist) {
			err = v.root.Mkdir(prefix, 0o755)
			if err == nil {
				continue
			}
			if errors.Is(err, fs.ErrExist) {
				info, err = v.root.Lstat(prefix)
			}
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a Vault directory", prefix)
		}
	}
	return nil
}

func (v *Vault) writeNewLifecycleFile(native, content string, mode fs.FileMode) error {
	dir, err := openVaultDirectory(v.root, filepath.Dir(native))
	if err != nil {
		return err
	}
	defer dir.Close()
	parent, err := dir.Open(".")
	if err != nil {
		return err
	}
	defer parent.Close()
	staged, err := stageLifecycleFile(dir, ".", strings.NewReader(content), mode, time.Time{})
	if err != nil {
		return err
	}

	name := filepath.Base(native)
	err = renameEntry(parent, staged, parent, name)
	if err == nil {
		return nil
	}
	// Linux can return EINVAL when the filesystem does not support the
	// fixed RENAME_NOREPLACE flag. Both paths here are sibling basenames.
	renameUnsupported := unsupportedLifecycleCommit(err) || (runtime.GOOS == "linux" && errors.Is(err, syscall.EINVAL))
	if renameUnsupported {
		err = dir.Link(staged, name)
		if unsupportedLifecycleCommit(err) {
			err = writeExclusiveLifecycleFile(dir, name, strings.NewReader(content), mode)
		}
	} else {
		err = &os.LinkError{Op: "rename", Old: staged, New: name, Err: err}
	}
	cleanupErr := dir.Remove(staged)
	if err != nil {
		return errors.Join(err, cleanupErr)
	}
	return nil
}

// Filesystems without either atomic publishing operation still reserve the
// final name exclusively. On failure, remove it only if it still identifies
// the file created here.
func writeExclusiveLifecycleFile(dir *os.Root, name string, content io.Reader, mode fs.FileMode) error {
	file, err := dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	created, identityErr := file.Stat()
	err = writeAndCloseLifecycleFile(file, content, mode)
	if err == nil {
		return nil
	}
	if identityErr != nil {
		return errors.Join(err, identityErr)
	}
	current, cleanupErr := dir.Lstat(name)
	if cleanupErr == nil && os.SameFile(created, current) {
		cleanupErr = dir.Remove(name)
	}
	if errors.Is(cleanupErr, fs.ErrNotExist) {
		return err
	}
	return errors.Join(err, cleanupErr)
}

func stageLifecycleFile(dir *os.Root, parent string, content io.Reader, mode fs.FileMode, modified time.Time) (_ string, err error) {
	staged := filepath.Join(parent, ".cyberwriter-new-"+rand.Text())
	file, err := dir.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, dir.Remove(staged))
		}
	}()
	if err := writeAndCloseLifecycleFile(file, content, mode); err != nil {
		return "", err
	}
	if !modified.IsZero() {
		if err := dir.Chtimes(staged, time.Time{}, modified); err != nil {
			return "", err
		}
	}
	return staged, nil
}

func writeAndCloseLifecycleFile(file *os.File, content io.Reader, mode fs.FileMode) error {
	_, err := io.Copy(file, content)
	if err == nil {
		err = file.Chmod(mode)
	}
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

// Move through a private directory so removing the source pathname cannot
// unlink an unrelated note that appeared there during the operation.
func (v *Vault) moveLifecycleFile(source, dest string, original fs.FileInfo) (err error) {
	stage := filepath.Join(filepath.Dir(source), moveStagePrefix+rand.Text())
	if err := v.root.Mkdir(stage, 0o700); err != nil {
		return err
	}
	backup := filepath.Join(stage, ".original")
	held := filepath.Join(stage, filepath.Base(source))
	var backedUp, holding, retain bool
	defer func() {
		if retain {
			return
		}
		var cleanupErr error
		if holding {
			cleanupErr = v.root.Remove(held)
		}
		if backedUp {
			cleanupErr = errors.Join(cleanupErr, v.root.Remove(backup))
		}
		cleanupErr = errors.Join(cleanupErr, v.root.Remove(stage))
		if err != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	// Confirm this filesystem can restore the source without overwriting any
	// replacement before moving the source into the private directory.
	if err := v.root.Link(source, backup); err != nil {
		return err
	}
	backedUp = true
	if err := v.sameLifecycleFile(backup, original); err != nil {
		return err
	}
	if err := v.checkLifecycleParent(source); err != nil {
		return err
	}
	if err := v.sameLifecycleFile(source, original); err != nil {
		return err
	}
	if err := v.root.Rename(source, held); err != nil {
		return err
	}
	holding = true
	err = v.sameLifecycleFile(held, original)
	if err == nil {
		err = v.checkLifecycleParent(dest)
	}
	if err == nil {
		err = v.root.Link(held, dest)
	}
	if err != nil {
		restoreErr := v.checkLifecycleParent(source)
		if restoreErr == nil {
			restoreErr = v.root.Link(held, source)
		}
		if restoreErr != nil {
			retain = true
			return errors.Join(err, fmt.Errorf("restore %q from %q: %w", source, held, restoreErr))
		}
	}
	return err
}

func (v *Vault) sameLifecycleFile(native string, original fs.FileInfo) error {
	info, err := v.root.Lstat(native)
	if err != nil {
		return err
	}
	if !os.SameFile(info, original) {
		return ErrChangedOnDisk
	}
	return nil
}

func (v *Vault) removeEmptyRecoveryDirs(dir string) error {
	for dir != TrashDirectory {
		info, err := v.root.Lstat(dir)
		if err == nil && info.IsDir() {
			err = v.root.Remove(dir)
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		dir = filepath.Dir(dir)
	}
	return nil
}
