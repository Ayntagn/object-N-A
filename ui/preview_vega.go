package ui

import (
	_ "embed"
	"encoding/json"
	"maps"
	"net/url"
	"path"
	"slices"
	"strings"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
)

const vegaBlockMarker = " data-vega-preview>"
const vegaScriptHashes = "'sha256-Rj89tqQLIOl0e07TjzftCt1QiDj5FBsc+DZnhLB7MMg=' " +
	"'sha256-WMJzWOJvLTGc9i9FvBekyDYvCGRQAd8uyNNB7uQJfH8=' " +
	"'sha256-EtAqz74+xZ75o33UgiouBOKWG1u7Zxu+Zh0iIXFbmdo=' " +
	"'sha256-IrzYc2a3nTkfvgAyowm/WKmIGdVCMCcccPtz+Y2y6VI='"

//go:embed webassets/vega/vega.min.js.gz
var vegaBundle []byte

//go:embed webassets/vega/vega-lite.min.js.gz
var vegaLiteBundle []byte

//go:embed webassets/vega/vega-embed.min.js.gz
var vegaEmbedBundle []byte

//go:embed webassets/vega/d3-dsv.min.js.gz
var dsvBundle []byte

//go:embed preview_vega.js
var previewVegaJS string

const previewVegaCSS = `
  [data-vega-preview] { width: 100%; overflow-x: auto; }
  [data-vega-preview] > .vega-stage { display: block; width: 100%; }
  .vega-source { display: block; white-space: pre-wrap; }
  :is([data-vega-done="rendered"], [data-vega-done="error"]) > .vega-source { display: none; }
  pre.vega-error { color: #e06c75; white-space: pre-wrap; font-size: .85em; margin: 0; padding: 0; background: transparent; }
`

var vegaLoader = previewAssetLoader(
	`typeof window.vegaEmbed === "function" && typeof window.vega?.parse === "function" && typeof window.vegaLite?.compile === "function" && typeof window.d3?.dsvFormat === "function"`,
	"cyberwriter:vega-ready", vegaBundle, vegaLiteBundle, vegaEmbedBundle, dsvBundle,
)

type chartDataReference struct {
	note string
	path string
}

type previewChartFiles struct {
	vault      *workspace.Vault
	references map[chartDataReference]workspace.ChartDataFile
	versions   map[string]string
}

func (files *previewChartFiles) changed() bool {
	if files != nil {
		for reference, file := range files.references {
			if files.vault.ChartDataChanged(file, reference.note, reference.path) {
				return true
			}
		}
		for path, version := range files.versions {
			if version != files.vault.IndexedTextVersion(path) {
				return true
			}
		}
	}
	return false
}

type previewChartAssetKey struct{ path, kind, property string }
type previewChartAsset struct{ source, token string }

type previewChartAssetFrame struct {
	page              *mygo.Page
	previous, current map[previewChartAssetKey]previewChartAsset
}

func (frame *previewChartAssetFrame) reuse(key previewChartAssetKey, source string) string {
	if asset, ok := frame.current[key]; ok {
		return asset.token
	}
	if asset, ok := frame.previous[key]; ok && asset.source == source {
		frame.keep(key, asset)
		return asset.token
	}
	return ""
}

func (frame *previewChartAssetFrame) add(key previewChartAssetKey, source, text, mime string) string {
	if token, ok := pageLocalAssets.AddText(frame.page, mime, text); ok {
		frame.keep(key, previewChartAsset{source: source, token: token})
		return token
	}
	return ""
}

func (frame *previewChartAssetFrame) keep(key previewChartAssetKey, asset previewChartAsset) {
	if frame.current == nil {
		frame.current = make(map[previewChartAssetKey]previewChartAsset)
	}
	frame.current[key] = asset
}

func (frame *previewChartAssetFrame) publish() map[previewChartAssetKey]previewChartAsset {
	for key, prev := range frame.previous {
		if frame.current[key].token != prev.token {
			pageLocalAssets.Revoke(frame.page, prev.token)
		}
	}
	return frame.current
}

// Vega receives a prepared copy of the spec. The source buffer and file stay intact.
func (m *previewMarkup) preparedVega(source, mode string) string {
	if m.resources == nil || !strings.Contains(source, `"url"`) {
		return source
	}
	spec, ok := decodeVegaJSON(source).(map[string]any)
	if !ok {
		return source
	}
	schema, _ := spec["$schema"].(string)
	switch {
	case strings.Contains(schema, "vega-lite"):
		mode = "vega-lite"
	case strings.Contains(schema, "/vega/"):
		mode = "vega"
	}
	if m.prepareVegaSources(spec, mode == "vega-lite") {
		if prepared, err := json.Marshal(spec); err == nil {
			return string(prepared)
		}
	}
	return source
}

func decodeVegaJSON(source string) any {
	if !json.Valid([]byte(source)) {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(source))
	decoder.UseNumber()
	var value any
	_ = decoder.Decode(&value)
	return value
}

func (m *previewMarkup) prepareVegaSources(value any, lite bool) bool {
	changed := false
	switch value := value.(type) {
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(value)) {
			child := value[key]
			if key == "values" {
				continue
			}
			if key == "data" {
				switch datasets := child.(type) {
				case map[string]any:
					changed = m.prepareVegaFile(datasets, lite) || changed
				case []any:
					for _, item := range datasets {
						if data, ok := item.(map[string]any); ok {
							changed = m.prepareVegaFile(data, lite) || changed
						}
					}
				}
			}
			changed = m.prepareVegaSources(child, lite) || changed
		}
	case []any:
		for _, child := range value {
			changed = m.prepareVegaSources(child, lite) || changed
		}
	}
	return changed
}

func (m *previewMarkup) prepareVegaFile(data map[string]any, lite bool) bool {
	raw, ok := data["url"].(string)
	if !ok {
		return false
	}
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || target.Scheme != "" || target.Host != "" || target.Path == "" || path.IsAbs(target.Path) {
		return false
	}
	res := m.resources
	if res.chartFiles == nil {
		res.chartFiles = &previewChartFiles{vault: res.vault, references: make(map[chartDataReference]workspace.ChartDataFile)}
		res.chartContents = make(map[string]string)
	}
	reference := chartDataReference{note: m.relative, path: target.Path}
	file, known := res.chartFiles.references[reference]
	if !known {
		file = res.vault.FindChartData(reference.note, reference.path)
		res.chartFiles.references[reference] = file
	}
	if file.Path == "" {
		return false
	}
	format, _ := data["format"].(map[string]any)
	kind, _ := format["type"].(string)
	if kind == "" {
		kind = strings.TrimPrefix(path.Ext(file.Path), ".")
	}
	kind = strings.ToLower(kind)
	if kind != "" && kind != "json" && kind != "csv" && kind != "tsv" {
		return false
	}
	text, cached := res.chartContents[file.Path]
	if !cached {
		if version := res.vault.IndexedTextVersion(file.Path); version != "" {
			if res.chartFiles.versions == nil {
				res.chartFiles.versions = make(map[string]string)
			}
			res.chartFiles.versions[file.Path] = version
		}
		text, err = res.vault.ReadChartData(file, min(res.remaining, 4<<20))
		if err != nil {
			return false
		}
		res.remaining -= int64(len(text))
		res.chartContents[file.Path] = text
	}
	delimited := kind == "csv" || kind == "tsv"
	property, _ := format["property"].(string)
	key := previewChartAssetKey{path: file.Path, kind: kind}
	if !delimited {
		key.property = property
	}
	var token string
	scoped := res.chartAssets != nil && len(text) >= localAssetThreshold
	if scoped {
		token = res.chartAssets.reuse(key, text)
	}
	var value any = text
	if token == "" && !delimited {
		value = decodeVegaJSON(text)
		if property != "" {
			object, _ := value.(map[string]any)
			value = object[property]
		}
		if value == nil {
			return false
		}
	}
	if scoped && token == "" {
		if delimited {
			token = res.chartAssets.add(key, text, text, "text/plain; charset=utf-8")
		} else if encoded, err := json.Marshal(value); err == nil && len(encoded) >= localAssetThreshold {
			token = res.chartAssets.add(key, text, string(encoded), "application/json")
		}
	}
	if delimited {
		if format == nil {
			format = map[string]any{"type": kind}
			if lite {
				format["parse"] = "auto"
			}
		}
		data["format"] = format
	} else {
		delete(data, "format")
	}
	if token != "" {
		data["url"] = localAssetOrigin + "/" + token
		delete(data, "values")
	} else {
		data["values"] = value
		delete(data, "url")
	}
	return true
}
