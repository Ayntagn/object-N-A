// spec: EV-UI-043. The PDF tool row retains direct parameters at the reference pane width.
package ui

import (
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestPDFToolbarParametersAndActionsFitThePreview(t *testing.T) {
	app, _, _ := snippetsWorkspace(t, "Preview.md", "# PDF preview\n", previewMode)
	app.pdfPreview.enabled = true
	app.pdfPreview.form = newExportPageForm(app.preferences.Export)
	const width = 572
	tester := mygoui.NewTester(app.previewToolbar, width, 60)
	var right float32
	for _, name := range []string{"Letter", "Portrait Rectangle", `¾"`, "Decrease Text Size", "Increase Text Size", "Font", "Zoom Out", "Share", "Grid View", "PDF Preview"} {
		control, ok := tester.Access(name)
		if !ok {
			t.Fatalf("%q is missing from the PDF toolbar", name)
		}
		if control.Bounds.X < right || control.Bounds.X+control.Bounds.W > width {
			t.Fatalf("%q is out of order or clipped: %+v", name, control.Bounds)
		}
		right = control.Bounds.X + control.Bounds.W
	}
	if err := tester.Click("Decrease Text Size"); err != nil {
		t.Fatal(err)
	}
	if got := app.pdfPreview.form.layout.FontScale; got != 80 {
		t.Fatalf("PDF font size is %.0f%% after one decrement", got)
	}
}
