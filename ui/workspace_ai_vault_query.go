package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"cyberwriter/internal/core/aiworkspace"
	"cyberwriter/internal/core/vaultcontext"
	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/unicode/norm"
)

type workspaceAIVaultQuery struct {
	index         *vaultcontext.Index
	structure     string
	excerptBudget int
}

func (app *workspaceWindow) aiUsesVaultContext() bool {
	if override := app.ai.vaultContextOverride; override != nil {
		return *override
	}
	return app.preferences.AIVaultContextEnabled
}

func (app *workspaceWindow) toggleAIUseVault() {
	enabled := !app.aiUsesVaultContext()
	app.ai.vaultContextOverride = &enabled
	app.invalidateAIChat()
}

func (app *workspaceWindow) aiVaultContextQuery(connection workspaceAISavedConnection) workspaceAIVaultQuery {
	state := app.aiVaultContextState()
	if state == nil || state.index == nil || !app.aiUsesVaultContext() {
		return workspaceAIVaultQuery{}
	}
	// EV-UI-309. The request's connection determines the retrieval budget.
	budget := 5_000
	if strings.Contains(strings.ToLower(connection.BaseURL), "apple") || strings.Contains(strings.ToLower(connection.Model), "apple") {
		budget = 3_000
	}
	query := workspaceAIVaultQuery{index: state.index, excerptBudget: budget}
	if app.preferences.AIVaultContextSendStructure {
		query.structure = vaultStructureSummary(app.entries)
	}
	return query
}

func (query workspaceAIVaultQuery) appendToMessages(ctx context.Context, prompt, preview string, messages []aiworkspace.Message) error {
	if err := ctx.Err(); query.index == nil || err != nil {
		return err
	}
	hint, err := query.context(ctx, prompt)
	if err != nil {
		return fmt.Errorf("Could not retrieve Vault context. %w", err)
	}
	if hint != "" {
		current := &messages[len(messages)-2]
		current.Content = strings.TrimSuffix(current.Content, preview) + hint + "\n\n---\n\n" + preview
	}
	return ctx.Err()
}

// EV-UI-289 / EV-UI-293 / EV-UI-311. Both sections use the mounted index
// gate. The captured structure is sent independently of match count.
func (query workspaceAIVaultQuery) context(ctx context.Context, prompt string) (string, error) {
	hits, err := searchAIVaultIndex(ctx, query.index, prompt)
	if err != nil {
		return "", err
	}
	var blocks []string
	if query.structure != "" {
		blocks = append(blocks, query.structure)
	}
	if excerpts := vaultContextExcerpts(hits, query.excerptBudget); excerpts != "" {
		blocks = append(blocks, excerpts)
	}
	return strings.Join(blocks, "\n\n---\n\n"), nil
}

func vaultStructureSummary(entries []workspace.Entry) string {
	type filePath struct {
		original, sortKey string
	}
	var files []filePath
	var collect func([]workspace.Entry)
	collect = func(entries []workspace.Entry) {
		for _, entry := range entries {
			if entry.IsDir {
				collect(entry.Children)
			} else if workspace.IsMarkdownPath(entry.Path) {
				files = append(files, filePath{original: entry.Path, sortKey: norm.NFC.String(entry.Path)})
			}
		}
	}
	collect(entries)
	if len(files) == 0 {
		return ""
	}
	// Swift String orders canonically equivalent paths together. Keep their
	// inventory order and emit the exact spelling from the file tree.
	slices.SortStableFunc(files, func(a, b filePath) int { return strings.Compare(a.sortKey, b.sortKey) })

	const limit = 50
	var summary strings.Builder
	fmt.Fprintf(&summary, "Vault structure (%d markdown files):", len(files))
	for _, file := range files[:min(limit, len(files))] {
		fmt.Fprintf(&summary, "\n- %s", file.original)
	}
	if remaining := len(files) - limit; remaining > 0 {
		fmt.Fprintf(&summary, "\n- … and %d more files", remaining)
	}
	return summary.String()
}

func searchAIVaultIndex(ctx context.Context, index *vaultcontext.Index, query string) ([]vaultcontext.Match, error) {
	if index.Engine() == vaultcontext.Fast {
		return index.Search(ctx, query)
	}
	return searchContextualVaultIndex(ctx, index, query)
}

func vaultContextExcerpts(hits []vaultcontext.Match, limit int) string {
	const header = "## Relevant from your vault\n\n" +
		"_When referring to these files in your response, use the `[[path/to/file.md]]` wikilink format so the reader can click to open them._\n\n"
	const cutoff = "\n\n_[chunk truncated to fit context budget]_"
	if len(hits) == 0 {
		return ""
	}
	var body strings.Builder
	body.WriteString(header)
	remaining := limit
	for _, hit := range hits {
		title := "### [[" + hit.Path + "]]\n\n"
		_, titleCount, _ := vaultContextPrefix(title, len(title))
		available := remaining - titleCount
		excerpt, _, truncated := vaultContextPrefix(hit.Text, max(0, available))
		if available < 0 || truncated {
			// EV-UI-309 reserves 30 graphemes before appending the full marker.
			excerpt, _, _ = vaultContextPrefix(hit.Text, max(0, available-30))
			excerpt += cutoff
		}
		block := title + excerpt + "\n\n"
		body.WriteString(block)
		_, count, _ := vaultContextPrefix(block, len(block))
		remaining -= count
		if remaining < 200 {
			break
		}
	}
	return body.String()
}

func vaultContextPrefix(text string, budget int) (prefix string, count int, truncated bool) {
	rs := []rune(text)
	var segments segmenter.Segmenter
	segments.Init(rs)
	for entries := segments.GraphemeIterator(); entries.Next(); {
		if count == budget {
			return string(rs[:entries.Grapheme().Offset]), count, true
		}
		count++
	}
	return text, count, false
}
