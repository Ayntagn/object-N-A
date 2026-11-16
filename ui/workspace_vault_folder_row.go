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

const vaultFileRowHeight = 24

type vaultTreeRows struct {
	next, first, end int
}

// Keep the row nodes used for keyboard, accessibility and drag navigation,
// and build their text and symbols only near the visible part of the tree.
func (r *vaultTreeRows) drawContents() bool {
	index := r.next
	r.next++
	return index >= r.first && index < r.end
}

func (app *workspaceWindow) drawVaultFileTree(c *mygoui.Context, scroll mygoui.Element) {
	// Keep row identities in the same Vault as the active drag payload.
	tree := mygoui.TreeBase(c.Key(app.root))
	colors := app.vaultSectionColors(c)
	clip, bounds := scroll.Bounds(), tree.Root.Bounds()
	const overscan = 8
	var offset float32
	if clip.H <= 0 {
		_, clip.H = c.Size()
	}
	if bounds.H > 0 {
		offset = clip.Y - bounds.Y
	}
	rows := &vaultTreeRows{
		first: max(0, int(offset/vaultFileRowHeight)-overscan),
		end:   max(0, int(offset+clip.H)/vaultFileRowHeight) + overscan,
	}
	tree.Root.Role(mygoui.RoleList).Label("Vault files").Disabled(app.modalOpen).Children(func() {
		app.fileRows(c, tree, ".", app.defaultVaultFileTree(), colors, rows)
	})
}

func (app *workspaceWindow) fileRows(c *mygoui.Context, tree mygoui.TreeParts, folder string, entries []workspace.Entry, colors vaultSectionColors, rows *vaultTreeRows) {
	order := app.preferences.VaultFileOrders[app.root]
	for _, entry := range order.arrange(folder, entries) {
		if entry.IsDir {
			app.folderRow(c, tree, entry, colors, rows)
		} else {
			app.fileRow(c, tree, entry, colors, rows.drawContents())
		}
	}
}

// EV-UI-194. The folder button has its own disclosure and folder symbols.
func (app *workspaceWindow) folderRow(c *mygoui.Context, tree mygoui.TreeParts, entry workspace.Entry, colors vaultSectionColors, rows *vaultTreeRows) {
	open := app.folder(entry.Path)
	c.Key(entry.Path)
	item := tree.Item(open)
	count := strconv.Itoa(countVaultFiles(entry.Children))
	inset := vaultTreeRowInset(item.Depth())
	row := item.Row.Role(mygoui.RoleButton).FillWidth().Height(vaultFileRowHeight).
		Padding(4, 12).PaddingStart(inset).Gap(4).AlignItems(mygoui.Center).
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
	if rows.drawContents() {
		row.Children(func() { app.drawVaultFolderTitle(c, entry.Name, count, *open, colors) })
	}
	group := item.Group(func() {
		app.fileRows(c, tree, entry.Path, entry.Children, colors, rows)
	})
	if len(entry.Children) == 0 {
		group = mygoui.Element{}
	}
	app.vaultTreeDragRow(c, row, group, entry.Path, inset)
}

func (app *workspaceWindow) drawVaultFolderTitle(c *mygoui.Context, name, count string, open bool, colors vaultSectionColors) {
	folderInk := app.vaultFileRowAccent(c, colors).Alpha(.7)
	arrowInk, countInk := colors.secondary.Alpha(.5), colors.secondary.Alpha(.4)
	if c.Preferences().HighContrast {
		folderInk, arrowInk, countInk = colors.accent, colors.headerText, colors.headerText
	}
	mygoui.Row(c).Width(10).Shrink(0).Center().Children(func() {
		arrow := sfsymbols.Symbol(c, "chevron.right").Size(14).Natural().Weight(sfsymbols.Semibold).
			Color(arrowInk).Fallback(chromeIconFallbacks["chevron.right"]).Role(mygoui.RoleNone)
		if open {
			arrow.Rotate(90)
		}
	})
	symbol, fallback := "folder", chromeIconFallbacks["folder"]
	if open {
		symbol, fallback = "folder.fill", vaultFolderFilledFallback
	}
	mygoui.Row(c).Width(14).Shrink(0).Center().Children(func() {
		sfsymbols.Symbol(c, symbol).Size(16).Natural().Color(folderInk).Fallback(fallback).Role(mygoui.RoleNone)
	})
	mygoui.Text(c, name).FontSize(13).FontWeight(500).FixedLineHeight(16).
		MinWidth(0).SingleLine().Ellipsis("…").TextColor(colors.headerText).Role(mygoui.RoleNone)
	mygoui.Spacer(c)
	mygoui.Text(c, count).Font("monospace").FontSize(11).FixedLineHeight(14).Shrink(0).
		TextColor(countInk).Role(mygoui.RoleNone)
}
