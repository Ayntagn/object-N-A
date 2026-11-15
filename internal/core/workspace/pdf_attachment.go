package workspace

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// IsPDFPath reports whether an ordinary Vault click opens a PDF viewer.
func IsPDFPath(relative string) bool {
	return strings.EqualFold(path.Ext(relative), ".pdf")
}

// ReadVaultPDF reads the bytes of a regular PDF attachment without following
// a symlink at the file or anywhere between the Vault and its directory.
func (v *Vault) ReadVaultPDF(relative string) ([]byte, error) {
	if !IsPDFPath(relative) {
		return nil, fmt.Errorf("invalid Vault PDF path %q", relative)
	}
	native, err := lifecyclePath(relative)
	if err != nil {
		return nil, err
	}
	dir, err := openVaultDirectory(v.root, filepath.Dir(native))
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	data, _, err := readRegularFile(dir, filepath.Base(native))
	return data, err
}
