package ui

import (
	"io/fs"
	"path"
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"
)

// wikiPDFIndex is an immutable snapshot of the PDF files listed in the Vault.
// An empty byName value indicates that more than one file has that name.
type wikiPDFIndex struct {
	paths  map[string]bool
	byName map[string]string
}

func (index *wikiPDFIndex) add(entry workspace.Entry) {
	index.paths[entry.Path] = true
	if _, duplicate := index.byName[entry.Name]; duplicate {
		index.byName[entry.Name] = ""
	} else {
		index.byName[entry.Name] = entry.Path
	}
}

// splitWikiPDFTarget reads a Wikilink target's optional one-based PDF page.
func splitWikiPDFTarget(target string) (name string, page int, ok bool) {
	name, fragment, hasFragment := strings.Cut(target, "#")
	name = strings.TrimSpace(name)
	if !workspace.IsPDFPath(name) || strings.HasPrefix(name, "/") ||
		strings.Contains(name, "://") || strings.ContainsAny(name, "\\\x00\r\n") {
		return "", 0, false
	}
	if !hasFragment {
		return name, 0, true
	}
	number, found := strings.CutPrefix(fragment, "page=")
	if !found || strings.Trim(number, "0123456789") != "" {
		return "", 0, false
	}
	page, err := strconv.Atoi(number)
	if err != nil || page < 1 {
		return "", 0, false
	}
	return name, page, true
}

func (res *previewResources) resolveWikiPDF(source, target string) (relative string, page int, ok bool) {
	if res == nil {
		return "", 0, false
	}
	return resolveWikiPDF(res.vault, res.wikiPDFs, source, target)
}

func (origin previewResourceOrigin) resolveWikiPDF(target string) (relative string, page int, ok bool) {
	return resolveWikiPDF(origin.vault, origin.wikiPDFs, origin.note, target)
}

func resolveWikiPDF(vault *workspace.Vault, index *wikiPDFIndex, source, target string) (relative string, page int, ok bool) {
	name, page, ok := splitWikiPDFTarget(target)
	if !ok || vault == nil || !fs.ValidPath(source) {
		return "", 0, false
	}
	relative = name
	switch {
	case strings.HasPrefix(name, "./"), strings.HasPrefix(name, "../"):
		relative = path.Join(path.Dir(source), name)
	case !strings.ContainsRune(name, '/'):
		relative = path.Join(path.Dir(source), name)
		if index != nil && !index.paths[relative] {
			relative = index.byName[name]
		}
	}
	if !fs.ValidPath(relative) {
		return "", 0, false
	}
	if index != nil {
		if !index.paths[relative] {
			return "", 0, false
		}
		return relative, page, true
	}
	// An individually opened note only has a scoped Vault. Check the exact
	// path without walking the filesystem to guess matches in other folders.
	info, err := vault.StatEntry(relative)
	if err != nil || !info.Mode().IsRegular() {
		return "", 0, false
	}
	return relative, page, true
}
