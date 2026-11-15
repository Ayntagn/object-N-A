// spec: FEAT-001 / EV-UI-056
package ui

import (
	"os"
	"slices"
	"testing"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

func TestDocumentTypeMenuAndSuggestion(t *testing.T) {
	const saved = "# Preview type\n\n**Markdown sample** and `text`.\n\n<div style=\"color: #d42\">HTML element</div>\n"
	app, tester, filename := snippetsWorkspace(t, "PreviewType.md", saved, splitMode)
	original, _ := tester.Access("Markdown")
	open := func(label string) []string {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		return tester.Menu()
	}
	pick := func(label string) {
		t.Helper()
		if err := tester.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"Markdown", "HTML", "Code", "Plain Text", "-", "Auto-detect"}
	if menu := open("Markdown"); !slices.Equal(menu, want) {
		t.Fatalf("type menu = %q", menu)
	}
	app.pdfPreview.enabled = true
	app.pdfPreview.viewer.payload = []byte("cached PDF")
	pick("Code")
	if app.previewDocumentType() != documentCode || !tester.HasText(sourceFilePreviewMessage) {
		t.Fatal("Code did not show the source preview")
	}
	if app.pdfPreview.enabled || app.pdfPreview.viewer.payload != nil {
		t.Fatal("type selection retained the PDF preview or cache")
	}
	if control, ok := tester.Access("Code"); !ok || control.Bounds != original.Bounds {
		t.Fatalf("type control changed its slot: %+v", control.Bounds)
	}
	open("Code")
	pick("Plain Text")
	if app.previewDocumentType() != documentPlainText || !tester.HasText(sourceFilePreviewMessage) {
		t.Fatal("Plain Text did not show the source preview")
	}
	if menu := open("Plain Text"); !slices.Equal(menu, want) {
		t.Fatalf("suggestion was not cleared after manual selection: %q", menu)
	}
	tester.CloseMenu()
	app.focusSource = true
	tester.Frame()
	tester.Key(mygoui.Cmd, mygoui.KeyA)
	const edited = "<div>HTML edit</div>"
	tester.Type(edited)
	tester.Advance(299 * time.Millisecond)
	if menu := open("Plain Text"); slices.Contains(menu, "Switch to HTML") {
		t.Fatal("suggestion appeared before the 300 ms delay")
	}
	tester.CloseMenu()
	tester.Advance(time.Millisecond)
	if menu := open("Plain Text"); !slices.Contains(menu, "Switch to HTML") {
		t.Fatalf("HTML suggestion absent after edit: %q", menu)
	}
	pick("Switch to HTML")
	if _, found := tester.AccessByRole(mygoui.RoleButton, "Open in Browser"); !found {
		t.Fatal("HTML toolbar is missing its browser action")
	}
	if _, found := tester.AccessByRole(mygoui.RoleButton, "PDF Preview"); found {
		t.Fatal("HTML toolbar exposed the PDF button")
	}
	app.togglePDFPreview()
	if app.pdfPreview.enabled || app.problem != htmlPDFExportProblem {
		t.Fatal("HTML entered PDF preview")
	}
	tester.Frame()
	open("HTML")
	pick("Auto-detect")
	if app.previewDocumentType() != documentHTML || app.documentType.overridden {
		t.Fatal("Auto-detect did not classify the edited HTML content")
	}
	open("HTML")
	pick("Markdown")
	if _, found := tester.AccessByRole(mygoui.RoleButton, "PDF Preview"); !found || app.problem != "" {
		t.Fatal("Markdown did not restore the PDF entry")
	}
	if disk, err := os.ReadFile(filename); err != nil || string(disk) != saved || app.draft.String() != edited {
		t.Fatalf("type menu changed the draft or source file: %q, %v", disk, err)
	}
}

func TestDocumentTypeDetectionPriority(t *testing.T) {
	for _, sample := range []struct {
		filename, body string
		want           documentType
	}{
		{"", " \n", documentMarkdown},
		{"prototype.HTML", "# Heading", documentHTML},
		{"demo.py", "# Heading", documentMarkdown},
		{"draft.md", "<div>**literal**</div>", documentHTML},
		{"draft.md", "# Heading\n<div>literal</div>", documentMarkdown},
		{"log.txt", "ordinary text", documentPlainText},
		{"file.swift", "let count = 2", documentCode},
	} {
		if got := detectDocumentType(sample.filename, sample.body); got != sample.want {
			t.Errorf("%q %q type = %v, want %v", sample.filename, sample.body, got, sample.want)
		}
	}
}
