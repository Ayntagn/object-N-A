package ui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"cyberwriter/internal/core/quickswitcher"
	"cyberwriter/internal/core/templates"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceQuickSwitcher struct {
	open, focus bool
	query       string
	selected    int
	files       []quickswitcher.File
	results     []quickswitcher.File
	list        mygoui.ListState
}

func (app *workspaceWindow) toggleQuickSwitcher() {
	if app.modalOpen || app.presentation != nil || app.drawing != nil {
		return
	}
	if app.quickSwitcher.open {
		app.dismissQuickSwitcher()
		return
	}
	app.commandPalette = workspaceCommandPalette{}
	app.focusSource, app.wys.focus = false, false
	focusWorkspaceSurface(app.win)
	app.quickSwitcher = workspaceQuickSwitcher{open: true, focus: true}
	app.refreshQuickSwitcherFiles()
}

func (app *workspaceWindow) dismissQuickSwitcher() {
	app.quickSwitcher = workspaceQuickSwitcher{}
}

func (app *workspaceWindow) refreshQuickSwitcherFiles() {
	switcher := &app.quickSwitcher
	selectedPath := ""
	if switcher.selected < len(switcher.results) {
		selectedPath = switcher.results[switcher.selected].Path
	}
	switcher.files = quickSwitcherFiles(app.entries)
	if app.multiFile.active {
		switcher.files = slices.DeleteFunc(switcher.files, func(file quickswitcher.File) bool {
			return !workspace.IsMarkdownPath(file.Path)
		})
	}
	switcher.filter()
	if selectedPath != "" {
		if i := slices.IndexFunc(switcher.results, func(file quickswitcher.File) bool { return file.Path == selectedPath }); i >= 0 {
			switcher.selected = i
			switcher.list.ScrollIntoView(i)
		}
	}
}

func (switcher *workspaceQuickSwitcher) filter() {
	switcher.results = quickswitcher.Search(switcher.files, switcher.query)
	switcher.selected = 0
	switcher.list.ScrollTo(0, mygoui.Start)
}

func (switcher *workspaceQuickSwitcher) moveSelection(key mygoui.TextKey) bool {
	if key.Mods != 0 {
		return false
	}
	var delta int
	switch key.Key {
	case mygoui.KeyUp:
		delta = -1
	case mygoui.KeyDown:
		delta = 1
	default:
		return false
	}
	if len(switcher.results) != 0 {
		next := max(0, min(switcher.selected+delta, len(switcher.results)-1))
		if next != switcher.selected {
			switcher.selected = next
			switcher.list.ScrollTo(next, mygoui.Center)
		}
	}
	return true
}

func (app *workspaceWindow) submitQuickSwitcher() {
	switcher := &app.quickSwitcher
	if switcher.selected < len(switcher.results) {
		app.openQuickSwitcherFile(switcher.results[switcher.selected])
	}
}

func (app *workspaceWindow) openQuickSwitcherFile(file quickswitcher.File) {
	if app.modalOpen || app.vault == nil {
		return
	}
	app.dismissQuickSwitcher()
	if app.multiFile.active {
		app.openNoteAt(file.Path, true, "")
		return
	}
	if file.Drawing {
		app.openDrawing(file.Path, true)
		return
	}
	tab, vault := app.workspaceTab, app.vault
	if tab.storage == nil && tab.document.Path == file.Path {
		return
	}
	app.wys.flush(app)
	replace := func() {
		if app.workspaceTab != tab || app.vault != vault {
			return
		}
		doc, err := vault.ReadDocument(file.Path)
		if err != nil {
			app.problem = fmt.Sprintf("Could not open %q. %v", file.Path, err)
			return
		}
		// Quick Switcher replaces this tab even when another tab has the
		// same file. Each tab retains its own buffer and disk revision.
		app.releaseTabStorage(tab)
		app.setTabDocument(tab, doc)
		app.activateTab(tab)
		app.focusSource, app.wys.focus = false, false
	}
	if tab.dirty() {
		if tab.document.Path == "" {
			if tab.inspection {
				app.saveAs(tab, "", replace)
				return
			}
			title := strings.Map(func(r rune) rune {
				if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\\|?*`, r) {
					return '-'
				}
				return r
			}, templates.Heading(tab.draft.String()))
			title = strings.TrimRight(title, " .")
			if title != "" {
				tab.templateTitle = title
				app.win.SetTitle(tab.title())
				title += ".md"
			}
			app.saveAs(tab, title, replace)
			return
		}
		if !app.saveTab(tab, func() { app.offerMissingDocumentSave(tab, replace) }) {
			return
		}
	}
	replace()
}

const (
	quickSwitcherWidth  = 480
	quickSwitcherTop    = 172
	quickSwitcherHeader = 46
	quickSwitcherList   = 350
	quickSwitcherFont   = "Space Grotesk, sans-serif"
)

type quickSwitcherColors struct {
	surface, edge, separator, selected, hover, title, accent, muted, icon mygoui.Color
}

func (app *workspaceWindow) drawQuickSwitcher(c *mygoui.Context) {
	switcher := &app.quickSwitcher
	if !switcher.open {
		return
	}
	width, height := c.Size()
	panelWidth := min(quickSwitcherWidth, width-16)
	top := float32(quickSwitcherTop)
	if len(app.tabs) > 1 && !app.focusMode {
		top += tabBarHeight + tabBarGap
	}
	top = min(top, max(titleBarHeight, height-quickSwitcherHeader-quickSwitcherList-8))
	listHeight := min(quickSwitcherList, height-top-quickSwitcherHeader-8)
	if panelWidth <= 0 || listHeight <= 0 {
		return
	}
	loadBundledFont("Space Grotesk")
	theme := c.Theme()
	colors := quickSwitcherColors{
		surface: theme.Surface, edge: theme.Border, separator: theme.Border.Alpha(0.5),
		selected: theme.SurfacePressed, hover: theme.SurfaceHover,
		title: theme.Text, accent: theme.Accent, muted: theme.TextMuted, icon: theme.Accent,
	}
	if app.blackout && !c.Preferences().HighContrast {
		// EV-UI-118: Blackout samples from the quick switcher.
		colors = quickSwitcherColors{
			surface: mygoui.RGB(6, 6, 8), edge: mygoui.RGB(25, 25, 29), separator: mygoui.RGB(19, 19, 21),
			selected: mygoui.RGB(25, 29, 38), hover: mygoui.RGB(16, 17, 21),
			title: mygoui.RGB(222, 223, 229), accent: mygoui.RGB(175, 198, 250),
			muted: mygoui.RGB(130, 132, 143), icon: mygoui.RGB(204, 144, 207),
		}
	}
	mygoui.Overlay(c, func() {
		shade := mygoui.Box(c.Key("quick switcher backdrop")).Absolute().Top(0).Left(0).Size(width, height).
			Background(mygoui.RGB(0, 0, 0).Alpha(0.4)).OnClick(app.dismissQuickSwitcher).Role(mygoui.RoleNone)
		shade.Children(func() {
			panel := mygoui.Column(c.Key("quick switcher")).Absolute().Top(top).Left((width-panelWidth)/2).
				Size(panelWidth, quickSwitcherHeader+listHeight).Role(mygoui.RoleDialog).Label("Quick Switcher").
				Radius(10).Clip().Background(colors.surface).
				Shadow(0, 8, 24, 0, mygoui.RGB(0, 0, 0).Alpha(0.3))
			if panel.OverlayShortcut(0, mygoui.KeyEscape) {
				app.dismissQuickSwitcher()
				c.Invalidate()
				return
			}
			panel.DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
				p.Stroke(r, colors.edge, 10, 1)
			}).Children(func() {
				app.quickSwitcherSearch(c, colors)
				app.quickSwitcherResults(c, listHeight, colors)
			})
		})
	})
}

func (app *workspaceWindow) quickSwitcherSearch(c *mygoui.Context, colors quickSwitcherColors) {
	const placeholder = "Jump to file..."
	switcher := &app.quickSwitcher
	mygoui.Row(c).FillWidth().Height(quickSwitcherHeader).Shrink(0).
		Padding(0, 16, 0, 17.5).Gap(9).AlignItems(mygoui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(colors.separator).Children(func() {
		sfsymbols.Symbol(c, "magnifyingglass").Size(14).Color(colors.title).
			Fallback(chromeIconFallbacks["magnifyingglass"]).Role(mygoui.RoleNone)
		input := mygoui.TextInputBase(c.Key("quick switcher query"), &switcher.query).
			Grow(1).MinWidth(0).Height(21).FontSize(16).Font(quickSwitcherFont).TextColor(colors.title).
			Label(placeholder).Placeholder(placeholder).PlaceholderColor(colors.muted).FocusRing(false).
			OnTextKey(switcher.moveSelection).OnChange(switcher.filter).OnSubmit(app.submitQuickSwitcher)
		if switcher.focus {
			input.Focus()
			switcher.focus = false
		}
	})
}

func (app *workspaceWindow) quickSwitcherResults(c *mygoui.Context, height float32, colors quickSwitcherColors) {
	switcher := &app.quickSwitcher
	if len(switcher.results) == 0 {
		message := "No files found"
		if app.vault == nil {
			message = "No vault open"
		}
		mygoui.Scroll(c.Key("quick switcher empty")).FillWidth().Height(height).Label("Files").Children(func() {
			mygoui.Row(c).FillWidth().Height(60).Shrink(0).Center().Children(func() {
				mygoui.Text(c, message).FontSize(12).TextColor(colors.muted).Role(mygoui.RoleStatus)
			})
		})
		return
	}
	results := switcher.results
	switcher.list.Key = func(i int) any { return results[i].Path }
	mygoui.List(c.Key("quick switcher files"), &switcher.list, len(results), func(i int) {
		file := results[i]
		selected := i == switcher.selected
		label := file.Name
		rowHeight := float32(29)
		if file.Folder != "" {
			label += ", " + file.Folder
			rowHeight = 43
		}
		row := mygoui.ButtonBase(c).FillWidth().Height(rowHeight).Shrink(0).
			Padding(0, 16, 0, 18).Gap(9).Radius(0).AlignItems(mygoui.Center).Justify(mygoui.Start).
			Role(mygoui.RoleOption).Label(label).AccessSelected(selected).KeepFocus().
			OnClick(func() { app.openQuickSwitcherFile(file) })
		ink, icon := colors.title, colors.icon
		weight := 400
		if selected {
			row.Background(colors.selected)
			ink, icon = colors.accent, colors.accent
			weight = 600
		} else if row.Hovered() {
			row.Background(colors.hover)
		}
		row.Children(func() {
			sfsymbols.Symbol(c, "doc.text").Size(13).Color(icon).
				Fallback(chromeIconFallbacks["doc.text"]).Role(mygoui.RoleNone)
			mygoui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				mygoui.Text(c, file.Name).FontSize(13).FontWeight(weight).Font(quickSwitcherFont).SingleLine().Ellipsis("…").
					TextColor(ink).Role(mygoui.RoleNone)
				if file.Folder != "" {
					mygoui.Text(c, file.Folder).FontSize(10).SingleLine().Ellipsis("…").
						TextColor(colors.muted).Role(mygoui.RoleNone)
				}
			})
			if selected {
				mygoui.Text(c, "↵").FontSize(11).TextColor(colors.muted).Role(mygoui.RoleNone)
			}
		})
	}).FillWidth().Height(height).Shrink(0).Padding(4, 0).Role(mygoui.RoleScroll).Label("Files")
}
