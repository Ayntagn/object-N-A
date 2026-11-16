package ui

import (
	"path"

	mygoui "github.com/egoist/mygo/ui"
)

type workspaceVaultTreeDrag struct {
	root     string
	relative string
}

func (app *workspaceWindow) vaultTreeDragRow(c *mygoui.Context, row, subtree mygoui.Element, relative string, inset float32) {
	if app.vault == nil || app.modalOpen {
		return
	}
	root := app.root
	row.Drag(workspaceVaultTreeDrag{root: root, relative: relative})
	valid := func(source workspaceVaultTreeDrag) bool {
		return source.root == root && source.relative != relative && path.Dir(source.relative) == path.Dir(relative)
	}
	if source, dropped := mygoui.Drop[workspaceVaultTreeDrag](row); dropped && valid(source) {
		_, y, _ := row.PointerPosition()
		app.reorderVaultDrop(source.relative, relative, y >= row.Bounds().H/2)
		return
	}
	source, over := mygoui.DragOver[workspaceVaultTreeDrag](row)
	if !over || !valid(source) {
		return
	}
	_, y, _ := row.PointerPosition()
	after := y >= row.Bounds().H/2
	ink := mygoui.Hex("#428cf7")
	if c.Preferences().HighContrast {
		ink = c.Theme().Accent
	}
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
