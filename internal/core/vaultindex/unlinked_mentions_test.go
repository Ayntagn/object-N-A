// spec: EV-UI-169 / EV-UI-171
package vaultindex

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestUnlinkedMentionsMatchAndPatchRawFile(t *testing.T) {
	vault, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	const target = "Project Atlas.md"
	const source = "Source.md"
	const before = "---\r\ntitle: Project Atlas\r\n---\r\n" +
		"👋 A start before Project Atlas, then project atlas.\r\n" +
		"BEACON STUDY and [[Project Atlas]] and `Project Atlas`.\n"
	const after = "---\r\ntitle: Project Atlas\r\n---\r\n" +
		"👋 A start before [[Project Atlas]], then [[Project Atlas|project atlas]].\r\n" +
		"[[Project Atlas|BEACON STUDY]] and [[Project Atlas]] and `Project Atlas`.\n"
	for path, text := range map[string]string{
		target:                    "---\naliases: [Beacon Study]\n---\nProject Atlas\n",
		source:                    before,
		"Archive.markdown":        "Project Atlas\n",
		workspace.VaultHealthFile: "Project Atlas\n",
		workspace.VaultStatsFile:  "Project Atlas\n",
	} {
		if _, err := vault.CreateDocument(path, text); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ScanUnlinkedMentions(context.Background(), vault, target)
	if err != nil || result.Total != 3 || len(result.Hits) != 3 || result.Truncated {
		t.Fatalf("scan = %+v, %v", result, err)
	}
	first := result.Hits[0]
	if first.Path != source || first.SourceName != "Source" || first.ExpectedText != "Project Atlas" ||
		!strings.HasPrefix(first.Snippet, "…") || !strings.HasSuffix(first.Snippet, "…") ||
		!strings.Contains(first.Snippet, "👋 A start before Project Atlas") {
		t.Fatalf("first hit = %+v", first)
	}
	if got := before[first.Start:first.End]; got != first.ExpectedText {
		t.Fatalf("raw byte span = %q", got)
	}
	doc, err := vault.ReadDocument(source)
	if err != nil {
		t.Fatal(err)
	}
	patched, err := ApplyUnlinkedMentions(doc, result.Hits)
	if err != nil || patched != after {
		t.Fatalf("raw patch = %q, %v", patched, err)
	}
	buffer := strings.ReplaceAll(before, "\r\n", "\n")
	edits, err := UnlinkedMentionBufferEdits(doc, buffer, result.Hits)
	if err != nil || len(edits) != 3 || edits[0].Start != first.Start-3 || edits[2].Start != result.Hits[2].Start-4 {
		t.Fatalf("buffer edits = %+v, %v", edits, err)
	}
	for i := len(edits) - 1; i >= 0; i-- {
		edit := edits[i]
		buffer = buffer[:edit.Start] + edit.Text + buffer[edit.End:]
	}
	if want := strings.ReplaceAll(after, "\r\n", "\n"); buffer != want {
		t.Fatalf("buffer patch = %q, want %q", buffer, want)
	}
	if _, err := UnlinkedMentionBufferEdits(doc, buffer, result.Hits); !errors.Is(err, ErrStaleUnlinkedMentions) {
		t.Fatalf("changed draft error = %v", err)
	}
	changed, err := vault.SaveDocument(doc, before+"Unrelated disk edit\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyUnlinkedMentions(changed, result.Hits); !errors.Is(err, ErrStaleUnlinkedMentions) {
		t.Fatalf("changed source revision error = %v", err)
	}
}

func TestUnlinkedMentionsCapsVisibleHits(t *testing.T) {
	vault, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer vault.Close()
	if _, err := vault.CreateDocument("Beacon.md", "# Beacon\n"); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 41; index++ {
		name := fmt.Sprintf("Source%02d.md", index)
		if _, err := vault.CreateDocument(name, strings.Repeat("Beacon.\n", 6)); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ScanUnlinkedMentions(context.Background(), vault, "Beacon.md")
	if err != nil || result.Total != 200 || len(result.Hits) != 200 || !result.Truncated ||
		result.Hits[4].Path != "Source00.md" || result.Hits[5].Path != "Source01.md" || result.Hits[199].Path != "Source39.md" {
		t.Fatalf("scan total = %d, truncated = %v, err = %v", result.Total, result.Truncated, err)
	}
}
