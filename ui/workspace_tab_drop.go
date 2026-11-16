package ui

import (
	"fmt"
	"path/filepath"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-MYGO-262. A top-level drop reuses the Open Document loader.
// Vault rows keep their separate sibling drop targets.
func (app *workspaceWindow) tabDropTarget(c *mygoui.Context, zone mygoui.Element, radius float32) {
	if app.modalOpen {
		return
	}
	source, over := mygoui.DragOver[workspaceVaultTreeDrag](zone)
	if zone.FileDragOver() || over && app.canDropVaultNote(source) {
		ink := c.Theme().Accent
		if app.blackout && !c.Preferences().HighContrast {
			ink = blackoutTitleAccent
		}
		zone.DrawOver(func(p *mygoui.Painter, frame mygoui.Rect) {
			frame.X++
			frame.Y++
			frame.W = max(0, frame.W-2)
			frame.H = max(0, frame.H-2)
			p.Stroke(frame, ink, radius, 1.5)
		})
	}
	if source, dropped := mygoui.Drop[workspaceVaultTreeDrag](zone); dropped && app.canDropVaultNote(source) {
		app.openNote(source.relative, true)
	}
	if paths := zone.DroppedFiles(); len(paths) > 0 {
		app.openDroppedTabFiles(paths)
	}
}

func (app *workspaceWindow) canDropVaultNote(source workspaceVaultTreeDrag) bool {
	return app.vault != nil && source.root == app.root && workspace.IsMarkdownPath(source.relative)
}

func (app *workspaceWindow) openDroppedTabFiles(paths []string) {
	var firstProblem string
	for _, filename := range paths {
		app.problem = ""
		app.openDroppedTabFile(filename)
		if firstProblem == "" {
			firstProblem = app.problem
		}
	}
	app.problem = firstProblem
}

func (app *workspaceWindow) openDroppedTabFile(filename string) {
	if !filepath.IsAbs(filename) {
		app.problem = fmt.Sprintf("Could not open %q. Expected an absolute file path.", filename)
		return
	}
	filename = filepath.Clean(filename)
	markdown := workspace.IsMarkdownPath(filename)
	if app.multiFile.active && !markdown {
		app.problem = multiFileMarkdownOnly
		return
	}
	if !markdown && !workspace.IsSourceTextPath(filename) && !isDrawingPath(filename) &&
		!workspace.IsPDFPath(filename) && !workspace.IsRasterImagePath(filename) {
		app.problem = fmt.Sprintf("Could not open %q. Unsupported file type.", filepath.Base(filename))
		return
	}
	if tab := app.findDroppedTab(filename); tab != nil {
		app.activateTab(tab)
		return
	}
	app.openPickedDocument(filename, false)
}

func (app *workspaceWindow) findDroppedTab(filename string) *workspaceTab {
	for _, tab := range app.tabs {
		if app.absoluteDocument(tab) == filename {
			return tab
		}
	}
	// Finder can hand us a resolved path for a tab opened via an alias.
	// Resolve aliases for comparison with existing tabs only.
	resolved, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return nil
	}
	for _, tab := range app.tabs {
		if opened := app.absoluteDocument(tab); opened != "" {
			if alias, err := filepath.EvalSymlinks(opened); err == nil && alias == resolved {
				return tab
			}
		}
	}
	return nil
}
