package snippets

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/go-text/typesetting/segmenter"
)

type VaultExportNote struct {
	Path, Content string
}

// VaultExportNotes renders the selected snippet or an aggregate note when
// selectedID is empty. The caller publishes the relative path in its Vault.
func VaultExportNotes(filename, selectedID string) ([]VaultExportNote, error) {
	return vaultExportNotesAt(filename, selectedID, time.Now())
}

func vaultExportNotesAt(filename, selectedID string, now time.Time) ([]VaultExportNote, error) {
	items, err := readExportCollection(filename)
	if err != nil {
		return nil, err
	}
	if selectedID == "" {
		return []VaultExportNote{{Path: "Snippets/Snippets Export.md", Content: exportMarkdown(items, now)}}, nil
	}
	selected, found := exportEntryByID(items, selectedID)
	if !found {
		return nil, errors.New("the snippet is no longer in the library")
	}
	return []VaultExportNote{{Path: path.Join("Snippets", vaultSnippetFilename(selected.Title)), Content: exportVaultSnippet(selected)}}, nil
}

func exportEntryByID(items []exportEntry, id string) (exportEntry, bool) {
	for _, item := range items {
		if !item.IsFolder && item.ID == id {
			return item, true
		}
		if child, found := exportEntryByID(item.children, id); found {
			return child, true
		}
	}
	return exportEntry{}, false
}

// EV-UI-239: use the original Vault filename sanitizer before adding .md.
func vaultSnippetFilename(title string) string {
	const restricted = "/\\:?%*|\"<>"
	name := strings.TrimSpace(strings.Map(func(r rune) rune {
		if strings.ContainsRune(restricted, r) {
			return '-'
		}
		return r
	}, title))
	if name == "" {
		name = "Snippet"
	}
	var segments segmenter.Segmenter
	_ = segments.InitWithString(name)
	characters := segments.GraphemeIterator()
	for i := 0; characters.Next(); i++ {
		if i == 80 {
			name = name[:characters.Grapheme().OffsetInBytes]
			break
		}
	}
	return name + ".md"
}

func exportVaultSnippet(item exportEntry) string {
	var out strings.Builder
	fmt.Fprintf(&out, "---\ntitle: %s\ncreated: %s\n", vaultSnippetYAML(item.Title), exportSnippetDate(item.CreatedAt).UTC().Format(time.RFC3339))
	if item.hasField("language") {
		fmt.Fprintf(&out, "language: %s\n", item.Language)
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(&out, "tags: [%s]\n", strings.Join(item.Tags, ", "))
	}
	if source, ok := item.exportSource(); ok {
		fmt.Fprintf(&out, "source: %s\n", vaultSnippetYAML(source))
	}
	out.WriteString("---\n\n")
	if item.Notes != "" {
		fmt.Fprintf(&out, "> %s\n\n", item.Notes)
	}
	fmt.Fprintf(&out, "\x60\x60\x60%s\n%s\n\x60\x60\x60\n", item.Language, item.Content)
	return out.String()
}

func vaultSnippetYAML(value string) string {
	if strings.ContainsAny(value, ":#") {
		return `"` + strings.ReplaceAll(value, `"`, `'`) + `"`
	}
	return value
}
