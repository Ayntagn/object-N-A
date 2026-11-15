package vaultindex

import (
	"cmp"
	"errors"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"cyberwriter/internal/core/workspace"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

type vaultHealth struct {
	notesScanned, wikilinks, mentionCount int
	broken                                []healthBrokenLink
	orphans                               []healthOrphan
	mentions                              []healthMention
	duplicates                            [][]workspace.Entry
}

type healthBrokenLink struct {
	source, target string
	line           int
}

type healthOrphan struct {
	name     string
	mentions int
}

type healthMention struct {
	name           string
	count, sources int
}

type healthNote struct {
	name   string
	source string
}

// EV-UI-166 and EV-UI-168. The report scans eligible saved Markdown files,
// while existence and degrees come from the Vault-wide index, including reports.
func scanVaultHealth(vault *workspace.Vault) (vaultHealth, error) {
	if vault == nil {
		return vaultHealth{}, errors.New("open a Vault before checking its health")
	}
	entries, err := vault.ReportEntries()
	if err != nil {
		return vaultHealth{}, err
	}
	graph := graphLinkIndex{forward: make(map[string]map[string]struct{}), reverse: make(map[string][]int)}
	titles, paths := make(map[string]bool), make(map[string]bool)
	aliases := make(map[string][]string)
	var notes []healthNote
	var eligible []workspace.Entry
	var names []string
	for id, entry := range healthFiles(entries) {
		title := healthFileTitle(entry)
		titles[healthKey(title)] = true
		paths[healthKey(entry.Path)] = true
		if !workspace.IsMarkdownPath(entry.Path) {
			continue
		}
		scan := mentionSourcePath(entry.Path)
		if scan {
			eligible = append(eligible, entry)
			names = append(names, title)
		}
		doc, err := vault.ReadDocument(entry.Path)
		if err != nil {
			continue
		}
		graph.add(id, entry.Path, doc.Text)
		if scan {
			notes = append(notes, healthNote{name: title, source: doc.Text})
			if list, ok := healthAliases(doc.Text); ok {
				aliases[norm.NFC.String(title)] = list
			}
		}
	}
	result := vaultHealth{notesScanned: len(notes)}
	entities := mentionEntities(names, aliases)
	mentions := make(map[string]healthMentionCount)
	for _, note := range notes {
		result.scanLinks(note, titles, paths)
		scanHealthMentions(note.source, note.name, entities, mentions)
	}
	result.collectNotes(eligible, graph, mentions)
	return result, nil
}

// The native title index strips the extension from the four text types.
// For attachments it uses the complete last path component as the title.
func healthFileTitle(entry workspace.Entry) string {
	switch strings.ToLower(path.Ext(entry.Name)) {
	case ".md", ".markdown", ".txt", ".rtf":
		return strings.TrimSuffix(entry.Name, path.Ext(entry.Name))
	default:
		return entry.Name
	}
}

func healthFiles(entries []workspace.Entry) []workspace.Entry {
	var files []workspace.Entry
	// EV-UI-168: the native title index overwrites in directory traversal order.
	names := collate.New(language.Und, collate.IgnoreCase)
	var walk func([]workspace.Entry)
	walk = func(entries []workspace.Entry) {
		entries = slices.Clone(entries)
		slices.SortFunc(entries, func(a, b workspace.Entry) int {
			return cmp.Or(names.CompareString(a.Name, b.Name), strings.Compare(a.Name, b.Name))
		})
		for _, entry := range entries {
			if entry.IsDir {
				walk(entry.Children)
			} else {
				files = append(files, entry)
			}
		}
	}
	walk(entries)
	return files
}

func mentionSourcePath(relative string) bool {
	return strings.EqualFold(path.Ext(relative), ".md") &&
		relative != workspace.VaultHealthFile && relative != workspace.VaultStatsFile
}

func healthAliases(source string) ([]string, bool) {
	properties, err := workspace.ReadProperties(source)
	if err != nil {
		return nil, false
	}
	for _, property := range properties.Rows {
		if property.Key == "aliases" && property.Kind == workspace.PropertyList {
			aliases := make([]string, 0, len(property.Items))
			for _, item := range property.Items {
				aliases = append(aliases, item.Text)
			}
			return aliases, true
		}
	}
	return nil, false
}

var healthWikilinkPattern = regexp.MustCompile(`!?\[\[([^\]\n]+)\]\]`)

func (result *vaultHealth) scanLinks(note healthNote, titles, paths map[string]bool) {
	previous, line := 0, 1
	for _, match := range healthWikilinkPattern.FindAllStringSubmatchIndex(note.source, -1) {
		result.wikilinks++
		target := note.source[match[2]:match[3]]
		if healthLinkResolves(target, titles, paths) {
			continue
		}
		line += strings.Count(note.source[previous:match[0]], "\n")
		previous = match[0]
		result.broken = append(result.broken, healthBrokenLink{note.name, target, line})
	}
}

func healthLinkResolves(target string, titles, paths map[string]bool) bool {
	target, _, _ = strings.Cut(target, "|")
	target, _, _ = strings.Cut(target, "#")
	target = healthKey(strings.TrimFunc(target, foundationWhitespace))
	if target == "" {
		return true
	}
	target = strings.TrimSuffix(target, ".md")
	if titles[target] || paths[target] || paths[target+".md"] {
		return true
	}
	basename := target[strings.LastIndexByte(target, '/')+1:]
	return titles[basename] || basename == "vault stats" || basename == "vault health"
}

func healthKey(text string) string {
	return norm.NFC.String(strings.ToLower(text))
}

func foundationWhitespace(r rune) bool {
	return r == '\t' || r == '\u200b' || unicode.Is(unicode.Zs, r)
}

func (result *vaultHealth) collectNotes(entries []workspace.Entry, graph graphLinkIndex, mentions map[string]healthMentionCount) {
	for name, mention := range mentions {
		result.mentions = append(result.mentions, healthMention{name, mention.count, len(mention.sources)})
		result.mentionCount += mention.count
	}
	slices.SortFunc(result.mentions, func(a, b healthMention) int {
		return cmp.Or(cmp.Compare(b.count, a.count), strings.Compare(a.name, b.name))
	})
	groups := make(map[string][]workspace.Entry)
	for _, entry := range entries {
		key := graphNoteName(entry.Path)
		groups[key] = append(groups[key], entry)
		if len(graph.forward[key]) == 0 && len(graph.reverse[key]) == 0 {
			name := markdownStem(entry.Name)
			result.orphans = append(result.orphans, healthOrphan{name, mentions[name].count})
		}
	}
	names := collate.New(language.Und, collate.IgnoreCase)
	slices.SortStableFunc(result.orphans, func(a, b healthOrphan) int { return names.CompareString(a.name, b.name) })
	for _, files := range groups {
		if len(files) > 1 {
			result.duplicates = append(result.duplicates, files)
		}
	}
	slices.SortFunc(result.duplicates, func(a, b []workspace.Entry) int {
		return strings.Compare(markdownStem(a[0].Name), markdownStem(b[0].Name))
	})
}

type mentionEntity struct {
	name, key string
	words     []string
}

type mentionToken struct {
	text       string
	start, end int
}

type healthMentionCount struct {
	count   int
	sources map[string]struct{}
}

func mentionEntities(names []string, aliases map[string][]string) map[string][]mentionEntity {
	entities := make(map[string][]mentionEntity)
	for _, name := range names {
		key := healthKey(name)
		nameAliases := aliases[norm.NFC.String(name)]
		for _, phrase := range append([]string{name}, nameAliases...) {
			tokens := mentionTokens(phrase)
			if len(tokens) == 0 || len(tokens) == 1 && shortMention(tokens[0].text) {
				continue
			}
			words := make([]string, len(tokens))
			for i, token := range tokens {
				words[i] = token.text
			}
			first := words[0]
			entities[first] = append(entities[first], mentionEntity{name, key, words})
		}
	}
	for _, matches := range entities {
		slices.SortStableFunc(matches, func(a, b mentionEntity) int { return cmp.Compare(len(b.words), len(a.words)) })
	}
	return entities
}

func shortMention(token string) bool {
	if len(token) < 3 {
		return true
	}
	if strings.IndexFunc(token, func(r rune) bool { return r > unicode.MaxASCII }) == -1 {
		return false
	}
	var segments segmenter.Segmenter
	segments.Init([]rune(token))
	graphemes := segments.GraphemeIterator()
	return !graphemes.Next() || !graphemes.Next() || !graphemes.Next()
}

func mentionTokens(source string) []mentionToken {
	var tokens []mentionToken
	start := -1
	for index, char := range source {
		if char <= 0xffff && (unicode.IsLetter(char) || unicode.IsMark(char) || unicode.IsNumber(char)) {
			if start == -1 {
				start = index
			}
		} else if start >= 0 {
			tokens = append(tokens, mentionToken{healthKey(source[start:index]), start, index})
			start = -1
		}
	}
	if start >= 0 {
		tokens = append(tokens, mentionToken{healthKey(source[start:]), start, len(source)})
	}
	return tokens
}

var (
	healthFenceMarker       = regexp.MustCompile("(?m)^(?:\x60\x60\x60|~~~)[^\\n]*$")
	healthMentionExclusions = []*regexp.Regexp{
		regexp.MustCompile("\x60[^\x60\\n]+\x60"),
		healthWikilinkPattern,
		regexp.MustCompile(`\[[^\]\n]*\]\([^)\n]*\)`),
		regexp.MustCompile(`https?://[^\s)>\]]+`),
	}
)

func healthCodeRanges(source string) [][]int {
	// Each opening line pairs with the next line using the same marker.
	markers := healthFenceMarker.FindAllStringIndex(source, -1)
	var ranges [][]int
	for i := 0; i < len(markers); i++ {
		opening := markers[i]
		for next := i + 1; next < len(markers); next++ {
			closing := markers[next]
			if source[opening[0]] == source[closing[0]] {
				ranges = append(ranges, []int{opening[0], closing[1]})
				i = next
				break
			}
		}
	}
	return ranges
}

func scanHealthMentions(source, noteName string, entities map[string][]mentionEntity, mentions map[string]healthMentionCount) {
	scanMentionRanges(source, noteName, entities, -1, func(name string, _, _ int) {
		mention := mentions[name]
		if mention.sources == nil {
			mention.sources = make(map[string]struct{})
		}
		mention.count++
		mention.sources[noteName] = struct{}{}
		mentions[name] = mention
	})
}

// scanMentionRanges reports raw UTF-8 offsets in document order. A negative
// limit visits every occurrence. An empty excluded name disables name filtering.
func scanMentionRanges(source, excludedName string, entities map[string][]mentionEntity, limit int, hit func(string, int, int)) {
	if len(entities) == 0 || limit == 0 {
		return
	}
	prefix, _, body := workspace.SplitFrontmatter(source)
	excluded := healthCodeRanges(body)
	for _, pattern := range healthMentionExclusions {
		excluded = append(excluded, pattern.FindAllStringIndex(body, -1)...)
	}
	slices.SortFunc(excluded, func(a, b []int) int { return cmp.Compare(a[0], b[0]) })
	self := healthKey(excludedName)
	tokens := mentionTokens(body)
	for i := 0; i < len(tokens); i++ {
		start := tokens[i].start
		for len(excluded) > 0 && excluded[0][1] <= start {
			excluded = excluded[1:]
		}
		for _, entity := range entities[tokens[i].text] {
			end := i + len(entity.words)
			if entity.key == self || end > len(tokens) ||
				len(excluded) > 0 && excluded[0][0] < tokens[end-1].end ||
				!mentionMatches(body, tokens[i:end], entity.words) {
				continue
			}
			hit(entity.name, len(prefix)+start, len(prefix)+tokens[end-1].end)
			if limit > 0 {
				limit--
				if limit == 0 {
					return
				}
			}
			i = end - 1
			break
		}
	}
}

func mentionMatches(source string, tokens []mentionToken, words []string) bool {
	for i, word := range words {
		if tokens[i].text != word {
			return false
		}
		if i > 0 {
			gap := source[tokens[i-1].end:tokens[i].start]
			if gap != "" && gap != " " && gap != "\t" {
				return false
			}
		}
	}
	return true
}
