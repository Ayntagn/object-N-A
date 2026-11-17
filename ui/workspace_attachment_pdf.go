package ui

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"slices"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspacePDFWindow struct {
	win           *mygo.Window
	absolute      string
	unreadable    bool
	savePrompt    bool
	zoom          func(workspacePDFZoom)
	requestedPage int
	jumpToPage    func(int)
}

type workspacePDFZoom string

const (
	workspacePDFZoomIn     workspacePDFZoom = "in"
	workspacePDFZoomOut    workspacePDFZoom = "out"
	workspacePDFActualSize workspacePDFZoom = "actual"
	workspacePDFFitWidth   workspacePDFZoom = "fit"
)

var errUnreadableWorkspacePDF = errors.New("could not read the PDF")

// EV-UI-177/178. PDFs have their own read-only document window. Opening
// the same file again raises that window and keeps its scroll position.
func (app *workspaceWindow) openVaultPDF(relative string) {
	app.openVaultPDFAt(app.vault, relative, 0)
}

// openVaultPDFAt accepts page numbers as written in links, starting from 1.
// A zero page keeps the reading position when the PDF is already open.
func (app *workspaceWindow) openVaultPDFAt(vault *workspace.Vault, relative string, page int) {
	if app.modalOpen || vault == nil || !workspace.IsPDFPath(relative) {
		return
	}
	native, err := filepath.Localize(relative)
	if err != nil {
		return
	}
	app.openPDF(filepath.Join(vault.RootPath(), native), relative, vault, page)
}

func (app *workspaceWindow) openPickedPDF(absolute string) {
	absolute = filepath.Clean(absolute)
	if relative, ok := app.relativeToVault(absolute); ok {
		app.openVaultPDF(relative)
		return
	}
	app.openPDF(absolute, filepath.Base(absolute), nil, 0)
}

func (app *workspaceWindow) openPDF(absolute, relative string, vault *workspace.Vault, page int) {
	a := app.application
	if current := a.pdfWindows[absolute]; current != nil {
		if win := current.win; win != nil {
			if win.IsMinimized() {
				win.Restore()
			}
			if win.IsVisible() {
				win.Show()
			}
		}
		current.requestPage(page)
		return
	}
	if a.pdfWindows == nil {
		a.pdfWindows = make(map[string]*workspacePDFWindow)
	}
	viewer := &workspacePDFWindow{absolute: absolute, requestedPage: page}
	a.pdfWindows[absolute] = viewer
	go func() {
		data, err := readWorkspacePDF(absolute, relative, vault)
		app.win.Update(func() {
			if a.pdfWindows[absolute] != viewer {
				return
			}
			if app.win.IsDestroyed() || vault != nil && app.vault != vault &&
				!slices.ContainsFunc(app.tabs, func(tab *workspaceTab) bool { return tab.storage == vault }) {
				delete(a.pdfWindows, absolute)
				return
			}
			if err != nil {
				delete(a.pdfWindows, absolute)
				app.problem = fmt.Sprintf("Could not open %q. %v", relative, err)
				return
			}
			viewer.open(app, data)
		})
	}()
}

func (viewer *workspacePDFWindow) requestPage(page int) {
	if page <= 0 {
		return
	}
	viewer.requestedPage = page
	if viewer.jumpToPage != nil {
		viewer.jumpToPage(page)
	}
}

func readWorkspacePDF(absolute, relative string, vault *workspace.Vault) ([]byte, error) {
	if vault == nil {
		var err error
		vault, err = workspace.Open(filepath.Dir(absolute))
		if err != nil {
			return nil, err
		}
		defer vault.Close()
	}
	return vault.ReadVaultPDF(relative)
}

// A sidebar move also changes the represented URL of an independent viewer.
func (a *workspaceApplication) relocatePDFWindow(from, to string) bool {
	viewer := a.pdfWindows[from]
	if viewer == nil || from == to {
		return false
	}
	delete(a.pdfWindows, from)
	if viewer.win == nil {
		return false // Its pending read still refers to the previous path.
	}
	displaced := a.pdfWindows[to]
	a.pdfWindows[to] = viewer
	viewer.absolute = to
	viewer.win.SetTitle(filepath.Base(to))
	viewer.win.SetRepresentedFile(to)
	setPDFDocumentURL(viewer)
	if displaced != nil && displaced != viewer && displaced.win != nil {
		displaced.win.Close()
	}
	return true
}

func (viewer *workspacePDFWindow) open(origin *workspaceWindow, data []byte) {
	a := origin.application
	title := filepath.Base(viewer.absolute)
	options := mygo.WindowOptions{Title: title, Width: 900, Height: 1100, Hidden: true}
	work := mygo.Screen.DisplayMatching(origin.win.Bounds()).WorkArea
	if work.Width > 0 && work.Height > 0 {
		options.Width = min(options.Width, work.Width)
		options.Height = min(options.Height, work.Height)
		options.X = work.X + (work.Width-options.Width)/2
		options.Y = work.Y + (work.Height-options.Height)/2
	}
	if runtime.GOOS == "darwin" {
		options.Content = mygoui.View(func(c *mygoui.Context) {
			if viewer.unreadable {
				drawPDFLoadFailure(c, filepath.Base(viewer.absolute))
			}
		})
	} else {
		options.Page = mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled}
	}
	win := mygo.NewWindow(options)
	if win == nil {
		delete(a.pdfWindows, viewer.absolute)
		origin.problem = fmt.Sprintf("Could not create a window for %q.", title)
		return
	}
	viewer.win = win
	win.SetRepresentedFile(viewer.absolute)
	bindPDFDocumentTitle(viewer, a)
	menu := viewer.menu()
	if runtime.GOOS == "darwin" {
		win.OnFocus(func() { mygo.App.SetMenu(menu) })
	} else {
		win.SetMenu(menu)
	}
	win.OnClosed(func() {
		viewer.zoom = nil
		viewer.jumpToPage = nil
		if a.pdfWindows[viewer.absolute] == viewer {
			delete(a.pdfWindows, viewer.absolute)
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
	go func() {
		err := viewer.mount(data)
		win.Update(func() {
			if win.IsDestroyed() {
				return
			}
			if errors.Is(err, errUnreadableWorkspacePDF) {
				viewer.unreadable = true
			} else if err != nil {
				win.Close()
				if !origin.win.IsDestroyed() {
					origin.win.Update(func() { origin.problem = fmt.Sprintf("Could not open %q. %v", title, err) })
				}
				return
			}
			win.Show()
		})
	}()
}

func drawPDFLoadFailure(c *mygoui.Context, title string) {
	mygoui.Column(c).Grow(1).FillWidth().FillHeight().AlignItems(mygoui.Center).
		Justify(mygoui.Center).Gap(12).Padding(40).Children(func() {
		chromeIconColor(c, "doc.text.magnifyingglass", 48, c.Theme().TextMuted)
		mygoui.Text(c, "Could not open PDF").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
		mygoui.Text(c, title).FontSize(12).TextColor(c.Theme().TextMuted).Wrap().TextAlign(mygoui.Center)
		mygoui.Text(c, "The file may be corrupt or password-protected.").
			FontSize(12).TextColor(c.Theme().TextMuted).Wrap().TextAlign(mygoui.Center)
	})
}

func (viewer *workspacePDFWindow) menu() *mygo.Menu {
	zoom := func(action workspacePDFZoom) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			if viewer.zoom != nil {
				viewer.zoom(action)
			}
		}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout}, mygo.Separator(), {Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, mygo.Separator(), {Role: mygo.RoleQuit},
		}},
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "Close", Accelerator: "CmdOrCtrl+W", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.win.Close() }},
			{Label: "Save", Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) { viewer.cannotSave() }},
		}},
		{Label: "Edit", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll},
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Zoom In", Accelerator: "CmdOrCtrl+Plus", Click: zoom(workspacePDFZoomIn)},
			{Label: "Zoom Out", Accelerator: "CmdOrCtrl+-", Click: zoom(workspacePDFZoomOut)},
			mygo.Separator(),
			{Label: "Actual Size", Accelerator: "CmdOrCtrl+0", Click: zoom(workspacePDFActualSize)},
			{Label: "Fit Width", Click: zoom(workspacePDFFitWidth)},
		}},
		{Role: mygo.RoleWindowMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleMinimize}, {Role: mygo.RoleZoom}, {Role: mygo.RoleFront},
		}},
	})
}

func (viewer *workspacePDFWindow) cannotSave() {
	if viewer.savePrompt {
		return
	}
	viewer.savePrompt = true
	title := filepath.Base(viewer.absolute)
	go func() {
		_, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent: viewer.win, Type: mygo.MessageError,
			Message: fmt.Sprintf("The document “%s” could not be saved. The requested operation is not supported.", title),
		})
		if err != nil {
			log.Printf("Show PDF save warning: %v", err)
		}
		viewer.win.Update(func() { viewer.savePrompt = false })
	}()
}
