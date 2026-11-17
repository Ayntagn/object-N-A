package ui

import (
	"path"
	"strings"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/go-text/typesetting/segmenter"
)

// truncateFileTitle applies the file row's middle ellipsis to the visual
// title. The caller retains the original text as the button's AX name.
func truncateFileTitle(c *mygoui.Context, title string, width float32) string {
	fits := func(text string) bool {
		measured, _ := c.MeasureText(0, mygoui.Span{Text: text, Size: 13})
		return measured <= width
	}
	if fits(title) {
		return title
	}
	var segments segmenter.Segmenter
	if err := segments.InitWithString(title); err != nil {
		return title
	}
	ends := []int{0}
	for it := segments.GraphemeIterator(); it.Next(); {
		g := it.Grapheme()
		ends = append(ends, g.OffsetInBytes+g.LengthInBytes)
	}
	count := len(ends) - 1
	result := "…"
	for low, high := 1, count-1; low <= high; {
		kept := (low + high) / 2
		candidate := title[:ends[(kept+1)/2]] + "…" + title[ends[count-kept/2]:]
		if fits(candidate) {
			result, low = candidate, kept+1
		} else {
			high = kept - 1
		}
	}
	return result
}

// EV-UI-173/183/194. Files, Search and Backlinks share the title, path and icon layout.
func drawVaultFileRowContents(c *mygoui.Context, title, folder, symbol string, colors vaultSectionColors, edited bool) {
	mygoui.Row(c).Width(14).Shrink(0).Justify(mygoui.Center).AlignItems(mygoui.Center).Children(func() {
		// Match the visible size of a native 12 pt symbol in a 14 pt column.
		chromeIconColor(c, symbol, 16, colors.accent)
	})
	mygoui.Column(c).Grow(1).MinWidth(0).Gap(1).AlignItems(mygoui.Start).Children(func() {
		mygoui.Row(c).FillWidth().Gap(4).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, title).FontSize(13).FixedLineHeight(16).MinWidth(0).SingleLine().Ellipsis("…").
				TextColor(colors.headerText).Role(mygoui.RoleNone)
			if edited {
				mygoui.Box(c).Size(5, 5).Shrink(0).Radius(2.5).Background(mygoui.RGB(255, 204, 0).Alpha(.8))
			}
		})
		if folder != "." {
			mygoui.Text(c, folder).FontSize(11).FixedLineHeight(14).MinWidth(0).SingleLine().Ellipsis("…").
				TextColor(colors.secondary.Alpha(.5)).Role(mygoui.RoleNone)
		}
	})
}

func (app *workspaceWindow) fileRow(c *mygoui.Context, tree mygoui.TreeParts, entry workspace.Entry, colors vaultSectionColors, drawContents bool) {
	c.Key(entry.Path)
	item := tree.Item(nil)
	label := app.noteLabel(entry.Name)
	selected := app.storage == nil && entry.Path == app.document.Path
	inset := vaultTreeRowInset(item.Depth())
	row := item.Row.Role(mygoui.RoleButton).FillWidth().Height(vaultFileRowHeight).
		Padding(4, 12).PaddingStart(inset).Gap(8).AlignItems(mygoui.Center).
		Label(label).Tooltip(entry.Path).AccessSelected(selected)
	row.OnClick(func() { app.openVaultFileByPreference(entry.Path, row.ClickModifiers(), false) }).
		ContextMenu(func(m *mygoui.Menu) {
			app.vaultFileMenu(m, entry.Path)
			m.Separator()
			app.vaultSiblingOrderMenu(m, entry.Path)
		})
	app.vaultTreeDragRow(c, row, mygoui.Element{}, entry, inset)
	if !drawContents {
		return
	}
	if selected {
		opacity := float32(.22)
		switch app.appAppearance() {
		case appearanceCyber, appearanceBlackout, appearancePLATO:
			opacity = .12
		}
		accent := app.vaultFileRowAccent(c, colors)
		row.Background(accent.Alpha(opacity))
		colors.headerText = accent
	} else if row.Hovered() {
		row.Background(colors.secondary.Alpha(.08))
	}
	symbol, ink := app.vaultFileRowIcon(c, entry.Path, colors, selected)
	colors.accent = ink
	edited := app.vaultFileRowEdited(entry.Path)
	titleWidth := vaultPanelWidth - 8 - inset - 12 - 14 - 8
	if edited {
		titleWidth -= 9
	}
	title := truncateFileTitle(c, label, titleWidth)
	row.Children(func() {
		drawVaultFileRowContents(c, title, ".", symbol, colors, edited)
	})
}

func (app *workspaceWindow) vaultFileRowIcon(c *mygoui.Context, relative string, colors vaultSectionColors, selected bool) (string, mygoui.Color) {
	mode := app.appAppearance()
	highContrast := c.Preferences().HighContrast
	symbol, ink := "doc", colors.secondary
	switch strings.ToLower(path.Ext(relative)) {
	case ".md", ".markdown":
		if selected {
			return "doc.text", app.vaultFileRowAccent(c, colors)
		}
		return "doc.text", app.vaultMarkdownIconInk(c, colors)
	case ".txt", ".text", ".log", ".rtf":
		symbol, ink = "doc.plaintext", mygoui.RGB(199, 191, 179)
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".heic":
		symbol, ink = "photo", mygoui.RGB(140, 199, 255)
	case ".mp3", ".m4a", ".wav", ".aac", ".ogg", ".flac":
		symbol, ink = "waveform", mygoui.RGB(255, 184, 128)
	case ".mp4", ".mov", ".m4v", ".webm":
		symbol, ink = "film", mygoui.RGB(255, 153, 191)
	case ".pdf":
		symbol, ink = "doc.richtext", mygoui.RGB(255, 140, 140)
	case ".csv", ".tsv":
		symbol, ink = "tablecells", mygoui.RGB(153, 230, 153)
	case ".excalidraw":
		symbol, ink = "pencil.and.outline", mygoui.RGB(204, 173, 255)
	default:
		if workspace.IsSourceTextPath(relative) {
			symbol, ink = "chevron.left.forwardslash.chevron.right", mygoui.RGB(140, 230, 217)
		} else if !selected {
			return symbol, ink
		}
	}
	if selected {
		return symbol, app.vaultFileRowAccent(c, colors)
	}
	if highContrast {
		ink = c.Theme().Text
	} else if mode == appearanceLight {
		ink = ink.Mix(mygoui.RGB(0, 0, 0), .55)
	}
	return symbol, ink
}

func (app *workspaceWindow) vaultMarkdownIconInk(c *mygoui.Context, colors vaultSectionColors) mygoui.Color {
	if c.Preferences().HighContrast {
		return colors.accent
	}
	switch app.appAppearance() {
	case appearancePLATO:
		return mygoui.Hex("#ff8c1f")
	case appearanceLight:
		return mygoui.Hex("#b83b9e")
	default:
		return blackoutSourceHead
	}
}

func (app *workspaceWindow) vaultFileRowAccent(c *mygoui.Context, colors vaultSectionColors) mygoui.Color {
	if c.Preferences().HighContrast {
		return colors.accent
	}
	switch app.appAppearance() {
	case appearancePLATO:
		return mygoui.Hex("#ffc46b")
	case appearanceLight:
		return mygoui.Hex("#3a5ea3")
	case appearanceSystem:
		if !c.Theme().Dark {
			return mygoui.Hex("#3a5ea3")
		}
	}
	return blackoutTitleAccent
}
