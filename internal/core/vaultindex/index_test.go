// spec: FEAT-001 / EV-UI-009 / EV-UI-025 / EV-UI-030 / EV-EDITOR-004
package vaultindex

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestResolveSameNameNotes(t *testing.T) {
	index := buildFixture(t, map[string]string{
		"A/Source.md": "",
		"A/Topic.md":  "",
		"B/Topic.md":  "",
		"Origin.md":   "",
		"Only.md":     "",
	})
	cases := []struct {
		source, target, want string
	}{
		{"A/Source.md", "Topic", "A/Topic.md"},
		{"Origin.md", "Topic", ""},
		{"Origin.md", "Topic.md", ""},
		{"A/Source.md", "B/Topic.md#Section|Read this", "B/Topic.md"},
		{"A/Source.md", "../B/Topic", "B/Topic.md"},
		{"A/Source.md", "Only", "Only.md"},
	}
	for _, tc := range cases {
		got, ok := index.Resolve(tc.source, tc.target)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("Resolve(%q, %q) = (%q, %t), want %q", tc.source, tc.target, got, ok, tc.want)
		}
	}
	if got, ok := index.ResolveMarkdown("A/Source.md", "Topic.md#section"); !ok || got != "A/Topic.md" {
		t.Errorf("relative Markdown link resolved to (%q, %t)", got, ok)
	}
	if got, ok := index.ResolveMarkdown("A/Source.md", "Only.md"); ok {
		t.Errorf("relative Markdown link incorrectly used global name lookup and resolved to %q", got)
	}
}

func TestSearchAndRelationshipsFromMarkdown(t *testing.T) {
	index := buildFixture(t, map[string]string{
		"A/Origin.md": "\ufeff---\r\ntags: [Review, \"#Team/Blue\"] # comment\r\n---\r\n" +
			"Project manuscript with [[Topic.md#Section|**Read this #alias**]].\n\n" +
			"[Next chapter](../Notes/Chapter%201.md#part) and [reference][chapter].\n\n" +
			"#inline **#Tagged** _#Italic_ \\#escaped \\[[False]] [outside](https://example.com/#fragment).\n\n" +
			"`[[False]] #code`\n\n> ~~~\n> [[False]] #fenced\n> ~~~\n\n    [[False]] #indented\n\n" +
			"[chapter]: ../Notes/Chapter%201.md\n",
		"A/Topic.md":         "---\ntags:\n  - Workflow\n  - '#Team/Blue' # comment\n---\n[[Origin]]\n",
		"B/Topic.md":         "An unrelated note.\n",
		"Broken.md":          "\xff",
		"False.md":           "",
		"Notes/Chapter 1.md": "Chapter body.\n",
	})

	for query, want := range map[string][]string{
		"project A/Origin #review": {"A/Origin.md"},
		"#TEAM/blue":               {"A/Origin.md", "A/Topic.md"},
		"chapter manuscript":       {"A/Origin.md"},
		"broken":                   {"Broken.md"},
		"#workflow":                {"A/Topic.md"},
		"#escaped":                 nil,
		"#code":                    nil,
		"#indented":                nil,
		"#fenced":                  nil,
		"#fragment":                nil,
	} {
		if got := entryPaths(index.Search(query)); !slices.Equal(got, want) {
			t.Errorf("Search(%q) = %v, want %v", query, got, want)
		}
	}
	if got, want := index.Tags("A/Origin.md"), []string{"inline", "italic", "review", "tagged", "team/blue"}; !slices.Equal(got, want) {
		t.Errorf("Tags(Origin) = %v, want %v", got, want)
	}
	if got, want := entryPaths(index.Neighbors("A/Origin.md")), []string{"A/Topic.md", "Notes/Chapter 1.md"}; !slices.Equal(got, want) {
		t.Errorf("Neighbors(Origin) = %v, want %v", got, want)
	}
	if got, want := entryPaths(index.Backlinks("Notes/Chapter 1.md")), []string{"A/Origin.md"}; !slices.Equal(got, want) {
		t.Errorf("Backlinks(Chapter) = %v, want %v", got, want)
	}
	if got, want := index.Edges(), []Edge{
		{Source: "A/Origin.md", Target: "A/Topic.md"},
		{Source: "A/Origin.md", Target: "Notes/Chapter 1.md"},
		{Source: "A/Topic.md", Target: "A/Origin.md"},
	}; !slices.Equal(got, want) {
		t.Errorf("Edges() = %v, want %v", got, want)
	}
}

func TestWikilinkTokensKeepSourceByteRanges(t *testing.T) {
	source := `前缀 \[[仅文本]] 和 ![[笔记.md#一节|标签]]，[[Another]].`
	links := Wikilinks(source)
	if len(links) != 2 {
		t.Fatalf("Wikilinks() = %v, want two tokens", links)
	}
	first, second := links[0], links[1]
	if got := source[first.Start:first.End]; got != "![[笔记.md#一节|标签]]" || first.Target != "笔记.md#一节" || first.Label != "标签" || !first.Embed {
		t.Errorf("first token = %+v, source %q", first, got)
	}
	if got := source[second.Start:second.End]; got != "[[Another]]" || second.Target != "Another" || second.Label != "Another" || second.Embed {
		t.Errorf("second token = %+v, source %q", second, got)
	}
}

func buildFixture(t *testing.T, notes map[string]string) *Index {
	t.Helper()
	root := t.TempDir()
	for relative, text := range notes {
		filename := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	index, err := Build(vault, nil)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func entryPaths(entries []workspace.Entry) []string {
	var paths []string
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}
