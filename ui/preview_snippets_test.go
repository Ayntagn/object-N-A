// spec: FEAT-001 / EV-UI-235 / EV-UI-236
package ui

import (
	"strings"
	"testing"
	"time"

	"cyberwriter/internal/core/snippets"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestPreviewSnippetReferences(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	title := "Swift 示例"
	item := snippets.Capture(`let value = "first <value>"`, "", title, time.Now())
	item.Language = "swift"
	folder := snippets.Capture("", "", "Examples", time.Now())
	folder.IsFolder, folder.Children = true, []snippets.Snippet{item}
	file, err := snippetFile(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snippets.Append(file, folder); err != nil {
		t.Fatal(err)
	}
	const source = "Inline [[snippet:Swift 示例]] end.\n\n![[snippet:Swift 示例]]\n\n" +
		"Missing [[snippet:Missing]].\n\n![[snippet:Missing]]\n\n`[[snippet:Swift 示例]]`\n"
	app, _, _ := snippetsWorkspace(t, "Host.md", source, splitMode)
	render := func() string {
		text := app.workingText()
		markup := previewMarkup{doc: parser.New(webPreviewExtensions), snippets: loadPreviewSnippets(text, true)}
		body, _ := markup.render(text, "Host", true)
		return body
	}
	body := render()
	for _, want := range []string{
		`data-snippet-card="1">✂️ Swift 示例</a>`, `class="callout-title">✂️ Swift 示例</div>`,
		`<template data-snippet-template="1">`, `data-snippet-card="0">✂️ Missing</a>`,
		`📎 snippet:Missing (not found)`, `first &lt;value&gt;`, `<code>[[snippet:Swift 示例]]</code>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preview omitted %q from %s", want, body)
		}
	}
	if strings.Count(body, `class="wikilink snippet-link"`) != 2 || strings.Contains(body, "<value>") {
		t.Fatalf("preview changed literal or emitted active snippet markup: %s", body)
	}
	name, ok := previewSnippetNavigation(previewSnippetURL(title))
	if !ok || name != title {
		t.Fatalf("preview link title = %q, %v", name, ok)
	}
	app.openPreviewSnippet(name)
	if !app.snippets.open || app.snippets.pinned != item.ID || !app.snippets.pinnedFromPreview {
		t.Fatal("preview link did not select the saved snippet in the same window")
	}
	app.editSnippetLibraryItem(item.ID, func(details *snippets.Details) {
		details.Content = `let value = "updated"`
	})
	if !app.previewPane.refreshPending {
		t.Fatal("saving the snippet left its rendered references stale")
	}
	body = render()
	if strings.Count(body, "updated") != 2 || strings.Contains(body, "first &lt;value&gt;") {
		t.Fatalf("latest snippet contents missing from embed or hover: %s", body)
	}
	if got := app.draft.String(); got != source || app.draft.Modified() {
		t.Fatalf("snippet preview edited the source to %q", got)
	}
}
