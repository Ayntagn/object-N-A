// spec: EV-UI-232 / EV-UI-233
package ui

import (
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

func TestWorkspaceSplitDividerDrag(t *testing.T) {
	app, tester, path := snippetsWorkspace(t, "Split.md", "# Heading\n\nDocument body.\n", splitMode)
	vault, err := workspace.Open(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	app.vault, app.vaultVisible = vault, true
	app.code.Select(2, 9)
	selection := app.code.Selection()
	tester.SetSize(1294, 1080)
	tester.SetScale(2)
	tester.Frame()
	assertPanes := func(sourceWidth, previewWidth, gap float32) {
		t.Helper()
		source, sourceOK := tester.AccessByRole(mygoui.RoleTextField, "Source editor")
		preview, previewOK := tester.AccessByRole(mygoui.RoleScroll, "Markdown Preview")
		if !sourceOK || !previewOK || !source.Focused || source.Bounds.X != 360 ||
			source.Bounds.W != sourceWidth || preview.Bounds.W != previewWidth ||
			preview.Bounds.X-source.Bounds.X-source.Bounds.W != gap {
			t.Fatalf("panes = source %+v, preview %+v; expected widths %g / %g, gap %g and source focus", source, preview, sourceWidth, previewWidth, gap)
		}
	}
	assertPanes(453.5, 453, 16.5)
	divider, ok := tester.AccessByRole(mygoui.RoleSplitter, "Resize source and preview")
	if !ok || divider.Bounds.W != 9 {
		t.Fatalf("expected nine-point splitter, got %+v", divider)
	}
	x, y := divider.Bounds.X+divider.Bounds.W/2, divider.Bounds.Y+divider.Bounds.H/2
	tester.Move(x, y)
	if got := tester.Cursor(); got != mygoui.CursorResizeEW {
		t.Fatalf("splitter cursor = %v", got)
	}
	tester.Press(x, y)
	tester.Move(x+0.5, y)
	assertPanes(453.5, 453, 16.5)
	tester.Move(x+40, y)
	assertPanes(481.5, 425, 16.5)
	tester.Move(x+240, y)
	assertPanes(487, 420, 17)
	tester.Move(x-260, y)
	assertPanes(400, 507, 17)
	tester.Move(x+40, y)
	tester.Release(x+40, y)
	assertPanes(481.5, 425, 16.5)
	if app.split.drag.held || app.split.drag.moved || app.code.Selection() != selection || app.draft.Modified() {
		t.Fatal("divider release changed the editor selection, document or drag state")
	}
	app.setMode(sourceMode)
	tester.Frame()
	if _, ok := tester.AccessByRole(mygoui.RoleSplitter, "Resize source and preview"); ok {
		t.Fatal("splitter remained in the source-only layout")
	}
	app.setMode(splitMode)
	tester.Frame()
	assertPanes(481.5, 425, 16.5)
	tester.SetSize(1210, 1080)
	assertPanes(403, 420, 17)
	tester.SetSize(1294, 1080)
	assertPanes(481.5, 425, 16.5)
}
