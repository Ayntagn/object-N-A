package workspace

import (
	"fmt"
	"time"
)

type DocumentInfo struct {
	Modified time.Time
	Created  time.Time
	Size     int64
}

func (v *Vault) DocumentInfo(relative string) (DocumentInfo, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return DocumentInfo{}, err
	}
	info, err := v.root.Lstat(native)
	if err != nil {
		return DocumentInfo{}, err
	}
	if !info.Mode().IsRegular() {
		return DocumentInfo{}, fmt.Errorf("%q is not a regular file", relative)
	}
	return DocumentInfo{Modified: info.ModTime(), Created: creationTime(info), Size: info.Size()}, nil
}
