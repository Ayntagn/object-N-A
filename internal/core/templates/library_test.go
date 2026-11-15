package templates

// spec: EV-UI-116

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveDraftCopiesMarkdownAndKeepsSameNameTemplates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Templates")
	const first = "---\r\ntitle: Other title\r\n---\r\n\r\n# 模板 / 草稿:😀\r\n\r\n[[Note]]\r\n"
	const second = first + "\nUnsaved paragraph."
	for _, text := range []string{first, second} {
		title, err := Save(dir, text)
		if err != nil || title != "模板 - 草稿-😀" {
			t.Fatalf("save = %q, %v", title, err)
		}
	}
	entries, err := List(dir)
	want := []Entry{
		{Filename: "模板 - 草稿-😀.md", Title: "模板 / 草稿:😀"},
		{Filename: "模板 - 草稿-😀 2.md", Title: "模板 / 草稿:😀"},
	}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatalf("library = %#v, %v", entries, err)
	}
	for index, text := range []string{first, second} {
		saved, err := Load(dir, entries[index].Filename)
		if err != nil || saved != text {
			t.Fatalf("copied file %d = %q, %v", index, saved, err)
		}
	}
}

func TestTemplateFilenameUsesFirstHeadingOrDefault(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		source, title string
	}{
		{"---\ntitle: Metadata\n---\n\n## Section\n", "Untitled Template"},
		{"```\n# Code Heading\n```\n\n# Body Heading\n", "Code Heading"},
		{"\n\t ", ""},
	} {
		title, err := Save(dir, test.source)
		if err != nil || title != test.title {
			t.Fatalf("save %q = %q, %v", test.source, title, err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("directory entries = %v, %v", files, err)
	}
}
