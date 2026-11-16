package ui

import (
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const multiFileMarkdownOnly = "Multi-file View opens Markdown files only."

const multiFileShortcutHint = "⌘⌥1 Columns · ⌘⌥2 Rows · ⌘⌥3 Grid\n⌘⌥4 Source / Rendered · ⌘⌥5 Edit / Done"

var multiFileShortcutModifiers = mygoui.Cmd | mygoui.Alt

type workspaceMultiFile struct {
	active, rendered bool
	editing          *workspaceTab
	focusReader      bool
	layout           workspaceMultiLayout
}

var multiFileArrangements = [...]struct {
	arrangement multiFileArrangement
	title       string
	key         mygoui.Key
}{
	{multiFileColumns, "Columns", mygoui.Key1},
	{multiFileRows, "Rows", mygoui.Key2},
	{multiFileGrid, "Grid", mygoui.Key3},
}

func multiFileTabEligible(tab *workspaceTab) bool {
	return tab != nil && tab.draft != nil && !tab.inspection && tab.drawing == nil &&
		(tab.document.Path == "" || workspace.IsMarkdownPath(tab.document.Path))
}

func (app *workspaceWindow) canToggleMultiFile() bool {
	return !app.modalOpen && (app.multiFile.active || app.vault != nil || slices.ContainsFunc(app.tabs, multiFileTabEligible))
}

func (app *workspaceWindow) permitsActiveDocument() bool {
	return !app.multiFile.active || multiFileTabEligible(app.workspaceTab)
}

func (app *workspaceWindow) toggleMultiFile() {
	if !app.canToggleMultiFile() {
		return
	}
	if app.multiFile.active {
		app.stopMultiFileRenderedEdit()
		app.multiFile.active = false
	} else {
		app.wys.flush(app)
		app.multiFile.active = true
		app.focusMode = false
		if !multiFileTabEligible(app.workspaceTab) {
			if index := slices.IndexFunc(app.tabs, multiFileTabEligible); index >= 0 {
				app.activateTab(app.tabs[index])
			}
		}
	}
	app.problem = ""
	app.modeChanged()
	app.multiFile.focusReader = app.multiFile.active && app.multiFile.rendered
	if app.multiFile.focusReader {
		app.focusSource = false
	}
}

func (app *workspaceWindow) setMultiFileRendered(rendered bool) {
	if app.modalOpen || !app.multiFile.active || app.multiFile.rendered == rendered {
		return
	}
	app.stopMultiFileRenderedEdit()
	app.multiFile.rendered = rendered
	app.focusSource = !rendered
	app.multiFile.focusReader = rendered
	app.problem = ""
}

func (app *workspaceWindow) toggleMultiFileRendered() {
	app.setMultiFileRendered(!app.multiFile.rendered)
}

func (app *workspaceWindow) selectMultiFileTile(tab *workspaceTab) {
	if app.modalOpen || !app.multiFile.active || app.workspaceTab == tab {
		return
	}
	problem := app.problem
	app.activateTab(tab)
	app.problem = problem
	// Pointer and keyboard events already have a concrete target.
	app.focusSource = false
}

func (app *workspaceWindow) editMultiFileRendered(tab *workspaceTab) {
	if app.modalOpen || !app.multiFile.active || !app.multiFile.rendered || !multiFileTabEligible(tab) {
		return
	}
	app.selectMultiFileTile(tab)
	app.wys.openSession(app)
	if app.wys.session.Document == nil {
		app.problem = "This Markdown uses features that require Source mode."
		return
	}
	app.multiFile.editing = tab
	app.wys.focus = true
	app.focusSource = false
	app.problem = ""
}

func (app *workspaceWindow) stopMultiFileRenderedEdit() {
	if app.multiFile.editing != nil {
		app.wys.flush(app)
		app.wys.closeFind()
		app.wys.closeBlockControls()
		app.wys.closeTableControls()
		app.multiFile.editing = nil
	}
}

func (app *workspaceWindow) toggleMultiFileRenderedEdit() {
	if app.modalOpen || !app.multiFile.active || !app.multiFile.rendered {
		return
	}
	if app.multiFile.editing != nil {
		app.stopMultiFileRenderedEdit()
		app.multiFile.focusReader = true
		return
	}
	app.editMultiFileRendered(app.workspaceTab)
}

func (app *workspaceWindow) registerMultiFileShortcuts(c *mygoui.Context) {
	if !app.multiFile.active {
		return
	}
	for _, item := range multiFileArrangements {
		c.OnWindowShortcut(multiFileShortcutModifiers, item.key, func() {
			app.multiFile.layout.arrangement = item.arrangement
		})
	}
	c.OnWindowShortcut(multiFileShortcutModifiers, mygoui.Key4, app.toggleMultiFileRendered)
	c.OnWindowShortcut(multiFileShortcutModifiers, mygoui.Key5, app.toggleMultiFileRenderedEdit)
}

func (app *workspaceWindow) multiFileContextMenu(menu *mygoui.Menu) {
	for _, item := range multiFileArrangements {
		if menu.Item(item.title).Radio(item.arrangement == app.multiFile.layout.arrangement).
			Shortcut(multiFileShortcutModifiers, item.key).Chosen() {
			app.multiFile.layout.arrangement = item.arrangement
		}
	}
	menu.Separator()
	mode := "Show Rendered"
	if app.multiFile.rendered {
		mode = "Show Source"
	}
	if menu.Item(mode).Shortcut(multiFileShortcutModifiers, mygoui.Key4).Chosen() {
		app.toggleMultiFileRendered()
	}
	edit := "Edit Current Rendered File"
	if app.multiFile.editing != nil {
		edit = "Finish Editing Rendered File"
	}
	if menu.Item(edit).Disabled(!app.multiFile.rendered || !multiFileTabEligible(app.workspaceTab)).
		Shortcut(multiFileShortcutModifiers, mygoui.Key5).Chosen() {
		app.toggleMultiFileRenderedEdit()
	}
}

func (app *workspaceWindow) drawMultiFileWorkspace(c *mygoui.Context) {
	var tiles []*workspaceTab
	for _, tab := range app.tabs {
		if multiFileTabEligible(tab) {
			tiles = append(tiles, tab)
		}
	}
	mygoui.Column(c.Key("multi-file workspace")).Role(mygoui.RoleGroup).Label("Multi-file workspace").
		Grow(1).MinWidth(0).MinHeight(0).Padding(0, 8, 4).AlignItems(mygoui.Stretch).Children(func() {
		if len(tiles) == 0 {
			mygoui.Text(c, "Open Markdown files to view them together.").FontSize(13).Padding(14, 8)
		}
		app.multiFile.layout.draw(c, tiles, app.drawMultiFileTile)
	})
}

func (app *workspaceWindow) drawMultiFileTile(c *mygoui.Context, tab *workspaceTab) {
	theme := c.Theme()
	edge := theme.Border
	if tab == app.workspaceTab {
		edge = theme.Accent.Alpha(0.7)
	}
	title := tab.title()
	mygoui.Column(c).Fill().MinHeight(0).MinWidth(0).Radius(7).Border(1, edge).Clip().
		Role(mygoui.RoleGroup).Label(title).Children(func() {
		switch {
		case !app.multiFile.rendered:
			app.drawSourceEditor(c, tab, "Source editor — "+title)
		case app.multiFile.editing == tab:
			app.drawMultiFileVisualEditor(c)
		default:
			app.drawMultiFileMarkdown(c, tab)
		}
	})
}

func (app *workspaceWindow) drawMultiFileVisualEditor(c *mygoui.Context) {
	w := &app.wys
	w.openSession(app)
	if w.session.Document == nil {
		app.multiFile.editing = nil
		app.drawMultiFileMarkdown(c, app.workspaceTab)
		return
	}
	if w.find.open {
		w.drawFind(c, app)
	}
	if w.linkOpen {
		w.linkEditor(c, app)
	}
	w.editor(c, app, 16)
}

type multiFileMarkdownState struct {
	vault   *workspace.Vault
	path    string
	options *markdown.Options
}

func (app *workspaceWindow) drawMultiFileMarkdown(c *mygoui.Context, tab *workspaceTab) {
	scroll := mygoui.Scroll(c.Key("multi-file Markdown")).Grow(1).MinWidth(0).MinHeight(0).
		Label("Rendered — "+tab.title()).Padding(12, 16).Focusable().IgnoreInput(app.modalOpen)
	if app.multiFile.focusReader && tab == app.workspaceTab {
		scroll.Focus()
		app.multiFile.focusReader = false
	}
	state := mygoui.Local(scroll, "multi-file Markdown assets", func() multiFileMarkdownState { return multiFileMarkdownState{} })
	vault, note := app.vault, tab.document.Path
	if tab.storage != nil {
		vault = tab.storage
	}
	if state.options == nil || state.vault != vault || state.path != note {
		*state = multiFileMarkdownState{vault: vault, path: note, options: withMarkdownCodeBadges(&markdown.Options{
			Extensions: wysiwyg.Extensions, WrapCode: true,
		})}
		state.options.OnLink = func(dest string) { app.openMultiFileLink(tab, dest) }
		if vault != nil && note != "" {
			state.options.LoadImage = func(dest string) (mygoui.ImageSource, error) {
				relative, ok := workspace.ResolveImagePath(note, dest)
				if !ok {
					return nil, fs.ErrNotExist
				}
				data, _, err := vault.ReadPreviewImage(relative, 4<<20)
				if err != nil {
					return nil, err
				}
				return mygoui.DecodeBitmap(data)
			}
		}
	}
	source := tab.draft.String()
	theme := app.resolvePreviewTheme(tab.previewThemeID).withEditorFont(app.preferences.EditorFont)
	scroll.FontFeatures(theme.editorFont.features)
	if !c.Preferences().HighContrast {
		scroll.Background(mygoui.Hex(theme.BackgroundColor))
		state.options.Theme = theme.markdownTheme(100, previewLineSpacing(source, app.preferences.Export.spacing()))
		state.options.Theme.CodeColors = previewCodeTheme(theme.dark())
	} else {
		state.options.Theme = &markdown.Theme{}
		theme.editorFont.applyNative(state.options.Theme)
	}
	if tab.previewAnchor != "" {
		state.options.Anchor = tab.previewAnchor
		state.options.AnchorVersion++
		tab.previewAnchor = ""
	}
	_, _, body := workspace.SplitFrontmatter(source)
	focused := mygoui.Local(scroll, "multi-file reader focused", func() bool { return false })
	if within := scroll.FocusWithin(); within != *focused {
		*focused = within
		if within {
			app.selectMultiFileTile(tab)
		}
	}
	scroll.ObserveInput(func(event mygoui.InputEvent) {
		if event.Kind == mygoui.InputPointerDown && event.Button == 0 {
			app.selectMultiFileTile(tab)
		}
	}).Children(func() {
		markdown.View(c, nativeHighlightedMarkdown(scroll, body, state.options.Extensions), state.options).FillWidth()
	})
}

func (app *workspaceWindow) openMultiFileLink(tab *workspaceTab, destination string) {
	link, err := url.Parse(destination)
	if err != nil || app.modalOpen || !app.multiFile.active {
		return
	}
	if link.Scheme != "" || link.Host != "" {
		openPreviewExternalLink(destination)
		return
	}
	if link.Path == "" {
		tab.previewAnchor = link.Fragment
		return
	}
	if path.IsAbs(link.Path) || strings.ContainsAny(link.Path, "\\\x00") || !workspace.IsMarkdownPath(link.Path) {
		app.problem = multiFileMarkdownOnly
		return
	}
	relative := path.Join(path.Dir(tab.document.Path), link.Path)
	if !fs.ValidPath(relative) {
		return
	}
	if tab.storage != nil {
		absolute := filepath.Join(tab.folderRoot, filepath.FromSlash(relative))
		app.openPickedFile(absolute)
		if app.absoluteDocument(app.workspaceTab) == absolute {
			app.followPreviewAnchor(link.Fragment)
		}
	} else {
		app.openNoteAt(relative, true, link.Fragment)
	}
}
