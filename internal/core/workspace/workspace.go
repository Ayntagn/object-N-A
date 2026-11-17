// Package workspace manages Markdown files within an open Vault directory.
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
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var ErrChangedOnDisk = errors.New("the note changed on disk. Your edits are still open")

type Vault struct {
	root              *os.Root
	writes            sync.Mutex
	indexedNoteDevice uint64
}

type Entry struct {
	Name     string
	Path     string // Slash-separated path relative to the Vault.
	IsDir    bool
	Children []Entry
	modified time.Time
	size     int64
}

// ModTime returns the modification time captured when the Vault was listed.
func (entry Entry) ModTime() time.Time { return entry.modified }

// Size is the file size captured when the Vault was listed.
func (entry Entry) Size() int64 { return entry.size }

type Document struct {
	Path string
	Text string

	vault    *Vault
	readPath string
	revision [sha256.Size]byte
	missing  bool
}

func Open(root string) (*Vault, error) {
	if root == "" {
		return nil, errors.New("choose a Vault folder")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	opened, err := os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	return &Vault{root: opened, indexedNoteDevice: indexedNoteCacheDevice(opened)}, nil
}

// RootPath is the absolute directory used to open this Vault.
func (v *Vault) RootPath() string { return v.root.Name() }

func (v *Vault) Close() error {
	v.writes.Lock()
	defer v.writes.Unlock()
	return v.root.Close()
}

func (v *Vault) Entries() ([]Entry, error) {
	return readEntries(v.root.FS(), ".", -1, func(file fs.DirEntry) bool {
		name := file.Name()
		return !strings.HasPrefix(name, ".") && (file.IsDir() || IsMarkdownPath(name) || strings.EqualFold(path.Ext(name), ".excalidraw"))
	})
}

// TerminalEntries lists files and folders from a Vault directory. Depth zero
// lists its immediate contents, and a negative depth lists its entire tree.
func (v *Vault) TerminalEntries(relative string, depth int) ([]Entry, error) {
	if relative == "" {
		relative = "."
	} else {
		native, err := lifecyclePath(relative)
		if err != nil {
			return nil, err
		}
		if err := v.lifecycleDirectory(native, false); err != nil {
			return nil, err
		}
	}
	return readEntries(v.root.FS(), relative, depth, func(file fs.DirEntry) bool {
		return !strings.HasPrefix(file.Name(), ".")
	})
}

// IsMarkdownPath reports whether a path has a supported Markdown extension.
func IsMarkdownPath(name string) bool {
	ext := path.Ext(name)
	return strings.EqualFold(ext, ".md") || strings.EqualFold(ext, ".markdown")
}

func readEntries(vaultFS fs.FS, dir string, depth int, include func(fs.DirEntry) bool) ([]Entry, error) {
	files, err := fs.ReadDir(vaultFS, dir)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, file := range files {
		name := file.Name()
		if (dir == "." && strings.EqualFold(name, TrashDirectory)) || strings.HasPrefix(name, moveStagePrefix) {
			continue
		}
		if !include(file) {
			continue
		}
		relative := path.Join(dir, name)
		entry := Entry{Name: name, Path: relative}
		switch {
		case file.IsDir():
			if depth != 0 {
				children, err := readEntries(vaultFS, relative, depth-1, include)
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					return nil, err
				}
				entry.Children = children
			}
			entry.IsDir = true
		case file.Type().IsRegular():
		default:
			continue
		}
		if info, err := file.Info(); err == nil {
			entry.modified, entry.size = info.ModTime(), info.Size()
		} else if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].IsDir && !entries[j].IsDir
	})
	return entries, nil
}

func (v *Vault) ReadDocument(relative string) (Document, error) {
	if !IsMarkdownPath(relative) {
		return Document{}, fmt.Errorf("invalid Vault note path %q", relative)
	}
	return v.ReadTextDocument(relative)
}

// ReadTextDocument opens a regular UTF-8 document by its exact Vault path.
func (v *Vault) ReadTextDocument(relative string) (Document, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return Document{}, err
	}
	data, _, err := readRegularFile(v.root, native)
	if err != nil {
		return Document{}, err
	}
	if !utf8.Valid(data) {
		return Document{}, fmt.Errorf("%q is not UTF-8 text", relative)
	}
	return Document{Path: relative, Text: string(data), vault: v, readPath: relative, revision: sha256.Sum256(data)}, nil
}

func (v *Vault) SaveDocument(document Document, content string) (Document, error) {
	if document.vault != v || document.Path != document.readPath {
		return Document{}, errors.New("open the note in this Vault before saving")
	}
	if document.missing {
		saved, err := v.CreateDocument(document.Path, content)
		if errors.Is(err, fs.ErrExist) {
			return Document{}, ErrChangedOnDisk
		}
		return saved, err
	}
	if !utf8.ValidString(content) {
		return Document{}, errors.New("the source contains invalid UTF-8")
	}
	native, err := filepath.Localize(document.Path)
	if err != nil {
		return Document{}, err
	}

	v.writes.Lock()
	defer v.writes.Unlock()
	dir, err := v.root.OpenRoot(filepath.Dir(native))
	if err != nil {
		return Document{}, err
	}
	defer dir.Close()
	name := filepath.Base(native)
	current, info, err := readRegularFile(dir, name)
	if err != nil {
		return Document{}, err
	}
	if sha256.Sum256(current) != document.revision {
		return Document{}, ErrChangedOnDisk
	}
	if string(current) != content {
		if info.Mode().Perm()&0o222 == 0 {
			return Document{}, fmt.Errorf("%q is read-only: %w", document.Path, fs.ErrPermission)
		}
		if err := replaceFile(dir, name, info, document.revision, content); err != nil {
			return Document{}, fmt.Errorf("save %q: %w", document.Path, err)
		}
	}
	document.Text = content
	document.revision = sha256.Sum256([]byte(content))
	return document, nil
}

func readRegularFile(dir *os.Root, name string) ([]byte, os.FileInfo, error) {
	return readRegularFileLimit(dir, name, 0)
}

func readRegularFileLimit(dir *os.Root, name string, limit int64) ([]byte, os.FileInfo, error) {
	var maximum int64
	if limit > 0 {
		maximum = limit + 1
	}
	data, info, err := readRegularFilePrefix(dir, name, maximum)
	if limit > 0 && int64(len(data)) > limit {
		return nil, nil, fmt.Errorf("%q exceeds the preview size limit", name)
	}
	return data, info, err
}

// readRegularFilePrefix reads at most maximum bytes, or the entire file when
// maximum is zero. Callers can use one extra byte to detect truncation.
func readRegularFilePrefix(dir *os.Root, name string, maximum int64) ([]byte, os.FileInfo, error) {
	info, err := dir.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	return readObservedRegularFile(dir, name, info, maximum)
}

func readObservedRegularFile(dir *os.Root, name string, info os.FileInfo, maximum int64) ([]byte, os.FileInfo, error) {
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%q is not a regular file", name)
	}
	file, err := dir.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, nil, ErrChangedOnDisk
	}
	var reader io.Reader = file
	if maximum > 0 {
		reader = io.LimitReader(file, maximum)
	}
	data, err := io.ReadAll(reader)
	return data, opened, err
}

func replaceFile(dir *os.Root, name string, info os.FileInfo, revision [sha256.Size]byte, content string) error {
	stagedName := ".cyberwriter-save-" + rand.Text()
	staged, err := dir.OpenFile(stagedName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dir.Remove(stagedName)
	_, writeErr := staged.WriteString(content)
	if writeErr == nil {
		writeErr = staged.Chmod(info.Mode().Perm())
	}
	if writeErr == nil {
		writeErr = staged.Sync()
	}
	if err := errors.Join(writeErr, staged.Close()); err != nil {
		return err
	}
	// Recheck after staging so a detected external edit leaves the original intact.
	current, latest, err := readRegularFile(dir, name)
	if err != nil {
		return err
	}
	if !os.SameFile(info, latest) || sha256.Sum256(current) != revision {
		return ErrChangedOnDisk
	}
	return dir.Rename(stagedName, name)
}
