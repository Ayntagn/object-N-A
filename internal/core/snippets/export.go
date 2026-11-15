package snippets

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"time"
)

type ExportFormat uint8

const (
	ExportMarkdown ExportFormat = iota
	ExportJSON
	ExportText
)

// exportEntry keeps optional-field presence from the saved collection. Native
// exports distinguish an empty language from a language that is absent.
type exportEntry struct {
	Snippet
	fields   map[string]json.RawMessage
	children []exportEntry
}

// ExportCollection encodes a snapshot of the selected local collection.
func ExportCollection(filename string, format ExportFormat) ([]byte, error) {
	return exportCollectionAt(filename, format, time.Now())
}

func exportCollectionAt(filename string, format ExportFormat, now time.Time) ([]byte, error) {
	items, err := readExportCollection(filename)
	if err != nil {
		return nil, err
	}
	switch format {
	case ExportMarkdown:
		return []byte(exportMarkdown(items, now)), nil
	case ExportJSON:
		return exportJSON(items)
	case ExportText:
		return []byte(exportText(items, now)), nil
	default:
		return nil, errors.New("unsupported Snippets export format")
	}
}

func readExportCollection(filename string) ([]exportEntry, error) {
	items, records, err := readCollection(filename)
	if err != nil {
		return nil, err
	}
	return exportEntries(items, records)
}

func exportEntries(items []Snippet, records []json.RawMessage) ([]exportEntry, error) {
	entries := make([]exportEntry, len(items))
	for i, item := range items {
		entry := &entries[i]
		entry.Snippet = item
		if err := json.Unmarshal(records[i], &entry.fields); err != nil {
			return nil, err
		}
		if entry.hasField("children") {
			var children []json.RawMessage
			if err := json.Unmarshal(entry.fields["children"], &children); err != nil {
				return nil, err
			}
			var err error
			entry.children, err = exportEntries(item.Children, children)
			if err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

func (entry exportEntry) hasField(key string) bool {
	raw, ok := entry.fields[key]
	return ok && string(raw) != "null"
}

func (entry exportEntry) exportSource() (string, bool) {
	if entry.hasField("sourceFileName") {
		return entry.SourceFileName, true
	}
	if !entry.hasField("sourceURL") {
		return "", false
	}
	if origin, err := url.Parse(entry.SourceURL); err == nil && origin.Host != "" {
		return origin.Hostname(), true
	}
	return entry.SourceURL, true
}

func exportSnippetCount(items []exportEntry) int {
	count := 0
	for _, item := range items {
		if item.IsFolder {
			count += exportSnippetCount(item.children)
		} else {
			count++
		}
	}
	return count
}

func exportSnippetDate(referenceSeconds float64) time.Time {
	const referenceEpoch = 978307200
	seconds, fraction := math.Modf(referenceSeconds)
	return time.Unix(referenceEpoch+int64(seconds), int64(fraction*1e9))
}

func exportLocalDate(date time.Time) string {
	return date.Format("Jan 2, 2006 at 15:04")
}
