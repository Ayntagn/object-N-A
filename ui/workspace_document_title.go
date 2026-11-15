package ui

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-234. The system popover adds its chrome around this content.
const documentTitlePopoverWidth, documentTitlePopoverHeight = 333, 124

type documentTitleIdentity struct {
	tab  *workspaceTab
	path string
}

type workspaceDocumentTitle struct {
	flyout     *mygo.Window
	anchor     mygo.Rectangle
	icon       *mygoui.Bitmap
	folderIcon *mygoui.Bitmap
	inline     bool
	file       fs.FileInfo

	name, tags, originalTags, folder string
	focusName, locked, metadata      bool
}

func (title *workspaceDocumentTitle) Close() error {
	title.inline = false
	if title.flyout != nil && !title.flyout.IsDestroyed() {
		title.flyout.Close()
	}
	return nil
}

func (app *workspaceWindow) canShowDocumentTitleActions() bool {
	return workspace.IsMarkdownPath(app.document.Path)
}

func (app *workspaceWindow) drawDocumentTitle(c *mygoui.Context) {
	absolute := app.absoluteDocument(app.workspaceTab)
	key := documentTitleIdentity{tab: app.workspaceTab, path: absolute}
	column := mygoui.Column(c.Key(key)).Shrink(1).MinWidth(0).PaddingStart(7)
	state := mygoui.Local(column, "document-title", func() workspaceDocumentTitle {
		var state workspaceDocumentTitle
		if documentTitleMetadataSupported {
			state.file, _ = os.Lstat(absolute)
			state.readMetadata(absolute)
		}
		if app.win != nil {
			if data, err := mygo.Shell.FileIcon(absolute, 36); err == nil {
				state.icon, _ = mygoui.DecodeBitmap(data)
			}
		}
		return state
	})
	visibleActions := column.Hovered() || column.FocusWithin() || state.flyout != nil || state.inline
	var status string
	switch {
	case state.locked:
		status = "Locked"
	case app.dirty():
		status = "Edited"
	}
	if status != "" {
		column.Margin(6, 0, 0)
	}
	ink, title := c.Theme().TextMuted, app.title()
	column.Children(func() {
		mygoui.Row(c).Height(26).Shrink(0).Gap(3.25).AlignItems(mygoui.Center).Children(func() {
			if visibleActions {
				if state.icon != nil {
					mygoui.Image(c, state.icon).Size(18, 18).Shrink(0).Label(title)
				} else {
					chromeIconColor(c, "doc.text", 18, ink)
				}
			}
			mygoui.Text(c, title).MinWidth(0).Shrink(1).SingleLine().Ellipsis("…").FontSize(13).FontWeight(700)
			disclosure := mygoui.ButtonBase(c.Key("document actions")).Size(20, 26).Role(mygoui.RoleButton).
				Label("go down").Disabled(app.modalOpen).KeepFocus().FocusRing(true)
			if state.flyout != nil {
				disclosure.Draw(func(_ *mygoui.Painter, frame mygoui.Rect) {
					anchor := documentTitleAnchor(frame)
					if state.flyout != nil && anchor != state.anchor {
						state.anchor = anchor
						state.flyout.SetAnchor(anchor)
					}
				})
			}
			if visibleActions {
				disclosure.Children(func() {
					arrowInk := ink
					if c.Preferences().HighContrast {
						arrowInk = c.Theme().Text
					} else if app.blackout {
						arrowInk = mygoui.RGB(140, 140, 140)
					}
					sfsymbols.Symbol(c, "chevron.down").Size(18).Natural().Weight(sfsymbols.Semibold).
						Color(arrowInk).Fallback(chromeIconFallbacks["chevron.down"]).Role(mygoui.RoleNone)
				})
			}
			if disclosure.Clicked() {
				app.openDocumentTitlePopover(disclosure.Bounds(), key, state)
			}
			if app.win == nil {
				mygoui.PopoverBase(c, disclosure, &state.inline, func(panel mygoui.Element) {
					panel.Size(documentTitlePopoverWidth, documentTitlePopoverHeight).AttachTo(disclosure, mygoui.AnchorBottom, mygoui.AnchorTop).
						Background(c.Theme().Surface).Radius(16)
					app.documentTitlePanel(c, key, state)
				})
			}
		})
		if status != "" {
			label := mygoui.Text(c, status).Label("Document status").FontSize(11).TextColor(ink).Height(16)
			if visibleActions {
				label.MarginStart(21.25)
			}
		}
	})
}

func (app *workspaceWindow) openDocumentTitlePopover(anchor mygoui.Rect, identity documentTitleIdentity, state *workspaceDocumentTitle) {
	if state.flyout != nil {
		state.Close()
		return
	}
	if state.inline {
		state.inline = false
		return
	}
	state.file, _ = os.Lstat(identity.path)
	state.name = filepath.Base(identity.path)
	state.folder = filepath.Base(filepath.Dir(identity.path))
	state.readMetadata(identity.path)
	state.focusName = !state.locked
	if app.win == nil {
		state.inline = true
		return
	}
	if data, err := mygo.Shell.FileIcon(filepath.Dir(identity.path), 32); err == nil {
		state.folderIcon, _ = mygoui.DecodeBitmap(data)
	}
	state.anchor = documentTitleAnchor(anchor)
	flyout := mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: app.win, Anchor: state.anchor,
		Placement: mygo.PlacementBottom,
		Width:     documentTitlePopoverWidth, Height: documentTitlePopoverHeight,
		Popover: true, Focusable: true, Shadow: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.applyAppearance(c)
			c.Root().Background(mygoui.Transparent)
			panel := app.documentTitlePanel(c, identity, state)
			if !c.Vibrancy() {
				panel.Background(c.Theme().Surface).Radius(16)
			}
		}),
	})
	if flyout == nil {
		return
	}
	state.flyout = flyout
	flyout.OnClosed(func() {
		if state.flyout == flyout {
			state.flyout = nil
		}
		if !app.win.IsDestroyed() {
			app.win.Invalidate()
		}
	})
}

func documentTitleAnchor(frame mygoui.Rect) mygo.Rectangle {
	return mygo.Rectangle{
		X: int(math.Round(float64(frame.X))), Y: int(math.Round(float64(frame.Y))),
		Width: int(math.Round(float64(frame.W))), Height: int(math.Round(float64(frame.H))),
	}
}

func documentTitleTextField(c *mygoui.Context, value *string, blackout, disabled bool) mygoui.Element {
	if !blackout {
		return mygoui.TextInput(c, value).Disabled(disabled)
	}
	theme := c.Theme()
	field := mygoui.TextInputBase(c, value).Padding(theme.Space(1.5), theme.Space(2.5)).Radius(theme.Radius).
		Background(theme.Surface.Mix(theme.Text, .08)).Border(1, theme.Border.Mix(theme.Text, .12)).
		SelectionColor(mygoui.RGB(73, 96, 126)).Disabled(disabled)
	if field.Focused() && !disabled {
		// EV-UI-234. The native focus band extends outside the field.
		focus := mygoui.RGB(80, 153, 194).Alpha(.74)
		if c.Preferences().ReduceTransparency {
			focus = mygoui.RGB(78, 126, 154)
		}
		field.DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
			outer := mygoui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4}
			p.Stroke(outer, focus, theme.Radius+2, 3)
		})
	}
	return field
}

func (app *workspaceWindow) documentTitlePanel(c *mygoui.Context, identity documentTitleIdentity, state *workspaceDocumentTitle) mygoui.Element {
	blackout := app.blackout && !c.Preferences().HighContrast
	whereFill := c.Theme().SurfacePressed
	if blackout {
		whereFill = whereFill.Mix(c.Theme().Text, .135)
	}
	return mygoui.Column(c).Fill().Padding(17, 19, 15).Gap(7).Children(func() {
		documentTitleRow(c, "Name:", func() {
			name := documentTitleTextField(c, &state.name, blackout, state.locked || !app.canMoveTitledDocument(identity, state)).
				Label("Name").Grow(1).Height(26).MinWidth(0).
				OnSubmit(func() { app.renameTitledDocument(identity, state) })
			if state.focusName && !name.IsDisabled() {
				name.Focus().SetTextSelection(0, utf8.RuneCountInString(state.name))
				state.focusName = false
			}
		})
		documentTitleRow(c, "Tags:", func() {
			documentTitleTextField(c, &state.tags, blackout, state.locked || !state.metadata).
				Label("tag editor").Grow(1).Height(24).MinWidth(0).
				OnSubmit(func() { app.commitTitledDocumentTags(identity, state) })
		})
		documentTitleRow(c, "Where:", func() {
			where := mygoui.ButtonBase(c).Grow(1).MinWidth(0).Height(26).Padding(0, 10).
				Radius(6).Background(whereFill).Role(mygoui.RolePopUpButton).Label(state.folder).
				Disabled(state.locked || !app.canMoveTitledDocument(identity, state)).
				Menu(func(m *mygoui.Menu) { app.titledDocumentFolderMenu(m, identity, state) })
			where.Children(func() {
				if state.folderIcon != nil {
					mygoui.Image(c, state.folderIcon).Size(16, 16).Shrink(0)
				} else {
					chromeIconColor(c, "folder.fill", 15, c.Theme().Accent)
				}
				mygoui.Text(c, state.folder).MinWidth(0).Grow(1).SingleLine().Ellipsis("…").MarginStart(4)
				chromeIconColor(c, "chevron.up.chevron.down", 10, c.Theme().Text)
			})
			symbol, label := "lock.open.fill", "unlock"
			if state.locked {
				symbol, label = "lock.fill", "lock"
			}
			lock := mygoui.CheckboxBase(c, &state.locked).Size(26, 11).Center().Label(label).
				Disabled(!state.metadata).Children(func() {
				chromeIconColor(c, symbol, 11, c.Theme().TextMuted)
			})
			if lock.Changed() {
				app.setTitledDocumentLock(identity, state)
				c.Invalidate()
			}
		})
	})
}

func (app *workspaceWindow) canMoveTitledDocument(identity documentTitleIdentity, state *workspaceDocumentTitle) bool {
	return app.vault != nil && identity.tab.storage == nil && state.file != nil && state.file.Mode().IsRegular()
}

func (app *workspaceWindow) titledDocumentStillCurrent(identity documentTitleIdentity, state *workspaceDocumentTitle) bool {
	if app.workspaceTab != identity.tab || app.absoluteDocument(identity.tab) != identity.path || state.file == nil {
		return false
	}
	current, err := os.Lstat(identity.path)
	return err == nil && os.SameFile(current, state.file)
}

// EV-UI-234. Where starts with the current directory and its parent chain.
// The workspace's existing move dialog scopes destinations to this Vault.
func (app *workspaceWindow) titledDocumentFolderMenu(menu *mygoui.Menu, identity documentTitleIdentity, state *workspaceDocumentTitle) {
	if state.locked || !app.canMoveTitledDocument(identity, state) {
		return
	}
	relative := identity.tab.document.Path
	current := path.Dir(relative)
	for folder := current; ; folder = path.Dir(folder) {
		name := path.Base(folder)
		if folder == "." {
			name = filepath.Base(app.root)
		}
		if menu.Item(name).ID(folder).Checked(folder == current).Chosen() && folder != current {
			app.moveTitledDocument(identity, state, path.Join(folder, path.Base(relative)))
		}
		if folder == "." {
			break
		}
	}
	menu.Separator()
	if menu.Item("Other…").Chosen() && app.titledDocumentStillCurrent(identity, state) {
		state.Close()
		app.chooseMoveFolder(relative)
	}
}

func (title *workspaceDocumentTitle) readMetadata(absolute string) {
	title.metadata, title.locked = false, false
	title.tags, title.originalTags = "", ""
	if !documentTitleMetadataSupported || title.file == nil || !title.file.Mode().IsRegular() {
		return
	}
	tags, locked, err := readDocumentTitleMetadata(absolute)
	if err != nil {
		return
	}
	title.metadata, title.locked = true, locked
	if len(tags) != 0 {
		var input strings.Builder
		writer := csv.NewWriter(&input)
		_ = writer.Write(tags)
		writer.Flush()
		title.tags = strings.TrimSuffix(input.String(), "\n")
		title.originalTags = title.tags
	}
}

// EV-UI-234. Lock changes the file flag and keeps the popover open.
func (app *workspaceWindow) setTitledDocumentLock(identity documentTitleIdentity, title *workspaceDocumentTitle) {
	if !title.metadata || !app.titledDocumentStillCurrent(identity, title) {
		title.locked = !title.locked
		return
	}
	if err := writeDocumentTitleLock(identity.path, title.locked); err != nil {
		title.locked = !title.locked
		app.problem = err.Error()
	}
	if app.win != nil {
		app.win.Invalidate()
	}
}

// EV-UI-234. Commit the platform tags on Return and keep the note's bytes.
func (app *workspaceWindow) commitTitledDocumentTags(identity documentTitleIdentity, title *workspaceDocumentTitle) {
	if !title.metadata || title.locked || !app.titledDocumentStillCurrent(identity, title) {
		return
	}
	if title.tags != title.originalTags {
		var tags []string
		if value := strings.TrimSpace(title.tags); value != "" {
			reader := csv.NewReader(strings.NewReader(value))
			reader.TrimLeadingSpace = true
			parsed, err := reader.Read()
			if err != nil {
				app.problem = "Could not read the tag names."
				return
			}
			for _, tag := range parsed {
				if tag = strings.TrimSpace(tag); tag != "" {
					tags = append(tags, tag)
				}
			}
		}
		if err := writeDocumentTitleTags(identity.path, tags); err != nil {
			app.problem = err.Error()
			return
		}
	}
	title.Close()
}

// EV-UI-234. A Return in Name commits a rename of the current Markdown file.
func (app *workspaceWindow) renameTitledDocument(identity documentTitleIdentity, state *workspaceDocumentTitle) {
	if state.locked || !app.canMoveTitledDocument(identity, state) || !app.titledDocumentStillCurrent(identity, state) {
		return
	}
	name := strings.TrimSpace(state.name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		app.problem = "Enter a single file name."
		return
	}
	from := identity.tab.document.Path
	app.moveTitledDocument(identity, state, path.Join(path.Dir(from), markdownFilename(name)))
}

func (app *workspaceWindow) moveTitledDocument(identity documentTitleIdentity, state *workspaceDocumentTitle, destination string) {
	if !app.titledDocumentStillCurrent(identity, state) {
		return
	}
	from := identity.tab.document.Path
	if err := app.moveNote(from, destination); err != nil {
		app.problem = fmt.Sprintf("Could not move %q. %v", path.Base(from), err)
		return
	}
	state.Close()
}

func documentTitleRow(c *mygoui.Context, label string, controls func()) {
	mygoui.Row(c.Key(label)).Height(26).FillWidth().Shrink(0).AlignItems(mygoui.Center).Gap(6).Children(func() {
		mygoui.Text(c, label).Width(45.5).Shrink(0).FontSize(13).TextColor(c.Theme().TextMuted).TextAlign(mygoui.End)
		controls()
	})
}
