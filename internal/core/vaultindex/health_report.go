package vaultindex

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
)

const healthReportHeading = "# Vault Health\n\n_Generated "
const healthReportFooter = "---\n\n_Regenerate any time: `⌘/` → Vault Health Report. Unlinked mentions can be linked one by one from the vault sidebar's Unlinked Mentions section._\n"

// HealthReport derives a Vault Health Markdown note from saved Vault files.
func HealthReport(vault *workspace.Vault, name string, now time.Time) (string, error) {
	if vault == nil {
		return "", errors.New("open a Vault before checking its health")
	}
	result, err := scanVaultHealth(vault)
	if err != nil {
		return "", err
	}
	var report strings.Builder
	fmt.Fprintf(&report, "%s%s · %s_\n\n", healthReportHeading, now.Format("2006-01-02 15:04"), oneLine(name))
	report.WriteString("## Summary\n\n| | |\n|---|---|\n")
	for _, row := range []statsRow{
		{"Notes scanned", result.notesScanned}, {"Wikilinks", result.wikilinks},
		{"Broken links", len(result.broken)}, {"Orphan notes", len(result.orphans)},
		{"Unlinked mentions", result.mentionCount}, {"Duplicate titles", len(result.duplicates)},
	} {
		fmt.Fprintf(&report, "| %s | %d |\n", row.name, row.count)
	}
	report.WriteByte('\n')
	writeHealthBroken(&report, result.broken)
	writeHealthOrphans(&report, result.orphans)
	writeHealthMentions(&report, result.mentions)
	writeHealthDuplicates(&report, result.duplicates)
	report.WriteString(healthReportFooter)
	return report.String(), nil
}

// IsGeneratedHealthReport recognizes the original format, which has no YAML marker.
func IsGeneratedHealthReport(source string) bool {
	return strings.HasPrefix(source, healthReportHeading) && strings.HasSuffix(source, healthReportFooter)
}

// HealthReportNoteResolver resolves the generated report's title links in the
// observed directory order. Later notes with the same title replace earlier ones.
func HealthReportNoteResolver(entries []workspace.Entry) func(string) (string, bool) {
	titles := make(map[string]string)
	for _, entry := range healthFiles(entries) {
		if workspace.IsMarkdownPath(entry.Path) {
			titles[healthKey(markdownStem(entry.Name))] = entry.Path
		}
	}
	return func(target string) (string, bool) {
		name, _, _ := strings.Cut(target, "#")
		resolved, ok := titles[healthKey(markdownStem(strings.TrimSpace(name)))]
		return resolved, ok
	}
}

func writeHealthBroken(report *strings.Builder, links []healthBrokenLink) {
	report.WriteString("## Broken Links\n\n")
	if len(links) == 0 {
		report.WriteString("None - every wikilink resolves. ✓\n\n")
		return
	}
	report.WriteString("_Links whose target doesn't exist in the vault. Click the source note, fix or remove the link - or create the missing note by clicking the link there._\n\n| Source | Broken link | Line |\n|---|---|---|\n")
	for _, link := range links[:min(100, len(links))] {
		fmt.Fprintf(report, "| %s | %s | %d |\n", reportCell(healthWikilink(link.source)), reportCell(healthCode("[["+link.target+"]]")), link.line)
	}
	writeHealthRemainder(report, len(links)-100)
}

func writeHealthOrphans(report *strings.Builder, notes []healthOrphan) {
	report.WriteString("## Orphan Notes\n\n")
	if len(notes) == 0 {
		report.WriteString("None - every note is connected. ✓\n\n")
		return
	}
	report.WriteString("_No links in, no links out. Notes with waiting mentions are one click from connected - see the Unlinked Mentions section in the vault sidebar._\n\n")
	for _, note := range notes[:min(50, len(notes))] {
		fmt.Fprint(report, "- ", healthWikilink(note.name))
		if note.mentions > 0 {
			noun := "mention"
			if note.mentions != 1 {
				noun += "s"
			}
			fmt.Fprintf(report, " - **%d unlinked %s** waiting", note.mentions, noun)
		}
		report.WriteByte('\n')
	}
	writeHealthRemainder(report, len(notes)-50)
}

func writeHealthRemainder(report *strings.Builder, count int) {
	if count > 0 {
		fmt.Fprintf(report, "\n_…and %d more._\n", count)
	}
	report.WriteByte('\n')
}

func writeHealthMentions(report *strings.Builder, notes []healthMention) {
	report.WriteString("## Unlinked Mentions\n\n")
	if len(notes) == 0 {
		report.WriteString("None found - mentions and links agree. ✓\n\n")
		return
	}
	report.WriteString("_Notes discussed by name without a wikilink. Open one and use the sidebar's Unlinked Mentions section to link them._\n\n| Note | Mentions | In notes |\n|---|---|---|\n")
	for _, note := range notes[:min(15, len(notes))] {
		fmt.Fprintf(report, "| %s | %d | %d |\n", reportCell(healthWikilink(note.name)), note.count, note.sources)
	}
	report.WriteByte('\n')
}

func writeHealthDuplicates(report *strings.Builder, groups [][]workspace.Entry) {
	report.WriteString("## Duplicate Titles\n\n")
	if len(groups) == 0 {
		report.WriteString("None - every note name is unique. ✓\n\n")
		return
	}
	report.WriteString("_Same name in different folders - `[[name]]` links resolve to only one of them._\n\n")
	for _, group := range groups {
		fmt.Fprintf(report, "- **%s**: ", oneLine(markdownStem(group[0].Name)))
		for i, entry := range group {
			if i != 0 {
				report.WriteString(" · ")
			}
			report.WriteString(healthCode(entry.Path))
		}
		report.WriteByte('\n')
	}
	report.WriteByte('\n')
}

func healthWikilink(name string) string {
	return "[[" + oneLine(name) + "]]"
}

func healthCode(value string) string {
	value = oneLine(value)
	if !strings.Contains(value, "`") {
		return "`" + value + "`"
	}
	longest := 0
	for _, ticks := range strings.FieldsFunc(value, func(r rune) bool { return r != '`' }) {
		longest = max(longest, len(ticks))
	}
	delimiter := strings.Repeat("`", longest+1)
	return delimiter + " " + value + " " + delimiter
}
