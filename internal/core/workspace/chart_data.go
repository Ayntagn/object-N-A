package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type ChartDataFile struct {
	Path     string
	size     int64
	modified time.Time
}

// FindChartData accepts Vault paths and bare filenames, like a Wikilink.
func (v *Vault) FindChartData(note, reference string) ChartDataFile {
	if reference == "" || path.IsAbs(reference) {
		return ChartDataFile{}
	}
	local := path.Join(path.Dir(note), reference)
	switch {
	case strings.HasPrefix(reference, "./"), strings.HasPrefix(reference, "../"):
		return v.statChartData(local)
	case strings.Contains(reference, "/"):
		if file := v.statChartData(path.Clean(reference)); file.Path != "" {
			return file
		}
		return v.statChartData(local)
	}
	if file := v.statChartData(local); file.Path != "" {
		return file
	}
	var match string
	err := fs.WalkDir(v.root.FS(), ".", func(relative string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() && strings.EqualFold(relative, TrashDirectory) {
			return fs.SkipDir
		}
		if entry.Type().IsRegular() && entry.Name() == reference {
			if match != "" {
				return fmt.Errorf("ambiguous chart data filename %q", reference)
			}
			match = relative
		}
		return nil
	})
	if err != nil || match == "" {
		return ChartDataFile{}
	}
	return v.statChartData(match)
}

func chartDataPath(relative string) (string, error) {
	native, err := filepath.Localize(relative)
	if err != nil || strings.EqualFold(strings.SplitN(relative, "/", 2)[0], TrashDirectory) {
		return "", fmt.Errorf("invalid chart data path %q", relative)
	}
	return native, nil
}

func (v *Vault) statChartData(relative string) ChartDataFile {
	native, err := chartDataPath(relative)
	if err != nil {
		return ChartDataFile{}
	}
	info, err := v.root.Lstat(native)
	if err != nil || !info.Mode().IsRegular() {
		return ChartDataFile{}
	}
	return ChartDataFile{Path: relative, size: info.Size(), modified: info.ModTime()}
}

func (v *Vault) ChartDataChanged(file ChartDataFile, note, reference string) bool {
	if file.Path == "" {
		return v.FindChartData(note, reference).Path != ""
	}
	return v.statChartData(file.Path) != file
}

func (v *Vault) ReadChartData(file ChartDataFile, limit int64) (string, error) {
	native, err := chartDataPath(file.Path)
	if err != nil || limit <= 0 {
		return "", fmt.Errorf("unavailable chart data %q", file.Path)
	}
	data, _, err := readRegularFileLimit(v.root, native, limit)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("chart data %q is not UTF-8", file.Path)
	}
	return strings.TrimPrefix(string(data), "\ufeff"), nil
}
