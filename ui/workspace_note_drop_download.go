package ui

import (
	"context"
	"slices"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/transfer"
)

func (app *workspaceWindow) applyNoteDrop(tab *workspaceTab, data transfer.Data, prepare func() bool, insert func(string, bool)) {
	items := classifyNoteDrop(data)
	if len(items) == 0 {
		return
	}
	vault := app.noteDropVault(tab)
	path, draft := tab.document.Path, tab.draft
	win := app.win
	finish := func(downloaded map[string]downloadedNoteImage) {
		if win != nil && (win.IsDestroyed() || app.win != win) {
			return
		}
		if app.modalOpen || !slices.Contains(app.tabs, tab) || tab.document.Path != path ||
			tab.draft != draft || app.noteDropVault(tab) != vault || !prepare() {
			app.problem = app.tr("The drop location changed. Drop the image again.", "插入位置已变化，请重新拖入图片。")
			return
		}
		if source, images := app.noteDropItemsMarkdown(tab, items, downloaded); source != "" {
			insert(source, images)
		}
	}
	if !slices.ContainsFunc(items, func(item noteDropItem) bool { return item.remoteImage }) || vault == nil || path == "" {
		finish(nil)
		return
	}

	app.problem = app.tr("Downloading image…", "正在下载图片…")
	download := app.noteDropDownload
	if download == nil {
		download = workspace.DownloadImage
	}
	fetch := func() map[string]downloadedNoteImage {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		downloads := make(map[string]downloadedNoteImage)
		for _, item := range items {
			if !item.remoteImage {
				continue
			}
			if _, visited := downloads[item.address]; !visited {
				data, err := download(ctx, item.address)
				downloads[item.address] = downloadedNoteImage{data: data, err: err}
			}
		}
		return downloads
	}
	if win == nil {
		finish(fetch())
		return
	}
	go func() {
		downloaded := fetch()
		win.Update(func() { finish(downloaded) })
	}()
}

func (app *workspaceWindow) noteDropVault(tab *workspaceTab) *workspace.Vault {
	if tab.storage != nil {
		return tab.storage
	}
	return app.vault
}

// Match the unchanged side of the drop point after an edit during the download.
// If both sides changed, the original insertion point may no longer exist.
func rebaseNoteDropOffset(before, after string, offset int) (int, bool) {
	if strings.HasPrefix(after, before[:offset]) {
		return offset, true
	}
	if suffix := before[offset:]; strings.HasSuffix(after, suffix) {
		return len(after) - len(suffix), true
	}
	return 0, false
}
