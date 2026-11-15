package drawing

// spec: EV-UI-025 / EV-UI-002

import (
	"encoding/json"
	"testing"
)

func TestSceneEditPreservesImportedData(t *testing.T) {
	imported := `{"type":"excalidraw","version":2,"source":"https://excalidraw.com",
		"elements":[{"id":"a","type":"ellipse","x":12,"y":18,"customData":{"attachment":"notes.md"}},
			{"id":"b","type":"future","newData":[1,2,3]}],
		"appState":{"viewBackgroundColor":"#abcdff","other":true},
		"files":{"abc":{"id":"abc","dataURL":"data:image/png;base64,AAAA"}},
		"custom":"value"}`
	scene, err := Parse([]byte(imported))
	if err != nil {
		t.Fatal(err)
	}
	element := scene.Elements()[0].Move(15, -10)
	updated := scene.Replace(0, element).Add(Shape(Line, []Point{{10, 20}, {5, 8}}))
	out, err := updated.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Custom   string
		Elements []struct {
			X, Y       float64
			CustomData map[string]string
			NewData    []int
		}
		AppState map[string]json.RawMessage
		Files    map[string]map[string]string
	}
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Elements) != 3 || want.Elements[0].X != 27 || want.Elements[0].Y != 8 || want.Elements[0].CustomData["attachment"] != "notes.md" ||
		len(want.Elements[1].NewData) != 3 || want.Custom != "value" || string(want.AppState["other"]) != "true" ||
		want.Files["abc"]["dataURL"] != "data:image/png;base64,AAAA" {
		t.Fatalf("lost scene data: %s", out)
	}
	if original := scene.Elements()[0]; original.X != 12 || original.Y != 18 {
		t.Fatalf("editing changed the original scene: %+v", original)
	}
}

func TestEditTextRetainsImportedTextFieldsAndBindings(t *testing.T) {
	imported := `{"type":"excalidraw","version":2,"source":"https://excalidraw.com",
		"elements":[
			{"id":"solo","type":"text","text":"old\nlabel","originalText":"old label","x":12,"y":34,
				"width":80,"height":46,"fontSize":20,"fontFamily":2,"lineHeight":1.15,"textAlign":"right",
				"autoResize":false,"version":4,"customData":{"attachment":"notes.md"}},
			{"id":"label","type":"text","text":"bound","containerId":"shape"},
			{"id":"shape","type":"rectangle","boundElements":[{"id":"label","type":"text"}]}
		],"appState":{},"files":{}}`
	scene, err := Parse([]byte(imported))
	if err != nil {
		t.Fatal(err)
	}
	if updated := scene.EditText(1, "incorrect", "incorrect", 40, 23); updated != scene {
		t.Fatal("a text bound to its shape was edited independently")
	}
	next := scene.EditText(0, "new label", "new\nlabel", 72, 46)
	data, err := next.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Elements []struct {
			ID, Text, OriginalText, TextAlign, ContainerID string
			X, Y, Width, LineHeight                        float64
			Version, FontFamily                            int
			AutoResize                                     bool
			CustomData                                     map[string]string
			BoundElements                                  []struct{ ID string }
		}
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	first := file.Elements[0]
	if first.ID != "solo" || first.Text != "new\nlabel" || first.OriginalText != "new label" ||
		first.X != 12 || first.Y != 34 || first.Width != 80 || first.FontFamily != 2 || first.LineHeight != 1.15 ||
		first.TextAlign != "right" || first.AutoResize || first.Version != 5 || first.CustomData["attachment"] != "notes.md" {
		t.Fatalf("unexpected edited text: %+v", first)
	}
	if file.Elements[1].Text != "bound" || file.Elements[1].ContainerID != "shape" ||
		len(file.Elements[2].BoundElements) != 1 || file.Elements[2].BoundElements[0].ID != "label" {
		t.Fatalf("binding lost: %s", data)
	}
}
