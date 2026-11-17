//go:build !darwin

package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

func readPickedImage(absolute string) ([]byte, error) {
	volume := filepath.VolumeName(absolute)
	rootPath := volume + string(filepath.Separator)
	source, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	name := strings.TrimPrefix(absolute, rootPath)
	dir, err := openVaultDirectory(source, filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	data, _, err := readRegularFileLimit(dir, filepath.Base(name), maxImageBytes)
	return data, err
}
