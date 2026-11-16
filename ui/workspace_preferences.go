package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"cyberwriter/internal/core/mdshortcuts"
	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

type workspacePreferences struct {
	InterfaceLanguage           workspaceLanguage           `json:"interfaceLanguage,omitempty"`
	Appearance                  workspaceAppearance         `json:"appAppearance"`
	PreviewThemeID              string                      `json:"selectedThemeId"`
	UserStylesheetEnabled       bool                        `json:"userStylesheetEnabled,omitempty"`
	LineNumbers                 bool                        `json:"lineNumbers"`
	TypewriterMode              bool                        `json:"typewriterMode"`
	TypewriterPosition          float64                     `json:"typewriterPosition"`
	AutoIndentParagraphs        bool                        `json:"autoIndentParagraphs,omitempty"`
	MarkdownShortcuts           mdshortcuts.Config          `json:"markdownShortcuts,omitzero"`
	FontSize                    int                         `json:"fontSize"`
	EditorFont                  string                      `json:"editorFontFamily,omitempty"`
	EditorLineHeight            float64                     `json:"editorLineHeight,omitempty"`
	EditorTheme                 string                      `json:"editorThemeRawValue,omitempty"`
	DisableSyntaxHighlighting   bool                        `json:"debugDisableHighlighting,omitempty"`
	SpellCheckEnabled           bool                        `json:"spellCheckEnabled"`
	CollapseHexColors           bool                        `json:"collapseHexColors,omitempty"`
	PreferredInputDeviceUID     string                      `json:"preferredInputDeviceUID,omitempty"`
	AIStreamIntoEditor          bool                        `json:"aiStreamIntoEditor"`
	AIConnection                workspaceAISavedConnection  `json:"aiConnection,omitzero"`
	AISystemPrompt              *string                     `json:"aiSystemPrompt,omitempty"`
	AIProfiles                  []workspaceAIProfile        `json:"aiProfiles"`
	AIActiveProfile             string                      `json:"aiActiveProfile,omitempty"`
	AIContextLimitPreset        string                      `json:"aiContextLimitPreset,omitempty"`
	AICustomContextLimit        int                         `json:"aiCustomContextLimit,omitempty"`
	AIContextMessageLimit       int                         `json:"aiContextMessageLimit,omitempty"`
	AIChatRawText               bool                        `json:"aiChatRawText,omitempty"`
	AIVaultContextEnabled       bool                        `json:"aiVaultContextEnabled,omitempty"`
	AIVaultContextAutoUpdate    *bool                       `json:"aiVaultContextAutoUpdate,omitempty"`
	AIVaultContextSendStructure bool                        `json:"aiVaultContextSendStructure,omitempty"`
	AIVaultContextDepth         vaultcontext.Depth          `json:"aiVaultContextDepth,omitempty"`
	AIPanelRight                bool                        `json:"aiPanelOnRight,omitempty"`
	ShowWelcomeOnLaunch         bool                        `json:"showWelcomeOnLaunch,omitempty"`
	ShowSplashOnLaunch          bool                        `json:"showSplashOnLaunch"`
	SplashLastHostFrame         mygo.Rectangle              `json:"splashLastHostFrame,omitzero"`
	RecentDocuments             []string                    `json:"recentDocuments,omitempty"`
	ScrollSyncEnabled           bool                        `json:"scrollSyncEnabled"`
	UseLineBasedScrollSync      bool                        `json:"useLineBasedScrollSync"`
	ScrollPastEnd               bool                        `json:"scrollPastEnd"`
	PDFPreviewGridLayout        bool                        `json:"pdfPreviewGridLayout"`
	NumberFigures               bool                        `json:"numberFigures,omitempty"`
	ShowExtension               bool                        `json:"showExtension"`
	VaultShowAllFiles           bool                        `json:"vaultShowAllFiles,omitempty"`
	VaultFileOrders             map[string]vaultFileOrder   `json:"vaultFileOrders,omitempty"`
	SidebarClickNewWindow       bool                        `json:"sidebarClickNewWindow,omitempty"`
	DrawingsOpenSeparately      bool                        `json:"drawingsOpenSeparately"`
	NewInVault                  bool                        `json:"newInVault"`
	DefaultView                 int                         `json:"defaultView"`
	Graph3D                     bool                        `json:"graphInline3D,omitempty"`
	GraphClustered              bool                        `json:"graphClustered"`
	GraphColorNodesByTag        bool                        `json:"graphColorNodesByTag"`
	GraphShowStats              bool                        `json:"graphShowStats,omitempty"`
	GraphShape                  graphShape                  `json:"graphShape,omitempty"`
	TagsColoredByHash           bool                        `json:"tagsColoredByHash"`
	VaultTagsExpanded           bool                        `json:"vaultTagsExpanded,omitempty"`
	VaultBacklinksExpanded      bool                        `json:"vaultBacklinksExpanded,omitempty"`
	VaultMentionsExpanded       bool                        `json:"vaultMentionsExpanded,omitempty"`
	Terminal                    glowPreferences             `json:"terminal"`
	WindowStates                map[string]mygo.WindowState `json:"windowStates,omitempty"`
	Export                      workspaceExportPreferences  `json:"export"`
	RecentVaults                []string                    `json:"recentVaults,omitempty"`
	APIAllowedHosts             []string                    `json:"apiAllowedHosts,omitempty"`
	Session                     savedSession                `json:"session"`

	VaultEmbeddingsOfferDecisions map[string]string `json:"vaultEmbeddingsOfferDecisions,omitempty"`
}

type savedSession struct {
	Vault string `json:"vault,omitempty"`
	savedWindow
	OtherWindows []savedWindow `json:"otherWindows,omitempty"`
}

type savedWindow struct {
	Notes       []savedTab `json:"notes,omitempty"`
	Active      string     `json:"active,omitempty"`
	ActiveRoot  string     `json:"activeRoot,omitempty"`
	ActiveTab   int        `json:"activeTab,omitempty"` // One-based position in Notes.
	Recent      []string   `json:"recent,omitempty"`
	PreviewZoom int        `json:"previewZoomLevel,omitempty"`
	GlowHost    glowHost   `json:"glowHost,omitempty"`
}

type savedTab struct {
	Path string `json:"path"`
	Root string `json:"root,omitempty"`
	Mode int    `json:"mode"`
}

func preferenceFile() (string, error) {
	if dir := os.Getenv("CYBERWRITER_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json"), nil
	}
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "CyberWriter Workspace", "settings.json"), err
}

func loadWorkspacePreferences() (workspacePreferences, error) {
	defaults := workspacePreferences{
		Appearance: appearanceBlackout, PreviewThemeID: systemPreviewThemeID,
		DefaultView: splitMode, EditorTheme: systemSourceThemeID, TypewriterPosition: 0.5,
		ScrollSyncEnabled: true, ScrollPastEnd: true, AIStreamIntoEditor: true,
		SpellCheckEnabled:      true,
		DrawingsOpenSeparately: true,
		ShowSplashOnLaunch:     true,
		GraphClustered:         true, GraphShape: graphAuto, GraphColorNodesByTag: true, TagsColoredByHash: true,
		Terminal: (glowPreferences{}).normalized(),
		Export:   workspaceExportPreferences{RememberLocation: true},
	}
	file, err := preferenceFile()
	if err != nil {
		return defaults, err
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	preferences := defaults
	preferences.Appearance = ""
	legacy := struct {
		*workspacePreferences
		Appearance mygo.ThemeSource `json:"appearance"`
		Blackout   bool             `json:"blackout"`
	}{workspacePreferences: &preferences}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return defaults, err
	}
	switch preferences.Appearance {
	case appearanceCyber, appearanceBlackout, appearancePLATO, appearanceLight, appearanceDark, appearanceSystem:
	case "":
		switch {
		case legacy.Blackout:
			preferences.Appearance = appearanceBlackout
		case legacy.Appearance == mygo.ThemeLight:
			preferences.Appearance = appearanceLight
		case legacy.Appearance == mygo.ThemeDark:
			preferences.Appearance = appearanceDark
		case legacy.Appearance == mygo.ThemeSystem:
			preferences.Appearance = appearanceSystem
		default:
			preferences.Appearance = defaults.Appearance
		}
	default:
		preferences.Appearance = defaults.Appearance
	}
	preferences.PreviewThemeID = normalizedPreviewThemeID(preferences.PreviewThemeID)
	if preferences.DefaultView < sourceMode || preferences.DefaultView > previewMode {
		preferences.DefaultView = splitMode
	}
	if preferences.FontSize != 0 {
		preferences.FontSize = min(sourceFontMax, max(sourceFontMin, preferences.FontSize))
	}
	preferences.EditorTheme = sourceThemeForID(preferences.EditorTheme).id
	preferences.GraphShape = preferences.GraphShape.choice().id
	preferences.Terminal = preferences.Terminal.normalized()
	preferences.migrateAIProfiles()
	return preferences, nil
}

func (app *workspaceWindow) writePreferences() {
	if err := app.preferences.write(); err != nil {
		app.problem = fmt.Sprintf("Could not save workspace settings. %v", err)
	}
}

func (a *workspaceApplication) LoadWindowStates() (map[string]mygo.WindowState, error) {
	return maps.Clone(a.preferences.WindowStates), nil
}

func (a *workspaceApplication) SaveWindowStates(states map[string]mygo.WindowState) error {
	a.preferences.WindowStates = states
	return a.preferences.write()
}

func (p workspacePreferences) write() error {
	file, err := preferenceFile()
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	pending, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(pending.Name())
	writeErr := json.NewEncoder(pending).Encode(p)
	if err := errors.Join(writeErr, pending.Close()); err != nil {
		return err
	}
	return os.Rename(pending.Name(), file)
}

func (app *workspaceWindow) persistSession() {
	app.preferences.Session = app.sessionSnapshot()
	app.writePreferences()
}

func (app *workspaceWindow) sessionSnapshot() savedSession {
	if app.application != nil {
		if active := app.application.activeWorkspace(); active != nil {
			app = active
		}
	}
	session := savedSession{Vault: app.root, savedWindow: app.windowSnapshot()}
	for _, window := range app.documentWindows() {
		if window != app {
			saved := window.windowSnapshot()
			if len(saved.Notes) != 0 {
				session.OtherWindows = append(session.OtherWindows, saved)
			}
		}
	}
	return session
}

func (app *workspaceWindow) windowSnapshot() savedWindow {
	session := savedWindow{Recent: slices.Clone(app.recentPaths), PreviewZoom: app.previewZoom}
	if app.glow.shown {
		session.GlowHost = app.glow.host
	}
	for _, tab := range app.tabs {
		if tab.document.Path == "" {
			continue
		}
		saved := savedTab{Path: tab.document.Path, Root: tab.folderRoot, Mode: tab.mode}
		session.Notes = append(session.Notes, saved)
		if tab == app.workspaceTab {
			session.Active, session.ActiveRoot = saved.Path, saved.Root
			session.ActiveTab = len(session.Notes)
		}
	}
	return session
}

func (app *workspaceWindow) restoreWindow(session savedWindow) {
	app.previewZoom = normalizedPreviewZoom(session.PreviewZoom)
	var tabs []*workspaceTab
	var selected *workspaceTab
	storages := map[string]*workspace.Vault{"": app.vault, app.root: app.vault}
	for i, saved := range session.Notes {
		storage := storages[saved.Root]
		if storage == nil {
			var err error
			storage, err = app.vaultForFolder(saved.Root)
			if err != nil {
				continue
			}
		}
		tab := &workspaceTab{mode: app.preferences.DefaultView}
		if saved.Mode >= sourceMode && saved.Mode <= wysiwygMode {
			tab.mode = saved.Mode
		}
		var err error
		if isDrawingPath(saved.Path) {
			var draw *workspaceDrawing
			draw, err = loadWorkspaceDrawing(storage, saved.Path)
			if err == nil {
				app.setTabDrawing(tab, draw)
			}
		} else {
			var doc workspace.Document
			doc, err = storage.ReadTextDocument(saved.Path)
			if err == nil {
				app.setTabDocument(tab, doc)
			}
		}
		if err != nil {
			if storages[saved.Root] == nil {
				app.closeUnusedVault(storage)
			}
			continue
		}
		storages[saved.Root] = storage
		if storage != app.vault {
			tab.storage, tab.folderRoot = storage, saved.Root
		}
		tabs = append(tabs, tab)
		if i+1 == session.ActiveTab || (session.ActiveTab == 0 && saved.Path == session.Active && saved.Root == session.ActiveRoot) {
			selected = tab
		}
	}
	app.recentPaths = slices.Clone(session.Recent)
	app.updateEntries(app.entries)
	if len(tabs) == 0 {
		return
	}
	app.tabs = tabs
	if selected == nil {
		selected = tabs[0]
	}
	app.activateTab(selected)
	if session.GlowHost == glowDock || session.GlowHost == glowShade {
		app.showEmbeddedGlow(session.GlowHost)
	}
}

func (app *workspaceWindow) preferencesChanged() {
	app.writePreferences()
	for _, window := range app.documentWindows() {
		if window.win != nil && !window.win.IsDestroyed() {
			window.win.Invalidate()
		}
		if window.settings != nil && !window.settings.IsDestroyed() {
			window.settings.Invalidate()
		}
		window.invalidateGraphWindow()
	}
	if app.application != nil && app.application.glow.pane != nil {
		app.application.glow.pane.refresh(nil)
	}
}
