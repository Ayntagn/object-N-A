// Package drawing models the portion of the Excalidraw scene format edited by
// the native canvas. Imported fields and unsupported elements remain in the file.
package drawing

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Kind string

const (
	Rectangle Kind = "rectangle"
	Diamond   Kind = "diamond"
	Ellipse   Kind = "ellipse"
	Arrow     Kind = "arrow"
	Line      Kind = "line"
	FreeDraw  Kind = "freedraw"
	Text      Kind = "text"
)

type Point [2]float64

const (
	DefaultTextFontSize   = 20
	DefaultTextLineHeight = 1.15
	HelveticaFontFamily   = 2
)

// Scene is a copy-on-write snapshot. The UI can keep earlier scenes for undo.
type Scene struct {
	fields     map[string]json.RawMessage
	elements   []Element
	background string
}

func NewScene() *Scene {
	return &Scene{
		fields: map[string]json.RawMessage{
			"type":     json.RawMessage(`"excalidraw"`),
			"version":  json.RawMessage("2"),
			"source":   json.RawMessage(`"cyberwriter"`),
			"appState": json.RawMessage(`{"viewBackgroundColor":"#ffffff","gridSize":20}`),
			"files":    json.RawMessage("{}"),
		},
		elements:   []Element{},
		background: "#ffffff",
	}
}

func Parse(data []byte) (*Scene, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("drawing is not UTF-8")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	var kind string
	if err := json.Unmarshal(fields["type"], &kind); err != nil || kind != "excalidraw" {
		return nil, errors.New("file is not an Excalidraw scene")
	}
	var version int
	if err := json.Unmarshal(fields["version"], &version); err != nil || version < 1 {
		return nil, errors.New("drawing is missing a valid scene version")
	}
	var elements []Element
	if err := json.Unmarshal(fields["elements"], &elements); err != nil || elements == nil {
		return nil, errors.New("drawing is missing an elements array")
	}
	delete(fields, "elements")
	state := struct {
		Background string `json:"viewBackgroundColor"`
	}{Background: "#ffffff"}
	if raw := fields["appState"]; raw != nil {
		// App state is preserved even if it contains a newer format.
		_ = json.Unmarshal(raw, &state)
	}
	return &Scene{fields: fields, elements: elements, background: state.Background}, nil
}

func (s *Scene) Encode() ([]byte, error) {
	elements, err := json.Marshal(s.elements)
	if err != nil {
		return nil, err
	}
	fields := maps.Clone(s.fields)
	fields["elements"] = elements
	data, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Elements returns the scene's ordered elements. Call Add or Replace to edit.
// The returned slice and any Points in its elements are read-only.
func (s *Scene) Elements() []Element { return s.elements }

func (s *Scene) Background() string { return s.background }

func (s *Scene) Add(element Element) *Scene {
	next := *s
	next.elements = make([]Element, len(s.elements)+1)
	copy(next.elements, s.elements)
	next.elements[len(s.elements)] = identifyElement(element)
	return &next
}

func identifyElement(element Element) Element {
	if element.ID == "" {
		element.ID = rand.Text()
		element.Version = 1
		element.Seed, element.VersionNonce = randomNumber(), randomNumber()
		element.Created = time.Now().UnixMilli()
		element.Updated = element.Created
	}
	return element
}

func (s *Scene) Replace(index int, element Element) *Scene {
	next := *s
	next.elements = slices.Clone(s.elements)
	next.elements[index] = element
	return &next
}

// EditText replaces the displayed and source text of an unbound element.
// A fixed-width element keeps the imported width after the UI wraps its text.
func (s *Scene) EditText(index int, source, displayed string, width, height float64) *Scene {
	if index < 0 || index >= len(s.elements) {
		return s
	}
	e := s.elements[index]
	if e.Type != Text || !e.Editable() || e.IsDeleted || source == e.SourceText() || strings.TrimSpace(source) == "" {
		return s
	}
	if !e.AutoResize && e.Width > 0 {
		width = e.Width
	}
	return s.Replace(index, e.edited(map[string]any{
		"text": displayed, "originalText": source, "width": width, "height": height,
	}))
}

func (s *Scene) Erase(indices []int) *Scene {
	if len(indices) == 0 {
		return s
	}
	next := *s
	next.elements = slices.Clone(s.elements)
	for _, index := range indices {
		next.elements[index] = next.elements[index].edited(map[string]any{"isDeleted": true})
	}
	return &next
}

type Element struct {
	ID              string         `json:"id"`
	Type            Kind           `json:"type"`
	X               float64        `json:"x"`
	Y               float64        `json:"y"`
	Width           float64        `json:"width"`
	Height          float64        `json:"height"`
	Angle           float64        `json:"angle"`
	StrokeColor     string         `json:"strokeColor"`
	BackgroundColor string         `json:"backgroundColor"`
	StrokeWidth     float64        `json:"strokeWidth"`
	StrokeStyle     string         `json:"strokeStyle"`
	Opacity         float64        `json:"opacity"`
	Points          []Point        `json:"points,omitempty"`
	Text            string         `json:"text,omitempty"`
	OriginalText    string         `json:"originalText,omitempty"`
	FontSize        float64        `json:"fontSize,omitempty"`
	FontFamily      int            `json:"fontFamily,omitempty"`
	LineHeight      float64        `json:"lineHeight,omitempty"`
	TextAlign       string         `json:"textAlign,omitempty"`
	VerticalAlign   string         `json:"verticalAlign,omitempty"`
	AutoResize      bool           `json:"autoResize,omitempty"`
	StartArrowhead  string         `json:"startArrowhead,omitempty"`
	EndArrowhead    string         `json:"endArrowhead,omitempty"`
	Link            string         `json:"link,omitempty"`
	ContainerID     string         `json:"containerId,omitempty"`
	BoundElements   []BoundElement `json:"boundElements,omitempty"`
	StartBinding    *Binding       `json:"startBinding,omitempty"`
	EndBinding      *Binding       `json:"endBinding,omitempty"`
	Roundness       *Roundness     `json:"roundness,omitempty"`
	Roughness       int            `json:"roughness"`
	Elbowed         bool           `json:"elbowed,omitempty"`
	IsDeleted       bool           `json:"isDeleted"`
	Locked          bool           `json:"locked"`
	Version         int            `json:"version"`
	VersionNonce    int            `json:"versionNonce"`
	Seed            int            `json:"seed"`
	Created         int64          `json:"created,omitempty"`
	Updated         int64          `json:"updated"`

	raw     json.RawMessage
	grouped bool
}

type BoundElement struct {
	ID   string `json:"id"`
	Type Kind   `json:"type"`
}

type Binding struct {
	ElementID string  `json:"elementId"`
	Focus     float64 `json:"focus"`
	Gap       float64 `json:"gap"`
}

type Roundness struct {
	Type int `json:"type"`
}

func (e *Element) UnmarshalJSON(data []byte) error {
	type renderElement Element
	element := struct {
		renderElement
		GroupIDs []string `json:"groupIds"`
		FrameID  string   `json:"frameId"`
	}{
		renderElement: renderElement{
			StrokeColor: "#1e1e1e", BackgroundColor: "transparent",
			StrokeWidth: 2, Opacity: 100, FontSize: DefaultTextFontSize, AutoResize: true,
		},
	}
	if err := json.Unmarshal(data, &element); err == nil {
		*e = Element(element.renderElement)
		e.grouped = len(element.GroupIDs) != 0 || element.FrameID != ""
	} else {
		// Preserve records that this editor cannot interpret.
		*e = Element{}
	}
	e.raw = bytes.Clone(data)
	return nil
}

func (e Element) MarshalJSON() ([]byte, error) {
	if e.raw != nil {
		return e.raw, nil
	}
	type renderElement Element
	type exportElement struct {
		renderElement
		FillStyle string         `json:"fillStyle"`
		GroupIDs  []string       `json:"groupIds"`
		FrameID   *string        `json:"frameId"`
		Roundness *Roundness     `json:"roundness"`
		Bound     []BoundElement `json:"boundElements"`
		Index     *string        `json:"index"`
		Link      *string        `json:"link"`
	}
	base := exportElement{
		renderElement: renderElement(e), FillStyle: "solid", GroupIDs: []string{},
		Roundness: e.Roundness, Bound: e.BoundElements,
	}
	if e.Link != "" {
		base.Link = &e.Link
	}
	switch e.Type {
	case Arrow, Line:
		var start, end *string
		if e.StartArrowhead != "" {
			start = &e.StartArrowhead
		}
		if e.EndArrowhead != "" {
			end = &e.EndArrowhead
		}
		return json.Marshal(struct {
			exportElement
			StartBinding   *Binding `json:"startBinding"`
			EndBinding     *Binding `json:"endBinding"`
			StartArrowhead *string  `json:"startArrowhead"`
			EndArrowhead   *string  `json:"endArrowhead"`
			Elbowed        bool     `json:"elbowed"`
		}{exportElement: base, StartBinding: e.StartBinding, EndBinding: e.EndBinding,
			StartArrowhead: start, EndArrowhead: end, Elbowed: e.Elbowed})
	case FreeDraw:
		return json.Marshal(struct {
			exportElement
			Pressures        []float64 `json:"pressures"`
			SimulatePressure bool      `json:"simulatePressure"`
		}{exportElement: base, Pressures: []float64{}, SimulatePressure: true})
	case Text:
		var container *string
		if e.ContainerID != "" {
			container = &e.ContainerID
		}
		return json.Marshal(struct {
			exportElement
			ContainerID  *string  `json:"containerId"`
			BaseFontSize *float64 `json:"baseFontSize"`
		}{exportElement: base, ContainerID: container})
	default:
		return json.Marshal(base)
	}
}

// Shape converts a gesture in scene coordinates to an Excalidraw element.
// Path elements start at the first point and store the other points relative to it.
func Shape(kind Kind, points []Point) Element {
	first, last := points[0], points[len(points)-1]
	e := newElement(kind, min(first[0], last[0]), min(first[1], last[1]),
		math.Abs(last[0]-first[0]), math.Abs(last[1]-first[1]))
	switch kind {
	case FreeDraw, Arrow, Line:
		e.X, e.Y = first[0], first[1]
		e.Points = make([]Point, len(points))
		minX, minY, maxX, maxY := e.X, e.Y, e.X, e.Y
		for i, point := range points {
			e.Points[i] = Point{point[0] - e.X, point[1] - e.Y}
			minX, maxX = min(minX, point[0]), max(maxX, point[0])
			minY, maxY = min(minY, point[1]), max(maxY, point[1])
		}
		e.Width, e.Height = maxX-minX, maxY-minY
		if kind == Arrow {
			e.EndArrowhead = "arrow"
		}
	}
	return e
}

func NewText(position Point, value string, width, height float64) Element {
	e := newElement(Text, position[0], position[1], width, height)
	e.Text, e.OriginalText = value, value
	e.FontSize, e.FontFamily, e.LineHeight = DefaultTextFontSize, HelveticaFontFamily, DefaultTextLineHeight
	e.TextAlign, e.VerticalAlign, e.AutoResize = "left", "top", true
	return e
}

func newElement(kind Kind, x, y, width, height float64) Element {
	return Element{
		Type: kind, X: x, Y: y, Width: width, Height: height,
		StrokeColor: "#1e1e1e", BackgroundColor: "transparent", StrokeWidth: 2,
		StrokeStyle: "solid", Opacity: 100,
	}
}

func (e Element) SourceText() string {
	if e.OriginalText != "" {
		return e.OriginalText
	}
	return e.Text
}

func (e Element) Move(dx, dy float64) Element {
	return e.edited(map[string]any{"x": e.X + dx, "y": e.Y + dy})
}

// Editable reports whether the native editor can move or erase the element
// without leaving related elements behind.
func (e Element) Editable() bool {
	return !e.Locked && !e.grouped && e.ContainerID == "" && len(e.BoundElements) == 0 &&
		e.StartBinding == nil && e.EndBinding == nil
}

// edited keeps imported fields intact, including any the canvas cannot display.
func (e Element) edited(changes map[string]any) Element {
	raw := e.raw
	if raw == nil {
		var err error
		raw, err = e.MarshalJSON()
		if err != nil {
			return e
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return e
	}
	changes["version"] = e.Version + 1
	changes["versionNonce"] = randomNumber()
	changes["updated"] = time.Now().UnixMilli()
	for key, value := range changes {
		encoded, err := json.Marshal(value)
		if err != nil {
			return e
		}
		fields[key] = encoded
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return e
	}
	var updated Element
	_ = updated.UnmarshalJSON(data)
	return updated
}

func randomNumber() int {
	var data [4]byte
	rand.Read(data[:])
	return int(binary.LittleEndian.Uint32(data[:]) & 0x7fffffff)
}
