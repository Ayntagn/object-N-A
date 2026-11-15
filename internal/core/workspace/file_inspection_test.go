// spec: EV-UI-175 / EV-UI-176
package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cyberwriter/internal/core/workspace"
)

func TestInspectPlainTextMatchesNativeDraft(t *testing.T) {
	dir := t.TempDir()
	vault := openFixture(t, dir)
	const note = "# Format Test\n\nHello 世界. **bold**\n[local](Harbor.md)\n[[Harbor]]\n"
	for _, example := range []struct{ name, original, title, text string }{
		{"Nested/Open Format.md", note, "Format Test", "Open Format.md · plain text - 68 bytes\n\n" + note},
		{"Raw Sample.txt", "ASCII\tα\r\nEnd\r\n", "Raw Sample.txt · plain text - 15 bytes", "Raw Sample.txt · plain text - 15 bytes\n\nASCII\tα\r\nEnd\r\n"},
		{"Invalid UTF8.txt", "AB\x00\xffC\x1b\x80\r\n", "Invalid UTF8.txt · plain text - 9 bytes", "Invalid UTF8.txt · plain text - 9 bytes\n\nAB\x00ÿC\x1b\u0080\r\n"},
		{"UTF16 BOM.txt", "\xff\xfeF\x00i\x00r\x00s\x00t\x00 \x00\xb1\x03\r\x00\n\x00E\x00n\x00d\x00\n\x00", "UTF16 BOM.txt · plain text - 28 bytes", "UTF16 BOM.txt · plain text - 28 bytes\n\nÿþF\x00i\x00r\x00s\x00t\x00 \x00±\x03\r\x00\n\x00E\x00n\x00d\x00\n\x00"},
		{"Empty Sample.txt", "", "Empty Sample.txt · plain text - Zero KB", "Empty Sample.txt · plain text - Zero KB\n\n"},
		{"Mixed Newlines.txt", "line1\rline2\r\nline3\n", "Mixed Newlines.txt · plain text - 19 bytes", "Mixed Newlines.txt · plain text - 19 bytes\n\nline1\rline2\r\nline3\n"},
		{"UTF8 BOM.txt", "\ufeff# BOM title\n", "BOM title", "UTF8 BOM.txt · plain text - 15 bytes\n\n# BOM title\n"},
	} {
		t.Run(example.name, func(t *testing.T) {
			writeFixture(t, dir, example.name, example.original)
			got, err := vault.InspectFile(example.name, workspace.InspectPlainText)
			if err != nil || got.Title != example.title || got.Text != example.text {
				t.Fatalf("inspection = %q, %q, %v; want %q, %q", got.Title, got.Text, err, example.title, example.text)
			}
			if onDisk, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(example.name))); err != nil || string(onDisk) != example.original {
				t.Fatalf("original after inspection = %q, %v", onDisk, err)
			}
		})
	}
}

func TestInspectHexDumpMatchesNativeRowsAndLimit(t *testing.T) {
	dir := t.TempDir()
	vault := openFixture(t, dir)
	const name = "Binary Probe.txt"
	const original = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !AZ~\x7f\x80\xff\xe4\xb8\xad"
	const title = "Binary Probe.txt · hex dump - 43 bytes"
	const want = title + "\n\n" +
		"00000000  00 01 02 03 04 05 06 07  08 09 0A 0B 0C 0D 0E 0F  |................|\n" +
		"00000010  10 11 12 13 14 15 16 17  18 19 1A 1B 1C 1D 1E 1F  |................|\n" +
		"00000020  20 21 41 5A 7E 7F 80 FF  E4 B8 AD                 | !AZ~......|\n"
	writeFixture(t, dir, name, original)
	got, err := vault.InspectFile(name, workspace.InspectHexDump)
	if err != nil || got.Title != title || got.Text != want {
		t.Fatalf("inspection = %q, %q, %v; want %q", got.Title, got.Text, err, want)
	}

	const limitName = "Truncation Probe.txt"
	const limitTitle = "Truncation Probe.txt · hex dump - first 2.1 MB"
	writeFixture(t, dir, limitName, strings.Repeat("A", 2<<20)+"Z")
	got, err = vault.InspectFile(limitName, workspace.InspectHexDump)
	if err != nil || got.Title != limitTitle || !strings.HasPrefix(got.Text, limitTitle+" (truncated)\n\n") ||
		!strings.HasSuffix(got.Text, "001FFFF0  41 41 41 41 41 41 41 41  41 41 41 41 41 41 41 41  |AAAAAAAAAAAAAAAA|\n") {
		t.Fatalf("unexpected truncated dump for %q with %d text bytes: %v", got.Title, len(got.Text), err)
	}
}

func TestFileInspectionTitleMatchesNativeRules(t *testing.T) {
	const combining = "e\u0301"
	for _, example := range []struct{ source, title string }{
		{"first line\n\n---\n```\n###### .[Résumé]__ /  title?.\n", "Résumé title"},
		{"header\n\n# Heading\r\nsecond\r\n", "header"},
		{"# " + strings.Repeat(combining, 51), strings.Repeat(combining, 50)},
		{"# Start " + strings.Repeat("a", 46), "Start"},
		{strings.Repeat("\n", 50) + "# Beyond the scan", "Untitled"},
		{strings.Repeat("😀", 2500) + "\n# Beyond the scan", strings.Repeat("😀", 50)},
	} {
		if title := workspace.FileInspectionTitle(example.source); title != example.title {
			t.Errorf("title = %q; want %q", title, example.title)
		}
	}
}
