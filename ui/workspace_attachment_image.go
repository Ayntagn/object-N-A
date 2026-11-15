package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const attachmentImageLimit = 32 << 20
const imageViewerToolbarHeight = 36
const imageViewerPadding = 16

type workspaceImageWindow struct {
	win      *mygo.Window
	absolute string
	bitmap   *mygoui.Bitmap
	zoom     int // Zero fits to the available room.
	fitZoom  int
}

func (app *workspaceWindow) openVaultImage(relative string) {
	vault := app.vault
	app.openWorkspaceImage(filepath.Join(app.root, filepath.FromSlash(relative)), func() (*mygoui.Bitmap, error) {
		return readWorkspaceImage(vault, relative)
	}, vault)
}

func (app *workspaceWindow) openPickedImage(absolute string) {
	if relative, ok := app.relativeToVault(absolute); ok {
		app.openVaultImage(relative)
		return
	}
	app.openWorkspaceImage(absolute, func() (*mygoui.Bitmap, error) {
		vault, err := workspace.Open(filepath.Dir(absolute))
		if err != nil {
			return nil, err
		}
		defer vault.Close()
		return readWorkspaceImage(vault, filepath.Base(absolute))
	}, nil)
}

func readWorkspaceImage(vault *workspace.Vault, relative string) (*mygoui.Bitmap, error) {
	data, _, err := vault.ReadPreviewImage(relative, attachmentImageLimit)
	if err != nil {
		return nil, err
	}
	return mygoui.DecodeBitmap(data)
}

func (app *workspaceWindow) openWorkspaceImage(absolute string, read func() (*mygoui.Bitmap, error), vault *workspace.Vault) {
	a := app.application
	if a == nil || app.win == nil || app.win.IsDestroyed() {
		return
	}
	if current := a.imageWindows[absolute]; current != nil {
		if win := current.win; win != nil {
			if win.IsMinimized() {
				win.Restore()
			}
			win.Show()
		}
		return
	}
	if a.imageWindows == nil {
		a.imageWindows = make(map[string]*workspaceImageWindow)
	}
	viewer := &workspaceImageWindow{absolute: absolute}
	a.imageWindows[absolute] = viewer
	go func() {
		bitmap, err := read()
		mygo.RunOnMain(func() {
			if a.imageWindows[absolute] != viewer {
				return
			}
			if app.win.IsDestroyed() || vault != nil && app.vault != vault {
				delete(a.imageWindows, absolute)
				return
			}
			if err != nil {
				delete(a.imageWindows, absolute)
				app.problem = fmt.Sprintf("Could not open %q. %v", filepath.Base(absolute), err)
				app.win.Invalidate()
				return
			}
			viewer.bitmap = bitmap
			viewer.open(app)
		})
	}()
}

func (a *workspaceApplication) relocateImageWindow(from, to string) {
	viewer := a.imageWindows[from]
	if viewer == nil || from == to {
		return
	}
	delete(a.imageWindows, from)
	if viewer.win == nil {
		return
	}
	displaced := a.imageWindows[to]
	a.imageWindows[to] = viewer
	viewer.absolute = to
	viewer.win.SetTitle(filepath.Base(to))
	viewer.win.SetRepresentedFile(to)
	viewer.win.Invalidate()
	if displaced != nil && displaced.win != nil {
		displaced.win.Close()
	}
}

func (viewer *workspaceImageWindow) open(origin *workspaceWindow) {
	a := origin.application
	title := filepath.Base(viewer.absolute)
	options := mygo.WindowOptions{Title: title, Width: 880, Height: 680, MinWidth: 300, MinHeight: 220,
		Hidden: true, Content: mygoui.View(viewer.draw)}
	work := mygo.Screen.DisplayMatching(origin.win.Bounds()).WorkArea
	if work.Width > 0 && work.Height > 0 {
		options.Width = min(options.Width, work.Width)
		options.Height = min(options.Height, work.Height)
		options.X = work.X + (work.Width-options.Width)/2
		options.Y = work.Y + (work.Height-options.Height)/2
	}
	viewer.win = mygo.NewWindow(options)
	if viewer.win == nil {
		delete(a.imageWindows, viewer.absolute)
		origin.problem = fmt.Sprintf("Could not create a window for %q.", title)
		origin.win.Invalidate()
		return
	}
	win := viewer.win
	win.SetRepresentedFile(viewer.absolute)
	menu := viewer.menu()
	if runtime.GOOS == "darwin" {
		win.OnFocus(func() { mygo.App.SetMenu(menu) })
	} else {
		win.SetMenu(menu)
	}
	win.OnClosed(func() {
		if a.imageWindows[viewer.absolute] == viewer {
			delete(a.imageWindows, viewer.absolute)
		}
		if !mygo.App.IsActive() || mygo.FocusedWindow() != nil {
			return
		}
		target := origin
		if target.win.IsDestroyed() {
			target = a.activeWorkspace()
		}
		if target != nil {
			target.win.Focus()
			a.activate(target)
		}
	})
	win.Show()
}

func (viewer *workspaceImageWindow) displayedZoom() int {
	if viewer.zoom == 0 {
		return viewer.fitZoom
	}
	return viewer.zoom
}

func (viewer *workspaceImageWindow) setZoom(zoom int) {
	viewer.zoom = zoom
	if viewer.win != nil {
		viewer.win.Invalidate()
	}
}

func (viewer *workspaceImageWindow) stepZoom(delta int) {
	current := viewer.displayedZoom()
	if delta < 0 && current <= previewZoomMin || delta > 0 && current >= previewZoomMax {
		return
	}
	viewer.setZoom(normalizedPreviewZoom(current + delta))
}

func (viewer *workspaceImageWindow) draw(c *mygoui.Context) {
	width, height := c.Size()
	availableHeight := max(1, height-imageViewerToolbarHeight)
	w, h := mygoui.ImageSize(viewer.bitmap)
	fit := min(1, max(1, width-2*imageViewerPadding)/w, max(1, availableHeight-2*imageViewerPadding)/h)
	viewer.fitZoom = max(1, int(math.Round(float64(fit)*100)))
	scale := fit
	if viewer.zoom != 0 {
		scale = float32(viewer.zoom) / 100
	}
	renderWidth, renderHeight := w*scale, h*scale
	zoom := viewer.displayedZoom()
	theme := c.Theme()
	mygoui.Column(c).Fill().Background(theme.Background).Children(func() {
		mygoui.Row(c).Height(imageViewerToolbarHeight).Shrink(0).Gap(6).FillWidth().
			AlignItems(mygoui.Center).Justify(mygoui.Center).Background(theme.Surface).
			Role(mygoui.RoleToolbar).Label("Image controls").Children(func() {
			mygoui.Button(c, "−").Label("Zoom Out").Disabled(zoom <= previewZoomMin).
				OnClick(func() { viewer.stepZoom(-previewZoomStep) })
			mygoui.Button(c, fmt.Sprintf("%d%%", zoom)).Label("Actual Size").
				Tooltip("Actual Size").OnClick(func() { viewer.setZoom(previewZoomDefault) })
			mygoui.Button(c, "+").Label("Zoom In").Disabled(zoom >= previewZoomMax).
				OnClick(func() { viewer.stepZoom(previewZoomStep) })
			mygoui.Button(c, "Fit").Label("Fit Image to Window").AccessSelected(viewer.zoom == 0).
				OnClick(func() { viewer.setZoom(0) })
		})
		mygoui.ScrollBoth(c.Key("image viewer viewport")).Grow(1).MinWidth(0).MinHeight(0).
			Label("Image viewport").Children(func() {
			mygoui.Box(c).Size(max(width, renderWidth+2*imageViewerPadding), max(availableHeight, renderHeight+2*imageViewerPadding)).
				Shrink(0).Center().Children(func() {
				mygoui.Image(c, viewer.bitmap).Size(renderWidth, renderHeight).Shrink(0).Label(filepath.Base(viewer.absolute))
			})
		})
	})
}

func (viewer *workspaceImageWindow) menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(), {Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "Close", Accelerator: "CmdOrCtrl+W", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.win.Close() }},
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Zoom In", Accelerator: "CmdOrCtrl+Plus", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.stepZoom(previewZoomStep) }},
			{Label: "Zoom Out", Accelerator: "CmdOrCtrl+-", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.stepZoom(-previewZoomStep) }},
			{Label: "Actual Size", Accelerator: "CmdOrCtrl+0", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.setZoom(previewZoomDefault) }},
			{Label: "Fit Image to Window", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.setZoom(0) }},
		}},
		{Role: mygo.RoleWindowMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize}, {Role: mygo.RoleZoom}, {Role: mygo.RoleFront},
		}},
	})
}
