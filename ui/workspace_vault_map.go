package ui

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"cyberwriter/internal/core/drawing"
	"cyberwriter/internal/core/vaultindex"
	"github.com/egoist/mygo"
)

//go:embed vault_map_layout.js
var vaultMapLayoutJS string

const vaultMapLayoutPage = `<!doctype html><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src '` + mermaidScriptHash + `'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>Map Vault</title><main></main>`

func (app *workspaceWindow) mapVault() {
	if app.vault == nil || app.modalOpen || app.vaultMapBuilding {
		return
	}
	if app.multiFile.active {
		app.problem = multiFileMarkdownOnly
		return
	}
	app.vaultMapBuilding = true
	vault, index := app.vault, app.index.data
	if app.index.building {
		index = nil
	}
	go func() {
		var err error
		var plan vaultindex.VaultMap
		var scene *drawing.Scene
		if index == nil {
			index, err = index.RefreshMetadata(vault, nil)
		}
		if err == nil {
			plan = index.Map()
			if len(plan.Edges) == 0 {
				err = errors.New("no linked notes to map")
			} else {
				scene, err = renderVaultMap(app.win, plan)
			}
		}
		app.win.Update(func() {
			if app.vault != vault {
				return
			}
			app.vaultMapBuilding = false
			if err != nil {
				app.problem = fmt.Sprintf("Could not map the Vault. %v", err)
				return
			}
			file, err := vault.CreateVaultMap(scene)
			if err != nil {
				app.problem = fmt.Sprintf("Could not save the Vault map. %v", err)
				return
			}
			app.openNewDrawing(file)
			if plan.Omitted > 0 && app.problem == "" {
				app.problem = fmt.Sprintf("Map Vault shows up to 60 notes. %d linked notes were omitted.", plan.Omitted)
			}
		})
	}()
}

func renderVaultMap(parent *mygo.Window, plan vaultindex.VaultMap) (*drawing.Scene, error) {
	if parent.IsDestroyed() || !mygo.WebViewAvailable() {
		return nil, errors.New("Map Vault requires an open window with a system WebView")
	}
	dir, err := os.MkdirTemp("", "cyberwriter-map-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "map.html")
	if err := os.WriteFile(file, []byte(vaultMapLayoutPage), 0o600); err != nil {
		return nil, err
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "Map Vault", Parent: parent, Hidden: true, UseContentSize: true, Width: 1300, Height: 800,
		Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled},
	})
	defer win.Destroy()
	page := win.Page()
	loaded := make(chan error, 1)
	notify := func(err error) {
		select {
		case loaded <- err:
		default:
		}
	}
	page.OnDidFinishLoad(func() { notify(nil) })
	page.OnDidFailLoad(func(err *mygo.LoadError) { notify(err) })
	page.OnRenderProcessGone(func(reason string) { notify(fmt.Errorf("map renderer stopped. %s", reason)) })
	win.OnClosed(func() { notify(errors.New("map renderer closed")) })
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := page.LoadFile(file); err != nil {
		return nil, err
	}
	select {
	case err := <-loaded:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	loader, err := mermaidLoader()
	if err != nil {
		return nil, err
	}
	if _, err := page.EvalContext(ctx, loader); err != nil {
		return nil, err
	}
	return measureVaultMap(ctx, page, plan)
}

func measureVaultMap(ctx context.Context, page *mygo.Page, plan vaultindex.VaultMap) (*drawing.Scene, error) {
	labels := make([]string, len(plan.Cards))
	tags := make(map[string]string)
	for i, card := range plan.Cards {
		labels[i] = card.Label
		if card.Tag != "" {
			tags[card.Tag] = ""
		}
	}
	legendNames := slices.Sorted(maps.Keys(tags))
	legendLabels := make([]string, len(legendNames))
	for i, name := range legendNames {
		legendLabels[i] = "#" + name
	}
	payload, err := json.Marshal(struct {
		Labels []string                  `json:"labels"`
		Edges  []vaultindex.VaultMapEdge `json:"edges"`
		Legend []string                  `json:"legend"`
	}{labels, plan.Edges, legendLabels})
	if err != nil {
		return nil, err
	}
	result, err := page.EvalContext(ctx, "const input = "+string(payload)+";\n"+vaultMapLayoutJS)
	if err != nil {
		return nil, err
	}
	encoded, ok := result.(string)
	var layout struct {
		Cards  []drawing.MapBounds
		Arrows [][]drawing.Point
		Legend []float64
	}
	if !ok || json.Unmarshal([]byte(encoded), &layout) != nil || len(layout.Cards) != len(labels) ||
		len(layout.Arrows) != len(plan.Edges) || len(layout.Legend) != len(legendLabels) {
		return nil, errors.New("the map renderer returned incomplete geometry")
	}
	// EV-UI-080 measured the first two colors in the fixture's sorted legend.
	palette := [...]string{"#ffaaf7", "#b2cc7f", "#a5d8ff", "#ffc9c9", "#ffec99", "#d0bfff", "#99e9f2", "#ffd8a8"}
	legend := make([]drawing.MapLegendItem, len(legendNames))
	for i, tag := range legendNames {
		color := palette[i%len(palette)]
		tags[tag] = color
		legend[i] = drawing.MapLegendItem{Label: legendLabels[i], Color: color, Width: layout.Legend[i]}
	}
	cards := make([]drawing.MapCard, len(labels))
	for i, box := range layout.Cards {
		if box.Width <= 0 || box.Height <= 0 {
			return nil, errors.New("the map renderer returned an empty card")
		}
		cards[i] = drawing.MapCard{Label: labels[i], Color: tags[plan.Cards[i].Tag], Bounds: box}
	}
	arrows := make([]drawing.MapArrow, len(plan.Edges))
	for i, edge := range plan.Edges {
		if len(layout.Arrows[i]) < 2 {
			return nil, errors.New("the map renderer returned an empty arrow")
		}
		arrows[i] = drawing.MapArrow{From: edge.From, To: edge.To, Points: layout.Arrows[i], Reciprocal: edge.Reciprocal}
	}
	return drawing.NewVaultMap(cards, arrows, legend), nil
}
