// spec: FEAT-001 / EV-UI-239 / EV-UI-240
package snippets

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnippetExportFormats(t *testing.T) {
	const id = "a0000000-0000-4000-8000-000000000002"
	const collection = `[{"id":"a0000000-0000-4000-8000-000000000001","title":"Folder","tags":[],"isFolder":true,"children":[{"id":"` + id + `","title":"Path: α/β?","content":"x  ≥  y\n\tindented\n","createdAt":0,"updatedAt":3600,"sourceURL":"https://example.invalid/fixture","language":"","notes":"First\nSecond","tags":["样本"]}]}]`
	file := filepath.Join(t.TempDir(), "snippets.json")
	if err := os.WriteFile(file, []byte(collection), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 16, 10, 0, 0, time.FixedZone("UTC+08", 8*60*60))
	markdown, err := exportCollectionAt(file, ExportMarkdown, now)
	if err != nil {
		t.Fatal(err)
	}
	wantMarkdown := "# Snippets Export\n\nExported: Nov 15, 2026 at 16:10\nTotal snippets: 1\n\n---\n\n" +
		"## 📁 Folder\n\n### Path: α/β?\n\n**Created:** Jan 1, 2001 at 08:00\n**Updated:** Jan 1, 2001 at 09:00\n" +
		"**Source:** example.invalid\n**Language:** \n**Tags:** 样本\n\n\x60\x60\x60\nx  ≥  y\n\tindented\n\n\x60\x60\x60\n\n> **Notes:** First\nSecond\n\n---\n"
	if string(markdown) != wantMarkdown {
		t.Fatalf("Markdown export = %q", markdown)
	}
	plain, err := exportCollectionAt(file, ExportText, now)
	if err != nil {
		t.Fatal(err)
	}
	wantText := "SNIPPETS EXPORT\n===============\n\nExported: Nov 15, 2026 at 16:10\nTotal snippets: 1\n\n" + strings.Repeat("=", 60) +
		"\n\n[Folder]\n\n  Path: α/β?\n  ----------\n\n  Created: Jan 1, 2001 at 08:00\n  Source: example.invalid\n" +
		"  Language: \n  Tags: 样本\n\n  Content:\n      x  ≥  y\n      \tindented\n      \n\n  Notes: First\nSecond\n\n  " + strings.Repeat("-", 40) + "\n"
	if string(plain) != wantText {
		t.Fatalf("text export = %q", plain)
	}
	encoded, err := ExportCollection(file, ExportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(encoded) || bytes.HasSuffix(encoded, []byte("\n")) {
		t.Fatalf("invalid JSON layout: %q", encoded)
	}
	for _, want := range []string{`"createdAt" : "2001-01-01T00:00:00Z"`, `"updatedAt" : "2001-01-01T01:00:00Z"`,
		`"id" : "A0000000-0000-4000-8000-000000000002"`, `"isExpanded" : true`, `"language" : ""`, `https:\/\/example.invalid\/fixture`, "\"tags\" : [\n\n"} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Errorf("JSON omitted %q", want)
		}
	}
	single, err := VaultExportNotes(file, id)
	if err != nil {
		t.Fatal(err)
	}
	wantSingle := "---\ntitle: \"Path: α/β?\"\ncreated: 2001-01-01T00:00:00Z\nlanguage: \ntags: [样本]\nsource: example.invalid\n---\n\n" +
		"> First\nSecond\n\n\x60\x60\x60\nx  ≥  y\n\tindented\n\n\x60\x60\x60\n"
	if len(single) != 1 || single[0] != (VaultExportNote{Path: "Snippets/Path- α-β-.md", Content: wantSingle}) {
		t.Fatalf("single Vault export = %+v", single)
	}
	all, err := vaultExportNotesAt(file, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0] != (VaultExportNote{Path: "Snippets/Snippets Export.md", Content: wantMarkdown}) {
		t.Fatalf("all Vault export = %+v", all)
	}
	saved, err := os.ReadFile(file)
	if err != nil || string(saved) != collection {
		t.Fatalf("exports changed the library, read error %v", err)
	}
}
