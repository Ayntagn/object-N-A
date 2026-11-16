package workspace

import (
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
	data, opened, err := readObservedRegularFile(v.root, name, observed, 0)
	if err != nil {
		return "", IndexedNoteStamp{}, false, err
	}
	if !utf8.Valid(data) {
		return "", IndexedNoteStamp{}, false, fmt.Errorf("%q is not UTF-8 text", relative)
	}
	stamp = v.indexedNoteStamp(opened)
	if stamp != (IndexedNoteStamp{}) {
		current, err := v.root.Lstat(name)
		if err != nil || v.indexedNoteStamp(current) != stamp {
			stamp = IndexedNoteStamp{}
		}
	}
	return string(data), stamp, false, nil
}
