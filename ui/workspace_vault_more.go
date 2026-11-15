package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

func (app *workspaceWindow) vaultMoreMenu(m *mygoui.Menu) {
	if m.Item("Vault syntax guide").Chosen() {
		app.vaultGuideOpen = true
	}
	if m.Item("Rescan vault").Disabled(app.vault == nil).Chosen() {
		app.refreshVault()
	}
	label := "Generate vault stats"
	if app.hasVaultStats() {
		label = "Regenerate vault stats"
	}
	if m.Item(label).Disabled(app.vault == nil || app.vaultStatsBuilding).Chosen() {
		app.generateVaultStats()
	}
	m.Separator()
	m.Submenu("File Order", app.vaultFileOrderMenu).Disabled(app.vault == nil)
}

func (app *workspaceWindow) installVaultStatsMenu() *mygo.MenuItem {
	app.vaultStatsMenuItem = &mygo.MenuItem{
		Label: "Generate Vault Stats", Disabled: true, Click: app.menuCommand(app.generateVaultStats),
	}
	return app.vaultStatsMenuItem
}

func (app *workspaceWindow) hasVaultStats() bool {
	return slices.ContainsFunc(app.entries, func(entry workspace.Entry) bool {
		return !entry.IsDir && entry.Path == workspace.VaultStatsFile
	})
}

func (app *workspaceWindow) updateVaultStatsMenu() {
	item := app.vaultStatsMenuItem
	if item == nil {
		return
	}
	label := "Generate Vault Stats"
	if app.hasVaultStats() {
		label = "Regenerate Vault Stats"
	}
	item.SetLabel(label)
	item.SetEnabled(app.vault != nil && !app.vaultStatsBuilding)
}

func (app *workspaceWindow) generateVaultStats() {
	if app.vault == nil || app.modalOpen || app.vaultStatsBuilding {
		return
	}
	app.vaultStatsBuilding = true
	app.updateVaultStatsMenu()
	vault, win, name := app.vault, app.win, filepath.Base(app.root)
	go func() {
		source, err := vaultindex.StatsReport(vault, name, time.Now())
		win.Update(func() {
			if app.vault != vault || win.IsDestroyed() {
				return
			}
			app.vaultStatsBuilding = false
			app.updateVaultStatsMenu()
			if err != nil {
				app.problem = fmt.Sprintf("Could not generate Vault stats. %v", err)
				return
			}
			app.openGeneratedVaultStats(source)
		})
	}()
}

func (app *workspaceWindow) openGeneratedVaultStats(source string) {
	app.wys.sync(app)
	tab := app.findVaultTab(workspace.VaultStatsFile)
	if tab != nil && tab.dirty() {
		app.problem = "Save or close the edited Vault Stats.md before regenerating it."
		return
	}
	vault := app.vault
	doc, err := vault.WriteVaultStats(source)
	if errors.Is(err, workspace.ErrUnmanagedVaultStats) {
		app.confirm("Replace Vault Stats.md?",
			"An existing note named Vault Stats.md has no generated report marker. Replace its contents with Vault statistics?",
			"Replace", func() {
				saved, saveErr := vault.SaveDocument(doc, source)
				app.finishVaultStats(saved, saveErr, tab)
			})
		return
	}
	app.finishVaultStats(doc, err, tab)
}

func (app *workspaceWindow) finishVaultStats(doc workspace.Document, err error, tab *workspaceTab) {
	if err != nil {
		app.problem = fmt.Sprintf("Could not write Vault stats. %v", err)
		return
	}
	app.refreshVault()
	if tab != nil {
		app.setTabDocument(tab, doc)
		app.activateTab(tab)
	} else {
		app.openDocument(doc.Path)
	}
}

type vaultGuideRow struct {
	symbol, sample, meaning string
}

var vaultGuideSections = []struct {
	title string
	rows  []vaultGuideRow
}{
	{"WIKILINKS", []vaultGuideRow{
		{"link", "[[Page Name]]", "link to another note"},
		{"link", "[[Page Name|Display Text]]", "link with custom label"},
		{"link", "[[Page Name#Section]]", "link to a heading"},
		{"link", "[[Folder/Page]]", "link with path"},
		{"link", "", "Copy a file’s Wikilink from the Vault menu, then paste it in the editor."},
	}},
	{"EMBEDS", []vaultGuideRow{
		{"photo", "![[image.png]]", "embed an image from the vault"},
		{"doc.text", "![[Note Name]]", "embed another note’s content"},
		{"photo", "![alt text](path/to/image.png)", "standard Markdown image"},
		{"photo", "", "Use a relative path to resolve an image in the Vault."},
	}},
	{"TAGS", []vaultGuideRow{
		{"number", "#tag", "inline tag in body text"},
		{"number", "#project/sub", "nested tag"},
		{"number", "", "YAML frontmatter: tags: [tag1, tag2]"},
		{"number", "", "YAML frontmatter: tags: tag1, tag2"},
		{"magnifyingglass", "", "Search file names, tags, and saved note content."},
	}},
	{"CALLOUT BLOCKS", []vaultGuideRow{
		{"exclamationmark.bubble", "> [!WARNING] Title", "mark a warning"},
		{"exclamationmark.bubble", "> [!NOTE]", "mark an aside"},
		{"exclamationmark.bubble", "> [!TIP]", "mark a tip"},
		{"exclamationmark.bubble", "> [!IMPORTANT]", "highlight an important point"},
		{"exclamationmark.bubble", "> [!CAUTION]", "mark a caution"},
	}},
	{"SUPPORTED IMAGE FORMATS", []vaultGuideRow{
		{"photo", "", "PNG, JPEG, GIF, WebP"},
		{"photo", "", "Images can be resolved from the Vault and displayed in preview."},
	}},
	{"GRAPH & BACKLINKS", []vaultGuideRow{
		{"point.3.connected.trianglepath.dotted", "", "Graph connects notes with Wikilinks."},
		{"link", "", "Backlinks list the notes that link to the current file."},
		{"arrow.up.left.and.arrow.down.right", "", "Open Graph in Window to explore the same Vault in a floating window."},
	}},
}

func (app *workspaceWindow) drawVaultGuide(c *mygoui.Context, anchor mygoui.Element) {
	mygoui.PopoverBase(c, anchor, &app.vaultGuideOpen, func(panel mygoui.Element) {
		fill, edge := c.Theme().Surface, c.Theme().Border
		if app.blackout {
			fill, edge = mygoui.Hex("#252426"), mygoui.Hex("#4b4a4c")
		}
		panel.Label("Vault & Wikilinks Syntax").AttachTo(anchor, mygoui.AnchorBottom, mygoui.AnchorTop).
			Top(-8).Margin(4, 0, 0).Padding(12).Border(1, mygoui.Transparent).
			Draw(func(p *mygoui.Painter, box mygoui.Rect) {
				body := mygoui.Rect{X: box.X + 13, Y: box.Y + 13, W: box.W - 26, H: box.H - 26}
				p.Shadow(body, 20, 0, 3, 9, 0, mygoui.RGB(0, 0, 0).Alpha(.3))
				p.Fill(body, fill, 20)
				p.Stroke(body, edge, 20, 1)
				center := box.X + box.W/2
				var arrow mygoui.Path
				arrow.MoveTo(center-14, body.Y+1).QuadTo(center-9, body.Y+1, center-4, box.Y+6).
					QuadTo(center, box.Y+2, center+4, box.Y+6).
					QuadTo(center+9, body.Y+1, center+14, body.Y+1).Close()
				p.FillPath(&arrow, fill)
			})
		_, windowHeight := c.Size()
		mygoui.Scroll(c).Width(360).Height(max(160, min(480, windowHeight-160))).Label("Vault syntax guide").
			Children(func() {
				mygoui.Column(c).FillWidth().Padding(16, 16, 12).Gap(16).Children(func() {
					mygoui.Row(c).Gap(10).AlignItems(mygoui.Center).Children(func() {
						chromeIconColor(c, "link", 14, c.Theme().Accent)
						mygoui.Text(c, "Vault & Wikilinks Syntax").FontSize(15).Bold().Role(mygoui.RoleHeading)
					})
					for _, section := range vaultGuideSections {
						mygoui.Column(c).FillWidth().Gap(8).Children(func() {
							mygoui.Text(c, section.title).FontSize(12).Bold().TextColor(c.Theme().TextMuted).
								Role(mygoui.RoleHeading)
							for _, row := range section.rows {
								mygoui.Row(c).FillWidth().Gap(10).AlignItems(mygoui.Start).Children(func() {
									chromeIconColor(c, row.symbol, 14, c.Theme().TextMuted)
									if row.sample == "" {
										mygoui.Text(c, row.meaning).Grow(1).MinWidth(0).FontSize(13).Wrap()
									} else {
										mygoui.RichText(c, mygoui.Span{Text: row.sample, Font: "monospace"}, mygoui.Span{Text: " - " + row.meaning}).
											Grow(1).MinWidth(0).FontSize(13).Wrap()
									}
								})
							}
						})
					}
					mygoui.Text(c, "Files auto-update on save").FontSize(11).TextColor(c.Theme().TextMuted)
				})
			})
	})
}
