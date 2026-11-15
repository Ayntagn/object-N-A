package ui

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

type vaultFileOrder struct {
	Enabled bool                `json:"enabled"`
	Folders map[string][]string `json:"folders,omitempty"`
}

// arrange overlays the saved names on the default sibling order for this folder.
func (o vaultFileOrder) arrange(folder string, entries []workspace.Entry) []workspace.Entry {
	names := o.Folders[folder]
	if !o.Enabled || len(names) == 0 || len(entries) < 2 {
		return entries
	}
	pending := make(map[string]workspace.Entry, len(entries))
	for _, entry := range entries {
		pending[entry.Name] = entry
	}
	ordered := make([]workspace.Entry, 0, len(entries))
	for _, name := range names {
		if entry, found := pending[name]; found {
			ordered = append(ordered, entry)
			delete(pending, name)
		}
	}
	for _, entry := range entries {
		if _, found := pending[entry.Name]; found {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

func (app *workspaceWindow) defaultVaultFileTree() []workspace.Entry {
	if app.vaultTreeEntries != nil {
		return app.vaultTreeEntries
	}
	return app.entries
}

func (app *workspaceWindow) vaultDefaultSiblings(folder string) []workspace.Entry {
	entries := app.defaultVaultFileTree()
	if folder == "." {
		return entries
	}
	for _, name := range strings.Split(folder, "/") {
		i := slices.IndexFunc(entries, func(entry workspace.Entry) bool { return entry.IsDir && entry.Name == name })
		if i == -1 {
			return nil
		}
		entries = entries[i].Children
	}
	return entries
}

func (app *workspaceWindow) vaultSiblingOrderMenu(m *mygoui.Menu, relative string) {
	folder := path.Dir(relative)
	order := app.preferences.VaultFileOrders[app.root]
	siblings := order.arrange(folder, app.vaultDefaultSiblings(folder))
	index := slices.IndexFunc(siblings, func(entry workspace.Entry) bool { return entry.Path == relative })
	m.Submenu("Sort Order", func(sortMenu *mygoui.Menu) {
		if sortMenu.Item("Move Up").Disabled(index < 1).Chosen() {
			app.reorderVaultSiblings(order, folder, siblings, index, index-1)
		}
		if sortMenu.Item("Move Down").Disabled(index == -1 || index+1 >= len(siblings)).Chosen() {
			app.reorderVaultSiblings(order, folder, siblings, index, index+1)
		}
	}).Disabled(app.vault == nil || index == -1 || len(siblings) < 2)
}

func (app *workspaceWindow) reorderVaultSiblings(order vaultFileOrder, folder string, siblings []workspace.Entry, from, to int) {
	names := make([]string, len(siblings))
	remaining := make(map[string]bool, len(siblings))
	for i, entry := range siblings {
		names[i] = entry.Name
		remaining[entry.Name] = true
	}
	names[from], names[to] = names[to], names[from]
	// Keep previously ranked names hidden by the current file type filter.
	var ranks []string
	for _, name := range order.Folders[folder] {
		if remaining[name] {
			delete(remaining, name)
			ranks = append(ranks, names[0])
			names = names[1:]
		} else {
			ranks = append(ranks, name)
		}
	}
	if order.Folders == nil {
		order.Folders = make(map[string][]string)
	} else {
		order.Folders = maps.Clone(order.Folders)
	}
	order.Folders[folder] = append(ranks, names...)
	order.Enabled = true
	app.saveVaultFileOrder(order)
}

func (app *workspaceWindow) vaultFileOrderMenu(m *mygoui.Menu) {
	order := app.preferences.VaultFileOrders[app.root]
	if m.Item("Use Custom Order").Checked(order.Enabled).Disabled(len(order.Folders) == 0).Chosen() {
		order.Enabled = !order.Enabled
		app.saveVaultFileOrder(order)
	}
	if m.Item("Reset Custom Orders").Disabled(len(order.Folders) == 0).Chosen() {
		app.saveVaultFileOrder(vaultFileOrder{})
	}
}

func (app *workspaceWindow) saveVaultFileOrder(order vaultFileOrder) {
	prefs := *app.preferences
	if prefs.VaultFileOrders == nil {
		prefs.VaultFileOrders = make(map[string]vaultFileOrder)
	} else {
		prefs.VaultFileOrders = maps.Clone(prefs.VaultFileOrders)
	}
	if len(order.Folders) == 0 {
		delete(prefs.VaultFileOrders, app.root)
	} else {
		prefs.VaultFileOrders[app.root] = order
	}
	if err := prefs.write(); err != nil {
		app.problem = fmt.Sprintf("Could not save file order. %v", err)
		return
	}
	app.preferences.VaultFileOrders = prefs.VaultFileOrders
	for _, win := range app.documentWindows() {
		if win.root == app.root && win.win != nil && !win.win.IsDestroyed() {
			win.win.Invalidate()
		}
	}
}
