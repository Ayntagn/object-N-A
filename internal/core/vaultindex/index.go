// Package vaultindex indexes saved Markdown notes in an open Vault.
package vaultindex

import (
	"errors"
	"net/url"
	"path"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
)

// Index is a read-only snapshot. A caller can rebuild and replace it when the
// Vault changes.
type Index struct {
	vault         *workspace.Vault
	notes         []note
	byPath        map[string]int
	byStem        map[string][]int
	byName        map[string][]int
	incoming      [][]int
	outgoing      [][]int
	graph         Graph
	graphTagFiles map[string][]workspace.Entry
	wikiLinks     map[string][]WikiLinkTarget
	wikiBacklinks map[string][]int
}

type note struct {
	entry        workspace.Entry
	stamp        workspace.IndexedNoteStamp
	lowerPath    string
	lowerBody    string
	tags         []string
	graphTags    []string
	graphTargets map[string]struct{}
	properties   map[string]workspace.Property
	links        []noteLink
}

// Edge is one directed relationship between two Vault-relative note paths.
type Edge struct {
	Source string
	Target string
}

// Build reads notes from entries, or lists the Vault when entries is nil.
// A note with unreadable content is still available by name and path.
func Build(vault *workspace.Vault, entries []workspace.Entry) (*Index, error) {
	return build(vault, entries, nil)
}

func build(vault *workspace.Vault, entries []workspace.Entry, readNote func(workspace.Document)) (*Index, error) {
	return buildSnapshot(vault, entries, nil, readNote)
}

// Refresh builds the next read-only snapshot, reusing parsed notes whose
// on-disk identity and change time are unchanged. New paths still participate
// in resolving links from every note.
func (previous *Index) Refresh(vault *workspace.Vault, entries []workspace.Entry) (*Index, error) {
	return buildSnapshot(vault, entries, previous, nil)
}

func buildSnapshot(vault *workspace.Vault, entries []workspace.Entry, previous *Index, readNote func(workspace.Document)) (*Index, error) {
	if vault == nil {
		return nil, errors.New("open a Vault before indexing")
	}
	if entries == nil {
		var err error
		entries, err = vault.Entries()
		if err != nil {
			return nil, err
		}
	}
	index := &Index{vault: vault, notes: collectNotes(make([]note, 0, countNotes(entries)), entries)}
	if previous != nil && previous.vault != vault {
		previous = nil
	}
	slices.SortFunc(index.notes, func(a, b note) int {
		return strings.Compare(a.entry.Path, b.entry.Path)
	})
	same := previous != nil && len(index.notes) == len(previous.notes)
	for id := range index.notes {
		current := &index.notes[id]
		var old *note
		if previous != nil {
			if from, ok := previous.byPath[current.entry.Path]; ok {
				old = &previous.notes[from]
			}
		}
		if current.load(vault, old, readNote) {
			same = same && current.entry.Name == old.entry.Name &&
				current.entry.ModTime() == old.entry.ModTime() && current.entry.Size() == old.entry.Size()
		} else {
			same = false
		}
	}
	if same {
		return previous, nil
	}
	index.buildRelationships()
	return index, nil
}

func (n *note) load(vault *workspace.Vault, old *note, readNote func(workspace.Document)) (reused bool) {
	source := n.entry.Path
	var text string
	if readNote != nil {
		document, err := vault.ReadDocument(source)
		if err != nil {
			return false
		}
		readNote(document)
		text = document.Text
	} else {
		var stamp workspace.IndexedNoteStamp
		if old != nil {
			stamp = old.stamp
		}
		content, currentStamp, unchanged, err := vault.ReadIndexedNote(source, stamp)
		if err != nil {
			return false
		}
		if unchanged {
			entry := n.entry
			*n = *old
			n.entry = entry
			return true
		}
		text, n.stamp = content, currentStamp
	}
	n.graphTargets = collectGraphTargets(text)
	_, properties, body := workspace.SplitFrontmatter(text)
	if properties != "" {
		if parsed, err := workspace.ReadProperties(text); err == nil && len(parsed.Rows) > 0 {
			n.properties = make(map[string]workspace.Property, len(parsed.Rows))
			for _, property := range parsed.Rows {
				n.properties[strings.ToLower(property.Key)] = property
			}
		}
	}
	n.graphTags = collectGraphTags(text, properties)
	n.lowerBody = strings.ToLower(body)
	metadata := parseMetadata(properties, body)
	n.tags = metadata.tags
	n.links = metadata.links
	if len(n.links) > 0 {
		// Retain each target only once and detach it from the source buffer.
		slices.SortFunc(n.links, func(a, b noteLink) int {
			if a.markdown != b.markdown {
				if a.markdown {
					return 1
				}
				return -1
			}
			return strings.Compare(a.target, b.target)
		})
		n.links = slices.Clone(slices.Compact(n.links))
		for at := range n.links {
			n.links[at].target = strings.Clone(n.links[at].target)
		}
	}
	return false
}

func (index *Index) buildRelationships() {
	index.byPath = make(map[string]int, len(index.notes))
	index.byStem = make(map[string][]int, len(index.notes))
	index.byName = make(map[string][]int, len(index.notes))
	index.incoming = make([][]int, len(index.notes))
	index.outgoing = make([][]int, len(index.notes))
	for id, note := range index.notes {
		relative := note.entry.Path
		stem := markdownStem(relative)
		name := markdownStem(path.Base(relative))
		index.byPath[relative] = id
		index.byStem[stem] = append(index.byStem[stem], id)
		index.byName[name] = append(index.byName[name], id)
	}
	graphLinks := graphLinkIndex{
		forward: make(map[string]map[string]struct{}),
		reverse: make(map[string][]int),
	}
	for id := range index.notes {
		note := &index.notes[id]
		source := note.entry.Path
		graphLinks.add(id, source, note.graphTargets)
		var targets []int
		for _, link := range note.links {
			var target string
			var ok bool
			if link.markdown {
				target, ok = index.ResolveMarkdown(source, link.target)
			} else {
				target, ok = index.Resolve(source, link.target)
			}
			if ok && source != target {
				targets = append(targets, index.byPath[target])
			}
		}
		slices.Sort(targets)
		targets = slices.Compact(targets)
		index.outgoing[id] = targets
		for _, target := range targets {
			index.incoming[target] = append(index.incoming[target], id)
		}
		if note.stamp == (workspace.IndexedNoteStamp{}) {
			note.links = nil
		}
	}
	index.graph = graphLinks.snapshot(index)
}

func countNotes(entries []workspace.Entry) int {
	count := 0
	for _, entry := range entries {
		if entry.IsDir {
			count += countNotes(entry.Children)
		} else if workspace.IsMarkdownPath(entry.Path) {
			count++
		}
	}
	return count
}

func collectNotes(notes []note, entries []workspace.Entry) []note {
	for _, entry := range entries {
		if entry.IsDir {
			notes = collectNotes(notes, entry.Children)
		} else if workspace.IsMarkdownPath(entry.Path) {
			entry.Children = nil
			notes = append(notes, note{entry: entry, lowerPath: strings.ToLower(entry.Path)})
		}
	}
	return notes
}

// Search matches each whitespace-delimited term against the path, source body,
// or tags, without case distinctions. A term starting with # matches a whole tag.
// Entries are ordered by path. An empty query returns every indexed note.
func (index *Index) Search(query string) []workspace.Entry {
	return index.SearchWithGraphTag(query, "")
}

// SearchWithGraphTag applies the Graph sidebar's selected tag to note search.
func (index *Index) SearchWithGraphTag(query, tag string) []workspace.Entry {
	terms := strings.Fields(strings.ToLower(query))
	var matches []workspace.Entry
	for _, note := range index.notes {
		if (tag == "" || slices.Contains(note.graphTags, tag)) && note.matches(terms) {
			matches = append(matches, note.entry)
		}
	}
	return matches
}

func (note note) matches(terms []string) bool {
	for _, term := range terms {
		if tag, ok := strings.CutPrefix(term, "#"); ok {
			if !slices.Contains(note.tags, tag) {
				return false
			}
			continue
		}
		if strings.Contains(note.lowerPath, term) || strings.Contains(note.lowerBody, term) {
			continue
		}
		if !slices.ContainsFunc(note.tags, func(tag string) bool { return strings.Contains(tag, term) }) {
			return false
		}
	}
	return true
}

// Resolve resolves a Wikilink target with an optional extension, heading or
// alias. A bare name first matches the source folder, then a unique Vault-wide
// name. Explicit paths are relative to the Vault, or to the source folder if
// prefixed by ./ or ../.
func (index *Index) Resolve(source, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "[[") || strings.HasPrefix(target, "![[") {
		links := Wikilinks(target)
		if len(links) != 1 || links[0].Start != 0 || links[0].End != len(target) {
			return "", false
		}
		target = links[0].Target
	} else {
		target, _ = wikiTargetAndLabel(target)
	}
	name, _, hasHeading := strings.Cut(target, "#")
	name = strings.TrimSpace(name)
	if name == "" {
		_, exists := index.byPath[source]
		if hasHeading && exists {
			return source, true
		}
		return "", false
	}
	if strings.HasSuffix(name, "/") {
		return "", false
	}
	switch {
	case strings.HasPrefix(name, "./"), strings.HasPrefix(name, "../"):
		return index.noteAt(path.Join(path.Dir(source), name))
	case strings.Contains(name, "/"):
		return index.noteAt(name)
	}
	if source != "" {
		if local, ok := index.noteAt(path.Join(path.Dir(source), name)); ok {
			return local, true
		}
	}
	var match string
	for _, id := range index.byName[markdownStem(name)] {
		entry := index.notes[id].entry
		if workspace.IsMarkdownPath(name) && path.Base(entry.Path) != name {
			continue
		}
		if match != "" {
			return "", false
		}
		match = entry.Path
	}
	return match, match != ""
}

// ResolveMarkdown resolves a relative Markdown URL against the source folder.
// Its URL path is decoded and must identify a note already in the index.
func (index *Index) ResolveMarkdown(source, destination string) (string, bool) {
	destination = strings.TrimSpace(destination)
	target, err := url.Parse(destination)
	if err != nil || target.IsAbs() || target.Host != "" ||
		strings.HasPrefix(target.Path, "/") || strings.HasSuffix(target.Path, "/") {
		return "", false
	}
	if target.Path == "" {
		_, exists := index.byPath[source]
		if exists && strings.HasPrefix(destination, "#") {
			return source, true
		}
		return "", false
	}
	return index.noteAt(path.Join(path.Dir(source), target.Path))
}

func (index *Index) noteAt(relative string) (string, bool) {
	relative = path.Clean(relative)
	if workspace.IsMarkdownPath(relative) {
		_, ok := index.byPath[relative]
		if ok {
			return relative, true
		}
	} else if matches := index.byStem[relative]; len(matches) == 1 {
		return index.notes[matches[0]].entry.Path, true
	}
	return "", false
}

func markdownStem(name string) string {
	if workspace.IsMarkdownPath(name) {
		return strings.TrimSuffix(name, path.Ext(name))
	}
	return name
}

// LinksFrom returns the distinct notes referenced by relative.
func (index *Index) LinksFrom(relative string) []workspace.Entry {
	if id, ok := index.byPath[relative]; ok {
		return index.entries(index.outgoing[id])
	}
	return nil
}

// Backlinks returns the distinct notes that reference relative.
func (index *Index) Backlinks(relative string) []workspace.Entry {
	if id, ok := index.byPath[relative]; ok {
		return index.entries(index.incoming[id])
	}
	return nil
}

// Neighbors returns the distinct notes linked to or from relative.
func (index *Index) Neighbors(relative string) []workspace.Entry {
	id, ok := index.byPath[relative]
	if !ok {
		return nil
	}
	neighbors := slices.Concat(index.incoming[id], index.outgoing[id])
	slices.Sort(neighbors)
	return index.entries(slices.Compact(neighbors))
}

func (index *Index) entries(ids []int) []workspace.Entry {
	if len(ids) == 0 {
		return nil
	}
	entries := make([]workspace.Entry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, index.notes[id].entry)
	}
	return entries
}

// Tags returns normalized tag names in lexical order, without leading #.
func (index *Index) Tags(relative string) []string {
	if id, ok := index.byPath[relative]; ok {
		return slices.Clone(index.notes[id].tags)
	}
	return nil
}

// Edges returns all distinct directed note relationships in path order.
func (index *Index) Edges() []Edge {
	var edges []Edge
	for source, targets := range index.outgoing {
		for _, target := range targets {
			edges = append(edges, Edge{
				Source: index.notes[source].entry.Path,
				Target: index.notes[target].entry.Path,
			})
		}
	}
	return edges
}
