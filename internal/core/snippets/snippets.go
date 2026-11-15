package snippets

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-text/typesetting/segmenter"
)

// Snippet uses the EV-UI-054 Codable fields. Dates are seconds since 2001-01-01 UTC.
type Snippet struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	CreatedAt       float64   `json:"createdAt"`
	UpdatedAt       float64   `json:"updatedAt"`
	RTFData         []byte    `json:"rtfData,omitempty"`
	SourceURL       string    `json:"sourceURL,omitempty"`
	SourceFileName  string    `json:"sourceFileName,omitempty"`
	SourcePageTitle string    `json:"sourcePageTitle,omitempty"`
	Language        string    `json:"language,omitempty"`
	Tags            []string  `json:"tags"`
	Notes           string    `json:"notes,omitempty"`
	IsFolder        bool      `json:"isFolder"`
	Children        []Snippet `json:"children,omitempty"`
	IsExpanded      bool      `json:"isExpanded"`
	FolderColorName string    `json:"folderColorName,omitempty"`
}

type Details struct {
	Title, Content, Language, Notes string
	Tags                            []string
}

var collectionWriteMu sync.Mutex

func Capture(content, fileName, pageTitle string, now time.Time) Snippet {
	var id [16]byte
	rand.Read(id[:])
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	captured := snippetTimestamp(now)
	language := detectLanguage(content)
	tags := []string{}
	if language != "" {
		tags = append(tags, language)
	}
	if fileName != "" {
		parts := strings.Split(fileName, ".")
		extension := strings.ToLower(parts[len(parts)-1])
		if extension != language && graphemeCount(extension, 6) <= 5 {
			tags = append(tags, extension)
		}
	}
	return Snippet{
		ID:    fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]),
		Title: title(content, pageTitle), Content: content, CreatedAt: captured, UpdatedAt: captured,
		SourceFileName: fileName, SourcePageTitle: pageTitle, Language: language, Tags: tags,
	}
}

func snippetTimestamp(now time.Time) float64 {
	const referenceEpoch = 978307200
	return float64(now.Unix()-referenceEpoch) + float64(now.Nanosecond())/1e9
}

func title(content, preferred string) string {
	if preferred != "" && graphemeCount(preferred, 51) <= 50 {
		return preferred
	}
	line := content
	if end := strings.IndexAny(line, "\n\r\v\f\u0085\u2028\u2029"); end >= 0 {
		line = line[:end]
	}
	line = strings.TrimFunc(line, unicode.IsSpace)
	if line == "" {
		return "Untitled Snippet"
	}
	var segments segmenter.Segmenter
	_ = segments.InitWithString(line)
	graphemes := segments.GraphemeIterator()
	count, prefix := 0, 0
	for graphemes.Next() {
		count++
		if count == 48 {
			prefix = graphemes.Grapheme().OffsetInBytes
		}
		if count > 50 {
			return line[:prefix] + "…"
		}
	}
	return line
}

func graphemeCount(text string, limit int) int {
	var segments segmenter.Segmenter
	_ = segments.InitWithString(text)
	graphemes := segments.GraphemeIterator()
	count := 0
	for count < limit && graphemes.Next() {
		count++
	}
	return count
}

func Load(filename string) ([]Snippet, error) {
	items, _, err := readCollection(filename)
	return items, err
}

func readCollection(filename string) ([]Snippet, []json.RawMessage, error) {
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var records []json.RawMessage
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, nil, err
	}
	if records == nil {
		return nil, nil, errors.New("expected a Snippets array")
	}
	items := make([]Snippet, len(records))
	for i, record := range records {
		if err := json.Unmarshal(record, &items[i]); err != nil {
			return nil, nil, err
		}
		if items[i].ID == "" || items[i].Tags == nil {
			return nil, nil, errors.New("a Snippets record is missing its ID or tags")
		}
	}
	return items, records, nil
}

func Append(filename string, item Snippet) ([]Snippet, error) {
	collectionWriteMu.Lock()
	defer collectionWriteMu.Unlock()
	items, records, err := readCollection(filename)
	if err != nil {
		return nil, err
	}
	record, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	// Keep fields and optional values of existing records as stored by the original.
	records = append(records, record)
	if err := writeCollection(filename, records); err != nil {
		return nil, err
	}
	return append(items, item), nil
}

// Update edits the current record by UUID, retaining unknown fields on it and its ancestors.
func Update(filename, id string, edit func(*Details)) ([]Snippet, error) {
	collectionWriteMu.Lock()
	defer collectionWriteMu.Unlock()
	items, records, err := readCollection(filename)
	if err != nil {
		return nil, err
	}
	found, err := updateDetails(items, records, id, edit)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("the snippet is no longer in the library")
	}
	if err := writeCollection(filename, records); err != nil {
		return nil, err
	}
	return items, nil
}

func updateDetails(items []Snippet, records []json.RawMessage, id string, edit func(*Details)) (bool, error) {
	for i := range items {
		item := &items[i]
		if item.ID != id && len(item.Children) == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(records[i], &fields); err != nil {
			return false, err
		}
		if item.ID == id {
			values := Details{item.Title, item.Content, item.Language, item.Notes, slices.Clone(item.Tags)}
			edit(&values)
			changed := make(map[string]any)
			if item.Title != values.Title {
				changed["title"] = values.Title
			}
			if item.Content != values.Content {
				changed["content"] = values.Content
			}
			if item.Language != values.Language {
				changed["language"] = values.Language
			}
			if item.Notes != values.Notes {
				changed["notes"] = values.Notes
			}
			if !slices.Equal(item.Tags, values.Tags) {
				changed["tags"] = append([]string{}, values.Tags...)
			}
			if len(changed) == 0 {
				return true, nil
			}
			item.Title, item.Content = values.Title, values.Content
			item.Language, item.Notes, item.Tags = values.Language, values.Notes, values.Tags
			item.UpdatedAt = snippetTimestamp(time.Now())
			changed["updatedAt"] = item.UpdatedAt
			for name, value := range changed {
				raw, err := json.Marshal(value)
				if err != nil {
					return false, err
				}
				fields[name] = raw
			}
		} else {
			var children []json.RawMessage
			if err := json.Unmarshal(fields["children"], &children); err != nil {
				return false, err
			}
			found, err := updateDetails(item.Children, children, id, edit)
			if err != nil {
				return false, err
			}
			if !found {
				continue
			}
			fields["children"], err = json.Marshal(children)
			if err != nil {
				return false, err
			}
		}
		updated, err := json.Marshal(fields)
		if err == nil {
			records[i] = updated
		}
		return true, err
	}
	return false, nil
}

func writeCollection(filename string, records []json.RawMessage) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	pending, err := os.CreateTemp(dir, ".snippets-*")
	if err != nil {
		return err
	}
	defer os.Remove(pending.Name())
	_, writeErr := pending.Write(data)
	if err := errors.Join(writeErr, pending.Close()); err != nil {
		return err
	}
	return os.Rename(pending.Name(), filename)
}

func Search(items []Snippet, query string) []Snippet {
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []Snippet
	var visit func([]Snippet)
	visit = func(entries []Snippet) {
		for _, item := range entries {
			if item.IsFolder {
				visit(item.Children)
				continue
			}
			if item.matches(query) {
				matches = append(matches, item)
			}
		}
	}
	visit(items)
	slices.SortStableFunc(matches, func(a, b Snippet) int {
		switch {
		case a.UpdatedAt > b.UpdatedAt:
			return -1
		case a.UpdatedAt < b.UpdatedAt:
			return 1
		default:
			return 0
		}
	})
	return matches
}

func (item Snippet) matches(query string) bool {
	for _, field := range []string{item.Title, item.Content, item.Notes} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return slices.ContainsFunc(item.Tags, func(tag string) bool {
		return strings.Contains(strings.ToLower(tag), query)
	})
}
