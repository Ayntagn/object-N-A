package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"cyberwriter/internal/core/vaultcontext"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// EV-UI-294. A Related Ideas row opens the file through the common Vault
// policy and offers the first 80 graphemes to the destination editor.
func (app *workspaceWindow) openVaultConceptMatch(hit vaultcontext.Match, mods mygoui.Modifiers) {
	vault := app.vault
	replaceInWindow := !app.multiFile.active && !app.preferences.SidebarClickNewWindow && mods&mygoui.Cmd == 0
	switchingDocument := app.currentVaultPath() != hit.Path
	needle, _, _ := vaultContextPrefix(strings.TrimSpace(hit.Text), 80)
	app.openVaultFileFollowing(hit.Path, mods, false, func(target *workspaceWindow) {
		if target.vault == vault && !target.modalOpen && target.currentVaultPath() == hit.Path {
			target.revealVaultConceptText(needle, replaceInWindow && target == app, switchingDocument)
		}
	})
}

func (app *workspaceWindow) revealVaultConceptText(needle string, replaceInWindow, switchingDocument bool) {
	if needle == "" || app.draft == nil {
		return
	}
	mode := app.displayedMode()
	if replaceInWindow {
		if !switchingDocument {
			// EV-UI-325. Keep the current viewport and consume the excerpt
			// when the Source editor returns for this same document.
			app.pendingConceptExcerpt = needle
			return
		}
		if mode == sourceMode {
			return
		}
	}
	if mode == wysiwygMode && !app.multiFile.active {
		app.wys.openSession(app)
		if replaceInWindow && switchingDocument {
			// Initialize the new editor before drawing so Files keeps its focus.
			if app.focusVaultSearch {
				app.wys.focus = false
			}
			return
		}
		if app.wys.editable(app) {
			doc := app.wys.session.Document
			visible := doc.Text().String()
			query := strings.TrimSpace(richtext.ParseMarkdown(needle).String())
			if start, end, ok := vaultConceptTextRange(visible, query); ok {
				doc.SetSelection(utf8.RuneCountInString(visible[:start]), utf8.RuneCountInString(visible[:end]))
				app.focusVaultSearch, app.wys.focus = false, true
			}
		}
		return
	}
	snapshot := app.draft.Snapshot()
	start, end, ok := vaultConceptTextRange(snapshot.String(), needle)
	if !ok {
		return
	}
	// EV-UI-321. Split leaves the Source selection and sidebar focus intact.
	if app.code != nil && mode != previewMode && !(replaceInWindow && mode == splitMode) {
		app.code.Select(start, end)
		app.focusVaultSearch, app.focusSource = false, true
	}
	if !app.multiFile.active && (mode == splitMode || mode == previewMode) && !app.pdfPreview.enabled {
		app.previewRevealLine = snapshot.LineOf(start) + 1
	}
}

func (app *workspaceWindow) revealPendingVaultConceptSource() {
	needle := app.pendingConceptExcerpt
	app.pendingConceptExcerpt = ""
	if needle == "" || app.code == nil {
		return
	}
	if start, end, ok := vaultConceptSourceRange(app.draft.String(), needle); ok {
		app.code.Select(start, end)
		app.focusVaultSearch, app.focusSource = false, true
	}
}

var vaultConceptCaseFold = cases.Fold()

func foldVaultConceptSource(s string) string {
	return norm.NFD.String(vaultConceptCaseFold.String(norm.NFD.String(s)))
}

// EV-UI-320. Search a contiguous case-insensitive range and map the folded
// offsets back to exact source bytes without splitting a case expansion.
func vaultConceptSourceRange(source, needle string) (int, int, bool) {
	haystack, target := foldVaultConceptSource(source), foldVaultConceptSource(needle)
	var starts, ends vaultConceptFoldOffset
	starts.source.InitString(norm.NFD, source)
	ends.source.InitString(norm.NFD, source)
	for next := 0; target != "" && next < len(haystack); {
		at := strings.Index(haystack[next:], target)
		if at < 0 {
			break
		}
		at += next
		start, startOK := starts.at(at)
		end, endOK := ends.at(at + len(target))
		if startOK && endOK {
			return start, end, true
		}
		next = at + 1
	}
	return 0, 0, false
}

type vaultConceptFoldOffset struct {
	source norm.Iter
	folded int
}

func (p *vaultConceptFoldOffset) at(index int) (int, bool) {
	for p.folded < index && !p.source.Done() {
		position := p.source.Pos()
		for p.source.Pos() == position && !p.source.Done() {
			p.folded += len(norm.NFD.String(vaultConceptCaseFold.String(string(p.source.Next()))))
		}
	}
	return p.source.Pos(), p.folded == index
}

func vaultConceptTextRange(source, needle string) (start, end int, found bool) {
	if needle == "" {
		return 0, 0, false
	}
	if at := strings.Index(source, needle); at >= 0 {
		return at, at + len(needle), true
	}
	// Quick collapses line and paragraph whitespace. Match that same excerpt
	// against the editable snapshot and retain its original byte bounds.
	words := strings.Fields(needle)
	if len(words) < 2 {
		return 0, 0, false
	}
	for offset := 0; offset < len(source); {
		next := strings.Index(source[offset:], words[0])
		if next < 0 {
			break
		}
		start = offset + next
		offset = start + len(words[0])
		remaining := source[offset:]
		match := true
		for _, word := range words[1:] {
			trimmed := strings.TrimLeftFunc(remaining, unicode.IsSpace)
			if len(trimmed) == len(remaining) || !strings.HasPrefix(trimmed, word) {
				match = false
				break
			}
			remaining = trimmed[len(word):]
		}
		if match {
			return start, len(source) - len(remaining), true
		}
	}
	return 0, 0, false
}
