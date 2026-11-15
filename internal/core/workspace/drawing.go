package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"cyberwriter/internal/core/drawing"
)

// Drawing is an Excalidraw scene opened through the Vault. Its stored revision
// must match the file on disk before a later edit can replace it.
type Drawing struct {
	Path     string
	Scene    *drawing.Scene
	document Document
}

// Relocated keeps the open scene and saved revision after its file is moved.
func (d Drawing) Relocated(move AttachmentMove) Drawing {
	d.document = move.RelocateDocument(d.document)
	d.Path = d.document.Path
	return d
}

func (v *Vault) CreateDrawing() (Drawing, error) {
	return v.createDrawing("Drawing", drawing.NewScene())
}

func (v *Vault) CreateVaultMap(scene *drawing.Scene) (Drawing, error) {
	return v.createDrawing("Vault Map", scene)
}

func (v *Vault) createDrawing(base string, scene *drawing.Scene) (Drawing, error) {
	data, err := scene.Encode()
	if err != nil {
		return Drawing{}, err
	}
	content := string(data)
	v.writes.Lock()
	defer v.writes.Unlock()
	for number := 1; ; number++ {
		name := base + ".excalidraw"
		if number > 1 {
			name = fmt.Sprintf("%s %d.excalidraw", base, number)
		}
		if _, err := v.root.Lstat(name); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return Drawing{}, err
		}
		err := v.writeNewLifecycleFile(name, content, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return Drawing{}, fmt.Errorf("create %q: %w", name, err)
		}
		return Drawing{Path: name, Scene: scene, document: lifecycleDocument(v, name, content)}, nil
	}
}

func (v *Vault) ReadDrawing(relative string) (Drawing, error) {
	native, err := lifecyclePath(relative)
	if err != nil {
		return Drawing{}, err
	}
	if !strings.EqualFold(path.Ext(relative), ".excalidraw") {
		return Drawing{}, fmt.Errorf("invalid Vault drawing path %q", relative)
	}
	data, _, err := readRegularFile(v.root, native)
	if err != nil {
		return Drawing{}, err
	}
	scene, err := drawing.Parse(data)
	if err != nil {
		return Drawing{}, fmt.Errorf("read %q: %w", relative, err)
	}
	return Drawing{Path: relative, Scene: scene, document: lifecycleDocument(v, relative, string(data))}, nil
}

func (v *Vault) SaveDrawing(file Drawing, scene *drawing.Scene) (Drawing, error) {
	if file.Scene == nil || scene == nil || file.Path != file.document.Path {
		return Drawing{}, errors.New("open the drawing in this Vault before saving")
	}
	data, err := scene.Encode()
	if err != nil {
		return Drawing{}, err
	}
	document, err := v.SaveDocument(file.document, string(data))
	if err != nil {
		return Drawing{}, err
	}
	return Drawing{Path: file.Path, Scene: scene, document: document}, nil
}
