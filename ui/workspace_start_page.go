package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (app *workspaceWindow) showsStartPage() bool {
	return app.draft != nil && app.previewDocumentType() == documentMarkdown && app.workingText() == ""
}

func (app *workspaceWindow) startBlankWriting() {
	app.useStartWritingTemplate("# ")
}

func (app *workspaceWindow) useStartWritingTemplate(markdown string) {
	if markdown == "" || !app.sourceEditing() || !app.showsStartPage() {
		return
	}
	app.draft.Edit(editor.Edit{Text: markdown})
	end := len(app.draft.String())
	app.code.Select(end, end)
	app.focusSource = true
	if app.document.Path == "" {
		app.startTitleFromSource = true
		app.workingText()
	}
}

func (app *workspaceWindow) canReplaceInitialTab() bool {
	return app.initialEmptyTab && app.showsStartPage() && !app.dirty()
}

func (app *workspaceWindow) startPageFilePicker() {
	origin := app.workspaceTab
	app.pickDocument(documentPickerTitle, documentPickerFilter, func(filename string) {
		app.openStartPageFile(origin, filename)
	})
}

// EV-UI-328/330: the welcome preview closes its origin after a successful
// replacement, including when the sidebar's opening policy is New window.
func (app *workspaceWindow) openStartPageFile(origin *workspaceTab, filename string) {
	filename = filepath.Clean(filename)
	if origin != app.workspaceTab || !app.showsStartPage() || !workspace.IsMarkdownPath(filename) {
		app.openPickedFile(filename)
		return
	}
	if filename == app.absoluteDocument(origin) {
		return
	}
	app.beforeLeavingTabs([]*workspaceTab{origin}, func() {
		var storage *workspace.Vault
		folder := ""
		vault, relative := app.vault, ""
		var err error
		if path, inside := app.relativeToVault(filename); inside {
			relative = path
		} else {
			folder, relative = filepath.Dir(filename), filepath.Base(filename)
			storage, err = app.vaultForFolder(folder)
			vault = storage
		}
		var doc workspace.Document
		if err == nil {
			doc, err = vault.ReadDocument(relative)
		}
		if err != nil {
			app.closeUnusedVault(storage)
			app.problem = fmt.Sprintf("Could not open %q. %v", filename, err)
			return
		}
		previous := origin.storage
		origin.storage, origin.folderRoot = storage, folder
		app.closeUnusedVault(previous)
		app.setTabDocument(origin, doc)
		app.activateTab(origin)
		app.closeStartupOrigin(origin)
		if app.win == nil || !app.win.IsDestroyed() {
			app.persistSession()
		}
	})
}

func (app *workspaceWindow) userGuideTitle() string {
	return applicationName + " " + app.tr("User Guide", "用户指南")
}

// EV-UI-319/328. The startup link supplies its origin for closing after a
// successful first open. The Help command uses the same document policy.
func (app *workspaceWindow) openUserGuide(mods mygoui.Modifiers, fromStartup bool) {
	if app.modalOpen {
		return
	}
	tab := app.workspaceTab
	if tab.userGuide && tab.document.Path == "" {
		return
	}
	separate := app.preferences.SidebarClickNewWindow || mods&mygoui.Cmd != 0
	if separate {
		if app.application == nil || app.win == nil {
			return
		}
		for _, window := range app.documentWindows() {
			if window.win == nil || window.win.IsDestroyed() {
				continue
			}
			for _, candidate := range window.tabs {
				if candidate.userGuide && candidate.document.Path == "" {
					if !window.modalOpen && window.workspaceTab != candidate {
						window.activateTab(candidate)
						window.win.Invalidate()
					}
					window.win.Focus()
					return
				}
			}
		}
		guide := app.blankTab()
		guide.mode = app.preferences.DefaultView
		app.setUserGuide(guide)
		next := app.application.newWindow(guide, app)
		next.win.Show()
		app.application.activate(next)
		next.persistSession()
		if fromStartup {
			app.closeStartupOrigin(tab)
		}
		return
	}
	app.beforeLeavingTabs([]*workspaceTab{tab}, func() {
		app.releaseTabStorage(tab)
		app.setUserGuide(tab)
		app.activateTab(tab)
		if fromStartup {
			app.closeStartupOrigin(tab)
		}
		if app.win == nil || !app.win.IsDestroyed() {
			app.persistSession()
		}
	})
}

func (app *workspaceWindow) closeStartupOrigin(tab *workspaceTab) {
	// With no other document, the macOS application menu stays available.
	if app.win != nil && (runtime.GOOS == "darwin" || len(app.tabs) > 1 || len(app.documentWindows()) > 1) {
		app.closeTab(tab)
	}
}

func (app *workspaceWindow) setUserGuide(tab *workspaceTab) {
	language := app.interfaceLanguage()
	title := app.userGuideTitle()
	var markdown strings.Builder
	fmt.Fprintf(&markdown, "# %s\n", title)
	for _, section := range featureGuideSections {
		fmt.Fprintf(&markdown, "\n## %s\n", language.text(section.title, section.titleChinese))
		for _, card := range section.cards {
			copy := card.localizedCopy(language)
			fmt.Fprintf(&markdown, "\n### %s\n\n%s\n", copy.title, copy.summary)
			if len(copy.hints) != 0 {
				markdown.WriteByte('\n')
				for _, hint := range copy.hints {
					fmt.Fprintf(&markdown, "- %s\n", shortcutLabel(hint))
				}
			}
		}
	}
	app.setTabDocument(tab, workspace.Document{Text: markdown.String()})
	tab.templateTitle = title
	tab.userGuide = true
}
