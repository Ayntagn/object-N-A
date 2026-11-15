package liveapi

import (
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	maxTableRows    = 100
	maxTableColumns = 100
)

// Table is a display snapshot with plain text cells and counts beyond the
// visible bounds. The full content remains in Result.Body.
type Table struct {
	Columns        []string
	Rows           [][]string
	OmittedRows    int
	OmittedColumns int
}

// Table projects saved JSON or an existing AI-formatted Markdown table.
func (result Result) Table() (Table, bool) {
	if result.Error || result.Format == FormatRaw {
		return Table{}, false
	}
	if result.Format == FormatAI {
		return markdownTable(result.Body)
	}
	decoder := json.NewDecoder(strings.NewReader(result.Body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil || decoder.Decode(new(any)) != io.EOF {
		return Table{}, false
	}
	switch value := value.(type) {
	case []any:
		if len(value) == 0 {
			return Table{}, false
		}
		if rows, ok := recordList(value); ok {
			return recordsTable(rows, nil), true
		}
		table := Table{Columns: []string{"Value"}, OmittedRows: max(len(value)-maxTableRows, 0)}
		for _, item := range value[:min(maxTableRows, len(value))] {
			table.Rows = append(table.Rows, []string{cellText(item)})
		}
		return table, true
	case map[string]any:
		return objectTable(value)
	default:
		return Table{}, false
	}
}

func objectTable(value map[string]any) (Table, bool) {
	if len(value) == 0 {
		return Table{}, false
	}
	keys := slices.Sorted(maps.Keys(value))
	for _, key := range []string{"data", "results", "items", "records", "rows", "list", "value"} {
		if rows, ok := recordList(value[key]); ok {
			return recordsTable(rows, nil), true
		}
	}
	for _, key := range keys {
		if rows, ok := recordList(value[key]); ok {
			return recordsTable(rows, nil), true
		}
	}
	if len(keys) > 1 {
		var records []map[string]any
		for _, key := range keys {
			if record, ok := value[key].(map[string]any); ok {
				records = append(records, record)
			}
		}
		if len(records) == len(keys) {
			return recordsTable(records, keys), true
		}
	}
	table := Table{Columns: []string{"Key", "Value"}}
	appendObjectRows(&table, value, "", 0)
	return table, true
}

func recordList(value any) ([]map[string]any, bool) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		rows = append(rows, record)
	}
	return rows, true
}

func recordsTable(records []map[string]any, labels []string) Table {
	keys := make(map[string]struct{})
	recordsShown := records[:min(maxTableRows, len(records))]
	for _, record := range recordsShown {
		for key := range record {
			keys[key] = struct{}{}
		}
	}
	columns := slices.Sorted(maps.Keys(keys))
	omitted := max(len(columns)-maxTableColumns, 0)
	columns = columns[:min(len(columns), maxTableColumns)]
	if len(columns) == 0 && labels == nil {
		columns = []string{"Value"}
	}
	table := Table{Columns: columns, OmittedRows: len(records) - len(recordsShown), OmittedColumns: omitted}
	if labels != nil {
		table.Columns = append([]string{""}, columns...)
	}
	for i, record := range recordsShown {
		var row []string
		if labels != nil {
			row = append(row, labels[i])
		}
		for _, key := range columns {
			row = append(row, cellText(record[key]))
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}

func appendObjectRows(table *Table, object map[string]any, prefix string, depth int) {
	for _, key := range slices.Sorted(maps.Keys(object)) {
		value := object[key]
		if child, ok := value.(map[string]any); ok && len(child) != 0 && depth < 4 {
			appendObjectRows(table, child, prefix+key+".", depth+1)
		} else if len(table.Rows) == maxTableRows {
			table.OmittedRows++
		} else {
			table.Rows = append(table.Rows, []string{prefix + key, cellText(value)})
		}
	}
}

func cellText(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case json.Number:
		return value.String()
	default:
		data, _ := json.Marshal(value)
		return string(data)
	}
}

func markdownTable(source string) (Table, bool) {
	document := parser.New(parser.ExtTables)
	document.Set(source)
	for block := range document.Children(0) {
		if document.Block(block).Kind != parser.Table {
			continue
		}
		var table Table
		for row := range document.Children(block) {
			header := document.Block(row).Header
			if !header && len(table.Rows) == maxTableRows {
				table.OmittedRows++
				continue
			}
			var cells []string
			for column := range document.Children(row) {
				if len(cells) == maxTableColumns {
					if header {
						table.OmittedColumns++
					}
					continue
				}
				cells = append(cells, parser.PlainText(document.Inlines(column)))
			}
			if header {
				table.Columns = cells
			} else {
				table.Rows = append(table.Rows, cells)
			}
		}
		return table, len(table.Columns) != 0
	}
	return Table{}, false
}
