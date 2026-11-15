package snippets

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"
)

// EV-UI-240: external JSON uses the Snippet tree and UTC ISO dates.
func exportJSON(items []exportEntry) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(exportJSONRecords(items)); err != nil {
		return nil, err
	}
	return formatSnippetExportJSON(bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))), nil
}

func exportJSONRecords(items []exportEntry) []map[string]any {
	records := make([]map[string]any, 0, len(items))
	for _, item := range items {
		fields := map[string]any{
			"id": strings.ToUpper(item.ID), "title": item.Title, "content": item.Content,
			"createdAt": exportSnippetDate(item.CreatedAt).UTC().Format(time.RFC3339),
			"updatedAt": exportSnippetDate(item.UpdatedAt).UTC().Format(time.RFC3339),
			"tags":      item.Tags, "isFolder": item.IsFolder, "isExpanded": item.IsExpanded || !item.hasField("isExpanded"),
		}
		for _, key := range []string{"rtfData", "sourceURL", "sourceFileName", "sourcePageTitle", "language", "notes", "folderColorName"} {
			if item.hasField(key) {
				var value string
				if err := json.Unmarshal(item.fields[key], &value); err == nil {
					fields[key] = value
				}
			}
		}
		if item.hasField("children") {
			fields["children"] = exportJSONRecords(item.children)
		}
		records = append(records, fields)
	}
	return records
}

// Match Foundation's whitespace and solidus escaping after Go has already
// encoded the values and sorted the record keys.
func formatSnippetExportJSON(data []byte) []byte {
	var result bytes.Buffer
	depth := 0
	quoted, escaped := false, false
	for i, ch := range data {
		if quoted {
			if escaped {
				escaped = false
			} else {
				switch ch {
				case '/':
					result.WriteByte('\\')
				case '\\':
					escaped = true
				case '"':
					quoted = false
				}
			}
			result.WriteByte(ch)
			continue
		}
		switch ch {
		case '"':
			quoted = true
		case ':':
			result.WriteByte(' ')
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
		result.WriteByte(ch)
		if ch == '[' && i+1 < len(data) && data[i+1] == ']' {
			result.WriteString("\n\n")
			result.WriteString(strings.Repeat("  ", depth-1))
		}
	}
	return result.Bytes()
}
