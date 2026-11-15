package vaultindex

import (
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"cyberwriter/internal/core/workspace"
)

// Graph is the Vault-wide graph and its statistics. Its slices are read-only.
type Graph struct {
	Notes  []GraphNote
	Edges  []GraphEdge
	Tags   []GraphTag
	Links  int // Distinct Wikilink targets per source, summed across sources.
	Linked int // Distinct targets across all notes, including unresolved targets.
	// Used when the layout's short-name lookups can resolve to different IDs.
	layoutTargets map[string][]WikiLinkTarget
}

// GraphNote preserves a note's Vault-relative path and normalized tags.
type GraphNote struct {
	Path string
	Tags []string
}

// GraphEdge is a visible connection between two indices into Graph.Notes.
type GraphEdge struct {
	From, To int
}

// GraphTag counts the notes carrying a tag.
type GraphTag struct {
	Name  string
	Count int
}

// Graph returns the same read-only graph snapshot for the lifetime of the index.
func (index *Index) Graph() *Graph { return &index.graph }

// GraphTagFiles returns the indexed files for a tag in snapshot order.
// The returned slice is read-only.
func (index *Index) GraphTagFiles(tag string) []workspace.Entry {
	if index == nil {
		return nil
	}
	return index.graphTagFiles[tag]
}

// WikiLinkTarget identifies a raw Wikilink target and its Vault path, if found.
type WikiLinkTarget struct {
	Name string
	Path string
}

// WikiLinksFrom returns the read-only directed targets from the raw Wikilink index.
func (index *Index) WikiLinksFrom(relative string) []WikiLinkTarget {
	if _, exists := index.byPath[relative]; exists {
		return index.wikiLinks[graphNoteName(relative)]
	}
	return nil
}

// WikiBacklinks returns the notes with raw Wikilinks to the named note.
func (index *Index) WikiBacklinks(relative string) []workspace.Entry {
	if _, exists := index.byPath[relative]; exists {
		return index.entries(index.wikiBacklinks[graphNoteName(relative)])
	}
	return nil
}

// The native graph indexes raw Wikilinks by normalized filename (EV-UI-070).
var graphWikilink = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|[^\]]*)?\]\]`)

// The Vault index also scans the saved source for tags, including examples in code.
var graphHashtag = regexp.MustCompile(`(?:^|[\s\p{Z}\x{0085}\v])#([A-Za-z][\p{L}\p{M}\p{Nd}\p{Pc}\x{200C}\x{200D}-]*)`)

func collectGraphTags(source, properties string) []string {
	tags := frontmatterTags(properties)
	for _, match := range graphHashtag.FindAllStringSubmatch(source, -1) {
		tag := strings.ToLower(match[1])
		if !isHexColor(tag) {
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)
	return slices.Compact(tags)
}

func isHexColor(tag string) bool {
	switch len(tag) {
	case 3, 4, 6, 8:
		return strings.IndexFunc(tag, func(r rune) bool {
			return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f')
		}) < 0
	}
	return false
}

type graphLinkIndex struct {
	forward map[string]map[string]struct{}
	reverse map[string][]int
}

func (links graphLinkIndex) add(sourceID int, source, text string) map[string]struct{} {
	matches := graphWikilink.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	targets := make(map[string]struct{})
	for _, match := range matches {
		target := graphTargetName(match[1])
		targets[target] = struct{}{}
	}
	for target := range targets {
		links.reverse[target] = append(links.reverse[target], sourceID)
	}
	links.forward[graphNoteName(source)] = targets
	return targets
}

func graphTargetName(target string) string {
	// Foundation's whitespaces set is horizontal tab, U+200B and space separators.
	target = strings.TrimFunc(target, func(r rune) bool {
		return r == '\t' || r == '\u200b' || unicode.Is(unicode.Zs, r)
	})
	target, _, _ = strings.Cut(target, "#")
	target = strings.TrimSuffix(target, ".md")
	target = target[strings.LastIndexByte(target, '/')+1:]
	return strings.ToLower(target)
}

func graphNoteName(relative string) string {
	return strings.ToLower(markdownStem(path.Base(relative)))
}

func (links graphLinkIndex) snapshot(index *Index) Graph {
	notes := index.notes
	graph := Graph{Notes: make([]GraphNote, len(notes)), Linked: len(links.reverse)}
	index.wikiLinks = make(map[string][]WikiLinkTarget, len(links.forward))
	index.wikiBacklinks = links.reverse
	names := make([]string, len(notes))
	byName := make(map[string]int, len(notes))
	index.graphTagFiles = make(map[string][]workspace.Entry)
	for id, note := range notes {
		name := graphNoteName(note.entry.Path)
		names[id] = name
		if _, exists := byName[name]; exists {
			graph.layoutTargets = index.wikiLinks
		}
		byName[name] = id
		graph.Notes[id] = GraphNote{Path: note.entry.Path, Tags: note.graphTags}
		for _, tag := range note.graphTags {
			index.graphTagFiles[tag] = append(index.graphTagFiles[tag], note.entry)
		}
	}
	for _, tag := range slices.Sorted(maps.Keys(index.graphTagFiles)) {
		graph.Tags = append(graph.Tags, GraphTag{Name: tag, Count: len(index.graphTagFiles[tag])})
	}

	seenSources := make(map[string]bool, len(links.forward))
	seenEdges := make(map[[2]int]struct{})
	for _, name := range names {
		targets := links.forward[name]
		if len(targets) == 0 || seenSources[name] {
			continue
		}
		seenSources[name] = true
		graph.Links += len(targets)
		from := byName[name]
		references := make([]WikiLinkTarget, 0, len(targets))
		for _, target := range slices.Sorted(maps.Keys(targets)) {
			to, exists := byName[target]
			reference := WikiLinkTarget{Name: target}
			if exists {
				reference.Path = notes[to].entry.Path
			}
			references = append(references, reference)
			if !exists || from == to {
				continue
			}
			pair := [2]int{min(from, to), max(from, to)}
			if _, seen := seenEdges[pair]; seen {
				continue
			}
			seenEdges[pair] = struct{}{}
			graph.Edges = append(graph.Edges, GraphEdge{From: from, To: to})
		}
		index.wikiLinks[name] = references
	}
	return graph
}
