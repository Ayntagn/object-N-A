package vaultindex

import (
	"cmp"
	"maps"
	"path"
	"slices"
	"strings"
)

const vaultMapLimit = 60

type VaultMap struct {
	Cards   []VaultMapCard
	Edges   []VaultMapEdge
	Omitted int
}

type VaultMapCard struct {
	Label string
	Tag   string
}

type VaultMapEdge struct {
	From, To   int
	Reciprocal bool
}

// Map groups saved Wikilinks by basename for the directed drawing export.
// EV-UI-080 observed one source and one displayed card for duplicate names.
// A stable path order makes the choice repeatable across index rebuilds.
func (index *Index) Map() VaultMap {
	notesByName := make(map[string]note, len(index.notes))
	linksByName := make(map[string]map[string]struct{})
	for _, note := range index.notes {
		name := graphNoteName(note.entry.Path)
		notesByName[name] = note
		if len(note.graphTargets) != 0 && linksByName[name] == nil {
			linksByName[name] = note.graphTargets
		}
	}

	type namedEdge struct {
		from, to   string
		reciprocal bool
	}
	var edges []namedEdge
	degrees := make(map[string]int)
	for _, from := range slices.Sorted(maps.Keys(linksByName)) {
		for _, to := range slices.Sorted(maps.Keys(linksByName[from])) {
			if _, found := notesByName[to]; !found || from == to {
				continue
			}
			_, reciprocal := linksByName[to][from]
			if reciprocal && to < from {
				continue
			}
			edges = append(edges, namedEdge{from, to, reciprocal})
			degrees[from]++
			degrees[to]++
		}
	}

	var result VaultMap
	if len(degrees) > vaultMapLimit {
		ranked := slices.SortedFunc(maps.Keys(degrees), func(a, b string) int {
			return cmp.Or(cmp.Compare(degrees[b], degrees[a]), strings.Compare(a, b))
		})
		result.Omitted = len(ranked) - vaultMapLimit
		for _, name := range ranked[vaultMapLimit:] {
			delete(degrees, name)
		}
	}
	ids := make(map[string]int, len(degrees))
	addCard := func(name string) int {
		if id, found := ids[name]; found {
			return id
		}
		note := notesByName[name]
		file := path.Base(note.entry.Path)
		card := VaultMapCard{Label: strings.TrimSuffix(file, path.Ext(file))}
		if len(note.tags) > 0 {
			card.Tag = note.tags[0]
		}
		id := len(result.Cards)
		ids[name] = id
		result.Cards = append(result.Cards, card)
		return id
	}
	for _, edge := range edges {
		if degrees[edge.from] > 0 && degrees[edge.to] > 0 {
			result.Edges = append(result.Edges, VaultMapEdge{From: addCard(edge.from), To: addCard(edge.to), Reciprocal: edge.reciprocal})
		}
	}
	return result
}
