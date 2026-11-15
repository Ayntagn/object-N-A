package ui

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

const customExportPaper = "Custom"

type exportPaper struct {
	label, description string
	pdf                mygo.PageSize
	word               mygo.PageSize // Points used by the Word generator.
}

var exportPapers = []struct {
	section string
	papers  []exportPaper
}{
	{"US Standard", []exportPaper{
		{"Letter", "US Letter (8.5 × 11 in)", mygo.PageLetter, mygo.PageSize{Width: 612, Height: 792}},
		{"Legal", "US Legal (8.5 × 14 in)", mygo.PageLegal, mygo.PageSize{Width: 612, Height: 1008}},
		{"Tabloid", "Tabloid (11 × 17 in)", mygo.PageTabloid, mygo.PageSize{Width: 792, Height: 1224}},
		{"Executive", "Executive (7.25 × 10.5 in)", mygo.PageSize{Width: 7.25, Height: 10.5}, mygo.PageSize{Width: 522, Height: 756}},
		{"Statement", "Statement (5.5 × 8.5 in)", mygo.PageSize{Width: 5.5, Height: 8.5}, mygo.PageSize{Width: 396, Height: 612}},
		{"Folio", "Folio (8.5 × 13 in)", mygo.PageSize{Width: 8.5, Height: 13}, mygo.PageSize{Width: 612, Height: 936}},
	}},
	{"ISO A Series", []exportPaper{
		{"A3", "A3 (297 × 420 mm)", mygo.PageA3, mygo.PageSize{Width: 842, Height: 1191}},
		{"A4", "A4 (210 × 297 mm)", mygo.PageA4, mygo.PageSize{Width: 595, Height: 842}},
		{"A5", "A5 (148 × 210 mm)", mygo.PageA5, mygo.PageSize{Width: 420, Height: 595}},
		{"A6", "A6 (105 × 148 mm)", mygo.PageSize{Width: 105 / 25.4, Height: 148 / 25.4}, mygo.PageSize{Width: 298, Height: 420}},
	}},
	{"ISO B Series", []exportPaper{
		{"B4", "B4 (250 × 353 mm)", mygo.PageSize{Width: 250 / 25.4, Height: 353 / 25.4}, mygo.PageSize{Width: 709, Height: 1001}},
		{"B5", "B5 (176 × 250 mm)", mygo.PageSize{Width: 176 / 25.4, Height: 250 / 25.4}, mygo.PageSize{Width: 499, Height: 709}},
		{"B6", "B6 (125 × 176 mm)", mygo.PageSize{Width: 125 / 25.4, Height: 176 / 25.4}, mygo.PageSize{Width: 354, Height: 499}},
	}},
}

var exportPaperNames = func() []string {
	var names []string
	for _, group := range exportPapers {
		for _, paper := range group.papers {
			names = append(names, paper.label)
		}
	}
	return names
}()

type exportCustomPaper struct {
	WidthMM     float64 `json:"widthMM"`
	HeightMM    float64 `json:"heightMM"`
	Millimeters bool    `json:"millimeters,omitempty"`
}

func findExportPaper(label string) (exportPaper, bool) {
	for _, group := range exportPapers {
		if index := slices.IndexFunc(group.papers, func(paper exportPaper) bool { return paper.label == label }); index >= 0 {
			return group.papers[index], true
		}
	}
	return exportPapers[0].papers[0], false
}

func (f *exportPageForm) setPaperSize() {
	if f.paper == customExportPaper {
		if f.customPaper.WidthMM <= 0 {
			f.customPaper.WidthMM = 215.9
		}
		if f.customPaper.HeightMM <= 0 {
			f.customPaper.HeightMM = 279.4
		}
		f.layout.PaperWidth = f.customPaper.WidthMM * 72 / 25.4
		f.layout.PaperHeight = f.customPaper.HeightMM * 72 / 25.4
		return
	}
	paper, found := findExportPaper(f.paper)
	if !found {
		f.paper = paper.label
	}
	f.layout.PaperWidth, f.layout.PaperHeight = paper.pdf.Width*72, paper.pdf.Height*72
}

func wordExportLayout(layout exportdoc.Layout, name string) exportdoc.Layout {
	if paper, ok := findExportPaper(name); ok {
		layout.PaperWidth, layout.PaperHeight = paper.word.Width, paper.word.Height
	}
	return layout
}

func (s *workspaceExportSheet) pageSetup(c *mygoui.Context) {
	mygoui.Column(c.Key("page-setup")).AlignItems(mygoui.Stretch).Gap(16.25).Margin(0, 0, 4).Children(func() {
		mygoui.Text(c, "Page Setup").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
		mygoui.Row(c.Key("paper")).Height(24).AlignItems(mygoui.Center).Gap(8).Children(func() {
			mygoui.Text(c, "Paper Size").Width(102.5)
			theme := c.Theme()
			trigger := chromeMenuControl(c, "Paper Size", s.paperMenu).Role(mygoui.RolePopUpButton).Size(124, 24).PaddingX(10).
				Value(s.form.paper).Background(theme.SurfaceHover).Disabled(s.busy)
			trigger.Row().AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, s.form.paper).SingleLine()
				mygoui.Spacer(c)
				chromeIconColor(c, "chevron.up.chevron.down", 10, theme.TextMuted)
			})
		})
		if s.form.paper == customExportPaper {
			s.customPaperFields(c.Key("custom-paper"))
		} else {
			paper, _ := findExportPaper(s.form.paper)
			mygoui.Text(c, paper.description).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		mygoui.Row(c.Key("orientation")).Height(24).AlignItems(mygoui.Center).Gap(8).Children(func() {
			mygoui.Text(c, "Orientation").Width(81)
			s.orientationControl(c)
		})
		mygoui.Divider(c).MarginY(3.5)
		mygoui.Row(c.Key("margin-presets")).Height(20).AlignItems(mygoui.Center).Gap(8).Children(func() {
			mygoui.Text(c, "Margins").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
			mygoui.Spacer(c)
			for i, preset := range exportMarginPresets {
				width := [...]float32{60.5, 58, 47}[i]
				mygoui.Button(c.Key(preset.label), preset.label).Size(width, 20).FontSize(11).PaddingX(8).Disabled(s.busy).
					OnClick(func() { s.form.setMargins(preset.points) })
			}
		})
		mygoui.Column(c.Key("page-margins")).AlignItems(mygoui.Stretch).Gap(10).Children(func() {
			for _, side := range []struct {
				label string
				value *float64
			}{
				{"Top", &s.form.layout.Top}, {"Bottom", &s.form.layout.Bottom},
				{"Left", &s.form.layout.Left}, {"Right", &s.form.layout.Right},
			} {
				s.marginField(c.Key(side.label), side.label, side.value)
			}
		})
		mygoui.Divider(c).MarginY(3.5)
	})
}

func (s *workspaceExportSheet) paperMenu(menu *mygoui.Menu) {
	for i, group := range exportPapers {
		if i > 0 {
			menu.Separator()
		}
		menu.Item(group.section).Disabled(true)
		for _, paper := range group.papers {
			if menu.Item(paper.label).Checked(s.form.paper == paper.label).Chosen() {
				s.form.paper = paper.label
				s.form.setPaperSize()
			}
		}
	}
	menu.Separator()
	menu.Item(customExportPaper).Disabled(true)
	if menu.Item(customExportPaper).Checked(s.form.paper == customExportPaper).Chosen() {
		s.form.paper = customExportPaper
		s.form.setPaperSize()
	}
}

func (s *workspaceExportSheet) orientationControl(c *mygoui.Context) {
	selected := 0
	if s.form.layout.Landscape {
		selected = 1
	}
	parts := mygoui.SegmentedBase(c, &selected, 2)
	parts.Track.Label("Orientation").Height(24).Radius(6).Background(c.Theme().SurfaceHover).
		OnChange(func() { s.form.layout.Landscape = selected == 1 }).Children(func() {
		for i, label := range []string{"Portrait", "Landscape"} {
			segment := parts.Segment(i).Label(label).Size(90.5, 24).Radius(6).Disabled(s.busy)
			ink := c.Theme().Text
			if i == selected {
				segment.Background(c.Theme().Accent)
				ink = mygoui.RGB(255, 255, 255)
			}
			segment.Children(func() { mygoui.Text(c, label).TextColor(ink) })
		}
	})
}

func (s *workspaceExportSheet) marginField(c *mygoui.Context, label string, value *float64) {
	mygoui.Row(c).Height(24).Gap(8).AlignItems(mygoui.Center).Children(func() {
		mygoui.Text(c, label).Width(50).TextAlign(mygoui.End)
		mygoui.Slider(c, value, 0, 144).Label(label).Width(200).MinWidth(65).Shrink(1).Step(1).Disabled(s.busy)
		field := mygoui.Row(c.Key("margin-value")).Gap(2).AlignItems(mygoui.Center)
		text := mygoui.Local(field, "margin", func() string { return fmt.Sprintf("%.0f", *value) })
		field.Children(func() {
			input := mygoui.TextInput(c, text).Label(label+" Margin").Size(50, 24).PaddingX(5).
				TextAlign(mygoui.End).Background(c.Theme().SurfaceHover).Disabled(s.busy)
			if !input.Focused() {
				*text = fmt.Sprintf("%.0f", *value)
			}
			input.OnChange(func() {
				if margin, err := strconv.Atoi(*text); err == nil && margin >= 0 && margin <= 144 {
					*value = float64(margin)
				}
			})
			mygoui.Stepper(c, value, 0, 144, 1).Label(label+" Margin Stepper").Size(20, 24).Disabled(s.busy)
		})
		mygoui.Text(c, "pt").FontSize(11).TextColor(c.Theme().TextMuted)
	})
}

func (s *workspaceExportSheet) customPaperFields(c *mygoui.Context) {
	paper := &s.form.customPaper
	unit, scale, precision := "in", 25.4, 2
	if paper.Millimeters {
		unit, scale, precision = "mm", 1, 1
	}
	mygoui.Column(c).Width(216).AlignSelf(mygoui.Start).Padding(10).Gap(10).Radius(8).
		Background(c.Theme().SurfaceHover).Children(func() {
		mygoui.Row(c.Key("units")).Gap(10).Height(24).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, "Units:").FontSize(11)
			selected := 0
			if paper.Millimeters {
				selected = 1
			}
			mygoui.Segmented(c, &selected, "Inches", "Millimeters").Height(24).Label("Units").Disabled(s.busy).
				OnChange(func() { paper.Millimeters = selected == 1 })
		})
		for _, dimension := range []struct {
			name  string
			value *float64
		}{{"Width", &paper.WidthMM}, {"Height", &paper.HeightMM}} {
			mygoui.Row(c.Key(dimension.name)).Height(24).Gap(8).AlignItems(mygoui.Center).Children(func() {
				mygoui.Text(c, dimension.name+":").Width(50).TextAlign(mygoui.End)
				state := mygoui.Box(c.Key(unit+dimension.name)).Size(80, 24)
				text := mygoui.Local(state, "dimension", func() string {
					return strconv.FormatFloat(*dimension.value/scale, 'f', precision, 64)
				})
				state.Children(func() {
					field := mygoui.TextInput(c, text).Label(dimension.name).Size(80, 24).Disabled(s.busy)
					if !field.Focused() {
						*text = strconv.FormatFloat(*dimension.value/scale, 'f', precision, 64)
					}
					field.OnChange(func() {
						value, err := strconv.ParseFloat(*text, 64)
						if err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
							*dimension.value = min(max(value*scale, 25.4), 558.8)
							s.form.setPaperSize()
						}
					})
				})
				mygoui.Text(c, unit)
			})
		}
		mygoui.Text(c, fmt.Sprintf("%.2f × %.2f in (%.0f × %.0f mm)", paper.WidthMM/25.4, paper.HeightMM/25.4,
			paper.WidthMM, paper.HeightMM)).FontSize(11).TextColor(c.Theme().TextMuted)
	})
}
