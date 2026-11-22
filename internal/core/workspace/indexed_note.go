package workspace

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"unicode/utf8"
)

// IndexedNoteStamp compares the actual file identity and last change time.
// The zero value means a previous read cannot be reused.
type IndexedNoteStamp struct {
	device, inode     uint64
	changed, modified int64
	size              int64
}

func (stamp IndexedNoteStamp) version() string {
	if stamp == (IndexedNoteStamp{}) {
		return ""
	}
	var key [40]byte
	binary.LittleEndian.PutUint64(key[0:8], stamp.device)
	binary.LittleEndian.PutUint64(key[8:16], stamp.inode)
	binary.LittleEndian.PutUint64(key[16:24], uint64(stamp.changed))
	binary.LittleEndian.PutUint64(key[24:32], uint64(stamp.modified))
	binary.LittleEndian.PutUint64(key[32:40], uint64(stamp.size))
	return hex.EncodeToString(key[:])
}

func (v *Vault) indexedNoteStamp(info os.FileInfo) IndexedNoteStamp {
	if v.indexedNoteDevice == 0 || !info.Mode().IsRegular() {
		return IndexedNoteStamp{}
	}
	device, inode, changed := indexedNoteIdentity(info)
	if device != v.indexedNoteDevice || inode == 0 || changed == 0 {
		return IndexedNoteStamp{}
	}
	return IndexedNoteStamp{device, inode, changed, info.ModTime().UnixNano(), info.Size()}
}

// ReadIndexedNote validates the saved note and reads it unless the exact file
// and its change stamp match a previously completed read.
func (v *Vault) ReadIndexedNote(relative string, previous IndexedNoteStamp) (
	text string, stamp IndexedNoteStamp, unchanged bool, err error,
) {
	name, err := lifecycleNotePath(relative)
	if err != nil {
		return "", IndexedNoteStamp{}, false, err
	}
	observed, err := v.root.Lstat(name)
	if err != nil {
		return "", IndexedNoteStamp{}, false, err
	}
	if previous != (IndexedNoteStamp{}) && v.indexedNoteStamp(observed) == previous {
		return "", previous, true, nil
	}
	text, stamp, err = v.readStampedText(name, observed, 0)
	return text, stamp, false, err
}

// HasIndexedTextVersions reports whether the mounted volume supplies reliable
// identity and change stamps for saved UTF-8 text.
func (v *Vault) HasIndexedTextVersions() bool {
	return v.indexedNoteDevice != 0
}

// IndexedTextVersion returns the current reusable version or the empty string.
func (v *Vault) IndexedTextVersion(relative string) string {
	if !v.HasIndexedTextVersions() {
		return ""
	}
	name, err := lifecyclePath(relative)
	if err != nil {
		return ""
	}
	info, err := v.root.Lstat(name)
	if err != nil {
		return ""
	}
	return v.indexedNoteStamp(info).version()
}

// ReadIndexedText returns a text version suitable for a derived cache. A known
// matching version permits reuse without reading the file. An empty version
// means there is no reusable result, including text exceeding the cache limit.
func (v *Vault) ReadIndexedText(relative, known string, limit int64) (text, version string, err error) {
	if !v.HasIndexedTextVersions() {
		return "", "", nil
	}
	name, err := lifecyclePath(relative)
	if err != nil {
		return "", "", err
	}
	observed, err := v.root.Lstat(name)
	if err != nil {
		return "", "", err
	}
	expected := v.indexedNoteStamp(observed).version()
	if expected == "" || (limit > 0 && observed.Size() > limit) {
		return "", "", nil
	}
	if expected == known {
		return "", expected, nil
	}
	content, stamp, err := v.readStampedText(name, observed, limit)
	return content, stamp.version(), err
}

func (v *Vault) readStampedText(name string, observed os.FileInfo, limit int64) (string, IndexedNoteStamp, error) {
	var maximum int64
	if limit > 0 {
		maximum = limit + 1
	}
	data, opened, err := readObservedRegularFile(v.root, name, observed, maximum)
	if err != nil {
		return "", IndexedNoteStamp{}, err
	}
	if limit > 0 && int64(len(data)) > limit {
		return "", IndexedNoteStamp{}, nil
	}
	if !utf8.Valid(data) {
		return "", IndexedNoteStamp{}, fmt.Errorf("%q is not UTF-8 text", name)
	}
	stamp := v.indexedNoteStamp(opened)
	if stamp != (IndexedNoteStamp{}) {
		current, err := v.root.Lstat(name)
		if err != nil || v.indexedNoteStamp(current) != stamp {
			stamp = IndexedNoteStamp{}
		}
	}
	return string(data), stamp, nil
}
