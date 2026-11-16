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
	sourceFontDefault       = 14
	sourceFontMin           = 8
	sourceFontMenuMin       = 10
	sourceFontMax           = 32
	sourceFontDefaultFamily = "Maple Mono"
	sourceMapleMonoNFCN     = "Maple Mono NF CN"
	systemMonoLabel         = "System Mono"
	// EV-MYGO-265: Obsidian's local profile applies to both Maple Mono faces.
	mapleMonoOpenTypeFeatures = "calt, cv02, cv05, cv09, cv10, cv61, ss07, ss08, ss09, ss10, ss11"
)

//go:embed fonts/*.ttf.gz
var sourceFontAssets embed.FS

type sourceFontChoice struct {
	name  string
	files []string
	load  sync.Once
}

// EV-UI-055 records the original recommended families. EV-MYGO-258 adds Maple Mono.
var sourceFontChoices = [...]sourceFontChoice{
	{name: sourceFontDefaultFamily, files: []string{"MapleMono-Regular", "MapleMono-Bold", "MapleMono-Italic", "MapleMono-BoldItalic"}},
	{name: "JetBrains Mono", files: []string{"JetBrainsMono-Regular"}},
	{name: "Fira Code", files: []string{"FiraCode-Regular"}},
	{name: "Cascadia Code", files: []string{"CascadiaCode-Regular"}},
	{name: "Space Grotesk", files: []string{"SpaceGrotesk-Variable"}},
	{name: "Nunito Sans", files: []string{"NunitoSans-Variable", "NunitoSans-Italic-Variable"}},
	{name: "Noto Sans", files: []string{"NotoSans-Variable", "NotoSans-Italic-Variable"}},
}

var preferredMapleMonoFamily = sync.OnceValue(func() string {
	for _, family := range mygoui.FontFamilies() {
		if strings.EqualFold(family.Name, sourceMapleMonoNFCN) {
			return family.Name
		}
	}
	return sourceFontDefaultFamily
})

// "Maple Mono" was also the default persisted by build 162. Resolve that
// preference through the local family's availability so existing settings
// pick up the same Maple variant as Obsidian (EV-MYGO-261).
func resolvedSourceFontFamily(family string) string {
	switch family {
	case sourceFontDefaultFamily, sourceMapleMonoNFCN:
		return preferredMapleMonoFamily()
	default:
		return family
	}
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
		return app.tr(systemMonoLabel, "系统等宽字体")
	}
	return resolvedSourceFontFamily(app.preferences.EditorFont)
}

func (app *workspaceWindow) editorFontMenu(m *mygoui.Menu) {
	app.editorFontMenuWithSystem(m, systemMonoLabel)
}

func (app *workspaceWindow) editorFontMenuWithSystem(m *mygoui.Menu, systemLabel string) {
	selected := resolvedSourceFontFamily(app.preferences.EditorFont)
	choose := func(label, family string) {
		if m.Item(label).Checked(selected == resolvedSourceFontFamily(family)).Chosen() {
			app.preferences.EditorFont = family
			app.preferencesChanged()
		}
	}
	choose(systemLabel, "")
	recommended := make(map[string]bool, len(sourceFontChoices)+1)
	for i := range sourceFontChoices {
		name := sourceFontChoices[i].name
		label := resolvedSourceFontFamily(name)
		choose(label, name)
		recommended[strings.ToLower(name)] = true
		recommended[strings.ToLower(label)] = true
	}
	m.Separator()
	families := mygoui.FontFamilies()
	// Surface additional Maple Mono variants first among the installed fonts.
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
		if !recommended[strings.ToLower(family.Name)] {
			choose(family.Name, family.Name)
		}
	}
}

func sourceEditorFontFamily(family string) string {
	family = resolvedSourceFontFamily(family)
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
