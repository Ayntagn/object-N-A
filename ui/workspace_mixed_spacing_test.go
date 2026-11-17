package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

// spec: EV-MYGO-270 / 中英文输入自动补一个半角空格。
func TestMixedLanguageInsertion(t *testing.T) {
	for _, tt := range []struct {
		name, source, input, want string
		trailing                  bool
	}{
		{"Latin after Han", "中文▏", "Go", " Go", false},
		{"Han after Latin", "Go▏", "中文", " 中文", false},
		{"both sides", "中▏文", "Go", " Go ", true},
		{"mixed commit", "▏", "中文Go文本", "中文 Go 文本", false},
		{"compound Latin accent", "中文▏", "Cafe\u0301文", " Cafe\u0301 文", false},
		{"existing spaces", "中 ▏ 文", "Go", "Go", false},
		{"punctuation and digits", "中，▏文", "2026", "2026", false},
		{"nested prose", "> - 中文▏", "Go", " Go", false},
		{"heading", "# 中文▏", "day", " day", false},
		{"multiline input", "中▏", "A\n中A", "A\n中A", false},
		{"fenced code after a blank line", "```go\n\n中文▏\n```", "Go", "Go", false},
		{"nested unclosed fence", "> - a\n>\n>   ~~~go\n>   中文▏", "Go", "Go", false},
		{"four-space code", "    中文▏", "Go", "Go", false},
		{"inline code", "`中文▏` 文", "Go", "Go", false},
		{"unclosed inline code", "`中文▏", "Go", "Go", false},
		{"after inline code", "`code` 中文▏", "Go", " Go", false},
		{"after previous paragraph's inline code", "`code`\n\n中文▏", "Go", " Go", false},
		{"dollar math", "文本 $中文▏$", "Go", "Go", false},
		{"open dollar math", "文本 $中文▏", "Go", "Go", false},
		{"open display math", "$$\n中文▏", "Go", "Go", false},
		{"open TeX math", "文本 \\(中文▏", "Go", "Go", false},
		{"frontmatter", "---\ntitle: 中文▏\n---", "Go", "Go", false},
		{"after frontmatter", "---\ntitle: 中文\n---\n中文▏", "Go", " Go", false},
		{"note reference", "[[中文▏|label]]", "Go", "Go", false},
		{"note alias", "[[other|中文▏]]", "Go", " Go", false},
		{"hashtag", "Tags: #笔记▏", "Go", "Go", false},
		{"URL", "obsidian://open/中文▏", "Go", "Go", false},
		{"link label", "[中文▏](note.md)", "Go", " Go", false},
		{"link destination", "[note](中文▏.md)", "Go", "Go", false},
		{"open link destination", "[note](中文▏", "Go", "Go", false},
		{"reference defined in another paragraph", "[中文Go]: note.md\n\n[here][中文▏]", "Go", "Go", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before, after, _ := strings.Cut(tt.source, "▏")
			snapshot := editor.NewBuffer(before + after).Snapshot()
			got, trailing := mixedLanguageInsertion(snapshot, len(before), len(before), tt.input)
			if got != tt.want || trailing != tt.trailing {
				t.Fatalf("got (%q, %t), want (%q, %t)", got, trailing, tt.want, tt.trailing)
			}
		})
	}
}
