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
	mygoDir     = "third_party/mygo-167"
)

//go:embed mygo/LICENSE mygo/ui/editor/*.go.txt
var mygoOverlay embed.FS

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
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(name)), data, 0o644); err != nil {
			return fmt.Errorf("apply MyGo file %s: %w", name, err)
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

func mygoOverlayFiles() (map[string][]byte, error) {
	const editorDir = "mygo/ui/editor"
	entries, err := mygoOverlay.ReadDir(editorDir)
	if err != nil {
		return nil, err
	}
	license, err := mygoOverlay.ReadFile("mygo/LICENSE")
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"LICENSE":           license,
		".object-na-source": []byte(mygoModule + " " + mygoVersion + " " + mygoSum + "\n"),
	}
	for _, entry := range entries {
		data, err := mygoOverlay.ReadFile(path.Join(editorDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		files[path.Join("ui/editor", strings.TrimSuffix(entry.Name(), ".txt"))] = data
	}
	return files, nil
}

func matchesMyGo(root *os.Root, files map[string][]byte) bool {
	for name, want := range files {
		data, err := root.ReadFile(path.Join(mygoDir, name))
		if err != nil || !bytes.Equal(data, want) {
			return false
		}
	}
	info, err := root.Lstat(path.Join(mygoDir, "go.mod"))
	return err == nil && info.Mode().IsRegular()
}
