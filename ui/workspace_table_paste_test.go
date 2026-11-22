// spec: FEAT-001 / EV-MYGO-295
package ui

import (
	"strings"
	"testing"
)

func TestClipboardMarkdownTableKeepsCells(t *testing.T) {
	const table = "| 对比 | 项目甲 | 项目乙 |\n| --- | --- | :---: |\n| 保存位置 | `struct entry` 内嵌 | 单独存放 |"
	clipboard := "\t \r\n" + strings.ReplaceAll(table, "\n", "\r\n") + "\r\n \t\r\n"
	if got, ok := pasteMarkdownTable(clipboard); !ok || got != table {
		t.Fatalf("recognized table = %q, %t", got, ok)
	}

	// GFM may render a ragged row by discarding its extra cells.
	if _, ok := pasteMarkdownTable(table + "\n| 1 | 2 | 3 | 4 |"); ok {
		t.Fatal("recognized a table whose fourth cell would be lost")
	}
}
