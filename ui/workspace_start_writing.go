package ui

import (
	"fmt"
	"strings"
	"time"

	"cyberwriter/internal/core/templates"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

type startWritingCard struct {
	id, title, description, symbol string
	choose                         func()
}

var startWritingGeneral = [len(templates.General)]struct{ description, symbol string }{
	{"Project docs", "doc.text"},
	{"Agenda & notes", "person.3"},
	{"Article layout", "newspaper"},
	{"Tasks & milestones", "list.clipboard"},
	{"Version history", "clock.arrow.circlepath"},
}

var startWritingVault = [len(templates.Vault)]struct{ description, symbol string }{
	{"Wikilinks & tags", "link"},
	{"Attendees & actions", "person.2.circle"},
	{"Today's journal", "calendar"},
}

// EV-UI-315: the empty editor centers a 500-point group below its toolbar.
// The adaptive grid allocates 120-point minimum tracks with 10-point spacing
// and 44-point side insets. The observed button frames include their stroke.
func (app *workspaceWindow) drawStartWriting(c *mygoui.Context, paneWidth float32) {
	const gridInset, groupMaxWidth, minimumCardWidth, trackGap = 44, 500, 120, 10
	const cardGap, cardHeight = trackGap - 1, 89
	trackWidth := min(paneWidth, groupMaxWidth) - 2*gridInset
	if trackWidth <= 0 {
		return
	}
	columns := max(1, int((trackWidth+trackGap)/(minimumCardWidth+trackGap)))
	gridWidth := min(trackWidth+1, groupMaxWidth-2*gridInset+.5)
	cards := app.startWritingCards()
	theme := c.Theme()
	headingInk, secondaryInk := theme.Text.Alpha(.7), theme.TextMuted
	cardInk, accent, fill, edge := theme.Text, theme.Accent, theme.Surface, theme.Border
	blackout := app.blackout && !c.Preferences().HighContrast
	if blackout {
		headingInk, secondaryInk = mygoui.Hex("#a8a8aa"), mygoui.Hex("#68686d")
		cardInk, accent, fill, edge = mygoui.Hex(blackoutTextCSS), mygoui.Hex("#aeccff"), mygoui.Hex("#0c0c0e"), mygoui.Hex("#0f0f10")
	}
	mygoui.Scroll(c.Key("start writing")).FillWidth().Grow(1).MinHeight(0).PassThrough().Children(func() {
		mygoui.Column(c).FillWidth().Padding(44, 0, 24).AlignItems(mygoui.Center).PassThrough().Children(func() {
			mygoui.Text(c, "Start Writing").Height(21).FontSize(18).FontWeight(600).FixedLineHeight(21).
				TextColor(headingInk).PassThrough()
			mygoui.Text(c, "Choose a template or start with a blank document").FontSize(12).FixedLineHeight(15).
				TextColor(secondaryInk).TextAlign(mygoui.Center).MaxWidth(gridWidth).Wrap().Margin(16, 0, 15.5).PassThrough()
			mygoui.Grid(c).Width(gridWidth).Columns(columns).Gap(cardGap).PassThrough().Children(func() {
				for _, card := range cards {
					button := mygoui.ButtonBase(c.Key(card.id)).FillWidth().Height(cardHeight).Role(mygoui.RoleButton).
						Radius(8).FocusRing(true).Cursor(mygoui.CursorPointer).Disabled(app.modalOpen).
						Label(card.title + ", " + card.description).OnClick(card.choose)
					cardFill, cardEdge := fill, edge
					if button.Hovered() {
						cardFill, cardEdge = fill.Mix(accent, .1), accent.Alpha(.4)
						if blackout {
							cardFill, cardEdge = mygoui.Hex("#664463"), mygoui.Hex("#8b89b6")
						}
					}
					button.Background(cardFill).Border(1, cardEdge).Children(func() {
						drawStartWritingCardContent(c, card, accent, cardInk, secondaryInk)
					})
				}
			})
		})
	})
}

func drawStartWritingCardContent(c *mygoui.Context, card startWritingCard, accent, titleInk, detailInk mygoui.Color) {
	const iconRowHeight = 24
	fallback, iconSize := card.symbol, float32(27)
	switch card.symbol {
	case "person.3":
		iconSize = 44
	case "person.2.circle":
		fallback = "person.3"
	case "calendar":
		fallback = "clock"
	case "doc.badge.plus":
		fallback = "doc"
	}
	mygoui.Column(c).FillWidth().Padding(0, 8).Gap(6).AlignItems(mygoui.Center).PassThrough().Children(func() {
		mygoui.Box(c).Size(iconSize, iconRowHeight).PassThrough().Children(func() {
			symbol := sfsymbols.Symbol(c, card.symbol).Size(iconSize).Color(accent).Fallback(chromeIconFallbacks[fallback])
			if card.symbol != "person.3" {
				symbol.Natural()
			}
			symbol.Absolute().Left(0).Top((iconRowHeight - iconSize) / 2).Role(mygoui.RoleNone).PassThrough()
		})
		mygoui.Text(c, card.title).FillWidth().FixedLineHeight(14).TextAlign(mygoui.Center).FontSize(11).
			FontWeight(600).TextColor(titleInk).SingleLine().Ellipsis("…").Role(mygoui.RoleNone).PassThrough()
		mygoui.Text(c, card.description).FillWidth().FixedLineHeight(14).TextAlign(mygoui.Center).FontSize(11).
			TextColor(detailInk).SingleLine().Ellipsis("…").Role(mygoui.RoleNone).PassThrough()
	})
}

func (app *workspaceWindow) startWritingCards() []startWritingCard {
	cards := []startWritingCard{{"Blank", "Blank", "Empty document", "doc", app.startBlankWriting}}
	appendBuiltin := func(builtin templates.Builtin, title, description, symbol string) {
		cards = append(cards, startWritingCard{builtin.Label, title, description, symbol, func() {
			app.useStartWritingTemplate(builtin.Markdown(time.Now()))
		}})
	}
	for index, builtin := range templates.General {
		appearance := startWritingGeneral[index]
		appendBuiltin(builtin, builtin.Label, appearance.description, appearance.symbol)
	}
	if app.vault != nil {
		for index, builtin := range templates.Vault {
			appearance := startWritingVault[index]
			appendBuiltin(builtin, strings.TrimPrefix(builtin.Label, "Vault "), appearance.description, appearance.symbol)
		}
	}
	for _, entry := range app.templates.entries {
		cards = append(cards, startWritingCard{"Custom " + entry.Filename, entry.Title, "Custom", "doc.badge.plus", func() {
			app.useCustomStartWritingTemplate(entry)
		}})
	}
	return cards
}

func (app *workspaceWindow) useCustomStartWritingTemplate(entry templates.Entry) {
	dir, err := templateDirectory()
	if err == nil {
		var markdown string
		markdown, err = templates.Load(dir, entry.Filename)
		if err == nil {
			app.useStartWritingTemplate(markdown)
			return
		}
	}
	app.problem = fmt.Sprintf("Could not open template %q. %v", entry.Title, err)
	app.refreshTemplateLibrary()
}
