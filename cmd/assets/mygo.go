package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const (
	mygoModule  = "github.com/egoist/mygo"
	mygoVersion = "v0.8.3"
	mygoSum     = "h1:aTLEhS221YhsX+gKGNLQ5xEKMLCcYYr3pH9AkTwIRs4="
	mygoDir     = "third_party/mygo-176"
)

//go:embed mygo/LICENSE mygo
var mygoOverlay embed.FS

type mygoFile struct {
	name string
	data []byte
}

func restoreMyGo(root *os.Root) error {
	if err := prepareDirectory(root, "third_party"); err != nil {
		return err
	}
	files, err := mygoOverlayFiles()
	if err != nil {
		return err
	}
	if info, err := root.Lstat(mygoDir); err == nil {
		if info.IsDir() && matchesMyGo(root, files) {
			fmt.Println("Verified patched MyGo", mygoVersion)
			return nil
		}
		return fmt.Errorf("move the existing %s aside before restoring MyGo %s", mygoDir, mygoVersion)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	command := exec.Command("go", "mod", "download", "-json", mygoModule+"@"+mygoVersion)
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("download pinned Go module: %w", err)
	}
	var download struct{ Path, Version, Dir, Sum string }
	if err := json.Unmarshal(output, &download); err != nil {
		return fmt.Errorf("read Go module metadata: %w", err)
	}
	if download.Path != mygoModule || download.Version != mygoVersion || download.Sum != mygoSum || !filepath.IsAbs(download.Dir) {
		return errors.New("the downloaded module does not match the pinned MyGo source")
	}

	stage, err := os.MkdirTemp(filepath.Join(root.Name(), "third_party"), ".mygo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(stage, os.DirFS(download.Dir)); err != nil {
		return fmt.Errorf("copy the verified MyGo source: %w", err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(file.name)), file.data, 0o644); err != nil {
			return fmt.Errorf("apply MyGo file %s: %w", file.name, err)
		}
	}
	// Publish the complete module in one rename. Another invocation may
	// have already completed the same restore while this one was copying.
	if err := root.Rename(path.Join("third_party", filepath.Base(stage)), mygoDir); err != nil && !matchesMyGo(root, files) {
		return fmt.Errorf("install the patched MyGo module: %w", err)
	}
	fmt.Println("Restored patched MyGo", mygoVersion)
	return nil
}

func mygoOverlayFiles() ([]mygoFile, error) {
	files := []mygoFile{
		{name: ".object-na-source", data: []byte(mygoModule + " " + mygoVersion + " " + mygoSum + "\n")},
	}
	err := fs.WalkDir(mygoOverlay, "mygo", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(name, "mygo/")
		if relative != "LICENSE" {
			if !strings.HasSuffix(relative, ".go.txt") {
				return fmt.Errorf("unexpected MyGo overlay file %s", relative)
			}
			relative = strings.TrimSuffix(relative, ".txt")
		}
		data, err := mygoOverlay.ReadFile(name)
		if err != nil {
			return err
		}
		files = append(files, mygoFile{name: relative, data: data})
		return nil
	})
	return files, err
}

func matchesMyGo(root *os.Root, files []mygoFile) bool {
	for _, file := range files {
		data, err := root.ReadFile(path.Join(mygoDir, file.name))
		if err != nil || !bytes.Equal(data, file.data) {
			return false
		}
	}
	info, err := root.Lstat(path.Join(mygoDir, "go.mod"))
	return err == nil && info.Mode().IsRegular()
}
