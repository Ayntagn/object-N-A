package workspace

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
)

const maxImageBytes = 4 << 20
const maxImagePixels = 1 << 25

// ResolveImagePath resolves a Markdown image destination relative to the note.
// Percent-encoded URL components are decoded once before checking the path.
func ResolveImagePath(notePath, destination string) (string, bool) {
	if _, err := lifecyclePath(notePath); err != nil {
		return "", false
	}
	link, err := url.Parse(destination)
	if err != nil || link.Scheme != "" || link.Host != "" || link.User != nil ||
		link.Path == "" || strings.HasPrefix(link.Path, "/") || strings.ContainsAny(link.Path, "\\\x00") {
		return "", false
	}
	relative := path.Join(path.Dir(notePath), link.Path)
	if _, err := lifecyclePath(relative); err != nil || imageMIME(relative) == "" {
		return "", false
	}
	return relative, true
}

// ReadPreviewImage reads a bounded, passive raster image from the Vault.
func (v *Vault) ReadPreviewImage(relative string, limit int64) ([]byte, string, error) {
	native, err := lifecyclePath(relative)
	if err != nil || limit <= 0 {
		return nil, "", fmt.Errorf("invalid preview image path %q", relative)
	}
	dir, err := openVaultDirectory(v.root, filepath.Dir(native))
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()
	data, _, err := readRegularFileLimit(dir, filepath.Base(native), limit)
	if err != nil {
		return nil, "", err
	}
	mime, err := checkImageData(relative, data)
	if err != nil {
		return nil, "", err
	}
	return data, mime, nil
}

// ImportImage returns a portable Markdown destination for a selected image.
// It copies an external image beside a saved note and leaves both inputs intact.
func (v *Vault) ImportImage(notePath, pickedAbsolute string) (string, error) {
	note, err := lifecycleNotePath(notePath)
	if err != nil {
		return "", err
	}
	picked := filepath.Clean(pickedAbsolute)
	if !filepath.IsAbs(pickedAbsolute) || strings.ContainsRune(pickedAbsolute, 0) ||
		filepath.ToSlash(picked) != filepath.ToSlash(pickedAbsolute) || strings.ContainsRune(filepath.ToSlash(picked), '\\') {
		return "", fmt.Errorf("invalid picked image path %q", pickedAbsolute)
	}
	root := systemImagePath(v.root.Name())
	picked = systemImagePath(picked)
	name := filepath.Base(picked)
	if imageMIME(name) == "" {
		return "", fmt.Errorf("%q is not a supported raster image", name)
	}

	v.writes.Lock()
	defer v.writes.Unlock()
	noteDir, err := openVaultDirectory(v.root, filepath.Dir(note))
	if err != nil {
		return "", err
	}
	defer noteDir.Close()
	relative, err := filepath.Rel(root, picked)
	if err == nil && filepath.IsLocal(relative) {
		source := filepath.ToSlash(relative)
		data, mime, err := v.ReadPreviewImage(source, maxImageBytes)
		if err != nil {
			return "", err
		}
		if err := validateImportedImage(name, data, mime); err != nil {
			return "", err
		}
		return imageDestination(note, relative)
	}

	info, err := noteDir.Lstat(filepath.Base(note))
	if err != nil {
		return "", fmt.Errorf("save the note before importing an image: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular note", notePath)
	}
	data, err := readPickedImage(picked)
	if err != nil {
		return "", err
	}
	mime, err := checkImageData(name, data)
	if err != nil {
		return "", err
	}
	if err := validateImportedImage(name, data, mime); err != nil {
		return "", err
	}
	const imageFolder = "images"
	if err := noteDir.Mkdir(imageFolder, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	images, err := openVaultDirectory(noteDir, imageFolder)
	if err != nil {
		return "", err
	}
	defer images.Close()
	name, err = publishImage(images, name, data)
	if err != nil {
		return "", fmt.Errorf("import %q: %w", name, err)
	}
	return imageDestination(note, filepath.Join(filepath.Dir(note), imageFolder, name))
}

func imageDestination(note, image string) (string, error) {
	relative, err := filepath.Rel(filepath.Dir(note), image)
	if err != nil {
		return "", err
	}
	target := url.URL{Path: filepath.ToSlash(relative)}
	// URL.String adds "./" when a colon in the first path segment would
	// otherwise be interpreted as a scheme.
	return target.String(), nil
}

func imageMIME(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}

// IsRasterImagePath reports whether the bundled raster decoder handles the extension.
func IsRasterImagePath(name string) bool {
	return imageMIME(name) != ""
}

func checkImageData(name string, data []byte) (string, error) {
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err == nil && config.Width > 0 && config.Height > 0 &&
			uint64(config.Width)*uint64(config.Height) <= maxImagePixels {
			return mime, nil
		}
		return "", fmt.Errorf("%q has invalid or oversized image dimensions", name)
	default:
		return "", fmt.Errorf("%q is not a supported raster image", name)
	}
}

func validateImportedImage(name string, data []byte, mime string) error {
	if mime != imageMIME(name) {
		return fmt.Errorf("%q does not match its image extension", name)
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("decode %q: %w", name, err)
	}
	return nil
}

// systemImagePath handles the macOS path spellings used by file dialogs and
// temporary folders. All remaining components are checked without symlinks.
func systemImagePath(name string) string {
	name = filepath.Clean(name)
	if runtime.GOOS == "darwin" {
		for _, prefix := range []string{"/var", "/tmp"} {
			if name == prefix || strings.HasPrefix(name, prefix+"/") {
				if target, err := os.Readlink(prefix); err == nil && target == "private"+prefix {
					return "/private" + name
				}
			}
		}
	}
	return name
}

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

// Open each checked directory relative to its pinned parent. Comparing the
// opened directory to its lstat result also detects replacements during open.
func openVaultDirectory(root *os.Root, directory string) (*os.Root, error) {
	if directory == "." {
		return root.OpenRoot(".")
	}
	parent := root
	for _, part := range strings.Split(filepath.ToSlash(directory), "/") {
		next, err := openVaultDirectoryEntry(parent, part)
		if parent != root {
			parent.Close()
		}
		if err != nil {
			return nil, err
		}
		parent = next
	}
	return parent, nil
}

func openVaultDirectoryEntry(parent *os.Root, name string) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a regular directory", name)
	}
	dir, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := dir.Stat(".")
	if err == nil && os.SameFile(info, opened) {
		return dir, nil
	}
	dir.Close()
	if err != nil {
		return nil, err
	}
	return nil, ErrChangedOnDisk
}

func publishImage(dir *os.Root, name string, data []byte) (string, error) {
	stage := ".cyberwriter-image-" + rand.Text()
	file, err := dir.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer dir.Remove(stage)
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return "", err
	}
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	candidate := name
	for suffix := 1; ; suffix++ {
		err := dir.Link(stage, candidate)
		if !errors.Is(err, fs.ErrExist) {
			return candidate, err
		}
		candidate = base + "-" + strconv.Itoa(suffix) + extension
	}
}
