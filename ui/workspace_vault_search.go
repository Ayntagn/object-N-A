package ui

import (
	"context"
	"fmt"
	"path"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

const vaultSearchDebounce = 180 * time.Millisecond

type vaultSidebarSearch struct {
	cancel                 context.CancelFunc
	results                []vaultindex.SidebarSearchHit
	related                *vaultRelatedSearch
	presentedRelated       vaultRelatedPresentation
	relatedRefreshDeferred bool
}

func (app *workspaceWindow) vaultSearchField(c *mygoui.Context) {
	theme := c.Theme()
	var input mygoui.Element
	field := mygoui.Row(c).FillWidth().Height(30).Padding(0, 7, 0, 8).Gap(5).AlignItems(mygoui.Center).
		Background(theme.Surface).Border(1, theme.Border).Radius(theme.Radius).Disabled(app.modalOpen).
		Cursor(mygoui.CursorText).OnClick(func() { input.Focus() })
	if app.blackout {
		field.Background(blackoutSearchFill).BorderColor(blackoutSidebarEdge).Radius(7)
	}
	field.Children(func() {
		mygoui.Row(c).Width(12.5).Shrink(0).Center().Children(func() {
			chromeIconColor(c, "magnifyingglass", 14, theme.TextMuted)
		})
		input = mygoui.TextInputBase(c, &app.fileSearch).Search().Grow(1).MinWidth(0).Height(16).
			Label("Search Vault files").Placeholder("Search files, tags, content, or concepts...").
			OnChange(app.vaultSearchChanged).
			OnTextKey(func(key mygoui.TextKey) bool { return key.Mods == 0 && key.Key == mygoui.KeyEscape })
		if app.focusVaultSearch {
			input.Focus()
			app.focusVaultSearch = false
		}
		if app.fileSearch != "" {
			mygoui.ButtonBase(c).Size(11, 16).Label("Clear").KeepFocus().TabStop(false).
				OnClick(func() {
					app.fileSearch = ""
					app.focusVaultSearch = true
					app.vaultSearchChanged()
				}).Children(func() { chromeIconColor(c, "xmark.circle.fill", 13, theme.TextMuted) })
		}
	})
	// EV-UI-327. Blackout keeps the same neutral stroke when search acquires focus.
	focused := input.Focused()
	if focused && (!app.blackout || c.Preferences().HighContrast) {
		field.BorderColor(theme.Accent)
	} else if !focused && field.Hovered() {
		field.BorderColor(theme.Border.Mix(theme.Text, .25))
	}
}

func (app *workspaceWindow) clearVaultSidebarSearch() {
	if app.sidebarSearch.cancel != nil {
		app.sidebarSearch.cancel()
	}
	app.sidebarSearch.related.stop()
	app.sidebarSearch = vaultSidebarSearch{}
}

// VaultSidebarView owns the input and chip. The already published Graph
// emphasis belongs to the Vault and survives a Files view being removed.
func (app *workspaceWindow) leaveVaultFiles() {
	app.fileSearch = ""
	app.focusVaultSearch = false
	app.graph.tag = ""
	app.clearVaultSidebarSearch()
}

func (app *workspaceWindow) vaultSearchChanged() {
	app.sidebarSearch.relatedRefreshDeferred = false
	input := app.fileSearch
	query := vaultindex.TrimSidebarSearchQuery(input)
	if query == "" || app.vault == nil {
		app.clearVaultSidebarSearch()
		app.publishVaultSearchGraph()
		return
	}
	// EV-UI-313: cancel the old task while keeping its published rows.
	if cancel := app.sidebarSearch.cancel; cancel != nil {
		cancel()
		app.sidebarSearch.cancel = nil
	}
	// The query captures the published file and label snapshots at input time
	// and remains visible across later index publications (EV-UI-188/189).
	vault, entries, index, win := app.vault, app.entries, app.index.data, app.win
	app.startVaultRelatedSearch(input, query)
	app.publishVaultSearchGraph()
	if win == nil {
		app.publishVaultSearch(index.SearchSidebar(context.Background(), vault, entries, query, ""))
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.sidebarSearch.cancel = cancel
	go func() {
		timer := time.NewTimer(vaultSearchDebounce)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		results := index.SearchSidebar(ctx, vault, entries, query, "")
		if ctx.Err() != nil {
			return
		}
		win.Update(func() {
			defer cancel()
			if ctx.Err() != nil || win.IsDestroyed() || app.vault != vault || app.fileSearch != input {
				return
			}
			app.publishVaultSearch(results)
		})
	}()
}

// EV-UI-187. All matching paths are significant, including attachments
// that have no node to highlight in either Graph canvas.
func (app *workspaceWindow) publishVaultSearch(results []vaultindex.SidebarSearchHit) {
	app.sidebarSearch.results, app.sidebarSearch.cancel = results, nil
	app.publishVaultSearchGraph()
}

func (app *workspaceWindow) searchVault(c *mygoui.Context, tagged []workspace.Entry) {
	state := &app.sidebarSearch
	length := vaultindex.SidebarSearchQueryLength(app.fileSearch)
	tree := mygoui.TreeBase(c)
	colors := app.vaultSectionColors(c)
	tree.Root.Role(mygoui.RoleList).Label("Vault files").Disabled(app.modalOpen).Children(func() {
		if app.fileSearch == "" {
			for _, entry := range tagged {
				app.searchResultRow(c, tree, vaultindex.SidebarSearchHit{Entry: entry}, colors)
			}
			return
		}
		if state.cancel != nil && length != 0 && len(state.results) == 0 {
			message := "Filtering by name…"
			if length >= 2 {
				message = fmt.Sprintf("Searching %d notes…", app.vaultFileCount)
			}
			mygoui.Text(c, message).Padding(6, 12).Role(mygoui.RoleStatus)
		}
		for _, result := range state.results {
			app.searchResultRow(c, tree, result, colors)
		}
		if length >= 3 {
			presentation := app.presentVaultRelatedSearch()
			if !presentation.buildHint {
				if len(presentation.matches) != 0 {
					app.drawVaultRelatedResults(c, presentation.matches, colors)
				}
				return
			}
			mygoui.Row(c.Key("concept search")).FillWidth().Padding(6, 12, 0).Gap(6).AlignItems(mygoui.Center).Children(func() {
				iconInk := app.vaultMarkdownIconInk(c, colors)
				mygoui.Row(c).Width(12).Shrink(0).Children(func() {
					chromeIconColor(c, "sparkles", 14, iconInk.Alpha(.7))
				})
				mygoui.Text(c, "Build vault embeddings in AI Settings to search by concept").
					Width(266.5).MinWidth(0).Shrink(1).FontSize(11).FixedLineHeight(14).
					TextColor(colors.secondary.Alpha(.7)).Wrap()
			})
		}
	})
}

// EV-UI-184/185. The summary is static text below the file button.
func (app *workspaceWindow) searchResultRow(c *mygoui.Context, tree mygoui.TreeParts, result vaultindex.SidebarSearchHit, colors vaultSectionColors) {
	entry := result.Entry
	title := app.noteLabel(entry.Name)
	folder := path.Dir(entry.Path)
	label, height := title, float32(24)
	if result.Snippet != "" {
		folder = "."
	}
	if folder != "." {
		label, height = title+", "+folder, 39
	}
	selected := app.storage == nil && entry.Path == app.document.Path
	c.Key(entry.Path)
	row := tree.Item(nil).Row.Role(mygoui.RoleButton).FillWidth().Height(height).
		Padding(4, 12).Gap(8).AlignItems(mygoui.Center).KeepFocus().
		Label(label).Tooltip(entry.Path).AccessSelected(selected)
	row.OnClick(func() { app.openVaultFileByPreference(entry.Path, row.ClickModifiers(), false) }).
		ContextMenu(func(menu *mygoui.Menu) { app.vaultFileMenu(menu, entry.Path) })
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
	// The shared title has an inner 12 pt margin at the trailing edge.
	titleWidth := float32(vaultPanelWidth - 8 - 36 - 14 - 8)
	if edited {
		titleWidth -= 9
	}
	title = truncateFileTitle(c, title, titleWidth)
	row.Children(func() {
		drawVaultFileRowContents(c, title, folder, symbol, colors, edited)
	})
	if result.Snippet != "" {
		width, _ := c.MeasureText(0, mygoui.Span{Text: result.Snippet, Size: 11, Font: "monospace"})
		mygoui.Text(c.Key("snippet:"+entry.Path), result.Snippet).KeepFocus().
			AlignSelf(mygoui.Start).Margin(0, 12, 4, 30).Width(min(width, 293)).Height(14).
			Font("monospace").FontSize(11).FixedLineHeight(14).SingleLine().Ellipsis("…").
			TextColor(colors.secondary.Alpha(.7))
	}
}
