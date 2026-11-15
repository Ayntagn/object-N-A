package vaultindex

import "slices"

// GraphLayout uses IDs into Graph.Notes for both its order and edges. Both
// slices are read-only so side and floating canvases can share the layout.
type GraphLayout struct {
	Order []int
	Edges []GraphEdge
}

// Layout orders nodes by link-connected components (EV-UI-202).
// scanOrder contains each note ID once in the localized Vault scan order.
func (graph *Graph) Layout(scanOrder []int) GraphLayout {
	if graph.layoutTargets == nil {
		return GraphLayout{
			Order: connectedGraphOrder(scanOrder, graph.Edges),
			Edges: graph.Edges,
		}
	}

	names := make([]string, len(graph.Notes))
	for id, note := range graph.Notes {
		names[id] = graphNoteName(note.Path)
	}
	byName := make(map[string]int, len(names))
	for _, id := range scanOrder {
		byName[names[id]] = id
	}
	var scanEdges []GraphEdge
	for from, name := range names {
		for _, target := range graph.layoutTargets[name] {
			if to, resolved := byName[target.Name]; resolved && from != to {
				scanEdges = append(scanEdges, GraphEdge{From: from, To: to})
			}
		}
	}
	order := connectedGraphOrder(scanOrder, scanEdges)
	// Drawing resolves short names again in the new model order. Preserve
	// the original IDs so node clicks still open the exact note path.
	for _, id := range order {
		byName[names[id]] = id
	}
	var edges []GraphEdge
	seen := make(map[GraphEdge]struct{}, len(scanEdges))
	for _, from := range order {
		if byName[names[from]] != from {
			continue
		}
		for _, target := range graph.layoutTargets[names[from]] {
			to, resolved := byName[target.Name]
			if !resolved || from == to {
				continue
			}
			pair := GraphEdge{From: min(from, to), To: max(from, to)}
			if _, duplicate := seen[pair]; !duplicate {
				seen[pair] = struct{}{}
				edges = append(edges, GraphEdge{From: from, To: to})
			}
		}
	}
	return GraphLayout{Order: order, Edges: edges}
}

func connectedGraphOrder(scanOrder []int, edges []GraphEdge) []int {
	if len(edges) == 0 {
		return scanOrder
	}
	neighbors := make([][]int, len(scanOrder))
	for _, edge := range edges {
		neighbors[edge.From] = append(neighbors[edge.From], edge.To)
		neighbors[edge.To] = append(neighbors[edge.To], edge.From)
	}
	rank := make([]int, len(scanOrder))
	for i, id := range scanOrder {
		rank[id] = i
	}

	visited := make([]bool, len(scanOrder))
	order := make([]int, 0, len(scanOrder))
	for _, root := range scanOrder {
		if visited[root] {
			continue
		}
		visited[root] = true
		// The newly appended portion is also the component's FIFO queue.
		front := len(order)
		order = append(order, root)
		for front < len(order) {
			adjacent := neighbors[order[front]]
			slices.SortFunc(adjacent, func(a, b int) int { return rank[a] - rank[b] })
			for _, id := range adjacent {
				if !visited[id] {
					visited[id] = true
					order = append(order, id)
				}
			}
			front++
		}
	}
	return order
}
