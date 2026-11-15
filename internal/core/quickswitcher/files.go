// Package quickswitcher filters the Vault's navigable files for quick switching.
package quickswitcher

import (
	"path"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

type File struct {
	Path, Name, Folder string
	Drawing            bool
	name, folder       []string
}

// Files builds the search catalog from the existing Vault listing. The
// platform comparison uses full relative filenames, including extensions.
func Files(entries []workspace.Entry, compare func(string, string) int) []File {
	var files []File
	lower := cases.Lower(language.Und)
	var seg segmenter.Segmenter
	var add func([]workspace.Entry)
	add = func(entries []workspace.Entry) {
		for _, entry := range entries {
			if entry.IsDir {
				add(entry.Children)
				continue
			}
			folder, name := path.Split(entry.Path)
			folder = strings.TrimSuffix(folder, "/")
			drawing := strings.EqualFold(path.Ext(name), ".excalidraw")
			if !drawing && !workspace.IsMarkdownPath(name) {
				continue
			}
			if !drawing {
				name = strings.TrimSuffix(name, path.Ext(name))
			}
			files = append(files, File{
				Path: entry.Path, Name: name, Folder: folder, Drawing: drawing,
				name: searchCharacters(&seg, lower.String(name)), folder: searchCharacters(&seg, lower.String(folder)),
			})
		}
	}
	add(entries)
	slices.SortStableFunc(files, func(a, b File) int { return compare(a.Path, b.Path) })
	return files
}

// Search returns the first twenty catalog entries for an empty query. For a
// nonempty query it preserves catalog order within each matching tier.
func Search(files []File, query string) []File {
	if query == "" {
		return files[:min(20, len(files))]
	}
	var seg segmenter.Segmenter
	key := searchCharacters(&seg, cases.Lower(language.Und).String(query))
	var exact, prefix, name, folder, subsequence []File
	for _, file := range files {
		switch {
		case slices.Equal(file.name, key):
			exact = append(exact, file)
		case len(file.name) >= len(key) && slices.Equal(file.name[:len(key)], key):
			prefix = append(prefix, file)
		case contains(file.name, key):
			name = append(name, file)
		case contains(file.folder, key):
			folder = append(folder, file)
		case isSubsequence(file.name, key):
			subsequence = append(subsequence, file)
		}
	}
	return slices.Concat(exact, prefix, name, folder, subsequence)
}

func searchCharacters(seg *segmenter.Segmenter, text string) []string {
	text = norm.NFC.String(text)
	if text == "" {
		return nil
	}
	if err := seg.InitWithString(text); err != nil {
		return []string{text}
	}
	var characters []string
	for it := seg.GraphemeIterator(); it.Next(); {
		characters = append(characters, string(it.Grapheme().Text))
	}
	return characters
}

func contains(text, query []string) bool {
	for i := 0; i+len(query) <= len(text); i++ {
		if slices.Equal(text[i:i+len(query)], query) {
			return true
		}
	}
	return false
}

func isSubsequence(text, query []string) bool {
	i := 0
	for _, char := range text {
		if char == query[i] {
			i++
			if i == len(query) {
				return true
			}
		}
	}
	return false
}
