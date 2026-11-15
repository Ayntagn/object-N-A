package ui

import (
	"bytes"
	"slices"
	"strconv"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/plugins/sfsymbols"
	mygoui "github.com/egoist/mygo/ui"
)

// Color the existing Lucide folder silhouette for the expanded non-AppKit row.
var vaultFolderFilledFallback = func() *mygoui.SVG {
	data, err := chromeIconFiles.ReadFile("chrome_icons/folder.svg")
	if err != nil {
		panic(err)
	}
	return mygoui.MustParseSVG(bytes.Replace(data, []byte(`fill="none"`), []byte(`fill="currentColor"`), 1))
}()

func vaultTreeRowInset(depth int) float32 {
	return 12 + 16*float32(depth)
}

// EV-UI-197. The tree visits folders first, with localized name order
// within each sibling group.
func orderedVaultTreeEntries(entries []workspace.Entry) []workspace.Entry {
	if len(entries) == 0 {
		return entries
	}
	var ordered []workspace.Entry
	withGraphNameComparer(func(compare func(string, string) int) {
		var order func([]workspace.Entry) []workspace.Entry
		order = func(entries []workspace.Entry) []workspace.Entry {
			rows := slices.Clone(entries)
			slices.SortStableFunc(rows, func(a, b workspace.Entry) int {
				if a.IsDir != b.IsDir {
					if a.IsDir {
						return -1
					}
					return 1
				}
				return compare(a.Name, b.Name)
			})
			for i := range rows {
				if rows[i].IsDir {
					rows[i].Children = order(rows[i].Children)
				}
			}
			return rows
		}
		ordered = order(entries)
	})
	return ordered
}

func (app *workspaceWindow) drawVaultFileTree(c *mygoui.Context) {
	tree := mygoui.TreeBase(c)
	colors := app.vaultSectionColors(c)
	tree.Root.Role(mygoui.RoleList).Label("Vault files").Disabled(app.modalOpen).Children(func() {
		app.fileRows(c, tree, ".", app.defaultVaultFileTree(), colors)
	})
}

func (app *workspaceWindow) fileRows(c *mygoui.Context, tree mygoui.TreeParts, folder string, entries []workspace.Entry, colors vaultSectionColors) {
	order := app.preferences.VaultFileOrders[app.root]
	for _, entry := range order.arrange(folder, entries) {
		if entry.IsDir {
			app.folderRow(c, tree, entry, colors)
		} else {
			app.fileRow(c, tree, entry, colors)
		}
	}
}

// EV-UI-194. The folder button has its own disclosure and folder symbols.
func (app *workspaceWindow) folderRow(c *mygoui.Context, tree mygoui.TreeParts, entry workspace.Entry, colors vaultSectionColors) {
	open := app.folder(entry.Path)
	c.Key(entry.Path)
	item := tree.Item(open)
	count := strconv.Itoa(countVaultFiles(entry.Children))
	row := item.Row.Role(mygoui.RoleButton).FillWidth().Height(24).
		Padding(4, 12).PaddingStart(vaultTreeRowInset(item.Depth())).Gap(4).AlignItems(mygoui.Center).
		Label(entry.Name + ", " + count).OnClick(func() { *open = !*open }).
		ContextMenu(func(m *mygoui.Menu) {
			if m.Item("New Document Here…").Chosen() {
				app.createNote(entry.Path, "")
			}
			if m.Item("New Folder…").Chosen() {
				app.createFolder(entry.Path, true)
			}
			m.Separator()
			app.vaultSiblingOrderMenu(m, entry.Path)
		})
	if row.Hovered() {
		row.Background(colors.secondary.Alpha(.06))
	}
	folderInk := app.vaultFileRowAccent(c, colors).Alpha(.7)
	arrowInk, countInk := colors.secondary.Alpha(.5), colors.secondary.Alpha(.4)
	if c.Preferences().HighContrast {
		folderInk, arrowInk, countInk = colors.accent, colors.headerText, colors.headerText
	}
	row.Children(func() {
		mygoui.Row(c).Width(10).Shrink(0).Center().Children(func() {
			arrow := sfsymbols.Symbol(c, "chevron.right").Size(14).Natural().Weight(sfsymbols.Semibold).
				Color(arrowInk).Fallback(chromeIconFallbacks["chevron.right"]).Role(mygoui.RoleNone)
			if *open {
				arrow.Rotate(90)
			}
		})
		symbol, fallback := "folder", chromeIconFallbacks["folder"]
		if *open {
			symbol, fallback = "folder.fill", vaultFolderFilledFallback
		}
		mygoui.Row(c).Width(14).Shrink(0).Center().Children(func() {
			sfsymbols.Symbol(c, symbol).Size(16).Natural().Color(folderInk).Fallback(fallback).Role(mygoui.RoleNone)
		})
		mygoui.Text(c, entry.Name).FontSize(13).FontWeight(500).FixedLineHeight(16).
			MinWidth(0).SingleLine().Ellipsis("…").TextColor(colors.headerText).Role(mygoui.RoleNone)
		mygoui.Spacer(c)
		mygoui.Text(c, count).Font("monospace").FontSize(11).FixedLineHeight(14).Shrink(0).
			TextColor(countInk).Role(mygoui.RoleNone)
	})
	item.Group(func() {
		app.fileRows(c, tree, entry.Path, entry.Children, colors)
	})
}
