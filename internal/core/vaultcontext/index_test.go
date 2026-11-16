package vaultcontext

// spec: EV-UI-289 / EV-UI-290 / EV-MYGO-195

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFastSearchUsesCorpusAndRestoresRank(t *testing.T) {
	ctx := context.Background()
	const prefix = "---\nprivate: headerterm\n---\n"
	const fence = "````go\ncodeword\n```\ncodeword\n````\n"
	docs := []Document{{Path: "Accent.md", Text: prefix + fence + strings.Repeat("shared QUARTZ ", 96) + "cutoffterm"}}
	for n := 1; n <= 6; n++ {
		docs = append(docs, Document{Path: fmt.Sprintf("Related/%d.md", n), Text: strings.Repeat("shared quartz ", 10)})
	}
	docs = append(docs, Document{Path: "Other.md", Text: strings.Repeat("shared orchid ", 10)})
	index, err := Build(ctx, docs, Quick)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := index.Search(ctx, "quartz shared")
	if err != nil || len(matches) != 5 {
		t.Fatalf("matches = %v, %v", matches, err)
	}
	first := matches[0]
	if first.Path != "Accent.md" || first.Score != 1 || first.Offset != len(prefix) || first.Length != len(first.Text) || len(strings.Fields(first.Text)) != 184 || matches[4].Path != "Related/4.md" {
		t.Fatalf("unexpected leading match or order: %v, last %s", first, matches[4].Path)
	}
	if miss, err := index.Search(ctx, "shared cutoffterm headerterm codeword"); len(miss) != 0 || err != nil {
		t.Fatalf("excluded words got %v, %v", miss, err)
	}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	var restored Index
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	reloaded, err := restored.Search(ctx, "quartz shared")
	if err != nil || !reflect.DeepEqual(reloaded, matches) || restored.Engine() != Fast || restored.ChunkCount() != 8 {
		t.Fatalf("restored results differ, %v", err)
	}
	if err := json.Unmarshal([]byte(`{"version":2}`), &restored); err == nil || restored.ChunkCount() != 8 {
		t.Fatal("a failed decode replaced the ready index")
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := index.Search(stopped, "quartz"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search returned %v", err)
	}
}

func TestPartitionDepthKeepsHeadingAndParagraphRanges(t *testing.T) {
	ctx := context.Background()
	const prefix = "+++\ntitle = \"fixture\"\n+++\n"
	first := "# Original\n" + strings.Repeat("合成段落 ", 88)
	second := strings.Repeat("paragraph ", 88)
	last := "### Next\n" + strings.Repeat("另外一组 ", 15)
	source := prefix + first + "\n\n" + second + "\n" + last
	balanced, err := splitDocument(ctx, source, Balanced)
	if err != nil || len(balanced) != 2 || balanced[0].text != first+"\n\n"+second || balanced[1].offset != len(source)-len(last) {
		t.Fatalf("balanced partition lengths = %d, error %v", len(balanced), err)
	}
	deep, err := splitDocument(ctx, source, Deep)
	if err != nil || len(deep) != 3 || deep[0].text != first || deep[1].text != second || deep[1].offset != len(prefix)+len(first)+2 || deep[2].offset != len(source)-len(last) {
		t.Fatalf("deep partition lengths = %d, error %v", len(deep), err)
	}
}
