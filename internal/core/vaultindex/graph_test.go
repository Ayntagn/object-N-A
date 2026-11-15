// spec: FEAT-001 / EV-UI-069 / EV-UI-070 / EV-UI-202
package vaultindex

import (
	"slices"
	"testing"
)

func TestGraphCountsWikilinksAndDrawsDistinctPairs(t *testing.T) {
	index := buildFixture(t, map[string]string{
		"A/Hub.md": "#alpha\n[[Old]] [[Missing old target]]\n",
		"B/Hub.markdown": "---\ntags: [Zulu, alpha]\nproperty: '[[Missing target]]'\n---\n" +
			"[[Deep/Target.md#Heading|Read it]] [[TARGET]] [[Hub]] [[#Heading]]\n" +
			"[Ordinary link](../Isolated.md)\n",
		"Isolated.md": "",
		"Old.md":      "#zulu\n",
		"Target.md":   "[[Hub]]\n",
	})
	graph := index.Graph()
	var paths []string
	for _, note := range graph.Notes {
		paths = append(paths, note.Path)
	}
	if want := []string{"A/Hub.md", "B/Hub.markdown", "Isolated.md", "Old.md", "Target.md"}; !slices.Equal(paths, want) {
		t.Errorf("notes = %v, want %v", paths, want)
	}
	if want := []GraphTag{{Name: "alpha", Count: 2}, {Name: "zulu", Count: 2}}; !slices.Equal(graph.Tags, want) {
		t.Errorf("tags = %v, want %v", graph.Tags, want)
	}
	if graph.Links != 5 || graph.Linked != 6 {
		t.Errorf("links/linked = %d/%d, want 5/6", graph.Links, graph.Linked)
	}
	if want := []GraphEdge{{From: 1, To: 4}}; !slices.Equal(graph.Edges, want) {
		t.Errorf("drawn edges = %v, want %v", graph.Edges, want)
	}
	if got, ok := index.ResolveMarkdown("B/Hub.markdown", "../Isolated.md"); !ok || got != "Isolated.md" {
		t.Errorf("ordinary Markdown navigation resolved to (%q, %t)", got, ok)
	}
}

func TestGraphLayoutResolvesDuplicateNamesAfterConnectedOrder(t *testing.T) {
	graph := buildFixture(t, map[string]string{
		"A.md": "[[Shared]]", "B/Shared.md": "[[Middle]]",
		"Middle.md": "", "Tail.md": "", "Z/Shared.md": "[[Middle]]",
	}).Graph()
	initial := []int{0, 1, 2, 3, 4}
	layout := graph.Layout(initial)
	if want := []int{0, 4, 2, 1, 3}; !slices.Equal(layout.Order, want) {
		t.Errorf("layout order = %v, want %v", layout.Order, want)
	}
	if want := []GraphEdge{{From: 0, To: 1}, {From: 1, To: 2}}; !slices.Equal(layout.Edges, want) {
		t.Errorf("layout edges = %v, want %v", layout.Edges, want)
	}
	if want := []GraphEdge{{From: 0, To: 4}, {From: 4, To: 2}}; !slices.Equal(graph.Edges, want) {
		t.Errorf("index edges = %v, want %v", graph.Edges, want)
	}
}
