package ui

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-242 / EV-UI-245. The native popover adds 26 pt around its client area.
const aiContextWidth, aiContextHeight = 420, 450

// EV-UI-267: one open document owns its Notes and attachment, including
// when the tab moves to a different window.
type workspaceAIContext struct {
	documentPath string
	snapshot     string
	characters   int
	notes        string
}

func (context *workspaceAIContext) hasNotes() bool {
	return strings.TrimSpace(context.notes) != ""
}

func (app *workspaceWindow) activeAIContext() *workspaceAIContext {
	tab := app.workspaceTab
	if tab == nil || tab.draft == nil {
		return nil
	}
	if tab.aiContext == nil {
		tab.aiContext = &workspaceAIContext{}
	}
	tab.aiContext.documentPath = app.absoluteDocument(tab)
	return tab.aiContext
}

func (app *workspaceWindow) reconcileAIContextDocument(tab *workspaceTab) {
	if tab.aiContext == nil {
		return
	}
	if tab == app.workspaceTab {
		app.closeAIContextReader()
	}
	// A reload uses the same named document. A replaced note or a new
	// untitled draft starts with its own context and previous-turn identity.
	if nextPath := app.absoluteDocument(tab); nextPath == "" || nextPath != tab.aiContext.documentPath {
		tab.aiContext = nil
	}
}

func (app *workspaceWindow) closeAIContextReader() {
	if flyout := app.ai.contextFlyout; flyout != nil {
		app.ai.contextFlyout = nil
		flyout.Close()
	}
}

func (app *workspaceWindow) showAIContext(anchor mygoui.Rect) {
	s := &app.ai
	context := app.activeAIContext()
	if app.win == nil || app.win.IsDestroyed() || context == nil || (context.snapshot == "" && !context.hasNotes()) {
		return
	}
	if s.contextFlyout != nil {
		app.closeAIContextReader()
		return
	}
	snapshot, characters := context.snapshot, context.characters
	var flyout *mygo.Window
	flyout = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: app.win, Anchor: documentTitleAnchor(anchor),
		Placement: mygo.PlacementTop, Width: aiContextWidth, Height: aiContextHeight,
		Popover: true, Focusable: true, Shadow: true,
		Content: mygoui.View(func(c *mygoui.Context) {
			app.applyAppearance(c)
			c.Root().Background(mygoui.Transparent)
			surface := drawAIContextReader(c, app.blackout, snapshot, characters, &context.notes, func() { flyout.Close() }, app.invalidateAIChat)
			if !c.Vibrancy() {
				surface.Background(c.Theme().Surface).Radius(16)
			}
		}),
	})
	if flyout == nil {
		return
	}
	s.contextFlyout = flyout
	flyout.OnClosed(func() {
		if s.contextFlyout == flyout {
			s.contextFlyout = nil
		}
	})
}

func drawAIContextReader(c *mygoui.Context, blackout bool, snapshot string, characters int, notes *string, closeReader, notesChanged func()) mygoui.Element {
	colors := aiWorkspaceColors(c, blackout)
	blackoutReader := blackout && !c.Preferences().HighContrast
	documentInk, notesInk, copyInk, notesFill := colors.link, colors.link, colors.link, colors.guide
	reader := mygoui.Column(c).Fill().Padding(12, 8)
	if blackoutReader {
		// EV-UI-261. These separators span the popover client area and do
		// not take space from either text field.
		documentInk, notesInk = mygoui.Hex("#3ae866"), mygoui.Hex("#d4d6e0")
		copyInk, notesFill = mygoui.Hex("#909090"), mygoui.Hex("#060608")
		reader.DrawOver(func(p *mygoui.Painter, bounds mygoui.Rect) {
			rule := mygoui.RGBA(255, 255, 255, .12)
			for _, offset := range [...]float32{40, bounds.H - 152, bounds.H - 45} {
				p.Fill(mygoui.Rect{X: bounds.X, Y: bounds.Y + offset, W: bounds.W, H: 1}, rule, 0)
			}
		})
	}
	if snapshot == "" {
		characters = 0
	}
	return reader.Children(func() {
		mygoui.Row(c).FillWidth().Height(16).Padding(0, 4).Gap(9).AlignItems(mygoui.Center).Children(func() {
			aiWorkspaceIcon(c, "doc.text.fill", "doc.text", 14, documentInk)
			if snapshot != "" {
				mygoui.Text(c, "Document").FontSize(13).Bold().TextColor(colors.text)
			}
			mygoui.Spacer(c)
			mygoui.Text(c, strconv.Itoa(characters)+" chars").FontSize(12).TextColor(colors.muted)
		})
		body := mygoui.TextArea(c.Key("AI context contents"), &snapshot).FillWidth().Grow(1).MinHeight(0).
			Margin(17, 0, 10).Padding(8, 10).Label("Attached context").ReadOnly(true).
			Font("monospace").FontSize(12).TextColor(colors.text).Background(colors.guide).Border(0, mygoui.Transparent).Radius(7)
		if snapshot == "" {
			body.Background(mygoui.Transparent)
			opened := mygoui.Local(body, "AI context initial focus", func() bool { return false })
			if !*opened {
				body.Focus()
				*opened = true
			}
		}
		mygoui.Column(c).FillWidth().Height(114).Shrink(0).Children(func() {
			mygoui.Row(c).FillWidth().Height(20).Padding(0, 5).Gap(4).AlignItems(mygoui.Center).Children(func() {
				aiWorkspaceIcon(c, "note.text", "doc.text", 11, notesInk)
				mygoui.Text(c, "Notes").FontSize(12).FontWeight(600).TextColor(colors.text)
				mygoui.Text(c, "- persists across messages").FontSize(11).TextColor(colors.muted)
			})
			mygoui.Column(c).FillWidth().Height(60).Shrink(0).Margin(7, 0, 0).Padding(0, 10).Children(func() {
				mygoui.TextArea(c.Key("AI context notes"), notes).Fill().MinHeight(0).
					Padding(4, 6).Label("Context notes").FontSize(12).
					Placeholder("Colors, file paths, style notes, constraints...").
					TextColor(colors.text).Background(notesFill).Border(0, mygoui.Transparent).Radius(7).OnChange(notesChanged)
			})
		})
		mygoui.Row(c).FillWidth().Height(20).Padding(0, 4).AlignItems(mygoui.Center).Children(func() {
			copyButton := mygoui.ButtonBase(c.Key("copy attached context")).Size(48, 14).Center().Gap(4).Label("Copy").
				OnClick(func() { c.WriteClipboard(snapshot) })
			if blackoutReader {
				copyButton.MarginStart(1.5).Height(14.125)
			}
			copyButton.Children(func() {
				aiWorkspaceIcon(c, "doc.on.doc", "", 11, copyInk)
				mygoui.Text(c, "Copy").FontSize(11).TextColor(copyInk).Role(mygoui.RoleNone)
			})
			mygoui.Spacer(c)
			if !blackoutReader {
				mygoui.Button(c.Key("close attached context"), "Done").Size(47.5, 20).Padding(0).OnClick(closeReader)
				return
			}
			mygoui.ButtonBase(c.Key("close attached context")).Size(47.5, 20).Center().Radius(5).
				Background(mygoui.Hex("#4095fa")).FocusRing(true).Label("Done").OnClick(closeReader).
				Children(func() {
					mygoui.Text(c, "Done").FontSize(11).FontWeight(600).TextColor(mygoui.RGB(255, 255, 255)).Role(mygoui.RoleNone)
				})
		})
	})
}
