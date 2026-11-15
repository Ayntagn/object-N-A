package ui

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime"
	"slices"
	"strings"
	"sync"

	mygoui "github.com/egoist/mygo/ui"
)

const (
	sourceFontDefault = 14
	sourceFontMin     = 8
	sourceFontMenuMin = 10
	sourceFontMax     = 32
	systemMonoLabel   = "System Mono"
)

//go:embed fonts/*.ttf.gz
var sourceFontAssets embed.FS

type sourceFontChoice struct {
	name  string
	files []string
	load  sync.Once
}

// EV-UI-055 records the recommended families and their order.
var sourceFontChoices = [...]sourceFontChoice{
	{name: "JetBrains Mono", files: []string{"JetBrainsMono-Regular"}},
	{name: "Fira Code", files: []string{"FiraCode-Regular"}},
	{name: "Cascadia Code", files: []string{"CascadiaCode-Regular"}},
	{name: "Space Grotesk", files: []string{"SpaceGrotesk-Variable"}},
	{name: "Nunito Sans", files: []string{"NunitoSans-Variable", "NunitoSans-Italic-Variable"}},
	{name: "Noto Sans", files: []string{"NotoSans-Variable", "NotoSans-Italic-Variable"}},
}

func (app *workspaceWindow) sourceFontSize() int {
	if app.preferences.FontSize != 0 {
		return app.preferences.FontSize
	}
	return sourceFontDefault
}

func (app *workspaceWindow) setSourceFontSize(size int) {
	app.preferences.FontSize = size
	app.preferencesChanged()
}

func (app *workspaceWindow) changeFontSize(step int) {
	if step == 0 {
		app.setSourceFontSize(sourceFontDefault)
		return
	}
	app.setSourceFontSize(min(sourceFontMax, max(sourceFontMin, app.sourceFontSize()+step)))
}

func (app *workspaceWindow) changeToolbarFontSize(step int) {
	size := app.sourceFontSize()
	if step < 0 {
		app.setSourceFontSize(max(sourceFontMenuMin, size+step))
		return
	}
	app.setSourceFontSize(min(sourceFontMax, size+step))
}

func (app *workspaceWindow) editorFontSizeMenu(m *mygoui.Menu) {
	modifier := "Ctrl+"
	if runtime.GOOS == "darwin" {
		modifier = "⌘"
	}
	if m.Item("Increase (" + modifier + "+)").Chosen() {
		app.changeToolbarFontSize(1)
	}
	if m.Item("Decrease (" + modifier + "-)").Chosen() {
		app.changeToolbarFontSize(-1)
	}
	m.Separator()
	if m.Item("Reset to Default").Chosen() {
		app.changeFontSize(0)
	}
	m.Separator()
	for _, size := range []int{12, 14, 16, 18, 20, 24} {
		if m.Item(fmt.Sprintf("%d pt", size)).Chosen() {
			app.setSourceFontSize(size)
		}
	}
}

func (app *workspaceWindow) editorFontLabel() string {
	if app.preferences.EditorFont == "" {
		return systemMonoLabel
	}
	return app.preferences.EditorFont
}

func (app *workspaceWindow) editorFontMenu(m *mygoui.Menu) {
	choose := func(label, family string) {
		if m.Item(label).Checked(app.preferences.EditorFont == family).Chosen() {
			app.preferences.EditorFont = family
			app.preferencesChanged()
		}
	}
	choose(systemMonoLabel, "")
	for i := range sourceFontChoices {
		name := sourceFontChoices[i].name
		choose(name, name)
	}
	m.Separator()
	families := mygoui.FontFamilies()
	// Surface installed Maple Mono variants without changing the built-in group.
	installedOrder := func(name string) int {
		if name == "maple mono" || strings.HasPrefix(name, "maple mono ") {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(families, func(a, b mygoui.FontFamily) int {
		an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if groupOrder := installedOrder(an) - installedOrder(bn); groupOrder != 0 {
			return groupOrder
		}
		return strings.Compare(an, bn)
	})
	for _, family := range families {
		choose(family.Name, family.Name)
	}
}

func sourceEditorFontFamily(family string) string {
	if family == "" {
		return ""
	}
	loadBundledFont(family)
	return family + ", monospace"
}

func loadBundledFont(family string) {
	for i := range sourceFontChoices {
		font := &sourceFontChoices[i]
		if font.name != family {
			continue
		}
		font.load.Do(func() {
			for _, file := range font.files {
				if err := registerSourceFont(file, family); err != nil {
					log.Printf("Register bundled font %s: %v", family, err)
					break
				}
			}
		})
		break
	}
}

func registerSourceFont(file, family string) error {
	data, err := bundledSourceFont(file)
	if err != nil {
		return err
	}
	return mygoui.RegisterFont(data, family)
}

func bundledSourceFont(file string) ([]byte, error) {
	font, err := sourceFontAssets.Open("fonts/" + file + ".ttf.gz")
	if err != nil {
		return nil, err
	}
	defer font.Close()
	reader, err := gzip.NewReader(font)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(reader)
	return data, errors.Join(err, reader.Close())
}
