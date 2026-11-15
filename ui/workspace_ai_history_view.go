package ui

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-250. The native popover adds 13 pt around the client area.
const aiHistoryWidth, aiHistoryHeight = 330, 350

type workspaceAIHistoryRow struct {
	ID, Document, Title, Model string
	UpdatedAt                  time.Time
	Active                     bool
	Pinned                     bool
}

type aiHistoryActions struct {
	selectRow func(string)
	setPinned func(string, bool)
	deleteRow func(string)
	newChat   func()
}

type aiHistoryColors struct {
	workspaceAIColors
	surface, rule, badge, selected, marker, pin mygoui.Color
}

func (s *workspaceAI) drawConversationHistory(c *mygoui.Context, blackout bool, rows []workspaceAIHistoryRow, actions aiHistoryActions) mygoui.Element {
	theme := c.Theme()
	colors := aiHistoryColors{
		workspaceAIColors: aiWorkspaceColors(c, blackout),
		surface:           theme.Surface, rule: theme.Border, badge: theme.SurfaceHover,
		selected: theme.Accent.Alpha(0.16), marker: theme.Accent, pin: theme.TextMuted,
	}
	if blackout && !c.Preferences().HighContrast {
		colors.surface, colors.rule = mygoui.Hex("#222224"), mygoui.Hex("#404042")
		colors.badge, colors.selected = mygoui.Hex("#434448"), mygoui.Hex("#3a3f4a")
		colors.marker = mygoui.Hex("#879dc8")
		colors.pin = mygoui.Hex("#d0d2dd")
	}
	panel := mygoui.Column(c).Fill().MinHeight(0).Padding(13, 0, 12).
		DrawOver(func(p *mygoui.Painter, r mygoui.Rect) {
			p.Fill(mygoui.Rect{X: r.X, Y: r.Y + 42, W: r.W, H: 1}, colors.rule, 0)
			p.Fill(mygoui.Rect{X: r.X, Y: r.Y + r.H - 39, W: r.W, H: 1}, colors.rule, 0)
		})
	if !c.Vibrancy() {
		panel.Background(colors.surface).Radius(16)
	}
	return panel.Children(func() {
		mygoui.Row(c).FillWidth().Height(16).Shrink(0).Padding(0, 12).
			AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, "Conversations").FontSize(13).FixedLineHeight(16).FontWeight(600).TextColor(colors.text)
			mygoui.Spacer(c)
			mygoui.Row(c).Height(18).MinWidth(20).Padding(0, 6).Radius(9).Background(colors.badge).Center().Children(func() {
				mygoui.Text(c, strconv.Itoa(len(rows))).FontSize(11).FixedLineHeight(14).TextColor(colors.text)
			})
		})
		mygoui.Scroll(c.Key("AI history scroll")).FillWidth().MinHeight(0).Grow(1).Margin(14, 0, 0).
			Label("Conversations list").Children(func() {
			mygoui.Column(c).FillWidth().Padding(4, 0, 0).Gap(2).Role(mygoui.RoleList).Children(func() {
				for _, row := range rows {
					drawAIHistoryRow(c, colors, row, actions)
				}
			})
		})
		mygoui.Row(c).FillWidth().Height(14).Shrink(0).Margin(13, 0, 0).Padding(0, 13).Children(func() {
			mygoui.ButtonBase(c.Key("AI history new chat")).Size(72.5, 14).Gap(8).Label("New Chat").
				Disabled(actions.newChat == nil).OnClick(actions.newChat).Children(func() {
				aiWorkspaceIcon(c, "plus.message", "bubble.left.and.text.bubble.right", 13, colors.muted)
				mygoui.Text(c, "New Chat").FontSize(11).TextColor(colors.muted).Role(mygoui.RoleNone)
			})
		})
	})
}

func drawAIHistoryRow(c *mygoui.Context, colors aiHistoryColors, row workspaceAIHistoryRow, actions aiHistoryActions) {
	var details []string
	if !row.UpdatedAt.IsZero() {
		details = append(details, row.UpdatedAt.Local().Format("15:04"))
	}
	if row.Model != "" {
		details = append(details, row.Model)
	}
	label := row.Title
	height := float32(45)
	if row.Document != "" {
		label = row.Document + ", " + label
		height = 60
	}
	if len(details) != 0 {
		label += ", " + strings.Join(details, ", ·, ")
	}
	button := mygoui.ButtonBase(c.Key(row.ID)).FillWidth().Height(height).MinWidth(0).
		Padding(0, 12).Gap(8).Justify(mygoui.Start).
		Label(label).AccessSelected(row.Active).Disabled(actions.selectRow == nil)
	if actions.selectRow != nil {
		button.OnClick(func() { actions.selectRow(row.ID) })
	}
	if actions.setPinned != nil || actions.deleteRow != nil {
		button.ContextMenu(func(m *mygoui.Menu) { drawAIHistoryRowMenu(m, row, actions) })
	}
	hovered := button.Hovered()
	if row.Active {
		button.Background(colors.selected)
	} else if hovered {
		button.Background(c.Theme().SurfaceHover)
	}
	button.Children(func() {
		mygoui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(colors.marker)
		mygoui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			if row.Document != "" {
				mygoui.Row(c).FillWidth().Height(12).Gap(3).AlignItems(mygoui.Center).Children(func() {
					aiWorkspaceIcon(c, "doc.text", "", 9, colors.marker)
					mygoui.Text(c, row.Document).Grow(1).MinWidth(0).FontSize(11).FixedLineHeight(13).FontWeight(500).
						SingleLine().Ellipsis("…").TextColor(colors.marker).Role(mygoui.RoleNone)
				})
			}
			mygoui.Text(c, row.Title).FontSize(13).FontWeight(600).FixedLineHeight(15).
				SingleLine().Ellipsis("…").TextColor(colors.text).Role(mygoui.RoleNone)
			if len(details) != 0 {
				mygoui.Text(c, strings.Join(details, " · ")).FontSize(11).FixedLineHeight(13).
					SingleLine().Ellipsis("…").TextColor(colors.muted).Role(mygoui.RoleNone)
			}
		})
		if hovered && actions.deleteRow != nil {
			ink := c.Theme().Danger
			if !c.Preferences().HighContrast {
				ink = ink.Alpha(.65)
			}
			mygoui.ButtonBase(c.Key("delete conversation")).Size(12, 20).Center().Shrink(0).
				Label("Delete conversation").Tooltip("Delete conversation").KeepFocus().TabStop(false).
				OnClick(func() { actions.deleteRow(row.ID) }).Children(func() {
				chromeIconColor(c, "trash", 13.5, ink)
			})
		} else if row.Pinned {
			sfsymbols.Symbol(c, "pin.fill").Size(11.5).Natural().Color(colors.pin).
				Fallback(chromeIconFallbacks["tag.fill"]).Rotate(45).Role(mygoui.RoleNone)
		}
	})
}

// EV-UI-268: each history row owns a two-action menu with native item symbols.
func drawAIHistoryRowMenu(menu *mygoui.Menu, row workspaceAIHistoryRow, actions aiHistoryActions) {
	icons := aiHistoryMenuIcons()
	label, symbol := "Pin", "pin"
	if row.Pinned {
		label, symbol = "Unpin", "pin.slash"
	}
	if menu.Item(label).Icon(icons[symbol]).Disabled(actions.setPinned == nil).Chosen() {
		actions.setPinned(row.ID, !row.Pinned)
	}
	menu.Separator()
	if menu.Item("Delete").Icon(icons["trash"]).Danger().Disabled(actions.deleteRow == nil).Chosen() {
		actions.deleteRow(row.ID)
	}
}

var aiHistoryMenuIcons = sync.OnceValue(func() map[string]mygoui.MenuIcon {
	icons := make(map[string]mygoui.MenuIcon)
	for _, symbol := range []string{"pin", "pin.slash", "trash"} {
		if data := nativeMenuSymbol(symbol); len(data) != 0 {
			if icon, err := mygo.NewMenuIcon(data, true); err == nil {
				icons[symbol] = icon
			}
		}
	}
	return icons
})
