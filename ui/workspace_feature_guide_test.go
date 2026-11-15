package ui

// spec: FEAT-FEATURE-GUIDE / EV-UI-122 / EV-UI-123

import (
	"slices"
	"testing"

	mygoui "github.com/egoist/mygo/ui"
)

func TestFeatureGuideSearch(t *testing.T) {
	guide := workspaceFeatureGuide{selected: "wikilinks"}
	guide.search("  ALIAS  ")
	if !slices.Equal(guide.matches, []string{"wikilinks", "wikibuilder"}) || guide.selected != "wikilinks" {
		t.Fatalf("alias results %v, selected %q", guide.matches, guide.selected)
	}
	guide.search("table")
	wantTables := []string{"headings", "tables", "figures", "pagebreak", "api", "query", "embeds",
		"custom-css", "toc", "slides", "smartpaste", "selectiontoolbar", "wysiwyg", "ai-panel"}
	if !slices.Equal(guide.matches, wantTables) || guide.selected != "headings" {
		t.Fatalf("table results %v, selected %q", guide.matches, guide.selected)
	}
	guide.move(1)
	if guide.selected != "tables" {
		t.Fatalf("Down should open Tables, got %q", guide.selected)
	}
	guide.search("PRESENT & EXPORT")
	want := []string{"export", "custom-css", "themes", "toc", "slides", "flashcards", "focus"}
	if !slices.Equal(guide.matches, want) || guide.selected != "export" {
		t.Fatalf("section results %v, selected %q", guide.matches, guide.selected)
	}
	guide.search("mermaid background")
	if !slices.Equal(guide.matches, []string{"mermaid-bg", "export"}) || guide.selected != "export" {
		t.Fatalf("multi-word results %v, selected %q", guide.matches, guide.selected)
	}
	guide.move(-1)
	guide.move(-1)
	if guide.selected != "mermaid-bg" {
		t.Fatalf("selection should stop at the first result, got %q", guide.selected)
	}
	guide.search("no-results-2026")
	guide.move(1)
	if len(guide.matches) != 0 || guide.selected != "mermaid-bg" {
		t.Fatalf("empty search should keep detail, got %v and %q", guide.matches, guide.selected)
	}
}

func TestHelpViewsUseSelectedLanguage(t *testing.T) {
	t.Setenv("CYBERWRITER_CONFIG_DIR", t.TempDir())
	app := &workspaceWindow{preferences: &workspacePreferences{}}
	guide := workspaceFeatureGuide{workspace: app}
	guide.search("")
	features := mygoui.NewTester(guide.draw, featureGuideWidth, featureGuideHeight)
	if err := features.Click("Search features"); err != nil {
		t.Fatal(err)
	}
	features.Type("知识图谱 wikilinks")
	if !slices.Equal(guide.matches, []string{"graph"}) || guide.selected != "graph" {
		t.Fatalf("mixed search matches %v, selection %q", guide.matches, guide.selected)
	}
	shortcuts := mygoui.NewTester(func(c *mygoui.Context) {
		app.shortcutSheetView(c, func() {})
	}, 340, 700)
	const document = "Field Notes, /Notes"
	welcome := mygoui.NewTester(func(c *mygoui.Context) {
		drawWelcomeContent(c, app.interfaceLanguage(), false,
			[]workspaceWelcomeDocument{{Path: "Field Notes.md", Title: "Field Notes", Folder: "/Notes"}},
			func(string) {}, func() {}, func() {})
	}, welcomeWidth, welcomeHeight)
	ai := mygoui.NewTester(func(c *mygoui.Context) {
		drawAIWorkspaceGuidePopover(c, app.interfaceLanguage(), false)
	}, aiWorkspaceGuideWidth, aiWorkspaceGuideHeight)
	for _, state := range []struct {
		language                 workspaceLanguage
		feature, keys, hello, ai string
	}{
		{languageChinese, "知识图谱", "键盘快捷键", "欢迎回来", "AI 工作区指南"},
		{languageEnglish, "Knowledge Graph", "Keyboard Shortcuts", "Welcome Back", "AI Workspace Guide"},
	} {
		if err := app.setInterfaceLanguage(state.language); err != nil {
			t.Fatal(err)
		}
		for _, view := range []struct {
			tester *mygoui.Tester
			name   string
		}{
			{features, state.feature}, {shortcuts, state.keys}, {welcome, state.hello}, {ai, state.ai},
		} {
			view.tester.Frame()
			if _, ok := view.tester.Access(view.name); !ok {
				t.Fatalf("%s: missing %q after switching an open view", state.language, view.name)
			}
		}
		if guide.query != "知识图谱 wikilinks" || guide.selected != "graph" {
			t.Fatal("changing language reset the guide search or selected topic")
		}
		if _, ok := welcome.AccessByRole(mygoui.RoleButton, document); !ok {
			t.Fatal("changing language changed a user document label")
		}
	}
}
