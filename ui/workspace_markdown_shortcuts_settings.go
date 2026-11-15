package ui

import (
	"errors"
	"fmt"
	"slices"

	"cyberwriter/internal/core/mdshortcuts"
	mygoui "github.com/egoist/mygo/ui"
)

const markdownSnippetsConflict = "Snippets changed in another Settings window."

type markdownSnippetsDraft struct {
	text, savedText, savedSource string
	savedRules                   []mdshortcuts.Rule
	status                       string
	problem                      bool
}

func newMarkdownSnippetsDraft(config mdshortcuts.Config) markdownSnippetsDraft {
	text := config.SnippetSource
	if text == "" && len(config.Rules) != 0 {
		text = mdshortcuts.FormatSnippets(config.Rules)
	}
	return markdownSnippetsDraft{
		text: text, savedText: text, savedSource: config.SnippetSource,
		savedRules: slices.Clone(config.Rules),
	}
}

func (draft *markdownSnippetsDraft) matches(config mdshortcuts.Config) bool {
	return draft.savedSource == config.SnippetSource && slices.Equal(draft.savedRules, config.Rules)
}

func (draft *markdownSnippetsDraft) report(err error) {
	draft.status, draft.problem = "", err != nil
	if err != nil {
		draft.status = err.Error()
	}
}

func (app *workspaceWindow) drawMarkdownShortcutsSettings(c *mygoui.Context) {
	current := app.preferences.MarkdownShortcuts
	// Keep the draft for the life of this Settings window, including page switches.
	draft := mygoui.Local(c.Root(), "Markdown snippet draft", func() markdownSnippetsDraft {
		return newMarkdownSnippetsDraft(current)
	})
	if draft.text == draft.savedText && !draft.matches(current) {
		*draft = newMarkdownSnippetsDraft(current)
	}

	app.settingsSection(c, "Markdown Shortcuts", "keyboard", func() {
		app.settingsCard(c.Key("Markdown shortcuts snippets")).Gap(12).Children(func() {
			enabled := current.Enabled
			mygoui.Checkbox(c.Key("enable Markdown shortcuts"), &enabled, "Enable Markdown Shortcuts").OnChange(func() {
				next := app.preferences.MarkdownShortcuts
				next.Enabled = enabled
				draft.report(app.saveMarkdownShortcutPreferences(next))
			})
			theme := c.Theme()
			mygoui.Column(c).FillWidth().Gap(4).Children(func() {
				mygoui.Text(c, "Snippets").FontSize(13).FontWeight(600).Role(mygoui.RoleNone)
				mygoui.Text(c, "Paste or edit a snippets array. Comments are kept.").FontSize(11).TextColor(theme.TextMuted)
			})
			input := mygoui.TextArea(c.Key("Markdown snippets source"), &draft.text).Label("Snippets").
				FillWidth().MinWidth(0).Height(355).Font("monospace").FontSize(13).Padding(10, 12).
				Placeholder("[]").Radius(6).Background(theme.Background).Border(1, theme.Border).
				OnChange(func() { draft.report(nil) })
			save := func() {
				if err := app.saveMarkdownSnippets(draft); err != nil {
					draft.report(err)
					input.Focus()
				}
			}
			input.OnShortcut(mygoui.Cmd, mygoui.KeyS, save)
			mygoui.Row(c).FillWidth().AlignItems(mygoui.Center).Gap(10).Children(func() {
				status, failed := draft.status, draft.problem
				conflict := !draft.matches(current)
				if conflict {
					status, failed = markdownSnippetsConflict, true
				}
				if status != "" {
					message := mygoui.Text(c, status).FontSize(11).Grow(1).MinWidth(0).
						Role(mygoui.RoleStatus).TextColor(theme.TextMuted)
					if failed {
						message.Role(mygoui.RoleAlert).TextColor(theme.Danger)
					}
				} else {
					mygoui.Spacer(c)
				}
				if conflict {
					mygoui.Button(c, "Load Saved Snippets").Shrink(0).KeepFocus().
						Tooltip("Replace this draft with the saved snippets").OnClick(func() {
						*draft = newMarkdownSnippetsDraft(app.preferences.MarkdownShortcuts)
					})
				}
				mygoui.Button(c, "Save Snippets").Height(28).Shrink(0).KeepFocus().
					Background(theme.Accent).TextColor(theme.AccentText).OnClick(save)
			})
		})
	})
}

func (app *workspaceWindow) saveMarkdownSnippets(draft *markdownSnippetsDraft) error {
	next := app.preferences.MarkdownShortcuts
	if !draft.matches(next) {
		return errors.New(markdownSnippetsConflict)
	}
	rules, err := mdshortcuts.ParseSnippets(draft.text)
	if err != nil {
		return err
	}
	next.Rules, next.SnippetSource = rules, draft.text
	if err := app.saveMarkdownShortcutPreferences(next); err != nil {
		return err
	}
	*draft = newMarkdownSnippetsDraft(next)
	draft.status = "Snippets saved"
	return nil
}

func (app *workspaceWindow) saveMarkdownShortcutPreferences(next mdshortcuts.Config) error {
	if _, err := next.Program(); err != nil {
		return err
	}
	prefs := app.preferences
	previous := prefs.MarkdownShortcuts
	prefs.MarkdownShortcuts = next
	if err := prefs.write(); err != nil {
		prefs.MarkdownShortcuts = previous
		return fmt.Errorf("could not save Markdown Shortcuts: %w", err)
	}
	for _, window := range app.documentWindows() {
		if window.settings != nil && !window.settings.IsDestroyed() {
			window.settings.Invalidate()
		}
	}
	return nil
}
