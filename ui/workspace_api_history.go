package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"cyberwriter/internal/core/apihistory"
	"cyberwriter/internal/core/liveapi"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type apiHistoryItem = apihistory.Item

const (
	apiHistoryWidth     = 780
	apiHistoryHeight    = 540
	apiHistoryMidpoint  = (apiHistoryWidth - 1) / 2.0
	apiTextPreviewLimit = 64 << 10
)

type workspaceAPIHistory struct {
	app      *workspaceWindow
	win      *mygo.Window
	filename string
	items    []apiHistoryItem
	selected int
	list     mygoui.ListState
	left     float32
	problem  string
}

func apiHistoryFile() (string, error) {
	file, err := preferenceFile()
	return filepath.Join(filepath.Dir(file), "relay-history.json"), err
}

func (app *workspaceWindow) showAPIHistory() {
	if app.modalOpen || app.win == nil || app.win.IsDestroyed() {
		return
	}
	filename, err := apiHistoryFile()
	history := &workspaceAPIHistory{app: app, filename: filename, selected: -1, left: apiHistoryMidpoint}
	if err == nil {
		history.items, err = apihistory.Load(filename)
	}
	if err != nil {
		history.problem = fmt.Sprintf("Could not load API history. %v", err)
	}
	app.apiHistory, app.modalOpen = history, true
	history.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "API History", Width: apiHistoryWidth, Height: apiHistoryHeight, UseContentSize: true,
		Parent: app.win, Modal: true, Hidden: true, DisableResize: true,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		Content: mygoui.View(func(c *mygoui.Context) { history.draw(c, history.win.Close) }),
	})
	history.win.OnClosed(func() {
		app.apiHistory, app.modalOpen = nil, false
		if !app.win.IsDestroyed() {
			app.win.Invalidate()
		}
	})
	history.win.Show()
	app.win.Invalidate()
}

func (app *workspaceWindow) recordAPIRun(item apiHistoryItem) {
	filename, err := apiHistoryFile()
	if err == nil {
		var items []apiHistoryItem
		items, err = apihistory.Record(filename, item)
		if err == nil {
			app.refreshAPIHistory(filename, items)
			return
		}
	}
	app.problem = fmt.Sprintf("Could not record API history. %v", err)
}

func (app *workspaceWindow) refreshAPIHistory(filename string, items []apiHistoryItem) {
	windows := []*workspaceWindow{app}
	if app.application != nil {
		windows = app.application.windows
	}
	for _, window := range windows {
		history := window.apiHistory
		if history == nil || history.filename != filename {
			continue
		}
		if history.selected >= 0 && history.selected < len(history.items) {
			id := history.items[history.selected].ID
			history.selected = slices.IndexFunc(items, func(item apiHistoryItem) bool { return item.ID == id })
		}
		history.items, history.problem = items, ""
		if len(items) == 0 {
			history.selected, history.left = -1, apiHistoryMidpoint
			history.list = mygoui.ListState{}
		}
		if history.win != nil && !history.win.IsDestroyed() {
			history.win.Invalidate()
		}
	}
}

func (h *workspaceAPIHistory) clear() {
	if err := apihistory.Clear(h.filename); err != nil {
		h.problem = fmt.Sprintf("Could not clear API history. %v", err)
		return
	}
	h.app.refreshAPIHistory(h.filename, nil)
}

func (h *workspaceAPIHistory) draw(c *mygoui.Context, done func()) {
	h.app.applyAppearance(c)
	theme := *c.Theme()
	theme.Background = theme.Surface
	listSurface := theme.Background
	codeFill := theme.Text.Alpha(0.06)
	listAccent := theme.Accent
	methodInk, errorInk := theme.Accent, theme.Danger
	if h.app.blackout && !c.Preferences().HighContrast {
		// EV-UI-165 Blackout surfaces, sampled in sRGB after matching the AX frames.
		theme.Background, theme.Surface, theme.Border = mygoui.Hex("#2a2a2d"), mygoui.Hex("#3a3a3d"), mygoui.Hex("#3f3f42")
		theme.Accent = mygoui.Hex("#4095fa")
		listSurface, codeFill, listAccent = mygoui.Hex("#2a2b2d"), mygoui.Hex("#333336"), mygoui.Hex("#2671cc")
		methodInk, errorInk = mygoui.Hex("#0091ff"), mygoui.Hex("#ff4246")
	}
	theme.Radius = 7.5
	c.SetTheme(&theme)
	c.OnWindowShortcut(0, mygoui.KeyEscape, done)
	c.OnWindowShortcut(0, mygoui.KeyEnter, done)
	mygoui.Column(c).Fill().Background(theme.Background).Children(func() {
		mygoui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 12).Gap(8).Children(func() {
			mygoui.Text(c, "API History").FontSize(13).FontWeight(700).Role(mygoui.RoleHeading).Level(1)
			mygoui.Spacer(c)
			if len(h.items) != 0 {
				mygoui.Button(c, "Clear All").Size(74.5, 24).Padding(0, 8).FontSize(13).
					BorderColor(mygoui.Transparent).OnClick(h.clear)
			}
			mygoui.PrimaryButton(c, "Done").Size(56, 24).Padding(0, 8).FontSize(13).OnClick(done)
		})
		mygoui.Divider(c).FillWidth()
		if h.problem != "" {
			mygoui.Text(c, h.problem).FillWidth().Padding(6, 12).Wrap().Role(mygoui.RoleAlert).TextColor(theme.Danger)
		}
		if len(h.items) == 0 {
			h.drawEmpty(c)
			return
		}
		mygoui.Split(c, &h.left, func() {
			listTheme := theme
			listTheme.Background, listTheme.Accent = listSurface, listAccent
			mygoui.Themed(c, &listTheme).Fill().Children(func() {
				h.drawList(c, methodInk, errorInk)
			})
		}, func() {
			h.drawDetails(c, codeFill, methodInk, errorInk)
		}).FillWidth().Grow(1).MinHeight(0).OnChange(func() {
			width, _ := c.Size()
			h.left = max(280, min(h.left, width-381))
		})
	})
}

func (h *workspaceAPIHistory) drawEmpty(c *mygoui.Context) {
	mygoui.Column(c).FillWidth().Grow(1).Center().Gap(8).Children(func() {
		chromeIconColor(c, "clock.arrow.circlepath", 28, c.Theme().TextMuted)
		mygoui.Text(c, "No API calls yet").FontSize(16).FontWeight(600).Role(mygoui.RoleHeading).Level(2)
		mygoui.Text(c, "Run an ```api block to see its request and response here.").FontSize(13).TextColor(c.Theme().TextMuted)
	})
}

func (h *workspaceAPIHistory) drawList(c *mygoui.Context, methodInk, errorInk mygoui.Color) {
	h.list.Key = func(row int) any { return h.items[row].ID }
	h.list.Selected = &h.selected
	h.list.Label = func(row int) string {
		item := h.items[row]
		return strings.Join([]string{item.Method, apiHistoryStatus(item.StatusCode), apiHistoryTime(item), item.URL}, ", ")
	}
	mygoui.List(c, &h.list, len(h.items), func(row int) {
		item := h.items[row]
		theme := c.Theme()
		secondary := theme.TextMuted
		if row == h.selected {
			secondary = theme.AccentText.Alpha(0.8)
		}
		mygoui.Column(c).FillWidth().Height(41).Padding(6).Gap(3).Children(func() {
			mygoui.Row(c).FillWidth().Height(13).Gap(6).Children(func() {
				drawAPIHistoryStatus(c, item, 11, methodInk, errorInk)
				mygoui.Spacer(c).MinWidth(0)
				mygoui.Text(c, apiHistoryTime(item)).FontSize(11).TextColor(secondary).SingleLine()
			})
			mygoui.Text(c, item.URL).FontSize(11).Height(13).TextColor(secondary).SingleLine().Ellipsis("…")
		})
	}).Fill().Padding(10).Label("API calls").OnChange(func() {
		// The detail view's minimum width is 380 points when a call is selected.
		if h.left == apiHistoryMidpoint && h.selected >= 0 {
			h.left = apiHistoryWidth - 381
		}
	})
}

func (h *workspaceAPIHistory) drawDetails(c *mygoui.Context, codeFill, methodInk, errorInk mygoui.Color) {
	if h.selected < 0 || h.selected >= len(h.items) {
		mygoui.Column(c).Fill().Center().Children(func() {
			mygoui.Text(c, "Select a call").FontSize(13).TextColor(c.Theme().TextMuted)
		})
		return
	}
	item := &h.items[h.selected]
	mygoui.Scroll(c.Key(item.ID)).Fill().Label("API call details").Children(func() {
		mygoui.Column(c).FillWidth().Padding(14).Gap(14).Children(func() {
			mygoui.Row(c).FillWidth().Height(15).Gap(8).Children(func() {
				drawAPIHistoryStatus(c, *item, 12, methodInk, errorInk)
				mygoui.Textf(c, "· %d ms · %s", item.Duration.Milliseconds(), liveapi.SizeLabel(item.Bytes)).
					FontSize(11).TextColor(c.Theme().TextMuted).SingleLine()
			})
			drawAPIHistorySection(c.Key("request"), "Request", item.Request, codeFill)
			if item.Sent != "" && item.Sent != item.Request {
				drawAPIHistorySection(c.Key("sent"), "Sent (resolved, secrets masked)", item.Sent, codeFill)
			}
			drawAPIHistorySection(c.Key("response"), "Response", item.Response, codeFill)
		})
	})
}

func drawAPIHistorySection(c *mygoui.Context, label, text string, fill mygoui.Color) {
	preview, shortened := apiTextPreview(text)
	if shortened {
		preview += "\n\n… Use Copy for the full text."
	}
	mygoui.Column(c).FillWidth().Gap(7).Role(mygoui.RoleGroup).Label(label).Children(func() {
		mygoui.Row(c).FillWidth().Height(16).Children(func() {
			mygoui.Text(c, label).FontSize(12).FontWeight(600).SingleLine()
			mygoui.Spacer(c)
			chromeButtonIcon(c, "doc.on.doc", "Copy", func() { c.Services().WriteClipboard(text) }, 13, c.Theme().TextMuted).
				Size(13, 16).Margin(0, 1.5, 0, 0).Radius(4).Description("Copy " + strings.ToLower(label))
		})
		mygoui.Column(c).FillWidth().Padding(8, 8, 9).Radius(6).Background(fill).Children(func() {
			mygoui.TextAreaBase(c, &preview).ReadOnly(true).Label(label).Font("monospace").FontSize(11.5).
				FixedLineHeight(13).FillWidth().MinHeight(25).Lines(1, 12).Padding(5.5, 8).TextColor(c.Theme().TextMuted)
		})
	})
}

// MyGo shapes entire paragraphs. Bound what the cards and history show on
// screen, including responses with one very long line.
func apiTextPreview(text string) (string, bool) {
	if len(text) <= apiTextPreviewLimit {
		return text, false
	}
	end := apiTextPreviewLimit
	for !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end], true
}

func drawAPIHistoryStatus(c *mygoui.Context, item apiHistoryItem, size float32, methodInk, errorInk mygoui.Color) {
	statusInk := errorInk
	if item.StatusCode >= 200 && item.StatusCode < 400 {
		statusInk = c.Theme().Success
	}
	mygoui.Text(c, item.Method).FontSize(size).FontWeight(600).TextColor(methodInk).SingleLine()
	mygoui.Text(c, apiHistoryStatus(item.StatusCode)).FontSize(size).FontWeight(600).TextColor(statusInk).SingleLine()
}

func apiHistoryStatus(code int) string {
	if code == 0 {
		return "error"
	}
	return strconv.Itoa(code)
}

func apiHistoryTime(item apiHistoryItem) string {
	return item.Timestamp.Local().Format("Jan 2 at 15:04")
}
