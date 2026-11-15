package vaultindex

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"path"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"golang.org/x/text/unicode/norm"
)

const (
	maxUnlinkedMentionsPerSource = 5
	maxUnlinkedMentions          = 200
)

var ErrStaleUnlinkedMentions = errors.New("the source changed since the unlinked mentions were scanned")

type UnlinkedMentionHit struct {
	Path           string
	SourceName     string
	Start, End     int // Byte offsets in the saved source, including frontmatter.
	ExpectedText   string
	Replacement    string
	Snippet        string
	SourceRevision [sha256.Size]byte
}

type UnlinkedMentionResult struct {
	Hits      []UnlinkedMentionHit
	Total     int // Displayed count. Truncated means further occurrences exist.
	Truncated bool
}

// ScanUnlinkedMentions reads saved Markdown for occurrences of one note's
// filename and aliases. EV-UI-169 and EV-UI-171 specify the side panel limits.
func ScanUnlinkedMentions(ctx context.Context, vault *workspace.Vault, targetPath string) (UnlinkedMentionResult, error) {
	if err := ctx.Err(); err != nil {
		return UnlinkedMentionResult{}, err
	}
	if vault == nil {
		return UnlinkedMentionResult{}, errors.New("open a Vault before scanning for unlinked mentions")
	}
	target, err := vault.ReadDocument(targetPath)
	if err != nil {
		return UnlinkedMentionResult{}, err
	}
	name := markdownStem(path.Base(target.Path))
	aliases, _ := healthAliases(target.Text)
	entities := mentionEntities([]string{name}, map[string][]string{norm.NFC.String(name): aliases})
	var phrases []string
	for _, candidate := range append([]string{name}, aliases...) {
		phrase := healthKey(candidate)
		if !shortMention(phrase) {
			phrases = append(phrases, phrase)
		}
	}
	if len(phrases) == 0 || len(entities) == 0 {
		return UnlinkedMentionResult{}, ctx.Err()
	}
	entries, err := vault.Entries()
	if err != nil {
		return UnlinkedMentionResult{}, err
	}
	var result UnlinkedMentionResult
	for _, entry := range healthFiles(entries) {
		if err := ctx.Err(); err != nil {
			return UnlinkedMentionResult{}, err
		}
		if entry.Path == target.Path || !mentionSourcePath(entry.Path) {
			continue
		}
		doc, err := vault.ReadDocument(entry.Path)
		if err != nil {
			continue
		}
		text := healthKey(doc.Text)
		if !slices.ContainsFunc(phrases, func(phrase string) bool { return strings.Contains(text, phrase) }) {
			continue
		}
		sourceName := markdownStem(entry.Name)
		revision := sha256.Sum256([]byte(doc.Text))
		endOfSource := len(result.Hits) + maxUnlinkedMentionsPerSource
		limit := min(maxUnlinkedMentions, endOfSource)
		scanMentionRanges(doc.Text, "", entities, limit-len(result.Hits)+1, func(title string, start, end int) {
			if len(result.Hits) == limit {
				result.Truncated = true
				return
			}
			matched := doc.Text[start:end]
			replacement := "[[" + title + "]]"
			if norm.NFC.String(title) != norm.NFC.String(matched) {
				replacement = "[[" + title + "|" + matched + "]]"
			}
			result.Hits = append(result.Hits, UnlinkedMentionHit{
				Path: doc.Path, SourceName: sourceName, Start: start, End: end,
				ExpectedText: matched, Replacement: replacement, SourceRevision: revision,
				Snippet: mentionSnippet(doc.Text, start, end),
			})
		})
		if len(result.Hits) == maxUnlinkedMentions && result.Truncated {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return UnlinkedMentionResult{}, err
	}
	result.Total = len(result.Hits)
	return result, nil
}

func mentionSnippet(source string, start, end int) string {
	var before, after string
	first, last := start, end
	for remaining := 30; first > 0 && remaining > 0; {
		r, size := utf8.DecodeLastRuneInString(source[:first])
		remaining -= utf16.RuneLen(r)
		if remaining < 0 {
			before = string(utf8.RuneError)
			break
		}
		first -= size
	}
	for remaining := 40; last < len(source) && remaining > 0; {
		r, size := utf8.DecodeRuneInString(source[last:])
		remaining -= utf16.RuneLen(r)
		if remaining < 0 {
			after = string(utf8.RuneError)
			break
		}
		last += size
	}
	text := strings.ReplaceAll(before+source[first:last]+after, "\n", " ")
	return "…" + strings.TrimFunc(text, foundationWhitespace) + "…"
}

type UnlinkedMentionEdit struct {
	Start, End int
	Text       string
}

// ApplyUnlinkedMentions builds a single validated patch for one saved file.
// Save the result with the same document to use the Vault revision check.
func ApplyUnlinkedMentions(doc workspace.Document, hits []UnlinkedMentionHit) (string, error) {
	edits, err := validatedUnlinkedMentionEdits(doc, hits)
	if err != nil || len(edits) == 0 {
		return doc.Text, err
	}
	var patched strings.Builder
	patched.Grow(len(doc.Text))
	previous := 0
	for _, edit := range edits {
		patched.WriteString(doc.Text[previous:edit.Start])
		patched.WriteString(edit.Text)
		previous = edit.End
	}
	patched.WriteString(doc.Text[previous:])
	return patched.String(), nil
}

// UnlinkedMentionBufferEdits maps validated raw file spans to an LF-normalized
// editor snapshot. The entire snapshot must still match the saved revision.
func UnlinkedMentionBufferEdits(doc workspace.Document, bufferText string, hits []UnlinkedMentionHit) ([]UnlinkedMentionEdit, error) {
	edits, err := validatedUnlinkedMentionEdits(doc, hits)
	if err != nil || len(edits) == 0 {
		return edits, err
	}
	if bufferText != strings.ReplaceAll(doc.Text, "\r\n", "\n") {
		return nil, ErrStaleUnlinkedMentions
	}
	previous, removed := 0, 0
	for i, edit := range edits {
		if splitsCRLF(doc.Text, edit.Start) || splitsCRLF(doc.Text, edit.End) {
			return nil, ErrStaleUnlinkedMentions
		}
		removed += strings.Count(doc.Text[previous:edit.Start], "\r\n")
		edits[i].Start -= removed
		removed += strings.Count(doc.Text[edit.Start:edit.End], "\r\n")
		edits[i].End -= removed
		previous = edit.End
	}
	return edits, nil
}

func splitsCRLF(source string, offset int) bool {
	return offset > 0 && offset < len(source) && source[offset-1:offset+1] == "\r\n"
}

func validatedUnlinkedMentionEdits(doc workspace.Document, hits []UnlinkedMentionHit) ([]UnlinkedMentionEdit, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	revision := sha256.Sum256([]byte(doc.Text))
	edits := make([]UnlinkedMentionEdit, 0, len(hits))
	for _, hit := range hits {
		if hit.Path != doc.Path || hit.SourceRevision != revision || hit.Start < 0 || hit.Start >= hit.End ||
			hit.End > len(doc.Text) || !utf8.RuneStart(doc.Text[hit.Start]) ||
			hit.End < len(doc.Text) && !utf8.RuneStart(doc.Text[hit.End]) ||
			doc.Text[hit.Start:hit.End] != hit.ExpectedText || !utf8.ValidString(hit.Replacement) {
			return nil, ErrStaleUnlinkedMentions
		}
		edits = append(edits, UnlinkedMentionEdit{Start: hit.Start, End: hit.End, Text: hit.Replacement})
	}
	slices.SortFunc(edits, func(a, b UnlinkedMentionEdit) int { return cmp.Compare(a.Start, b.Start) })
	for i := 1; i < len(edits); i++ {
		if edits[i-1].End > edits[i].Start {
			return nil, ErrStaleUnlinkedMentions
		}
	}
	return edits, nil
}
