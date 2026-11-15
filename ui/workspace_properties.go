package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type workspaceProperties struct {
	buffer   *editor.Buffer
	version  uint64
	model    workspace.Properties
	parseErr error
	problem  string
	inputs   map[string]*propertyInput
	metrics  workspace.TextMetrics
	lines    int
	infoDoc  workspace.Document
	info     workspace.DocumentInfo

	adding   bool
	addFocus bool
	key      string
	kind     workspace.PropertyKind
}

type propertyInput struct {
	text    string
	checked bool
	date    time.Time
}

func (p *workspaceProperties) refresh(app *workspaceWindow) {
	snapshot := app.draft.Snapshot()
	if p.buffer != app.draft {
		*p = workspaceProperties{buffer: app.draft}
	}
	if p.version != snapshot.Version() || p.inputs == nil {
		p.version = snapshot.Version()
		source := snapshot.String()
		p.model, p.parseErr = workspace.ReadProperties(source)
		p.inputs = make(map[string]*propertyInput, len(p.model.Rows))
		_, _, body := workspace.SplitFrontmatter(source)
		p.metrics = workspace.MeasureText(body)
		p.metrics.Characters = app.metrics.Characters
		p.lines = strings.Count(source, "\n") + 1
	}
	if p.infoDoc != app.document {
		p.infoDoc, p.info = app.document, workspace.DocumentInfo{}
		storage := app.vault
		if app.storage != nil {
			storage = app.storage
		}
		if storage != nil && app.document.Path != "" {
			p.info, _ = storage.DocumentInfo(app.document.Path)
		}
	}
}

func (p *workspaceProperties) apply(app *workspaceWindow, change workspace.PropertyEdit) bool {
	if app.modalOpen || app.draft == nil || &app.properties != p || app.draft != p.buffer {
		return false
	}
	app.wys.sync(app)
	before := app.workingText()
	model, err := workspace.ReadProperties(before)
	var after string
	if err == nil {
		after, err = model.Edit(change)
	}
	if err != nil {
		p.problem = err.Error()
		return false
	}
	p.problem = ""
	if after == before {
		return true
	}
	prefix, _, body := workspace.SplitFrontmatter(before)
	if !strings.HasSuffix(after, body) {
		p.problem = "Could not preserve the note body."
		return false
	}
	app.draft.Edit(editor.Edit{Start: 0, End: len(prefix), Text: after[:len(after)-len(body)]})
	if app.wys.session != nil {
		app.wys.session.RefreshFrontmatter(app.draft)
	}
	return true
}

func (app *workspaceWindow) propertiesPanel(c *mygoui.Context) {
	app.workingText()
	p := &app.properties
	p.refresh(app)
	_, muted := app.propertyInks(c)
	mygoui.Scroll(c.Key(app.draft)).Grow(1).MinHeight(0).Margin(0, 3, 0, 4).Padding(12, 0, 8).
		Label("Document properties").Children(func() {
		mygoui.Column(c).FillWidth().Children(func() {
			mygoui.Row(c).Height(18).Margin(0, 12, 2).AlignItems(mygoui.Center).Gap(8).Children(func() {
				chromeIconColor(c, "tag", 14, muted)
				mygoui.Text(c, "Properties").FontSize(12).FixedLineHeight(14).FontWeight(600).TextColor(muted).Role(mygoui.RoleHeading)
				mygoui.Spacer(c)
				icon, label := "plus", "Add property"
				if p.adding {
					icon, label = "minus", "Cancel new property"
				}
				chromeButtonIcon(c, icon, label, func() {
					p.adding = !p.adding
					p.addFocus, p.key, p.kind = p.adding, "", workspace.PropertyText
				}, 16, muted).Size(16, 18).Disabled(p.parseErr != nil || app.modalOpen)
			})
			if p.parseErr != nil {
				mygoui.Text(c, "Could not read properties. "+p.parseErr.Error()).FontSize(12).Wrap().Margin(0, 12, 8).Role(mygoui.RoleStatus)
			} else if len(p.model.Rows) == 0 && !p.adding {
				mygoui.Text(c, "No properties yet - tap + to add one, or write a --- block at the top of the note.").
					FontSize(12).TextColor(c.Theme().TextMuted).Wrap().Margin(0, 12, 10)
			}
			for i, row := range p.model.Rows {
				p.row(c.Key(i), app, row)
			}
			if p.adding {
				p.addRow(c, app)
			}
			if p.problem != "" {
				mygoui.Text(c, p.problem).FontSize(12).Wrap().Margin(4, 12, 0).Role(mygoui.RoleStatus)
			}
			p.fileInfo(c, app)
		})
	})
}

func (app *workspaceWindow) propertyDivider(c *mygoui.Context) mygoui.Color {
	if app.blackout && !c.Preferences().HighContrast {
		return mygoui.RGB(25, 25, 25)
	}
	return c.Theme().Border
}

func (app *workspaceWindow) propertyInks(c *mygoui.Context) (text, muted mygoui.Color) {
	if app.blackout && !c.Preferences().HighContrast {
		return mygoui.RGB(216, 216, 216), mygoui.RGB(140, 140, 140)
	}
	return c.Theme().Text, c.Theme().TextMuted
}

func propertySymbol(kind workspace.PropertyKind) string {
	switch kind {
	case workspace.PropertyNumber:
		return "number"
	case workspace.PropertyCheckbox:
		return "checkmark.square"
	case workspace.PropertyDate:
		return "calendar"
	case workspace.PropertyList:
		return "list.bullet"
	case workspace.PropertyObject:
		return "curlybraces"
	default:
		return "textformat"
	}
}

func (p *workspaceProperties) row(c *mygoui.Context, app *workspaceWindow, property workspace.Property) {
	state := p.inputs[property.Key]
	if state == nil {
		state = &propertyInput{checked: property.Text == "true"}
		if property.Kind != workspace.PropertyList {
			state.text = property.Text
		}
		if property.Kind == workspace.PropertyDate {
			state.date, _ = time.ParseInLocation(time.DateOnly, property.Text, time.Local)
		}
		p.inputs[property.Key] = state
	}
	ink, _ := app.propertyInks(c)
	box := mygoui.Column(c.Key(property.Key)).FillWidth().Gap(2).Padding(6, 12, 6).
		BorderWidth(0, 0, 1, 0).BorderColor(app.propertyDivider(c))
	showDelete := box.Hovered() || box.FocusWithin()
	box.Children(func() {
		mygoui.Row(c).Height(17).AlignItems(mygoui.Center).Gap(6).Children(func() {
			chromeIconColor(c, propertySymbol(property.Kind), 12, ink)
			mygoui.Text(c, property.Key).FontSize(12).FontWeight(650).TextColor(ink).SingleLine().Ellipsis("…")
			if showDelete {
				mygoui.Spacer(c)
				chromeButton(c, "trash", "Remove property "+property.Key, func() {
					p.apply(app, workspace.PropertyEdit{Action: workspace.RemoveProperty, Key: property.Key})
				}).Size(16, 17).Disabled(app.modalOpen)
			}
		})
		p.value(c, app, property, state)
	})
}

func (p *workspaceProperties) value(c *mygoui.Context, app *workspaceWindow, property workspace.Property, state *propertyInput) {
	set := func(text string) {
		p.apply(app, workspace.PropertyEdit{Action: workspace.SetProperty, Key: property.Key, Text: text})
	}
	switch property.Kind {
	case workspace.PropertyCheckbox:
		mygoui.Switch(c, &state.checked).Label(property.Key).Size(44, 20).Disabled(app.modalOpen).
			OnChange(func() { set(strconv.FormatBool(state.checked)) })
	case workspace.PropertyDate:
		mygoui.DateInput(c, &state.date).Label(property.Key).MinWidth(0).Size(112, 23).FontSize(13).Padding(0, 4).
			Border(0, c.Theme().Border).Background(c.Theme().SurfacePressed).Radius(6).
			Disabled(app.modalOpen).OnChange(func() { set(state.date.Format(time.DateOnly)) })
	case workspace.PropertyList:
		p.list(c, app, property, state)
	case workspace.PropertyObject, workspace.PropertyNull:
		readOnlyProperty(c, property)
	default:
		placeholder := "Empty"
		if property.Kind == workspace.PropertyNumber {
			placeholder = "0"
		}
		var input mygoui.Element
		if strings.Contains(property.Text, "\n") {
			input = mygoui.TextAreaBase(c, &state.text).Height(50).
				OnShortcut(mygoui.Cmd, mygoui.KeyEnter, func() { set(state.text) })
		} else {
			input = mygoui.TextInputBase(c, &state.text).Height(18).OnSubmit(func() { set(state.text) })
		}
		input.Label(property.Key).Placeholder(placeholder).FontSize(13).FillWidth().Padding(0).Disabled(app.modalOpen)
		if input.FocusLeft() && state.text != property.Text {
			set(state.text)
		}
	}
}

func (p *workspaceProperties) list(c *mygoui.Context, app *workspaceWindow, property workspace.Property, state *propertyInput) {
	ink, muted := app.propertyInks(c)
	fill := c.Theme().SurfacePressed
	if app.blackout && !c.Preferences().HighContrast {
		fill, ink = mygoui.RGB(21, 21, 21), mygoui.RGB(219, 219, 219)
	}
	if len(property.Items) != 0 {
		mygoui.Row(c).FillWidth().Wrap().Gap(6).Margin(2, 0, 2).Children(func() {
			for i, item := range property.Items {
				mygoui.Row(c.Key(i)).MaxWidth(312).Height(20).Padding(0, 5, 0, 8).Gap(4).
					Background(fill).Radius(10).AlignItems(mygoui.Center).Children(func() {
					mygoui.Text(c, item.Text).FontSize(11).FontWeight(600).TextColor(ink).MinWidth(0).SingleLine().Ellipsis("…")
					chromeButtonIcon(c, "xmark.circle.fill", "Remove "+item.Text+" from "+property.Key, func() {
						p.apply(app, workspace.PropertyEdit{Action: workspace.RemovePropertyItem, Key: property.Key, Item: i})
					}, 11, ink).Size(10, 10).Shrink(0).Disabled(app.modalOpen)
				})
			}
		})
	}
	mygoui.TextInputBase(c, &state.text).FillWidth().Height(15).Padding(0).FontSize(12).
		Placeholder("Add…").PlaceholderColor(muted).Label("Add to " + property.Key).Disabled(app.modalOpen).OnSubmit(func() {
		if p.apply(app, workspace.PropertyEdit{Action: workspace.AddPropertyItem, Key: property.Key, Text: state.text}) {
			state.text = ""
		}
	})
}

func readOnlyProperty(c *mygoui.Context, property workspace.Property) {
	if property.Kind != workspace.PropertyObject {
		mygoui.Text(c, property.Text).FontSize(12).Wrap()
		return
	}
	for _, item := range property.Items {
		mygoui.Row(c).Gap(4).Children(func() {
			mygoui.Text(c, item.Key+":").FontSize(12).TextColor(c.Theme().TextMuted)
			mygoui.Column(c).MinWidth(0).Children(func() { readOnlyProperty(c, item) })
		})
	}
}

func (p *workspaceProperties) addRow(c *mygoui.Context, app *workspaceWindow) {
	mygoui.Row(c).FillWidth().Height(25).Padding(0, 12).Gap(4).AlignItems(mygoui.Center).Children(func() {
		chromeMenu(c, propertySymbol(p.kind), string(p.kind), func(m *mygoui.Menu) {
			for _, kind := range []workspace.PropertyKind{workspace.PropertyText, workspace.PropertyNumber, workspace.PropertyCheckbox, workspace.PropertyDate, workspace.PropertyList} {
				if m.Item(string(kind)).Checked(p.kind == kind).Chosen() {
					p.kind = kind
				}
			}
		}).Size(26, 18).Disabled(app.modalOpen)
		key := strings.TrimSpace(p.key)
		valid := key != ""
		for _, row := range p.model.Rows {
			valid = valid && row.Key != key
		}
		commit := func() {
			if valid && p.apply(app, workspace.PropertyEdit{Action: workspace.AddProperty, Key: key, Kind: p.kind}) {
				p.adding, p.key = false, ""
			}
		}
		input := mygoui.TextInputBase(c, &p.key).Grow(1).MinWidth(0).Height(18).FontSize(13).
			Padding(0).Label("Property name").Placeholder("Property name").Disabled(app.modalOpen).OnSubmit(commit)
		if p.addFocus {
			input.Focus()
			p.addFocus = false
		}
		chromeButton(c, "checkmark.circle", "Create property", commit).Size(17, 18).Disabled(!valid || app.modalOpen)
		chromeButton(c, "xmark", "Cancel property", func() { p.adding = false }).Size(17, 18).Disabled(app.modalOpen)
	})
}

func (p *workspaceProperties) fileInfo(c *mygoui.Context, app *workspaceWindow) {
	type infoRow struct{ icon, label, value string }
	vault := "Not in a vault"
	if app.vault != nil && app.storage == nil {
		vault = filepath.Base(app.root)
	}
	rows := []infoRow{
		{"doc.text", "Name", app.title()},
		{"tray.full", "Vault", vault},
		{"folder", "Path", app.absoluteDocument(app.workspaceTab)},
		{"textformat", "Words", strconv.Itoa(p.metrics.Words)},
		{"character", "Characters", strconv.Itoa(p.metrics.Characters)},
		{"list.bullet", "Lines", strconv.Itoa(p.lines)},
		{"clock", "Reading", fmt.Sprintf("~%d min", p.metrics.ReadingMinutes)},
	}
	if !p.info.Modified.IsZero() {
		modified := p.info.Modified.Local()
		rows = append(rows, infoRow{"pencil", "Modified",
			fmt.Sprintf("%s at %d:%02d", modified.Format("Jan 2, 2006"), modified.Hour(), modified.Minute())})
	}
	if !p.info.Created.IsZero() {
		rows = append(rows, infoRow{"calendar", "Created", p.info.Created.Local().Format("Jan 2, 2006")})
	}
	if app.document.Path != "" {
		rows = append(rows, infoRow{"internaldrive", "Size", fmt.Sprintf("%d bytes", p.info.Size)})
	}
	status := "Saved"
	if app.dirty() {
		status = "Unsaved changes"
	}
	rows = append(rows, infoRow{"checkmark.circle", "Status", status})
	ink, muted := app.propertyInks(c)
	mygoui.Column(c).FillWidth().Margin(6, 0, 0).Padding(12, 12, 0).Gap(8).
		BorderWidth(1, 0, 0, 0).BorderColor(app.propertyDivider(c)).Children(func() {
		mygoui.Row(c).Height(14).AlignItems(mygoui.Center).Gap(8).Children(func() {
			chromeIconColor(c, "info.circle", 13, muted)
			mygoui.Text(c, "File Info").FontSize(12).FixedLineHeight(14).FontWeight(600).TextColor(muted).Role(mygoui.RoleHeading)
		})
		for _, row := range rows {
			if row.value == "" {
				continue
			}
			mygoui.Row(c.Key(row.label)).FillWidth().AlignItems(mygoui.Start).Gap(8).Children(func() {
				mygoui.Row(c).Size(14, 15).Shrink(0).Center().Children(func() {
					chromeIconColor(c, row.icon, 12, ink)
				})
				mygoui.Text(c, row.label).Width(78).Shrink(0).FontSize(12).FixedLineHeight(15).TextColor(ink)
				mygoui.Text(c, row.value).Grow(1).MinWidth(0).FontSize(12).FixedLineHeight(15).TextColor(ink).Wrap()
				if row.label == "Path" {
					mygoui.Row(c).Size(15, 15).Shrink(0).Padding(2, 2.5, 0).Children(func() {
						chromeButtonIcon(c, "doc.on.doc", "Copy", func() { mygo.Clipboard.WriteText(row.value) },
							12, muted, "Copy path").Size(10, 12.5).Shrink(0)
					})
				}
			})
		}
	})
}
