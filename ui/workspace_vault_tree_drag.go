package ui

import (
	"fmt"
	"path"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceVaultTreeDrag struct {
	root     string
	relative string
	isDir    bool
}

func (app *workspaceWindow) vaultTreeDragRow(c *mygoui.Context, row, subtree mygoui.Element, entry workspace.Entry, inset float32) {
	if app.vault == nil || app.modalOpen {
		return
	}
	row.Drag(workspaceVaultTreeDrag{root: app.root, relative: entry.Path, isDir: entry.IsDir})
	source, dropped := vaultTreeDrop(row)
	if source.relative == "" || source.root != app.root || source.relative == entry.Path {
		return
	}
	_, y, _ := row.PointerPosition()
	height := row.Bounds().H
	sourceFolder := path.Dir(source.relative)
	siblings := sourceFolder == path.Dir(entry.Path)
	// The middle of a sibling folder admits a file, while its edges still reorder.
	center := y >= height/4 && y < 3*height/4
	if entry.IsDir && !source.isDir && sourceFolder != entry.Path && (!siblings || center) {
		app.vaultTreeFolderDrop(c, row, source.relative, entry.Path, dropped)
		return
	}
	if !siblings {
		return
	}
	after := y >= height/2
	if dropped {
		app.reorderVaultDrop(source.relative, entry.Path, after)
		return
	}
	ink := vaultTreeDropInk(c)
	guide := row
	if after && subtree != (mygoui.Element{}) {
		guide = subtree
	}
	guide.DrawOver(func(p *mygoui.Painter, frame mygoui.Rect) {
		y := frame.Y
		if after {
			y += frame.H - 1
		}
		p.Fill(mygoui.Rect{X: frame.X + inset, Y: y, W: max(0, frame.W-inset-12), H: 1}, ink, 0)
	})
}

func vaultTreeDrop(row mygoui.Element) (workspaceVaultTreeDrag, bool) {
	if source, dropped := mygoui.Drop[workspaceVaultTreeDrag](row); dropped {
		return source, true
	}
	source, _ := mygoui.DragOver[workspaceVaultTreeDrag](row)
	return source, false
}

func (app *workspaceWindow) vaultTreeRootDrop(c *mygoui.Context, heading mygoui.Element) {
	if app.vault == nil || app.modalOpen {
		return
	}
	source, dropped := vaultTreeDrop(heading)
	if source.relative != "" && source.root == app.root && !source.isDir && path.Dir(source.relative) != "." {
		app.vaultTreeFolderDrop(c, heading, source.relative, ".", dropped)
	}
}

func (app *workspaceWindow) vaultTreeFolderDrop(c *mygoui.Context, row mygoui.Element, source, folder string, dropped bool) {
	if dropped {
		if err := app.moveVaultFileToFolder(source, folder); err != nil {
			app.problem = fmt.Sprintf("Could not move %q. %v", source, err)
		}
		return
	}
	ink := vaultTreeDropInk(c)
	row.Background(ink.Alpha(.2)).DrawOver(func(p *mygoui.Painter, frame mygoui.Rect) {
		p.Stroke(mygoui.Rect{X: frame.X + 1, Y: frame.Y + 1, W: frame.W - 2, H: frame.H - 2}, ink, 3, 1)
	})
}

func vaultTreeDropInk(c *mygoui.Context) mygoui.Color {
	if c.Preferences().HighContrast {
		return c.Theme().Accent
	}
	return mygoui.Hex("#428cf7")
}
