// spec: FEAT-001 / EV-UI-184 / EV-UI-185
package vaultindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestSidebarSearchMatchesSavedSourceAndPreservesResultOrder(t *testing.T) {
	root := t.TempDir()
	fixture := map[string]string{
		"Content/CwNeedle File And Body.md": "# File and body\n\nThis note also has a body cwneedle occurrence.\n",
		"Content/cwneedle File Only.md":     "Unrelated saved body. No other sentinel here.\n",
		"Content/Alpha Body.md":             "A heading before the selected line.\n\nAlpha near cwneedle close. Second CWNEEDLE later on the same line.\n\nAnother cwneedle occurs on a different line.\n",
		"Content/Zulu Body.md":              "上半行。🍋 😀 before. This is the first full sentence before the interesting stretch. First left marker AAAAA BBBBB CCCCC DDDDD EEEEE FFFFF GGGGG HHHHH IIIII JJJJJ KKKKK 甲乙丙丁。 cwneedle after-match AAA00 BBB00 CCC00 DDD00 EEE00 FFF00 GGG00 HHH00 III00 JJJ00 KKK00 LLL00 MMM00 NNN00 OOO00 PPP00 QQQ00 RRR00 SSS00 end.\n\nThird line marker.\n",
		"Content/Plain Corpus.txt":          "first line\nTXT cwneedle sample. Second target CWPLAINTEXT remains in this row.\nlast line\n",
		"Content/Small Data.csv":            "Name,Value\ncwneedle,3\nCWCSVCONTENT,7\n",
		"Content/Terms Phrase.md":           "The phrase cwfirst cwsecond is adjacent.\n",
		"Content/Terms Split.md":            "cwfirst unrelated cwsecond\n",
		"Content/Frontmatter Tag.md":        "---\ntags: [cwtagneedle, group/shared]\nprobe: CWFRONTPROP\n---\n\nAn ordinary body.\n",
		"Content/Inline Tag.md":             "An ordinary body with #cwtagneedle and #group/shared.\n",
		"Content/Line Breaks.md":            "Prefix at line one without punctuation\n    prefix at line two.\n  CWLINEBREAK   tabbed   and    spaced.\nSuffix at the next line.\n",
		"needle-cw-pathmark/Only Parent.md": "Unrelated body.\n",
		"A rank marker.pdf":                 "",
		"B rank folder/Child.md":            "",
		"c rank marker.md":                  "",
		"Unicode/Café.txt":                  strings.Repeat("e\u0301", 31) + " ZÉRÖ " + strings.Repeat("👩‍💻", 31) + "\n",
		"Unicode/Case Fold.md":              strings.Repeat("a", 32) + " sß\n",
	}
	for name, source := range fixture {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	vault, err := workspace.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	entries, err := vault.SidebarEntries(false)
	if err != nil {
		t.Fatal(err)
	}
	index, err := Build(vault, entries)
	if err != nil {
		t.Fatal(err)
	}
	type hit struct{ path, snippet string }
	mixed := []hit{
		{"Content/CwNeedle File And Body.md", "This note also has a body cwneedle occurrence."},
		{"Content/cwneedle File Only.md", ""},
		{"Content/Alpha Body.md", "Alpha near cwneedle close. Second CWNEEDLE later…"},
		{"Content/Plain Corpus.txt", "TXT cwneedle sample. Second target CWPLAIN…"},
		{"Content/Zulu Body.md", "…HHHHH IIIII JJJJJ KKKKK 甲乙丙丁。 cwneedle after-match AAA00 BBB00 CCC00…"},
	}
	for _, test := range []struct {
		query, graphTag string
		want            []hit
	}{
		{" \tCWNEEDLE\r\n", "", mixed},
		{"cwneedle", "cwtagneedle", mixed},
		{"cwfirst cwsecond", "", []hit{{"Content/Terms Phrase.md", "The phrase cwfirst cwsecond is adjacent."}}},
		{"cwfirst  cwsecond", "", nil},
		{"#cwtagneedle", "", []hit{{"Content/Inline Tag.md", "An ordinary body with #cwtagneedle and #group/shared."}}},
		{"CWFRONTPROP", "", []hit{{"Content/Frontmatter Tag.md", "probe: CWFRONTPROP"}}},
		{"CWLINEBREAK", "", []hit{{"Content/Line Breaks.md", "CWLINEBREAK   tabbed   and    spaced."}}},
		{"cw-pathmark", "", []hit{{"needle-cw-pathmark/Only Parent.md", ""}}},
		{"w", "", []hit{{"Content/CwNeedle File And Body.md", ""}, {"Content/cwneedle File Only.md", ""},
			{"Content/Frontmatter Tag.md", ""}, {"Content/Inline Tag.md", ""}, {"needle-cw-pathmark/Only Parent.md", ""}}},
		{"rank", "", []hit{{"A rank marker.pdf", ""}, {"B rank folder/Child.md", ""}, {"c rank marker.md", ""}}},
		{"cafe\u0301", "", []hit{{"Unicode/Café.txt", ""}}},
		{"ze\u0301ro\u0308", "", []hit{{"Unicode/Café.txt", "…" + strings.Repeat("e\u0301", 29) + " ZÉRÖ " + strings.Repeat("👩‍💻", 29) + "…"}}},
		{"ss", "", []hit{{"Content/Zulu Body.md", "…NNN00 OOO00 PPP00 QQQ00 RRR00 SSS00 end."},
			{"Unicode/Case Fold.md", "…" + strings.Repeat("a", 28) + " sß"}}},
		{" ", "group/shared", []hit{{"Content/Frontmatter Tag.md", ""}}},
	} {
		t.Run(test.query, func(t *testing.T) {
			var got []hit
			for _, entry := range index.SearchSidebar(context.Background(), vault, entries, test.query, test.graphTag) {
				got = append(got, hit{entry.Entry.Path, entry.Snippet})
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("hits = %+v; want %+v", got, test.want)
			}
		})
	}
}
