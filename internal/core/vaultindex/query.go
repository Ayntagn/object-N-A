package vaultindex

import (
	"cmp"
	"errors"
	"path"
	"slices"
	"strconv"
	"strings"

	"cyberwriter/internal/core/workspace"

	"golang.org/x/text/cases"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// ErrInvalidQuery indicates that the source is empty or has a malformed clause.
var ErrInvalidQuery = errors.New("invalid query")

// QueryResult contains the visible property headings and matching notes.
// Nil Columns represents LIST. TABLE always includes a separate File column.
type QueryResult struct {
	Columns []string
	Rows    []QueryRow
}

// QueryRow contains the display name and exact Vault-relative path of a note.
// Cells follow the order of the property headings in QueryResult.Columns.
type QueryRow struct {
	Path  string
	Name  string
	Cells []string
}

var queryCaseFold = cases.Fold()

// Query evaluates a fenced query's body against the saved Vault snapshot.
// A nil index acts as an empty snapshot, including when there is no open Vault.
func (index *Index) Query(source string) (QueryResult, error) {
	query, err := parseQuery(source)
	if err != nil {
		return QueryResult{}, err
	}
	result := QueryResult{Columns: query.columns}
	if index == nil || query.limit == 0 {
		return result, nil
	}
	type match struct {
		note *note
		name string
	}
	var matches []match
	for id := range index.notes {
		note := &index.notes[id]
		if query.matches(note) {
			matches = append(matches, match{note: note, name: markdownStem(path.Base(note.entry.Path))})
		}
	}
	if len(matches) > 1 {
		if query.sortField == "" {
			names := collate.New(language.Und, collate.IgnoreCase)
			slices.SortStableFunc(matches, func(a, b match) int {
				return names.CompareString(a.name, b.name)
			})
		} else {
			slices.SortStableFunc(matches, func(a, b match) int {
				order := compareQueryValues(a.note.properties[query.sortField], b.note.properties[query.sortField])
				if query.descending {
					return -order
				}
				return order
			})
		}
	}
	if query.limit >= 0 && query.limit < len(matches) {
		matches = matches[:query.limit]
	}
	columnKeys := make([]string, len(query.columns))
	for i, column := range query.columns {
		columnKeys[i] = strings.ToLower(column)
	}
	result.Rows = make([]QueryRow, 0, len(matches))
	for _, match := range matches {
		row := QueryRow{Path: match.note.entry.Path, Name: match.name}
		if len(columnKeys) != 0 {
			row.Cells = make([]string, len(columnKeys))
			for i, key := range columnKeys {
				row.Cells[i] = queryPropertyText(match.note.properties[key])
			}
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func (query querySpec) matches(note *note) bool {
	switch query.scope {
	case queryFolder:
		prefix := query.source
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if note.entry.Path != query.source && !strings.HasPrefix(note.entry.Path, prefix) {
			return false
		}
	case queryTag:
		if !slices.Contains(note.graphTags, query.source) {
			return false
		}
	}
	for _, condition := range query.conditions {
		field, ok := note.properties[condition.field]
		if !condition.matches(field, ok) {
			return false
		}
	}
	return true
}

func (condition queryCondition) matches(field workspace.Property, exists bool) bool {
	if !exists {
		return condition.op == "!="
	}
	switch condition.op {
	case "=", "==":
		return equalQueryValues(field, condition.value)
	case "!=":
		return !equalQueryValues(field, condition.value)
	case "contains":
		if field.Kind == workspace.PropertyList {
			return slices.ContainsFunc(field.Items, func(item workspace.Property) bool {
				return equalQueryValues(item, condition.value)
			})
		}
		return strings.Contains(queryFoldText(field), queryFoldText(condition.value))
	case ">":
		return compareQueryValues(field, condition.value) > 0
	case "<":
		return compareQueryValues(field, condition.value) < 0
	case ">=":
		return compareQueryValues(field, condition.value) >= 0
	case "<=":
		return compareQueryValues(field, condition.value) <= 0
	}
	return false
}

func equalQueryValues(a, b workspace.Property) bool {
	if a.Kind == workspace.PropertyNumber && b.Kind == workspace.PropertyNumber {
		x, _ := strconv.ParseFloat(a.Text, 64)
		y, _ := strconv.ParseFloat(b.Text, 64)
		return x == y
	}
	return queryFoldText(a) == queryFoldText(b)
}

func compareQueryValues(a, b workspace.Property) int {
	aMissing := a.Kind == "" || a.Kind == workspace.PropertyNull
	bMissing := b.Kind == "" || b.Kind == workspace.PropertyNull
	if aMissing && bMissing {
		return 0
	}
	if aMissing {
		return 1
	}
	if bMissing {
		return -1
	}
	if a.Kind == workspace.PropertyNumber && b.Kind == workspace.PropertyNumber {
		x, _ := strconv.ParseFloat(a.Text, 64)
		y, _ := strconv.ParseFloat(b.Text, 64)
		return cmp.Compare(x, y)
	}
	if a.Kind == workspace.PropertyDate && b.Kind == workspace.PropertyDate {
		// The Properties reader normalizes dates to year-month-day.
		return strings.Compare(a.Text, b.Text)
	}
	return strings.Compare(queryFoldText(a), queryFoldText(b))
}

func queryFoldText(property workspace.Property) string {
	return norm.NFC.String(queryCaseFold.String(queryPropertyText(property)))
}

func queryPropertyText(property workspace.Property) string {
	switch property.Kind {
	case "", workspace.PropertyNull:
		return ""
	case workspace.PropertyList:
		values := make([]string, len(property.Items))
		for i, item := range property.Items {
			values[i] = queryPropertyText(item)
		}
		return strings.Join(values, ", ")
	case workspace.PropertyObject:
		values := make([]string, len(property.Items))
		for i, item := range property.Items {
			values[i] = item.Key + ": " + queryPropertyText(item)
		}
		return strings.Join(values, ", ")
	}
	return property.Text
}
