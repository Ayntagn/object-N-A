// spec: FEAT-001 / EV-UI-315 / EV-UI-331
package ui

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestStartPreviewUsesRecentDocumentsAtBothWidths(t *testing.T) {
	tab := &workspaceTab{mode: splitMode}
	app := &workspaceWindow{workspaceTab: tab, tabs: []*workspaceTab{tab}, preferences: &workspacePreferences{}}
	width := float32(453)
	view := mygoui.NewTester(func(c *mygoui.Context) {
		app.drawStartPreview(c, width)
	}, int(width), 712)
	if !view.HasText("No recent documents yet.") {
		t.Fatal("the welcome preview should display the empty recent documents card")
	}
	for _, title := range []string{"📂 Open File...", "📖 User Guide"} {
		if _, ok := view.AccessByRole(mygoui.RoleButton, title); !ok {
			t.Fatalf("welcome preview is missing button %q", title)
		}
	}
	if !view.HasText("Open AI from the toolbar") || view.HasText("Voice Notes") {
		t.Fatal("Quick Start should direct users to the available AI entry and omit unavailable voice recording")
	}

	folder := t.TempDir()
	file := filepath.Join(folder, "Research.md")
	if err := os.WriteFile(file, []byte("# Isolated research\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.preferences.RecentDocuments = []string{file}
	homeVariable := "HOME"
	if runtime.GOOS == "windows" {
		homeVariable = "USERPROFILE"
	}
	t.Setenv(homeVariable, filepath.Dir(folder))
	view.Frame()
	if !view.HasText(filepath.Join("~", filepath.Base(folder))) {
		t.Fatal("the visible recent folder should abbreviate the home directory")
	}
	for _, paneWidth := range []float32{453, 706} {
		width = paneWidth
		view.SetSize(int(width), 712)
		recent, hasRecent := view.AccessByRole(mygoui.RoleHeading, "RECENT DOCUMENTS")
		quick, hasQuick := view.AccessByRole(mygoui.RoleHeading, "QUICK START")
		if !hasRecent || !hasQuick {
			t.Fatalf("welcome cards are missing at width %.0f pt", width)
		}
		if width == 453 && (recent.Bounds.X != quick.Bounds.X || recent.Bounds.Y >= quick.Bounds.Y) {
			t.Fatalf("expected stacked cards at 453 pt, recent=%+v quick=%+v", recent.Bounds, quick.Bounds)
		}
		if width == 706 && (recent.Bounds.Y != quick.Bounds.Y || recent.Bounds.X >= quick.Bounds.X) {
			t.Fatalf("expected adjacent cards at 706 pt, recent=%+v quick=%+v", recent.Bounds, quick.Bounds)
		}
	}
	view.Scroll(40, 150, 0, 10000)
	shortcutHint := "Press " + shortcutLabel("⌘ ⇧ /") + " anytime to see all keyboard shortcuts"
	if footer, ok := view.Find(shortcutHint); !ok || footer.Y < 0 || footer.Y >= 712 {
		t.Fatalf("the welcome preview should scroll to its final shortcuts hint, got %+v", footer)
	}
	view.Scroll(40, 150, 0, -10000)
	if err := view.Click("Research, " + folder); err != nil {
		t.Fatal(err)
	}
	if app.storage != nil {
		t.Cleanup(func() { _ = app.storage.Close() })
	}
	if app.workingText() != "# Isolated research\n" {
		t.Fatalf("click should open the recent Markdown file, got %q", app.workingText())
	}
}

func TestStartPreviewZoomUsesViewportAnchor(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app, view, _ := snippetsWorkspace(t, "Untitled.md", "", splitMode)
	const windowWidth, windowHeight, viewportHeight = 1470, 923, 731
	view.SetSize(windowWidth, windowHeight)
	read := func() (mygoui.Rect, mygoui.Rect) {
		t.Helper()
		viewport, haveViewport := view.AccessByRole(mygoui.RoleScroll, "Welcome preview")
		heading, haveHeading := view.AccessByRole(mygoui.RoleHeading, "Let's Get To Work, Together")
		if !haveViewport || !haveHeading {
			t.Fatal("welcome preview viewport or title is missing")
		}
		return viewport.Bounds, heading.Bounds
	}
	viewport, _ := read()
	view.SetSize(windowWidth, windowHeight+viewportHeight-int(viewport.H))
	viewport, heading := read()
	if viewport.W != 718 || viewport.H != viewportHeight {
		t.Fatalf("welcome preview viewport = %+v, want 718 × 731", viewport)
	}
	startY, startCenter := heading.Y-viewport.Y, heading.X+heading.W/2-viewport.X
	measure := func() (float32, float32) {
		viewport, heading := read()
		zoom := float32(app.previewZoom) / 100
		return startY*zoom - (heading.Y - viewport.Y), heading.X + heading.W/2 - viewport.X
	}
	check := func(label string, got, want float32) {
		t.Helper()
		if math.Abs(float64(got-want)) > .6 {
			t.Errorf("%s = %.3f, want %.3f", label, got, want)
		}
	}
	zoomTo := func(label string, before, want float32, zoom int) {
		t.Helper()
		current, _ := measure()
		view.Scroll(viewport.X+viewport.W/2, viewport.Y+viewport.H/2, 0, before-current)
		position, _ := measure()
		check("wheel input", position, before)
		if err := view.Click(label); err != nil {
			t.Fatal(err)
		}
		if app.previewZoom != zoom {
			t.Fatalf("%s set zoom to %d%%, want %d%%", label, app.previewZoom, zoom)
		}
		offset, center := measure()
		t.Logf("%s to %d%% offsets %.3f → %.3f, title center %.3f", label, zoom, position, offset, center)
		check(label+" scroll", offset, want)
		view.Frame()
		offset, _ = measure()
		check(label+" scroll after redraw", offset, want)
	}
	zoomTo("Zoom In", 600, 293.125, 125)
	zoomTo("125%", 293.125, 0, 100)
	zoomTo("Zoom Out", 600, 175.875, 75)
	_, center := measure()
	check("75% title center from the viewport's leading edge", center, startCenter*.75)
	zoomTo("75%", 776, 547.333, 100)
}
