package snippets

import (
	"fmt"
	"strings"
	"time"
)

// EV-UI-240: Markdown and Text group snippets in collection order.
func exportMarkdown(items []exportEntry, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Snippets Export\n\nExported: %s\nTotal snippets: %d\n\n---\n\n", exportLocalDate(now), exportSnippetCount(items))
	writeMarkdownSnippets(&out, items, 2, now.Location())
	return strings.TrimSuffix(out.String(), "\n")
}

func writeMarkdownSnippets(out *strings.Builder, items []exportEntry, level int, timezone *time.Location) {
	for _, item := range items {
		heading := strings.Repeat("#", min(level, 6))
		if item.IsFolder {
			fmt.Fprintf(out, "%s 📁 %s\n\n", heading, item.Title)
			writeMarkdownSnippets(out, item.children, level+1, timezone)
			continue
		}
		fmt.Fprintf(out, "%s %s\n\n", heading, item.Title)
		fmt.Fprintf(out, "**Created:** %s\n", exportLocalDate(exportSnippetDate(item.CreatedAt).In(timezone)))
		if item.UpdatedAt != item.CreatedAt {
			fmt.Fprintf(out, "**Updated:** %s\n", exportLocalDate(exportSnippetDate(item.UpdatedAt).In(timezone)))
		}
		if source, ok := item.exportSource(); ok {
			fmt.Fprintf(out, "**Source:** %s\n", source)
		}
		if item.hasField("language") {
			fmt.Fprintf(out, "**Language:** %s\n", item.Language)
		}
		if len(item.Tags) > 0 {
			fmt.Fprintf(out, "**Tags:** %s\n", strings.Join(item.Tags, ", "))
		}
		fmt.Fprintf(out, "\n\x60\x60\x60%s\n%s\n\x60\x60\x60\n\n", item.Language, item.Content)
		if item.Notes != "" {
			fmt.Fprintf(out, "> **Notes:** %s\n\n", item.Notes)
		}
		out.WriteString("---\n\n")
	}
}

func exportText(items []exportEntry, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "SNIPPETS EXPORT\n===============\n\nExported: %s\nTotal snippets: %d\n\n%s\n\n",
		exportLocalDate(now), exportSnippetCount(items), strings.Repeat("=", 60))
	writeTextSnippets(&out, items, "", now.Location())
	return strings.TrimSuffix(out.String(), "\n")
}

func writeTextSnippets(out *strings.Builder, items []exportEntry, indent string, timezone *time.Location) {
	for _, item := range items {
		if item.IsFolder {
			fmt.Fprintf(out, "%s[%s]\n\n", indent, item.Title)
			writeTextSnippets(out, item.children, indent+"  ", timezone)
			continue
		}
		underline := strings.Repeat("-", graphemeCount(item.Title, len(item.Title)))
		fmt.Fprintf(out, "%s%s\n%s%s\n\n", indent, item.Title, indent, underline)
		fmt.Fprintf(out, "%sCreated: %s\n", indent, exportLocalDate(exportSnippetDate(item.CreatedAt).In(timezone)))
		if source, ok := item.exportSource(); ok {
			fmt.Fprintf(out, "%sSource: %s\n", indent, source)
		}
		if item.hasField("language") {
			fmt.Fprintf(out, "%sLanguage: %s\n", indent, item.Language)
		}
		if len(item.Tags) > 0 {
			fmt.Fprintf(out, "%sTags: %s\n", indent, strings.Join(item.Tags, ", "))
		}
		fmt.Fprintf(out, "\n%sContent:\n", indent)
		for line := range strings.SplitSeq(item.Content, "\n") {
			fmt.Fprintf(out, "%s    %s\n", indent, line)
		}
		if item.Notes != "" {
			fmt.Fprintf(out, "\n%sNotes: %s\n", indent, item.Notes)
		}
		fmt.Fprintf(out, "\n%s%s\n\n", indent, strings.Repeat("-", 40))
	}
}
