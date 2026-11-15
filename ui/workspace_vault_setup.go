package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

var vaultSetupPresets = [...]struct {
	label   string
	detail  string
	folders []string
}{
	{"Empty", "Just the vault. Add folders whenever you want them.", nil},
	{"PARA", "Projects, Areas, Resources, Archive.", []string{"Projects", "Areas", "Resources", "Archive"}},
	{"Zettelkasten", "Inbox for capture, Notes for permanent notes, plus sources.", []string{"Inbox", "Notes", "Sources", "Attachments"}},
	{"Work", "Meetings, projects, and somewhere for the files notes refer to.", []string{"Meetings", "Projects", "People", "Attachments"}},
}

type vaultSetupState struct {
	name    string
	preset  int
	folders []*vaultSetupFolder
	nextRow int
}

type vaultSetupFolder struct {
	id   int
	name string
}

func (s *vaultSetupState) addFolder(name string) {
	s.nextRow++
	s.folders = append(s.folders, &vaultSetupFolder{id: s.nextRow, name: name})
}

func (s *vaultSetupState) applyPreset(index int) {
	s.folders = nil
	for _, folder := range vaultSetupPresets[index].folders {
		s.addFolder(folder)
	}
}

func (s *vaultSetupState) plan() (workspace.VaultSetupPlan, error) {
	names := make([]string, len(s.folders))
	for index, row := range s.folders {
		names[index] = row.name
	}
	return workspace.PrepareVaultSetup(s.name, names)
}

func (app *workspaceWindow) rememberVault(root string) {
	recent := app.preferences.RecentVaults
	recent = slices.DeleteFunc(recent, func(existing string) bool { return existing == root })
	app.preferences.RecentVaults = slices.Insert(recent, 0, root)
}

func (app *workspaceWindow) switchVault(root string) {
	if app.openVault(root) {
		app.persistSession()
	}
}

func (app *workspaceWindow) vaultGearMenu(m *mygoui.Menu) {
	if m.Item("Open Vault Folder...").Chosen() {
		app.chooseVault()
	}
	if m.Item("Create New Vault...").Chosen() {
		app.createVault()
	}
	if m.Item("Restore Example Vault").Chosen() {
		app.restoreExampleVault()
	}
	if len(app.preferences.RecentVaults) != 0 {
		m.Separator()
		m.Item("Recent Vaults").Disabled(true)
	}
	for _, root := range app.preferences.RecentVaults {
		if m.Item(filepath.Base(root)).Chosen() {
			app.switchVault(root)
			return
		}
	}
}

func (app *workspaceWindow) revealVault() {
	if app.root != "" {
		mygo.Shell.ShowItemInFolder(app.root)
	}
}

func (app *workspaceWindow) openVaultTrash() {
	if app.root == "" {
		return
	}
	recovery := filepath.Join(app.root, workspace.TrashDirectory)
	info, err := os.Lstat(recovery)
	if err != nil || !info.IsDir() {
		app.problem = "The Vault Trash is empty or unavailable."
	} else if err := mygo.Shell.OpenPath(recovery); err != nil {
		app.problem = fmt.Sprintf("Could not open the Vault Trash. %v", err)
	}
}

func (app *workspaceWindow) chooseVault() {
	if app.modalOpen {
		return
	}
	app.pickVaultFolder(mygo.OpenDialogOptions{
		Title: "Open", DefaultPath: app.root, Directory: true,
		ButtonLabel: "Open Vault", Message: "Select your vault folder (File vault or any Markdown folder)",
	}, nil)
}

func (app *workspaceWindow) createVault() {
	if app.modalOpen {
		return
	}
	app.vaultSetup = &vaultSetupState{}
	app.modalOpen = true
}

func (app *workspaceWindow) chooseVaultLocation() {
	if app.vaultSetup == nil {
		return
	}
	plan, err := app.vaultSetup.plan()
	if err != nil {
		return
	}
	app.vaultSetup = nil
	var parent string
	if app.root != "" {
		parent = filepath.Dir(app.root)
	}
	app.pickVaultFolder(mygo.OpenDialogOptions{
		Title: "Choose a Location", DefaultPath: parent, Directory: true,
		ButtonLabel: "Create Here", Message: fmt.Sprintf("Where should “%s” go?", plan.Name),
		CreateDirectories: true,
	}, &plan)
}

func (app *workspaceWindow) pickVaultFolder(options mygo.OpenDialogOptions, plan *workspace.VaultSetupPlan) {
	app.modalOpen = true
	win := app.win
	// The macOS Vault picker runs as its own modal window.
	if runtime.GOOS != "darwin" {
		options.Parent = win
	}
	go func() {
		paths, pickerErr := mygo.Dialog.Open(options)
		var root string
		var createErr error
		if pickerErr == nil && len(paths) > 0 {
			root = paths[0]
			if plan != nil {
				root, createErr = workspace.CreateVault(root, *plan)
			}
		}
		win.Update(func() {
			app.modalOpen = false
			switch {
			case pickerErr != nil:
				app.problem = fmt.Sprintf("Could not open the Vault picker. %v", pickerErr)
			case createErr != nil:
				app.problem = fmt.Sprintf("Could not create the Vault. %v", createErr)
			case root != "":
				app.switchVault(root)
			}
		})
	}()
}

type vaultSetupColors struct {
	surface, edge, field, control, selected, selectedText mygoui.Color
	secondary, icon, link, focus, disabled, disabledText  mygoui.Color
}

func (app *workspaceWindow) vaultSetupColors(c *mygoui.Context) vaultSetupColors {
	t := c.Theme()
	if app.blackout && !c.Preferences().HighContrast {
		return vaultSetupColors{
			surface: mygoui.RGB(42, 42, 44), edge: mygoui.RGB(61, 61, 63), field: mygoui.RGB(25, 25, 25),
			control: mygoui.RGB(58, 58, 60), selected: mygoui.RGB(72, 147, 242), selectedText: mygoui.RGB(255, 255, 255),
			secondary: mygoui.RGB(198, 199, 208), icon: mygoui.RGB(163, 163, 165),
			link: blackoutTitleAccent, focus: mygoui.RGB(70, 116, 139),
			disabled: mygoui.RGB(50, 50, 52), disabledText: mygoui.RGB(108, 108, 108),
		}
	}
	return vaultSetupColors{
		surface: t.Surface, edge: t.Border, field: t.Background, control: t.SurfaceHover,
		selected: t.Accent, selectedText: t.AccentText, secondary: t.TextMuted, icon: t.TextMuted,
		link: t.Accent, focus: t.Accent, disabled: t.Surface, disabledText: t.TextMuted,
	}
}

func (app *workspaceWindow) drawVaultSetup(c *mygoui.Context) {
	setup := app.vaultSetup
	if setup == nil {
		return
	}
	colors := app.vaultSetupColors(c)
	_, height := c.Size()
	plan, err := setup.plan()
	mygoui.DialogBase(c.Key("new vault"), &app.modalOpen, func(backdrop, panel mygoui.Element) {
		backdrop.Background(mygoui.RGBA(0, 0, 0, 0.2))
		panel.Label("New Vault").Width(460).Padding(21).Radius(17).AlignItems(mygoui.Stretch).
			Background(colors.surface).Border(1, colors.edge).
			Shadow(0, 12, 35, 0, mygoui.RGBA(0, 0, 0, 0.4))
		mygoui.Text(c, "New Vault").Role(mygoui.RoleHeading).FontSize(20).Bold().Height(26)
		app.vaultSetupInput(c.Key("vault name"), &setup.name, "Vault name", colors).
			Margin(18, 0, 0).OnSubmit(app.chooseVaultLocation)
		mygoui.Text(c, "A vault is a folder of Markdown files. You’ll choose where it goes next.").
			FontSize(12).Height(15).TextColor(colors.secondary).Margin(6, 0, 0)
		vaultSetupHeading(c, "START FROM", colors.secondary)
		app.drawVaultPresets(c, setup, colors)
		if setup.preset >= 0 {
			mygoui.Text(c, vaultSetupPresets[setup.preset].detail).FontSize(12).Height(15).
				TextColor(colors.secondary).Margin(8, 0, 0)
		}
		vaultSetupHeading(c, "FOLDERS", colors.secondary)
		addMargin := float32(16)
		if len(setup.folders) != 0 {
			addMargin = 8
			folderHeight := min(float32(len(setup.folders)*30-6), max(24, height-371))
			mygoui.Scroll(c.Key("vault setup folders")).Height(folderHeight).Shrink(0).FillWidth().
				Label("Folders").Margin(8, 0, 0).Children(func() {
				mygoui.Column(c).FillWidth().Gap(6).Children(func() {
					for index, folder := range setup.folders {
						app.drawVaultSetupFolder(c.Key(folder.id), setup, folder, index, colors)
					}
				})
			})
		}
		mygoui.ButtonBase(c.Key("add folder")).Label("Add Folder").Height(16).AlignSelf(mygoui.Start).
			Margin(addMargin, 0, 0).Gap(9).AlignItems(mygoui.Center).OnClick(func() {
			setup.preset = -1
			setup.addFolder("")
		}).Children(func() {
			vaultSetupCircle(c, "plus", colors.link, colors.surface)
			mygoui.Text(c, "Add Folder").FontSize(13).TextColor(colors.link).Role(mygoui.RoleNone)
		})
		if len(setup.folders) != 0 {
			label := fmt.Sprintf("%d folders will be created.", len(plan.Folders))
			if len(plan.Folders) == 1 {
				label = "1 folder will be created."
			}
			mygoui.Text(c, label).FontSize(12).Height(15).TextColor(colors.secondary).Margin(8, 0, 0)
		}
		mygoui.Row(c).Height(24).Gap(8).Justify(mygoui.End).Margin(18, 0, 0).Children(func() {
			vaultSetupButton(c, "Cancel", 66, colors.control, c.Theme().Text, func() {
				app.modalOpen, app.vaultSetup = false, nil
			})
			choose, chooseText := colors.selected, colors.selectedText
			if err != nil {
				choose, chooseText = colors.disabled, colors.disabledText
			}
			vaultSetupButton(c, "Choose Location…", 136, choose, chooseText, app.chooseVaultLocation).Disabled(err != nil)
		})
	})
	if !app.modalOpen {
		app.vaultSetup = nil
	}
}

func vaultSetupHeading(c *mygoui.Context, label string, color mygoui.Color) {
	mygoui.Text(c, label).FontSize(10).FontWeight(650).Height(13).LetterSpacing(1).
		TextColor(color).Margin(18, 0, 0)
}

func (app *workspaceWindow) vaultSetupInput(c *mygoui.Context, value *string, name string, colors vaultSetupColors) mygoui.Element {
	input := mygoui.TextInputBase(c, value).Label(name).Placeholder(name).Height(24).FontSize(13).
		MinWidth(0).Padding(0, 6).Radius(6).Background(colors.field).Border(1, colors.edge)
	if input.Focused() {
		input.Shadow(0, 0, 0, 3, colors.focus)
	}
	return input
}

func (app *workspaceWindow) drawVaultPresets(c *mygoui.Context, setup *vaultSetupState, colors vaultSetupColors) {
	mygoui.RadioGroup(c.Key("vault presets"), func() {
		for index, preset := range vaultSetupPresets {
			button := mygoui.RadioBase(c.Key(index), &setup.preset, index).Label(preset.label).
				Center().Grow(1).Basis(0).MinWidth(0).Height(24).Radius(6).
				OnChange(func() { setup.applyPreset(index) })
			ink := c.Theme().Text
			if setup.preset == index {
				button.Background(colors.selected)
				ink = colors.selectedText
			} else if index > 0 && setup.preset != index-1 {
				button.Draw(func(p *mygoui.Painter, r mygoui.Rect) {
					p.Fill(mygoui.Rect{X: r.X, Y: r.Y + 5, W: 1, H: 14}, colors.icon.Alpha(0.6), 0)
				})
			}
			button.Children(func() {
				mygoui.Text(c, preset.label).FontSize(13).TextColor(ink).SingleLine().Role(mygoui.RoleNone)
			})
		}
	}).Row().Label("Start from").Width(398).Height(24).Gap(0).
		Margin(8, 0, 0).Radius(6).Background(colors.control)
}

func (app *workspaceWindow) drawVaultSetupFolder(c *mygoui.Context, setup *vaultSetupState, folder *vaultSetupFolder, index int, colors vaultSetupColors) {
	mygoui.Row(c).Height(24).Shrink(0).FillWidth().Gap(8).AlignItems(mygoui.Center).Children(func() {
		mygoui.Box(c).Size(17, 24).Center().Children(func() {
			chromeIconColor(c, "folder", 17, colors.icon)
		})
		app.vaultSetupInput(c.Key("name"), &folder.name, "Folder name", colors).Grow(1).
			OnChange(func() { setup.preset = -1 }).OnSubmit(app.chooseVaultLocation)
		mygoui.ButtonBase(c.Key("remove")).Label("Remove").Size(14, 24).Center().OnClick(func() {
			setup.preset = -1
			setup.folders = slices.Delete(setup.folders, index, index+1)
		}).Children(func() { vaultSetupCircle(c, "minus", colors.icon, colors.surface) })
	})
}

func vaultSetupCircle(c *mygoui.Context, symbol string, fill, ink mygoui.Color) {
	mygoui.Box(c).Size(13, 13).Radius(6.5).Background(fill).Center().Children(func() {
		chromeIconColor(c, symbol, 7, ink)
	})
}

func vaultSetupButton(c *mygoui.Context, label string, width float32, fill, ink mygoui.Color, action func()) mygoui.Element {
	button := mygoui.ButtonBase(c).Label(label).Size(width, 24).Radius(6).Center().
		Background(fill).OnClick(action)
	button.Children(func() {
		mygoui.Text(c, label).Role(mygoui.RoleNone).FontSize(13).TextColor(ink).SingleLine()
	})
	return button
}
