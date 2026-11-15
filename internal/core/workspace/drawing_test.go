package workspace

// spec: EV-UI-025 / EV-UI-002

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cyberwriter/internal/core/drawing"
)

func TestCreateDrawingSaveAndReopen(t *testing.T) {
	root := t.TempDir()
	vault, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	file, err := vault.CreateDrawing()
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != "Drawing.excalidraw" {
		t.Fatalf("path = %q", file.Path)
	}
	scene := file.Scene.Add(drawing.Shape(drawing.Rectangle, []drawing.Point{{50, 90}, {10, 30}}))
	scene = scene.Add(drawing.Shape(drawing.FreeDraw, []drawing.Point{{20, 10}, {24, 16}, {15, 18}}))
	if _, err := vault.SaveDrawing(file, scene); err != nil {
		t.Fatal(err)
	}
	second, err := vault.CreateDrawing()
	if err != nil {
		t.Fatal(err)
	}
	if second.Path == file.Path {
		t.Fatal("second drawing reused the original path")
	}
	content, err := os.ReadFile(filepath.Join(root, file.Path))
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Type     string
		Version  int
		Source   string
		Elements []struct {
			ID                  string
			Type                string
			X, Y, Width, Height float64
			Points              [][2]float64
		}
		AppState struct {
			ViewBackgroundColor string
			GridSize            int
		}
		Files map[string]json.RawMessage
	}
	if err := json.Unmarshal(content, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Type != "excalidraw" || stored.Version != 2 || stored.Source != "cyberwriter" || stored.Files == nil ||
		stored.AppState.ViewBackgroundColor != "#ffffff" || stored.AppState.GridSize != 20 {
		t.Fatalf("invalid scene envelope: %s", content)
	}
	if len(stored.Elements) != 2 {
		t.Fatalf("expected two elements, got %d", len(stored.Elements))
	}
	rect, stroke := stored.Elements[0], stored.Elements[1]
	if rect.ID == "" || stroke.ID == "" || rect.ID == stroke.ID {
		t.Fatalf("elements need distinct IDs: %q, %q", rect.ID, stroke.ID)
	}
	if rect.Type != "rectangle" || rect.X != 10 || rect.Y != 30 || rect.Width != 40 || rect.Height != 60 {
		t.Fatalf("invalid rectangle: %+v", rect)
	}
	if stroke.Type != "freedraw" || stroke.X != 20 || stroke.Y != 10 || len(stroke.Points) != 3 || stroke.Points[2] != [2]float64{-5, 8} {
		t.Fatalf("invalid stroke: %+v", stroke)
	}
	reopened, err := vault.ReadDrawing(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if elements := reopened.Scene.Elements(); len(elements) != 2 || elements[1].Type != drawing.FreeDraw {
		t.Fatalf("reopened elements = %+v", elements)
	}
}

func TestSaveDrawingDetectsExternalChange(t *testing.T) {
	root := t.TempDir()
	vault, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vault.Close() })
	file, err := vault.CreateDrawing()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, file.Path)
	external := `{"type":"excalidraw","version":2,"source":"external","elements":[],"appState":{},"files":{}}`
	if err := os.WriteFile(target, []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	scene := file.Scene.Add(drawing.Shape(drawing.Ellipse, []drawing.Point{{10, 20}, {30, 50}}))
	if _, err := vault.SaveDrawing(file, scene); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("conflicting save returned %v", err)
	}
	saved, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != external {
		t.Fatalf("external scene was overwritten: %s", saved)
	}
}
