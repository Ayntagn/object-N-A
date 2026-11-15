package wysiwyg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

// TableAction describes a structural change to a visual Markdown table.
type TableAction uint8

const (
	AddTableRow TableAction = iota
	AddTableColumn
	DeleteTableRow
	DeleteTableColumn
	MoveTableRow
	MoveTableColumn
	AlignTableColumn
)

// TableGridMarkdown generates the source inserted by the size picker. Its row
// count includes the header, with a minimum of one body row.
func TableGridMarkdown(rows, cols int) string {
	rows, cols = max(2, rows), max(1, cols)
	var markdown strings.Builder
	markdown.WriteByte('|')
	for col := range cols {
		fmt.Fprintf(&markdown, " Column %d |", col+1)
	}
	markdown.WriteString("\n|" + strings.Repeat(" --- |", cols) + "\n")
	body := "|" + strings.Repeat("   |", cols) + "\n"
	for range rows - 1 {
		markdown.WriteString(body)
	}
	return markdown.String()
}

// ChangeTableStructure replaces one table version as a single rich-text undo
// step. The index is an insertion slot for add actions and an existing row or
// column otherwise. Move destinations are final indices. Deleting the sole
// column removes the table and returns nil without error.
func (s *Session) ChangeTableStructure(offset int, previous *Table, action TableAction, index, destination int, alignment parser.Align) (*Table, error) {
	current, ok := s.TableAt(offset)
	if !ok || current != previous || !s.Current(s.draft) {
		return nil, errStaleTable
	}
	if index < 0 {
		return nil, errInvalidTable
	}
	codec := TableEdit{source: current.markdown, region: &current.region}
	rows := codec.Rows()
	columns := current.ColumnCount()
	delimiter := codec.delimiters(columns)
	var afterTable bool
	switch action {
	case AddTableRow:
		if index == 0 || index > len(rows) {
			return nil, errInvalidTable
		}
		rows = slices.Insert(rows, index, emptyTableRow(columns))
	case AddTableColumn:
		if index > columns {
			return nil, errInvalidTable
		}
		for row := range rows {
			rows[row] = slices.Insert(rows[row], index, tableEmptyParagraphMarkdown)
		}
		delimiter = slices.Insert(delimiter, index, ":-----")
	case DeleteTableRow:
		if index == 0 || index >= len(rows) {
			return nil, errInvalidTable
		}
		afterTable = len(rows) == 2
		if afterTable {
			rows[1] = emptyTableRow(columns)
		} else {
			rows = slices.Delete(rows, index, index+1)
		}
	case DeleteTableColumn:
		if index >= columns {
			return nil, errInvalidTable
		}
		if columns == 1 {
			s.removeVisualTable(offset, current)
			return nil, nil
		}
		for row := range rows {
			rows[row] = slices.Delete(rows[row], index, index+1)
		}
		delimiter = slices.Delete(delimiter, index, index+1)
	case MoveTableRow:
		if index >= len(rows) || destination < 0 || destination >= len(rows) {
			return nil, errInvalidTable
		}
		if index == destination {
			return current, nil
		}
		rows = moveTableValue(rows, index, destination)
	case MoveTableColumn:
		if index >= columns || destination < 0 || destination >= columns {
			return nil, errInvalidTable
		}
		if index == destination {
			return current, nil
		}
		for row := range rows {
			rows[row] = moveTableValue(rows[row], index, destination)
		}
		delimiter = moveTableValue(delimiter, index, destination)
	case AlignTableColumn:
		if index >= columns || alignment > parser.AlignRight {
			return nil, errInvalidTable
		}
		if current.Align(index) == alignment {
			return current, nil
		}
		marker := strings.Trim(delimiter[index], ":")
		if alignment == parser.AlignLeft || alignment == parser.AlignCenter {
			marker = ":" + marker
		}
		if alignment == parser.AlignRight || alignment == parser.AlignCenter {
			marker += ":"
		}
		delimiter[index] = marker
	default:
		return nil, errInvalidTable
	}

	markdown := writeTableMarkdown(rows, delimiter)
	region, ok := singleTableRegion(markdown)
	if !ok {
		return nil, errInvalidTable
	}
	if markdown == current.markdown {
		if afterTable {
			s.MoveAfterTable(offset)
		}
		return current, nil
	}
	next := newVisualTable(markdown, region, current.id, 0)
	s.Document.Edit(func(edit *richtext.Edit) {
		replaceVisualTable(edit, offset, next)
		if afterTable {
			moveAfterTable(edit, offset)
		}
	})
	return next, nil
}

func emptyTableRow(columns int) []string {
	row := make([]string, columns)
	for col := range row {
		row[col] = tableEmptyParagraphMarkdown
	}
	return row
}

func moveTableValue[T any](values []T, from, to int) []T {
	value := values[from]
	return slices.Insert(slices.Delete(values, from, from+1), to, value)
}
