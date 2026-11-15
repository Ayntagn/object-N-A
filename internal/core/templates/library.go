package templates

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"runtime"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
)

const untitled = "Untitled Template"

type Entry struct {
	Filename string
	Title    string
}

// Heading follows the template manager's line-based title extraction.
func Heading(markdown string) string {
	for line := range strings.SplitSeq(markdown, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			if title = strings.TrimSpace(title); title != "" {
				return title
			}
		}
	}
	return ""
}

// Save copies the current Markdown and returns the name used in the notice.
// It assigns a new filename even when the contents match an earlier template.
func Save(dir, markdown string) (string, error) {
	if strings.TrimSpace(markdown) == "" {
		return "", nil
	}
	name := strings.NewReplacer("/", "-", ":", "-", "\x00", "").Replace(Heading(markdown))
	if name == "" {
		name = untitled
	}
	if runtime.GOOS == "windows" {
		name = windowsFilenameStem(name)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	vault, err := workspace.Open(dir)
	if err != nil {
		return "", err
	}
	defer vault.Close()
	for number := 1; ; number++ {
		filename := name + ".md"
		if number > 1 {
			filename = fmt.Sprintf("%s %d.md", name, number)
		}
		if _, err := vault.CreateDocument(filename, markdown); !errors.Is(err, fs.ErrExist) {
			return name, err
		}
	}
}

func windowsFilenameStem(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>"\|?*`, r) {
			return '-'
		}
		return r
	}, name)
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return untitled
	}
	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	stem = strings.TrimRight(stem, " ")
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		"COM¹", "COM²", "COM³", "LPT¹", "LPT²", "LPT³":
		return "_" + name
	}
	return name
}

func List(dir string) ([]Entry, error) {
	vault, err := workspace.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer vault.Close()
	files, err := vault.TerminalEntries("", 0)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, file := range files {
		if file.IsDir || !strings.EqualFold(path.Ext(file.Name), ".md") {
			continue
		}
		doc, err := vault.ReadDocument(file.Name)
		if err != nil {
			continue
		}
		title := Heading(doc.Text)
		if title == "" {
			title = stem(file.Name)
		}
		entries = append(entries, Entry{Filename: file.Name, Title: title})
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)), cmp.Compare(stem(a.Filename), stem(b.Filename)))
	})
	return entries, nil
}

func Load(dir, filename string) (string, error) {
	if path.Base(filename) != filename || !strings.EqualFold(path.Ext(filename), ".md") {
		return "", fmt.Errorf("invalid template filename %q", filename)
	}
	vault, err := workspace.Open(dir)
	if err != nil {
		return "", err
	}
	defer vault.Close()
	doc, err := vault.ReadDocument(filename)
	return doc.Text, err
}

func stem(filename string) string {
	return strings.TrimSuffix(filename, path.Ext(filename))
}
