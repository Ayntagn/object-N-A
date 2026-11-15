package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/snippets"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type snippetSortOrder uint8

const (
	snippetsRecentlyModified snippetSortOrder = iota
	snippetsRecentlyCreated
	snippetsAlphabetical
	snippetsFoldersFirst
)

var snippetSortLabels = [...]string{"Recently Modified", "Recently Created", "Alphabetical", "Folders First"}

// SwiftUI spring(response: 0.42, dampingFraction: 0.86, blendDuration: 0).
// MyGo expresses the damping ratio as 1 - Bounce.
var snippetShadeEntrance = mygoui.Animation{
	Spring: mygoui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: .14},
}

type snippetHeightDrag struct {
	active         bool
	startY, height float32
}

// EV-UI-054 / EV-UI-235. The shade remains inside the document window and
// presents two independently scrollable panes below its controls.
func (app *workspaceWindow) drawSnippets(c *mygoui.Context) {
	s := &app.snippets
	mygoui.Overlay(c, func() {
		mygoui.Box(c.Key("snippets shade layer")).Fill().PassThrough().Children(func() {
			if !s.open || app.win != nil && !app.win.IsFocused() {
				return
			}
			windowWidth, windowHeight := c.Size()
			top := float32(titleBarHeight + 4)
			if len(app.tabs) > 1 && !app.focusMode {
				top += tabBarHeight + tabBarGap
			}
			width := min(float32(640), windowWidth-16)
			maxHeight := windowHeight - top - 8
			height := s.height
			if height == 0 {
				height = 380
			}
			height = min(height, maxHeight)
			if width <= 0 || height <= 0 {
				return
			}
			surface, edge := app.snippetColors(c)
			panel := mygoui.Column(c.Key("snippets shade")).Absolute().Top(top).Left((windowWidth-width)/2).
				Size(width, height).Role(mygoui.RoleGroup).Label("Snippets Shade").Clip().
				Background(surface).Border(1, edge).Radius(10)
			if !c.Preferences().ReduceMotion {
				offscreen := &mygoui.Motion{Y: -120, YPercent: -100}
				panel.Transition(mygoui.ElementTransition{
					Animation:    &snippetShadeEntrance,
					ExitDuration: 220 * time.Millisecond, ExitEase: mygoui.EaseInOut,
					Position: true, Enter: offscreen, Exit: offscreen,
				})
			}
			if panel.OverlayShortcut(0, mygoui.KeyEscape) {
				s.open = false
				c.Invalidate()
				return
			}
			panel.Children(func() {
				app.snippetLibrary(c, width, edge, false)
				s.snippetResizeGrip(c, height, maxHeight)
			})
		})
	})
}

func (app *workspaceWindow) snippetColors(c *mygoui.Context) (surface, edge mygoui.Color) {
	if app.blackout && !c.Preferences().HighContrast {
		return mygoui.RGB(2, 2, 3), mygoui.RGB(60, 60, 66)
	}
	return c.Theme().Surface, c.Theme().Border
}

func (app *workspaceWindow) snippetLibrary(c *mygoui.Context, width float32, edge mygoui.Color, floating bool) {
	s := &app.snippets
	if pinned, found := snippetByID(s.items, s.pinned); found && !floating {
		app.snippetPinnedView(c, pinned, edge, floating)
		return
	}
	app.snippetHeader(c, edge, floating)
	if s.problem != "" {
		mygoui.Text(c, s.problem).Role(mygoui.RoleAlert).TextColor(c.Theme().Danger).Padding(4, 10)
	}
	items := s.items
	if strings.TrimSpace(s.query) != "" {
		items = snippets.Search(items, s.query)
	}
	selected, found := snippetByID(items, s.selected)
	mygoui.Row(c).Grow(1).MinHeight(0).AlignItems(mygoui.Stretch).Children(func() {
		mygoui.Scroll(c.Key("snippets library")).Width(min(210, width*.46)).Shrink(0).
			MinHeight(0).Label("Snippet list").BorderWidth(0, 1, 0, 0).BorderColor(edge).
			Children(func() {
				mygoui.Column(c).FillWidth().Padding(8).Gap(2).Children(func() {
					if len(items) == 0 && s.problem == "" {
						empty := "No snippets"
						if s.notes {
							empty = "No notes"
						}
						mygoui.Text(c, empty).Role(mygoui.RoleStatus).TextColor(c.Theme().TextMuted)
					}
					app.snippetRows(c, items, 0)
				})
			})
		if found && !selected.IsFolder {
			app.snippetDetail(c.Key(selected.ID), selected)
		} else {
			prompt := "Select or create a snippet"
			if s.notes {
				prompt = "Select or create a note"
			}
			mygoui.Box(c).Grow(1).Justify(mygoui.Center).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, prompt).TextColor(c.Theme().TextMuted)
			})
		}
	})
	s.snippetExportNotice(c)
}

func (app *workspaceWindow) snippetPinnedView(c *mygoui.Context, item snippets.Snippet, edge mygoui.Color, floating bool) {
	// EV-UI-235: Pin switches the existing shade into a full-width reader.
	s := &app.snippets
	backLabel := "Back to Snippets"
	if s.pinnedFromPreview {
		backLabel = "Close Snippet Reader"
	}
	mygoui.Row(c).FillWidth().Height(39.5).Shrink(0).Gap(8).Padding(0, 12).AlignItems(mygoui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(edge).Children(func() {
		snippetSymbol(c, "pin.fill", "tag.fill", c.Theme().Accent)
		mygoui.Text(c, item.Title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").Bold()
		// EV-UI-238: leaving the reader resumes the surface that opened it.
		chromeControl(c.Key("exit pinned snippet"), backLabel, false, backLabel).Size(22, 22).KeepFocus().
			OnClick(func() {
				if s.pinnedFromPreview {
					s.open = false
				}
				s.pinned, s.pinnedFromPreview = "", false
			}).Children(func() {
			snippetSymbol(c, "rectangle.expand.vertical", "rectangle", c.Theme().TextMuted)
		})
		chromeControl(c.Key("insert pinned snippet"), "Insert as Callout", !app.canInsertSnippet(), "Insert as Callout").
			Size(22, 22).KeepFocus().OnClick(func() { app.insertSnippet(snippetCallout(item)) }).Children(func() {
			snippetSymbol(c, "text.quote", "quote.opening", c.Theme().TextMuted)
		})
		app.snippetWindowControls(c, floating)
	})
	content := item.Content
	mygoui.TextArea(c.Key(item.ID), &content).ReadOnly(true).Label("Pinned snippet").
		FillWidth().Grow(1).MinHeight(0).Margin(10, 12).Padding(10).Radius(6).
		Font("monospace").FontSize(13).Background(c.Theme().Background).Border(0, mygoui.Transparent)
}

func snippetSymbol(c *mygoui.Context, symbol, fallback string, ink mygoui.Color) {
	sfsymbols.Symbol(c, symbol).Size(14).Natural().Fallback(chromeIconFallbacks[fallback]).Color(ink).Role(mygoui.RoleNone)
}

func (app *workspaceWindow) showSnippetsWindow() {
	if app.win == nil || app.win.IsDestroyed() {
		return
	}
	s := &app.snippets
	if s.window != nil && !s.window.IsDestroyed() {
		s.window.Focus()
		return
	}
	s.open = false
	// EV-UI-235: the standalone library uses an 820 × 540 content rect.
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "Snippets", Width: 820, Height: 540, UseContentSize: true, Parent: app.win,
		Hidden: true, Content: mygoui.View(app.snippetsWindowView),
	})
	s.window = win
	win.OnClosed(func() {
		if s.window == win {
			s.window = nil
		}
	})
	win.Show()
}

func (app *workspaceWindow) snippetsWindowView(c *mygoui.Context) {
	app.applyAppearance(c)
	width, _ := c.Size()
	surface, edge := app.snippetColors(c)
	mygoui.Column(c).Fill().Background(surface).Children(func() {
		app.snippetLibrary(c, width, edge, true)
	})
}

func (s *workspaceSnippets) snippetResizeGrip(c *mygoui.Context, height, maximum float32) {
	grip := mygoui.Row(c.Key("resize snippets")).Height(14).FillWidth().Shrink(0).
		AlignItems(mygoui.Center).Justify(mygoui.Center).Role(mygoui.RoleSplitter).
		Label("Resize Snippets Shade").Cursor(mygoui.CursorResizeNS).Bind(&s.heightGrip)
	grip.Children(func() {
		mygoui.Box(c).Size(39, 3).Radius(1.5).Background(c.Theme().TextMuted.Alpha(.65)).Role(mygoui.RoleNone)
	})
	grip.HandleInput(func(event mygoui.InputEvent) bool {
		y := s.heightGrip.Bounds(c).Y + event.Y
		switch event.Kind {
		case mygoui.InputPointerDown:
			if event.Button != 0 {
				return false
			}
			s.resize = snippetHeightDrag{active: true, startY: y, height: height}
		case mygoui.InputPointerMove, mygoui.InputPointerUp:
			if !s.resize.active {
				return false
			}
			if !event.Canceled {
				const minimum = 240
				s.height = min(maximum, max(minimum, s.resize.height+y-s.resize.startY))
				c.Invalidate()
			}
			if event.Kind == mygoui.InputPointerUp {
				s.resize = snippetHeightDrag{}
			}
		default:
			return false
		}
		return true
	})
}

func (app *workspaceWindow) snippetHeader(c *mygoui.Context, edge mygoui.Color, floating bool) {
	s := &app.snippets
	mygoui.Row(c).FillWidth().Height(41).Shrink(0).Gap(8).Padding(0, 12).
		AlignItems(mygoui.Center).BorderWidth(0, 0, 1, 0).BorderColor(edge).
		Role(mygoui.RoleToolbar).Label("Snippet library controls").Children(func() {
		symbol := "scissors"
		if s.notes {
			symbol = "note.text"
		}
		chromeIconColor(c, symbol, 16.5, c.Theme().Accent)
		app.snippetCollectionPicker(c)
		app.snippetSearch(c)
		snippetMenu(c.Key("snippet sort"), "arrow.up.arrow.down", "Sort", func(m *mygoui.Menu) {
			for i, label := range snippetSortLabels {
				if m.Item(label).Checked(s.order == snippetSortOrder(i)).Chosen() {
					s.order = snippetSortOrder(i)
				}
			}
		})
		snippetMenu(c.Key("add snippet"), "plus", "Add", func(m *mygoui.Menu) {
			label := "New Snippet"
			if s.notes {
				label = "New Note"
			}
			if m.Item(label).Chosen() {
				app.addSnippetLibraryItem(label, false)
			}
			if m.Item("New Folder").Chosen() {
				app.addSnippetLibraryItem("New Folder", true)
			}
		})
		app.snippetShareButton(c, floating)
		item, found := snippetByID(s.items, s.selected)
		pinned := found && s.pinned == item.ID
		pinSymbol := "pin"
		if pinned {
			pinSymbol = "pin.fill"
		}
		chromeControl(c.Key("pin snippet"), "Pin", !found || item.IsFolder, "Focus this snippet").
			Size(22, 22).KeepFocus().AccessSelected(pinned).OnClick(func() {
			if pinned {
				s.pinned = ""
			} else {
				s.pinned = item.ID
			}
		}).Children(func() {
			snippetSymbol(c, pinSymbol, "tag", c.Theme().TextMuted)
		})
		app.snippetWindowControls(c, floating)
	})
}

func (app *workspaceWindow) snippetCollectionPicker(c *mygoui.Context) {
	s, theme, prefs := &app.snippets, c.Theme(), c.Preferences()
	selected := 0
	if s.notes {
		selected = 1
	}
	trackFill, activeFill, activeInk := theme.SurfaceHover, theme.Accent, theme.AccentText
	if app.blackout && !prefs.HighContrast {
		// EV-UI-235. Both segments share the same full-size radio geometry.
		trackFill = mygoui.RGB(18, 18, 18)
		activeFill, activeInk = mygoui.RGB(69, 151, 248), mygoui.RGB(255, 255, 255)
	}
	parts := mygoui.SegmentedBase(c.Key("snippet collections"), &selected, 2)
	parts.Track.Size(155, 24).Radius(6).Background(trackFill).Label("Snippet collection").
		OnChange(func() { s.load(selected == 1) }).Children(func() {
		for i, label := range [...]string{"Snippets", "Notes"} {
			fill, ink := trackFill, theme.Text
			if i == selected {
				fill, ink = activeFill, activeInk
			}
			segment := parts.Segment(i).Size(77.5, 24).Radius(6).Background(fill).Label(label)
			if !prefs.ReduceMotion && !prefs.HighContrast {
				segment.Transition(mygoui.ElementTransition{Colors: true})
			}
			segment.Children(func() { mygoui.Text(c, label).FontSize(13).SingleLine().TextColor(ink) })
		}
	})
}

func (app *workspaceWindow) snippetSearch(c *mygoui.Context) {
	s, theme := &app.snippets, c.Theme()
	fill, border := theme.Surface, theme.Border
	if app.blackout && !c.Preferences().HighContrast {
		fill, border = mygoui.RGB(3, 3, 6), mygoui.Transparent
	}
	box := mygoui.Row(c.Key("snippet search field")).Height(24).MinWidth(32).Grow(1).Padding(0, 7).
		AlignItems(mygoui.Center).Radius(6).Background(fill).Border(1, border)
	box.Children(func() {
		search := mygoui.TextInputBase(c.Key("snippet search"), &s.query).Search().Label("Search snippets").
			Placeholder("Search…").Height(15).FontSize(12).MinWidth(0).Grow(1)
		if s.focusSearch {
			search.Focus()
			s.focusSearch = false
		}
		if search.Focused() {
			box.BorderColor(theme.Focus)
		}
	})
}

func (app *workspaceWindow) snippetWindowControls(c *mygoui.Context, floating bool) {
	s := &app.snippets
	if !floating {
		chromeControl(c.Key("expand snippets"), "Expand Snippets", app.win == nil, "Open in a window").
			Size(22, 22).KeepFocus().OnClick(app.showSnippetsWindow).Children(func() {
			snippetSymbol(c, "macwindow", "rectangle", c.Theme().TextMuted)
		})
	}
	chromeButton(c.Key("close snippets"), "xmark", "Close Snippets", func() {
		if floating && s.window != nil {
			s.window.Close()
		} else {
			s.open = false
		}
	}).Size(22, 22).KeepFocus()
}

func snippetMenu(c *mygoui.Context, symbol, label string, build func(*mygoui.Menu)) mygoui.Element {
	return chromeMenuControl(c, label, build).Size(22, 22).KeepFocus().Children(func() {
		chromeIcon(c, symbol, 14)
	})
}

func (app *workspaceWindow) addSnippetLibraryItem(title string, folder bool) {
	s := &app.snippets
	item := snippets.Capture("", "", title, time.Now())
	item.IsFolder = folder
	item.IsExpanded = folder
	file, err := snippetFile(s.notes)
	if err == nil {
		var items []snippets.Snippet
		items, err = snippets.Append(file, item)
		if err == nil {
			s.items = items
			s.query, s.selected, s.problem, s.copied = "", item.ID, "", ""
			if !s.notes {
				app.refreshSnippetPreviews()
			}
		}
	}
	if err != nil {
		s.problem = fmt.Sprintf("Could not create item. %v", err)
	}
}

func (app *workspaceWindow) editSnippetLibraryItem(id string, apply func(*snippets.Details)) {
	s := &app.snippets
	file, err := snippetFile(s.notes)
	if err == nil {
		var items []snippets.Snippet
		items, err = snippets.Update(file, id, apply)
		if err == nil {
			s.items, s.problem, s.copied = items, "", ""
			if !s.notes {
				app.refreshSnippetPreviews()
			}
			return
		}
	}
	s.problem = fmt.Sprintf("Could not save changes. %v", err)
}

func snippetByID(items []snippets.Snippet, id string) (snippets.Snippet, bool) {
	if id == "" {
		return snippets.Snippet{}, false
	}
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
		if child, found := snippetByID(item.Children, id); found {
			return child, true
		}
	}
	return snippets.Snippet{}, false
}

func (s *workspaceSnippets) sorted(items []snippets.Snippet) []snippets.Snippet {
	sorted := slices.Clone(items)
	slices.SortStableFunc(sorted, func(a, b snippets.Snippet) int {
		switch s.order {
		case snippetsRecentlyCreated:
			return cmp.Compare(b.CreatedAt, a.CreatedAt)
		case snippetsAlphabetical:
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		case snippetsFoldersFirst:
			if a.IsFolder != b.IsFolder {
				if a.IsFolder {
					return -1
				}
				return 1
			}
		}
		return cmp.Compare(b.UpdatedAt, a.UpdatedAt)
	})
	return sorted
}

func (app *workspaceWindow) snippetRows(c *mygoui.Context, items []snippets.Snippet, depth int) {
	s := &app.snippets
	for _, item := range s.sorted(items) {
		if item.IsFolder {
			expanded, changed := s.expanded[item.ID]
			if !changed {
				expanded = item.IsExpanded
			}
			fold := "chevron.right"
			if expanded {
				fold = "chevron.down"
			}
			mygoui.ButtonBase(c.Key(item.ID)).Height(24).FillWidth().Padding(0, 6).MarginStart(float32(depth) * 9).
				Justify(mygoui.Start).AlignItems(mygoui.Center).Gap(6).Label(item.Title).Expanded(expanded).
				KeepFocus().OnClick(func() {
				if s.expanded == nil {
					s.expanded = make(map[string]bool)
				}
				s.expanded[item.ID] = !expanded
			}).Children(func() {
				chromeIcon(c, fold, 9)
				chromeIconColor(c, "folder.fill", 13, c.Theme().Accent)
				mygoui.Text(c, item.Title).Bold().SingleLine().Ellipsis("…").Role(mygoui.RoleNone)
			})
			if expanded {
				app.snippetRows(c, item.Children, depth+1)
			}
			continue
		}
		selected := s.selected == item.ID
		row := mygoui.ButtonBase(c.Key(item.ID)).FillWidth().Height(42).MinWidth(0).
			MarginStart(float32(depth)*9).Padding(5, 8).Radius(6).Gap(4).Justify(mygoui.Start).
			Label(item.Title).AccessSelected(selected).KeepFocus().
			OnClick(func() { s.selected, s.problem, s.copied = item.ID, "", "" }).
			ContextMenu(func(m *mygoui.Menu) {
				if m.Item("Insert at Cursor").Disabled(!app.canInsertSnippet()).Chosen() {
					app.insertSnippet(item.Content)
				}
				if m.Item("Copy").Chosen() {
					s.copy(c, item)
				}
			})
		border := c.Theme().Border.Alpha(.52)
		if selected {
			row.Background(c.Theme().Accent.Alpha(.23))
			border = c.Theme().Accent.Alpha(.52)
		} else if row.Hovered() {
			row.Background(c.Theme().SurfaceHover)
		}
		row.Border(1, border).Children(func() {
			mygoui.Column(c).FillWidth().MinWidth(0).Gap(2).Children(func() {
				mygoui.Row(c).FillWidth().Gap(5).AlignItems(mygoui.Center).Children(func() {
					if !s.notes && item.Language != "" {
						mygoui.Text(c, item.Language).Font("monospace").FontSize(9).Padding(1, 5).
							Radius(6).Background(c.Theme().Accent.Alpha(.32)).Role(mygoui.RoleNone)
					}
					mygoui.Text(c, item.Title).FontSize(12).Bold().SingleLine().MinWidth(0).
						Ellipsis("…").Role(mygoui.RoleNone)
				})
				preview, _, _ := strings.Cut(strings.TrimSpace(item.Content), "\n")
				mygoui.Text(c, preview).FontSize(10).SingleLine().Ellipsis("…").MinWidth(0).
					TextColor(c.Theme().TextMuted).Role(mygoui.RoleNone)
			})
		})
	}
}

func (s *workspaceSnippets) copy(c *mygoui.Context, item snippets.Snippet) {
	services := c.Services()
	expiry := c.Now().Add(1500 * time.Millisecond)
	c.WriteClipboardThen(item.Content, func(err error) {
		if err != nil {
			s.problem = fmt.Sprintf("Could not copy. %v", err)
		} else {
			s.copied = item.ID
			s.copiedUntil = expiry
		}
		services.Invalidate()
	})
}

func (app *workspaceWindow) snippetDetail(c *mygoui.Context, item snippets.Snippet) {
	s := &app.snippets
	mygoui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Padding(10, 12).Gap(8).Children(func() {
		mygoui.Row(c).FillWidth().Height(24).Shrink(0).Gap(6).AlignItems(mygoui.Center).Children(func() {
			title := item.Title
			mygoui.TextInput(c.Key("title"), &title).Height(24).MinWidth(42).Grow(1).Padding(0).Bold().
				Background(mygoui.Transparent).Border(0, mygoui.Transparent).FocusRing(false).
				Label("Snippet title").OnChange(func() { app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) { d.Title = title }) })
			mygoui.Button(c.Key("insert"), "Insert").Size(69, 24).KeepFocus().Disabled(!app.canInsertSnippet()).
				Tooltip("Insert as callout").OnClick(func() { app.insertSnippet(snippetCallout(item)) }).
				MarginEnd(-6)
			snippetMenu(c.Key("snippet insertion options"), "chevron.down", "Snippet insertion options", func(m *mygoui.Menu) {
				if m.Item("Insert as Callout").Chosen() {
					app.insertSnippet(snippetCallout(item))
				}
				link := "[[snippet:" + item.Title + "]]\n"
				if m.Item("Insert Embed (live)").Chosen() {
					app.insertSnippet("!" + link)
				}
				if m.Item("Insert Link (hover)").Chosen() {
					app.insertSnippet(link)
				}
			}).Size(27, 24).Disabled(!app.canInsertSnippet())
			symbol, help := "doc.on.doc", "Copy"
			if s.copied == item.ID && c.Now().Before(s.copiedUntil) {
				symbol, help = "checkmark", "Copied"
				c.After(s.copiedUntil.Sub(c.Now()))
			}
			chromeButton(c.Key("copy"), symbol, "Copy", func() { s.copy(c, item) }).
				Size(22, 22).KeepFocus().Tooltip(help)
			chromeButton(c.Key("trash"), "trash", "Trash", func() { app.deleteSnippetLibraryItem(item.ID) }).
				Size(22, 22).MarginStart(2).KeepFocus().Tooltip("Delete")
		})
		content := item.Content
		editor := mygoui.TextArea(c.Key("content"), &content).FillWidth().Grow(1).MinHeight(0).
			Padding(10).Radius(6).Background(c.Theme().Background).Border(0, mygoui.Transparent).
			Label("Snippet content").OnChange(func() { app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) { d.Content = content }) })
		if !s.notes && item.Language != "" {
			editor.Font("monospace").FontSize(13)
		}
		app.snippetMetadata(c, item)
	})
}

// EV-UI-235: the detail header inserts a titled Markdown callout. The row's
// Insert at Cursor action still supplies the original text.
func snippetCallout(item snippets.Snippet) string {
	var body strings.Builder
	body.WriteString("> [!note]")
	if title := strings.TrimSpace(item.Title); title != "" {
		body.WriteByte(' ')
		body.WriteString(title)
	}
	body.WriteByte('\n')
	language := item.Language
	fence := ""
	if language != "" && language != "markdown" {
		fence = "```"
		for strings.Contains(item.Content, fence) {
			fence += "`"
		}
		body.WriteString("> " + fence + language + "\n")
	}
	for _, line := range strings.Split(strings.TrimSuffix(item.Content, "\n"), "\n") {
		body.WriteString("> " + line + "\n")
	}
	if fence != "" {
		body.WriteString("> " + fence + "\n")
	}
	return body.String()
}

func (app *workspaceWindow) snippetMetadata(c *mygoui.Context, item snippets.Snippet) {
	s := &app.snippets
	mygoui.Column(c).FillWidth().Shrink(0).Gap(5).Children(func() {
		if !s.notes {
			app.snippetLanguageMenu(c.Key("language"), item)
		}
		if len(item.Tags) != 0 {
			mygoui.Row(c).FillWidth().Gap(4).Clip().Children(func() {
				for i, tag := range item.Tags {
					mygoui.Row(c.Key(i)).Height(16).Padding(0, 3, 0, 5).Gap(2).AlignItems(mygoui.Center).
						Background(c.Theme().SurfaceHover).Radius(8).Children(func() {
						mygoui.Text(c, tag).FontSize(10).TextColor(c.Theme().TextMuted)
						chromeButtonIcon(c, "xmark", "Remove tag "+tag, func() {
							app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) {
								d.Tags = slices.DeleteFunc(d.Tags, func(value string) bool { return value == tag })
							})
						}, 7, c.Theme().TextMuted).Size(12, 14).KeepFocus()
					})
				}
			})
		}
		tag := mygoui.Local(mygoui.Box(c.Key("tag draft")), "tag", func() string { return "" })
		mygoui.TextInput(c.Key("tag"), tag).FillWidth().Height(16).Padding(0, 4).FontSize(11).
			Background(mygoui.Transparent).Border(0, mygoui.Transparent).Placeholder("add tag, ⏎").Label("Add tag").
			OnSubmit(func() {
				value := strings.TrimSpace(*tag)
				if value != "" {
					app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) {
						if !slices.Contains(d.Tags, value) {
							d.Tags = append(d.Tags, value)
						}
					})
					*tag = ""
				}
			})
		notes := item.Notes
		mygoui.TextInput(c.Key("notes"), &notes).FillWidth().Height(24).FontSize(11).Placeholder("Notes…").
			Label("Snippet notes").OnChange(func() { app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) { d.Notes = notes }) })
		if item.SourceFileName != "" {
			mygoui.Text(c, item.SourceFileName).FontSize(10).TextColor(c.Theme().TextMuted).
				SingleLine().Ellipsis("…").Label("Source: " + item.SourceFileName)
		}
	})
}

// EV-UI-237. The native menu lists these IDs in this order.
var snippetLanguageChoices = [...]string{
	"", "swift", "python", "javascript", "typescript", "go", "rust", "java",
	"cpp", "html", "css", "json", "markdown", "bash", "sql", "yaml", "ruby", "php",
}

func (app *workspaceWindow) snippetLanguageMenu(c *mygoui.Context, item snippets.Snippet) {
	language := strings.ToLower(item.Language)
	label := language
	if label == "" {
		label = "Plain Text"
	}
	ink := c.Theme().TextMuted
	mygoui.ButtonBase(c).Height(16).MinWidth(73).AlignSelf(mygoui.Start).Gap(4).Padding(0, 2).Justify(mygoui.Start).
		Role(mygoui.RoleMenuButton).Label(label).Description("Language").Menu(func(menu *mygoui.Menu) {
		for _, choice := range snippetLanguageChoices {
			text := choice
			if choice == "" {
				text = "Auto / None"
			}
			if menu.Item(text).Checked(language == choice).Chosen() {
				app.editSnippetLibraryItem(item.ID, func(d *snippets.Details) { d.Language = choice })
			}
		}
	}).Children(func() {
		chromeIconColor(c, "chevron.left.forwardslash.chevron.right", 17, ink)
		mygoui.Text(c, label).FontSize(12).TextColor(ink).SingleLine()
		chromeIconColor(c, "chevron.down", 8, ink)
	})
}
