package workspace

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxNoteVersions      = 20
	noteVersionTimestamp = "20060102T150405.000000000Z"
)

// VersionHistory stores the previous saved Markdown at an application-owned
// location, keyed by the absolute path of the note.
type VersionHistory struct {
	Directory string
}

type NoteVersion struct {
	RecordedAt time.Time
	name       string
}

func (h VersionHistory) Record(absolutePath, text string) error {
	dir, err := h.openNote(absolutePath, true)
	if err != nil {
		return err
	}
	defer dir.Close()

	id := rand.Text()
	staged := ".pending-" + id
	file, err := dir.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer dir.Remove(staged)
	_, writeErr := io.WriteString(file, text)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	name := time.Now().UTC().Format(noteVersionTimestamp) + "-" + id + ".md"
	if err := dir.Rename(staged, name); err != nil {
		return err
	}

	versions, err := readNoteVersions(dir)
	if err != nil {
		return err
	}
	var pruneErr error
	for _, version := range versions[min(len(versions), maxNoteVersions):] {
		if err := dir.Remove(version.name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			pruneErr = errors.Join(pruneErr, err)
		}
	}
	return pruneErr
}

// List returns the newest versions first. Text is loaded when a version is read.
func (h VersionHistory) List(absolutePath string) ([]NoteVersion, error) {
	dir, err := h.openNote(absolutePath, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	versions, err := readNoteVersions(dir)
	return versions[:min(len(versions), maxNoteVersions)], err
}

func (h VersionHistory) Read(absolutePath string, version NoteVersion) (string, error) {
	dir, err := h.openNote(absolutePath, false)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	data, _, err := readRegularFile(dir, version.name)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("the version is not UTF-8 Markdown")
	}
	return string(data), nil
}

// Move transfers history after an in-app file move. Previous history at a
// reused destination is retained separately from the moved note's history.
func (h VersionHistory) Move(from, to string) error {
	source, destination := noteVersionKey(from), noteVersionKey(to)
	if source == destination {
		return nil
	}
	root, err := os.OpenRoot(h.Directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	if err := retireNoteVersions(root, destination); err != nil {
		return err
	}
	err = root.Rename(source, destination)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Retire preserves history off the active path after the note is trashed.
func (h VersionHistory) Retire(absolutePath string) error {
	root, err := os.OpenRoot(h.Directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return retireNoteVersions(root, noteVersionKey(absolutePath))
}

func retireNoteVersions(root *os.Root, name string) error {
	if _, err := root.Lstat(name); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	const retired = "retired"
	if err := root.Mkdir(retired, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return root.Rename(name, filepath.Join(retired, name+"-"+rand.Text()))
}

func noteVersionKey(absolutePath string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(absolutePath))))
}

func (h VersionHistory) openNote(absolutePath string, create bool) (*os.Root, error) {
	if create {
		if err := os.MkdirAll(h.Directory, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := os.OpenRoot(h.Directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	name := noteVersionKey(absolutePath)
	if create {
		if err := root.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("history folder %q is not a directory", name)
	}
	return root.OpenRoot(name)
}

func readNoteVersions(dir *os.Root) ([]NoteVersion, error) {
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return nil, err
	}
	var versions []NoteVersion
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		stamp, _, _ := strings.Cut(entry.Name(), "-")
		if date, err := time.Parse(noteVersionTimestamp, stamp); err == nil {
			versions = append(versions, NoteVersion{RecordedAt: date, name: entry.Name()})
		}
	}
	return versions, nil
}
