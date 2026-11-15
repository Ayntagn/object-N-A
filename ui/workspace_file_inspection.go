package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type workspaceLargePreview struct {
	previewUnits int
	renderAnyway bool
}

var inspectionCountPrinter = message.NewPrinter(language.English)

// EV-UI-175/176. The inspection result is a new document in the tab group.
func (app *workspaceWindow) inspectVaultFile(relative string, mode workspace.FileInspectionMode) {
	if app.modalOpen || app.vault == nil || app.win == nil {
		return
	}
	if app.multiFile.active {
		app.problem = multiFileMarkdownOnly
		return
	}
	vault, origin := app.vault, app.workspaceTab
	go func() {
		result, err := vault.InspectFile(relative, mode)
		app.win.Update(func() {
			if app.vault != vault || app.modalOpen || app.multiFile.active {
				return
			}
			if err != nil {
				app.problem = fmt.Sprintf("Could not inspect %q. %v", relative, err)
				return
			}
			tab := app.blankTab()
			tab.mode = splitMode
			app.setTabDocument(tab, workspace.Document{Text: result.Text})
			tab.inspection = true
			tab.largePreview.measure(result.Text)
			tab.templateTitle = result.Title
			tab.documentType = workspaceDocumentType{kind: documentPlainText, overridden: true, version: tab.textVersion}
			position := slices.Index(app.tabs, origin)
			if position < 0 {
				position = slices.Index(app.tabs, app.workspaceTab)
			}
			app.tabs = slices.Insert(app.tabs, position+1, tab)
			app.openFilesSidebar()
			app.fileSearch = ""
			app.vaultSearchChanged()
			app.pdfPreview.enabled = false
			app.focusMode = false
			app.activateTab(tab)
		})
	}()
}

func inspectionSaveFilename(text string) string {
	title := strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, workspace.FileInspectionTitle(text)))
	if title == "" {
		title = "Untitled"
	}
	if runtime.GOOS == "windows" && !filepath.IsLocal(title+".md") {
		title = "_" + title
	}
	return title + ".md"
}

func (tab *workspaceTab) inspectionTextForSave() string {
	// The editor normalizes CRLF for display. The untouched generated buffer
	// still has its exact input text, including mixed newline sequences.
	if !tab.draft.Modified() {
		return tab.document.Text
	}
	return tab.draft.String()
}

const largePreviewLimit = 2_000_000

func (state *workspaceLargePreview) measure(text string) {
	state.previewUnits = 0
	if len(text) > largePreviewLimit {
		for _, r := range text {
			state.previewUnits++
			if r > '\uffff' {
				state.previewUnits++
			}
		}
	}
}

func (app *workspaceWindow) largePreviewPaused(c *mygoui.Context) bool {
	state := &app.largePreview
	if state.renderAnyway || state.previewUnits <= largePreviewLimit {
		return false
	}
	shortcut := "Ctrl+1"
	if runtime.GOOS == "darwin" {
		shortcut = "⌘1"
	}
	mygoui.Column(c.Key("large preview paused")).Grow(1).FillWidth().MinHeight(0).
		AlignItems(mygoui.Center).Justify(mygoui.Center).Padding(16).Gap(12).Children(func() {
		mygoui.Text(c, "📄").FontSize(36)
		mygoui.Text(c, "Preview paused for this large file").FontSize(18).Bold().Role(mygoui.RoleHeading).Wrap()
		mygoui.Text(c, inspectionCountPrinter.Sprintf("This document is %d characters - past the %d-character preview limit. Rendering it live would stall the app.",
			state.previewUnits, largePreviewLimit)).MaxWidth(360).Wrap().TextAlign(mygoui.Center)
		mygoui.Text(c, "Press "+shortcut+" to switch to Editor mode and keep working at full speed.").
			FontSize(13).MaxWidth(360).Wrap().TextAlign(mygoui.Center)
		mygoui.Button(c, "Render anyway").Disabled(app.modalOpen).OnClick(func() {
			state.renderAnyway = true
			app.previewPane.refresh()
		})
		mygoui.Text(c, "May be slow or unresponsive for a while.").FontSize(11).TextColor(c.Theme().TextMuted)
	})
	return true
}
