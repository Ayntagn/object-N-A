package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

type workspaceExportPreferences struct {
	Paper               string          `json:"paper,omitempty"`
	Landscape           bool            `json:"landscape,omitempty"`
	OmitPageNumbers     bool            `json:"omitPageNumbers,omitempty"`
	LineSpacing         string          `json:"lineSpacing,omitempty"`
	RememberLocation    bool            `json:"rememberLocation"`
	LastDirectory       string          `json:"lastDirectory,omitempty"`
	RememberPDFSettings bool            `json:"rememberPDFSettings,omitempty"`
	SavedPDFSettings    *savedPDFExport `json:"savedPDFSettings,omitempty"`
}

var exportSpacingOptions = []string{"Theme default", "1.0", "1.15", "1.25", "1.5", "1.75", "2.0", "2.5"}
var exportSpacingValues = [...]float64{0, 1, 1.15, 1.25, 1.5, 1.75, 2, 2.5}

func (p workspaceExportPreferences) spacingIndex() int {
	// Before the native menu was captured, persisted choices used the × suffix.
	spacing := strings.TrimSuffix(p.LineSpacing, "×")
	if spacing == "2" {
		spacing = "2.0"
	}
	return max(0, slices.Index(exportSpacingOptions, spacing))
}

func (p workspaceExportPreferences) spacing() float64 {
	return exportSpacingValues[p.spacingIndex()]
}

var exportMarginPresets = [...]struct {
	label  string
	points float64
}{{"Minimal", 24}, {"Normal", 54}, {"Wide", 72}}

type exportPageForm struct {
	paper       string
	layout      exportdoc.Layout
	pagination  exportdoc.PDFPagination
	appearance  pdfAppearance
	customPaper exportCustomPaper
}

type savedPDFExport struct {
	Paper       string                  `json:"paper"`
	Layout      exportdoc.Layout        `json:"layout"`
	Pagination  exportdoc.PDFPagination `json:"pagination"`
	Appearance  pdfAppearance           `json:"appearance"`
	CustomPaper exportCustomPaper       `json:"customPaper,omitzero"`
}

func (p workspaceExportPreferences) rememberedPDFForm() exportPageForm {
	if p.RememberPDFSettings && p.SavedPDFSettings != nil {
		saved := p.SavedPDFSettings
		form := exportPageForm{paper: saved.Paper, layout: saved.Layout, pagination: saved.Pagination,
			appearance: saved.Appearance, customPaper: saved.CustomPaper}
		form.layout.LineSpacing = p.spacing()
		form.setPaperSize()
		return form
	}
	return newExportPageForm(p)
}

func (app *workspaceWindow) documentPDFForm() exportPageForm {
	form := app.quickPDFForm
	if form.paper == "" {
		return newExportPageForm(app.preferences.Export)
	}
	form.layout.LineSpacing = app.preferences.Export.spacing()
	return form
}

func newExportPageForm(p workspaceExportPreferences) exportPageForm {
	f := exportPageForm{paper: p.Paper, layout: exportdoc.DefaultLayout()}
	f.layout.Landscape = p.Landscape
	f.layout.LineSpacing = p.spacing()
	f.pagination.OmitNumbers = p.OmitPageNumbers
	f.setPaperSize()
	return f
}

func (f *exportPageForm) setMargins(points float64) {
	f.layout.Top, f.layout.Right, f.layout.Bottom, f.layout.Left = points, points, points, points
}

type exportFormat int

const htmlPDFExportProblem = "HTML documents cannot be exported to PDF. Use 'Open in Browser' to view and print."
const htmlDocumentExportProblem = "HTML documents cannot be exported to PDF or DOCX. Use 'Open in Browser' to view and print from there."

const (
	exportPDF exportFormat = iota
	exportDOCX
	exportHTML
)

var exportFormats = [...]struct{ label, extension, action string }{
	{"PDF", "pdf", "Export PDF"},
	{"DOCX", "docx", "Export .docx"},
	{"HTML", "html", "Export HTML"},
}

type exportDocument struct {
	source, name  string
	theme         string
	customCSS     string
	editorFont    string
	hasEditorFont bool // An empty editorFont explicitly selects System Mono.
	layout        exportdoc.Layout
	pagination    exportdoc.PDFPagination
	appearance    pdfAppearance
	numberFigures bool
	paper         string
	origin        previewResourceOrigin
	kind          documentType
}

type workspaceExportSheet struct {
	app              *workspaceWindow
	win              *mygo.Window
	form             exportPageForm
	format           exportFormat
	document         exportDocument
	directory        string
	preview          pdfViewer
	busy             bool
	problem          string
	rememberSettings bool
	gridPages        bool
	applyNotice      *time.Timer
	resultPath       string
	resultError      string
}

const (
	exportSheetWide   = 900
	exportSheetNarrow = 480
	exportSheetHeight = 640
)

func (app *workspaceWindow) showExport() {
	if app.draft == nil || app.modalOpen || !app.permitsActiveDocument() {
		return
	}
	if app.previewDocumentType() == documentHTML {
		app.problem = htmlDocumentExportProblem
		app.win.Invalidate()
		return
	}
	app.modalOpen = true
	form := app.documentPDFForm()
	if app.preferences.Export.RememberPDFSettings {
		form = app.preferences.Export.rememberedPDFForm()
	}
	sheet := &workspaceExportSheet{
		app: app, form: form, document: app.exportDocument(form.layout), directory: app.exportDirectory(),
		rememberSettings: app.preferences.Export.RememberPDFSettings,
	}
	sheet.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Export Document", Width: exportSheetWide, Height: exportSheetHeight,
		MinWidth: exportSheetNarrow, MinHeight: 380,
		Parent: app.win, Modal: true, Hidden: true, UseContentSize: true, DisableMinimize: true, DisableMaximize: true,
		TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		Content: mygoui.View(sheet.draw),
	})
	sheet.win.OnClose(func(event *mygo.CloseEvent) {
		if sheet.busy {
			event.PreventDefault()
		}
	})
	sheet.win.OnClosed(func() {
		if sheet.applyNotice != nil {
			sheet.applyNotice.Stop()
		}
		sheet.preview.clear()
		app.modalOpen = false
		app.win.Invalidate()
	})
	sheet.win.Show()
	app.win.Invalidate()
}

func (app *workspaceWindow) quickExportPDF() {
	app.quickExport(exportPDF, app.documentPDFForm())
}

func (app *workspaceWindow) quickExport(format exportFormat, form exportPageForm) {
	if app.draft == nil || app.modalOpen {
		return
	}
	if format == exportPDF && app.previewDocumentType() == documentHTML {
		app.problem = htmlPDFExportProblem
		app.win.Invalidate()
		return
	}
	app.modalOpen = true
	document := app.exportDocument(form.layout)
	document.pagination = form.pagination
	document.appearance = form.appearance
	if format == exportDOCX {
		document.paper = form.paper
	}
	if format == exportPDF && document.kind == documentMarkdown && app.preferences.UserStylesheetEnabled {
		// EV-UI-129: direct PDF uses the stylesheet loaded with this note.
		document.customCSS = app.quickPDFStylesheet
	}
	directory := app.exportDirectory()
	win, blackout := app.win, app.blackout
	var cachedPDF []byte
	tab := app.workspaceTab
	if format == exportPDF {
		document.numberFigures = tab.quickPDFNumberFigures
		if !document.numberFigures && app.pdfPreview.viewer.generated == document {
			cachedPDF = app.pdfPreview.viewer.payload
		}
	}
	// EV-UI-133: Quick PDF has its own renderer for the lifetime of the note.
	counter := tab.quickPDFFigureCounter
	retainCounter := format == exportPDF && document.numberFigures
	win.Invalidate()
	go func() {
		var data []byte
		var err error
		if retainCounter {
			var output pdfOutput
			output, err = generateExportPDFWithFigures(win, document, &counter)
			data = output.Data
		} else {
			data, err = shareExportPayload(win, document, format, blackout, cachedPDF)
		}
		var written string
		if err == nil {
			written, err = saveExportPayload(win, document.name, directory, format, data)
		}
		win.Update(func() {
			if retainCounter {
				tab.quickPDFFigureCounter = counter
			}
			app.modalOpen = false
			message := "Could not export " + exportFormats[format].label + "."
			if err != nil {
				app.problem = fmt.Sprintf("%s %v", message, err)
			} else if written != "" {
				if strings.HasPrefix(app.problem, message) {
					app.problem = ""
				}
				app.rememberExport(written)
				if format == exportDOCX || format == exportHTML {
					app.modalOpen = true
					go app.showQuickExportResult(win, format, written)
				}
			}
		})
	}()
}

func (app *workspaceWindow) exportDocument(layout exportdoc.Layout) exportDocument {
	app.wys.sync(app)
	name := noteName(app.title())
	if name == "" {
		name = "Untitled"
	}
	kind := app.previewDocumentType()
	var customCSS string
	if kind == documentMarkdown {
		customCSS = app.activeCustomStylesheet()
	}
	return exportDocument{source: app.workingText(), name: name, layout: layout, origin: app.previewOrigin(),
		kind: kind, theme: app.resolvePreviewTheme(app.previewThemeSelection()).ID, customCSS: customCSS,
		numberFigures: app.preferences.NumberFigures, editorFont: app.preferences.EditorFont, hasEditorFont: true}
}

func (app *workspaceWindow) exportDirectory() string {
	preferences := app.preferences.Export
	if preferences.RememberLocation && preferences.LastDirectory != "" {
		if info, err := os.Stat(preferences.LastDirectory); err == nil && info.IsDir() {
			return preferences.LastDirectory
		}
	}
	if app.folderRoot != "" {
		return app.folderRoot
	}
	if app.root != "" {
		return filepath.Join(app.root, filepath.Dir(filepath.FromSlash(app.document.Path)))
	}
	dir, _ := mygo.App.Path(mygo.PathDocuments)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	dir, _ = os.UserHomeDir()
	return dir
}

func (app *workspaceWindow) rememberExport(filename string) {
	p := &app.preferences.Export
	directory := filepath.Dir(filename)
	if p.RememberLocation && p.LastDirectory != directory {
		p.LastDirectory = directory
		app.writePreferences()
	}
}

func (s *workspaceExportSheet) draw(c *mygoui.Context) {
	s.app.applyAppearance(c)
	c.OnWindowShortcut(0, mygoui.KeyEscape, s.win.Close)
	theme := c.Theme()
	surface, buttonFace, buttonBorder := theme.Surface, theme.SurfaceHover, theme.Border
	if s.app.blackout && !c.Preferences().HighContrast {
		// EV-UI-133: the export sheet uses the Blackout background and subdued controls.
		surface, buttonFace, buttonBorder = theme.Background, mygoui.Hex("#131313"), mygoui.Transparent
		sheetTheme := *theme
		sheetTheme.SurfaceHover = buttonFace
		sheetTheme.AccentText = mygoui.RGB(255, 255, 255)
		sheetTheme.InactiveAccent = sheetTheme.Accent
		c.SetTheme(&sheetTheme)
		theme = c.Theme()
	}
	mygoui.Column(c).Fill().Background(surface).Children(func() {
		mygoui.Row(c).FillWidth().Height(56).Shrink(0).PaddingX(16).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, "Export Document").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
			mygoui.Spacer(c)
			chromeButtonIcon(c, "xmark.circle.fill", "Close", paneAction(!s.busy, s.win.Close), 17, theme.TextMuted).Size(17, 17)
			// The format track keeps its position as the sheet changes width (EV-UI-135).
			mygoui.Box(c.Key("format-position")).Absolute().Left(143.5).Right(0).Top(16).Height(24).Center().Children(func() {
				s.formatControl(c.Key("format"), buttonFace)
			})
		})
		mygoui.Row(c).FillWidth().Height(1).Shrink(0).Background(theme.Border)
		mygoui.Row(c).Grow(1).MinHeight(0).FillWidth().AlignItems(mygoui.Stretch).Children(func() {
			if s.format == exportPDF {
				width, _ := c.Size()
				mygoui.Column(c.Key("export-pages")).Width(min(240, width*0.32)).Shrink(0).Children(func() {
					mygoui.Row(c).Height(26).PaddingX(12).AlignItems(mygoui.Center).Children(func() {
						mygoui.Text(c, "Pages").FontSize(11).FontWeight(600).Role(mygoui.RoleHeading)
						mygoui.Spacer(c)
						symbol, label := "rectangle.grid.1x2", "Grid View"
						if s.gridPages {
							symbol, label = "square.grid.2x2", "Column View"
						}
						chromeButtonIcon(c, symbol, label, func() { s.gridPages = !s.gridPages }, 13, theme.TextMuted).Size(16, 16)
					})
					document := s.selectedDocument()
					style := pdfThumbnailStyle{layout: pdfThumbnailsSheetColumn,
						background: pdfCSSColor(surface), text: pdfCSSColor(theme.TextMuted)}
					if s.gridPages {
						style.layout = pdfThumbnailsSheetGrid
					}
					s.preview.setExportThumbnails(style)
					if document.pagination.Continuous {
						s.preview.requestIfChanged(s.win, document)
						s.continuousPreview(c)
					} else {
						s.preview.draw(c, s.win, document, 100, 0)
					}
				})
				mygoui.Row(c).Width(1).Shrink(0).Background(theme.Border)
			}
			mygoui.Scroll(c.Key("export-options")).Label("Export options").Grow(1).MinWidth(0).Padding(16).
				Children(func() { s.options(c) })
		})
		mygoui.Row(c).FillWidth().Height(1).Shrink(0).Background(theme.Border)
		if s.problem != "" {
			mygoui.Text(c, s.problem).MarginX(16).TextColor(theme.Danger)
		}
		mygoui.Row(c).FillWidth().Height(56).Shrink(0).PaddingX(16).AlignItems(mygoui.Center).Gap(8).Children(func() {
			if s.busy {
				mygoui.Text(c, "Exporting…").TextColor(c.Theme().TextMuted)
			} else if s.applyNotice != nil {
				mygoui.Text(c, "Settings saved").TextColor(c.Theme().TextMuted)
			}
			mygoui.Spacer(c)
			mygoui.Button(c, "Cancel").Size(66, 24).PaddingX(8).Background(buttonFace).BorderColor(buttonBorder).
				OnClick(s.win.Close).Disabled(s.busy)
			mygoui.Button(c, "Apply").Size(59, 24).PaddingX(8).Background(buttonFace).BorderColor(buttonBorder).
				OnClick(s.apply).Disabled(s.busy)
			width := float32(92.5)
			if s.format == exportDOCX {
				width = 100.5
			} else if s.format == exportHTML {
				width = 103.5
			}
			mygoui.PrimaryButton(c, exportFormats[s.format].action).MinWidth(width).Height(24).PaddingX(8).
				TextColor(mygoui.RGB(255, 255, 255)).OnClick(s.start).Disabled(s.busy || !s.exportReady())
		})
	})
	s.exportResult(c)
}

func (s *workspaceExportSheet) formatControl(c *mygoui.Context, surface mygoui.Color) {
	selected := int(s.format)
	mygoui.Row(c).Width(184.5).AlignItems(mygoui.Center).Children(func() {
		mygoui.Text(c, "Format").Absolute().Left(-51).Top(4).FontSize(13).Width(43)
		segments := mygoui.SegmentedBase(c, &selected, len(exportFormats))
		segments.Track.Label("Format").Height(24).Radius(6).Background(surface).
			OnChange(func() { s.selectFormat(exportFormat(selected)) }).Children(func() {
			for i, format := range exportFormats {
				part := segments.Segment(i).Label(format.label).Size(61.5, 24).Radius(6).Disabled(s.busy)
				ink := c.Theme().Text
				if i == selected {
					part.Background(c.Theme().Accent)
					ink = mygoui.RGB(255, 255, 255)
				}
				part.Children(func() { mygoui.Text(c, format.label).FontSize(13).TextColor(ink) })
			}
		})
	})
}

func (s *workspaceExportSheet) selectFormat(format exportFormat) {
	if s.busy || s.format == format {
		return
	}
	wasWide := s.format == exportPDF
	s.format, s.problem = format, ""
	if wasWide == (format == exportPDF) {
		return
	}
	bounds := s.win.ContentBounds()
	width := exportSheetNarrow
	if format == exportPDF {
		width = exportSheetWide
	}
	bounds.X += (bounds.Width - width) / 2
	bounds.Width = width
	s.win.SetContentBounds(bounds)
}

func (s *workspaceExportSheet) continuousPreview(c *mygoui.Context) {
	message := s.preview.problem
	if message == "" {
		message = "Preview not available for non-paginated export"
	}
	mygoui.Column(c).Grow(1).Center().Gap(12).Padding(20).Children(func() {
		chromeIconColor(c, "scroll", 42, c.Theme().TextMuted).Role(mygoui.RoleImage).Label("Scroll Document")
		mygoui.Text(c, "Continuous Page").FontWeight(600)
		mygoui.Text(c, message).FontSize(11).MaxWidth(150).TextAlign(mygoui.Center).TextColor(c.Theme().TextMuted)
	})
}

func (s *workspaceExportSheet) options(c *mygoui.Context) {
	mygoui.Column(c).AlignItems(mygoui.Stretch).Gap(9).Children(func() {
		s.pageSetup(c)
		mygoui.Text(c, "Appearance").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading).Margin(0, 0, 4)
		s.appearanceOptions(c)
		if s.format == exportPDF {
			s.pdfOptions(c.Key("pdf-options"))
		} else {
			mygoui.Divider(c).MarginY(10)
			mygoui.Text(c, "Options").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
		}
		mygoui.Checkbox(c, &s.rememberSettings, "Remember settings").AlignSelf(mygoui.Start).Disabled(s.busy)
		mygoui.Button(c, "Reset All to Defaults").Size(147, 24).PaddingX(8).AlignSelf(mygoui.Start).Disabled(s.busy).OnClick(func() {
			s.form = newExportPageForm(workspaceExportPreferences{LineSpacing: s.app.preferences.Export.LineSpacing})
		})
		if s.format == exportPDF {
			s.pdfPageRange(c.Key("pdf-range"))
		}
	})
}

var pdfNumberPositions = []string{"Bottom Center", "Bottom Right", "Bottom Left", "Top Center", "Top Right", "Top Left"}
var pdfNumberFormats = []string{"1, 2, 3", "i, ii, iii", "I, II, III", "a, b, c", "A, B, C"}

func (s *workspaceExportSheet) pdfOptions(c *mygoui.Context) {
	p := &s.form.pagination
	split, numbered := !p.Continuous, !p.OmitNumbers
	mygoui.Divider(c).MarginY(10)
	mygoui.Text(c, "Options").FontSize(13).FontWeight(600).Role(mygoui.RoleHeading)
	mygoui.Column(c.Key("pdf-numbering")).AlignItems(mygoui.Start).Gap(12.5).Children(func() {
		mygoui.Checkbox(c, &split, "Split into pages").Disabled(s.busy).OnChange(func() {
			p.Continuous = !split
		})
		mygoui.Checkbox(c, &s.app.preferences.NumberFigures, "Number figures").
			Disabled(s.busy).OnChange(s.app.preferencesChanged)
		if split {
			mygoui.Checkbox(c, &numbered, "Include page numbers").Disabled(s.busy).OnChange(func() {
				p.OmitNumbers = !numbered
			})
		}
	})
	if split {
		if numbered {
			position := pdfNumberPositions[max(0, min(int(p.NumberPosition), len(pdfNumberPositions)-1))]
			format := pdfNumberFormats[max(0, min(int(p.NumberFormat), len(pdfNumberFormats)-1))]
			startAt := float64(p.FirstNumber())
			mygoui.Column(c.Key("page-number-options")).PaddingStart(20).Gap(10).Children(func() {
				mygoui.Row(c.Key("position")).AlignItems(mygoui.Center).Gap(8).Children(func() {
					mygoui.Text(c, "Position").Width(50)
					mygoui.Select(c, &position, pdfNumberPositions).Size(136, 24).Background(c.Theme().SurfaceHover).
						Label("Position").Disabled(s.busy).OnChange(func() {
						p.NumberPosition = exportdoc.PDFNumberPosition(max(0, slices.Index(pdfNumberPositions, position)))
					})
				})
				mygoui.Row(c.Key("number-format")).AlignItems(mygoui.Center).Gap(8).Children(func() {
					mygoui.Text(c, "Format").Width(68)
					mygoui.Select(c, &format, pdfNumberFormats).Size(89, 24).MinWidth(89).PaddingX(8).Background(c.Theme().SurfaceHover).
						Label("Page Number Format").Disabled(s.busy).OnChange(func() {
						p.NumberFormat = exportdoc.PDFNumberFormat(max(0, slices.Index(pdfNumberFormats, format)))
					})
				})
				mygoui.Row(c.Key("number-start")).AlignItems(mygoui.Center).Gap(8).Children(func() {
					mygoui.Text(c, "Start at").Width(68)
					mygoui.Text(c, strconv.Itoa(int(startAt)))
					mygoui.Stepper(c, &startAt, 1, exportdoc.MaxPDFStartAt, 1).Height(24).Label("Start at").Disabled(s.busy).OnChange(func() {
						p.StartAt = int(startAt)
					})
				})
			})
		}
	}
}

func (s *workspaceExportSheet) pdfPageRange(c *mygoui.Context) {
	pagination := &s.form.pagination
	mygoui.Divider(c).MarginY(10)
	mygoui.Row(c).AlignItems(mygoui.Center).Gap(8).Children(func() {
		mygoui.Text(c, "Estimated:").FontSize(12).SingleLine().Width(60)
		var count string
		switch {
		case pagination.Continuous:
			count = "Continuous (1 page)"
		case s.preview.pages == 1:
			count = "1 page"
		case s.preview.pages > 1:
			count = strconv.Itoa(s.preview.pages) + " pages"
		}
		if count != "" {
			mygoui.Text(c, count).FontSize(12).FontWeight(600)
		}
	})
	mygoui.Row(c).AlignItems(mygoui.Center).Gap(8).Children(func() {
		mygoui.Text(c, "Pages:").FontSize(12).SingleLine().Width(39)
		mygoui.TextInput(c, &pagination.Range).Label("Page Range").Placeholder("All").Size(120, 24).
			Background(c.Theme().SurfaceHover).Disabled(s.busy)
	})
	mygoui.Text(c, "Range: 1-3  Individual: 1,3,5  Combined: 1-3,5,7-9").
		FontSize(11).TextColor(c.Theme().TextMuted)
}

func (s *workspaceExportSheet) storePDFSettings(remember bool) error {
	p := &s.app.preferences.Export
	previous := *p
	p.RememberPDFSettings = remember
	if remember {
		p.SavedPDFSettings = &savedPDFExport{
			Paper: s.form.paper, Layout: s.form.layout, Pagination: s.form.pagination, Appearance: s.form.appearance, CustomPaper: s.form.customPaper,
		}
	}
	if *p == previous {
		return nil
	}
	if err := s.app.preferences.write(); err != nil {
		*p = previous
		return fmt.Errorf("could not save PDF settings: %w", err)
	}
	return nil
}

func (s *workspaceExportSheet) apply() {
	s.problem = ""
	if err := s.storePDFSettings(true); err != nil {
		s.problem = err.Error()
		return
	}
	s.rememberSettings = true
	s.app.quickPDFForm = s.form
	s.app.pdfPreview.form = s.form
	s.app.win.Invalidate()
	if s.applyNotice != nil {
		s.applyNotice.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(2*time.Second, func() {
		s.win.Update(func() {
			if s.applyNotice == timer {
				s.applyNotice = nil
			}
		})
	})
	s.applyNotice = timer
}

func (s *workspaceExportSheet) selectedDocument() exportDocument {
	document := s.document
	document.layout, document.pagination, document.appearance = s.form.layout, s.form.pagination, s.form.appearance
	if s.format == exportDOCX || s.format == exportHTML {
		document.paper = s.form.paper
	}
	return document
}

func (s *workspaceExportSheet) exportReady() bool {
	return s.format != exportPDF || (s.preview.generated == s.selectedDocument() && len(s.preview.payload) != 0)
}

func (s *workspaceExportSheet) start() {
	if s.busy || !s.exportReady() {
		return
	}
	document, format := s.selectedDocument(), s.format
	if err := s.storePDFSettings(s.rememberSettings); err != nil {
		s.problem = err.Error()
		return
	}
	s.busy, s.problem = true, ""
	var pdf []byte
	if s.preview.generated == document {
		pdf = s.preview.payload
	}
	// EV-UI-137/138: HTML starts with the figure count from this sheet's PDF renderer.
	initialCounter := s.preview.figureCounter
	go func() {
		counter := initialCounter
		var data []byte
		var err error
		if format == exportHTML {
			var markup string
			markup, err = renderExportHTML(document, &counter)
			data = []byte(markup)
		} else {
			data, err = exportPayload(s.win, document, format, pdf)
		}
		var written string
		if err == nil {
			written, err = saveExportPayload(s.win, document.name, s.directory, format, data)
		}
		s.win.Update(func() {
			s.busy = false
			if s.win.IsDestroyed() {
				return
			}
			if format == exportHTML && s.preview.figureCounter == initialCounter {
				s.preview.figureCounter = counter
			}
			if err != nil {
				if format == exportDOCX || format == exportHTML {
					s.resultError = err.Error()
				} else {
					s.problem = err.Error()
				}
			} else if written != "" {
				s.app.rememberExport(written)
				if format == exportDOCX || format == exportHTML {
					s.resultPath = written
				} else {
					s.win.Close()
				}
			} else if format == exportHTML {
				s.resultError = "Export was cancelled."
			}
		})
	}()
}

func exportPayload(win *mygo.Window, doc exportDocument, format exportFormat, pdf []byte) ([]byte, error) {
	switch format {
	case exportPDF:
		if len(pdf) != 0 {
			return pdf, nil
		}
		return printExportPDF(win, doc)
	case exportDOCX:
		return renderExportDOCX(win, doc)
	case exportHTML:
		markup, err := renderExportHTML(doc, nil)
		return []byte(markup), err
	}
	return nil, errors.New("unsupported export format")
}

func saveExportPayload(win *mygo.Window, name, directory string, format exportFormat, data []byte) (string, error) {
	if win.IsDestroyed() {
		return "", nil
	}
	choice := exportFormats[format]
	options := mygo.SaveDialogOptions{
		Parent: win, Title: choice.action, ButtonLabel: "Export", CreateDirectories: true,
		DefaultPath: filepath.Join(directory, name+"."+choice.extension),
		Filters:     []mygo.FileFilter{{Name: choice.label, Extensions: []string{choice.extension}}},
	}
	if format == exportHTML {
		options.ButtonLabel = "Save"
		options.Message = "Choose a location to save the HTML file"
		if runtime.GOOS == "darwin" {
			options.Title = ""
		}
	}
	filename, err := mygo.Dialog.Save(options)
	if err != nil || filename == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(filename), "."+choice.extension) {
		return "", fmt.Errorf("Choose a filename ending in .%s.", choice.extension)
	}
	if err := exportdoc.WriteFile(filename, data); err != nil {
		return "", err
	}
	return filename, nil
}

func exportPDFPage(body, title string, document exportDocument, theme documentPreviewTheme, policy, extraCSS string) string {
	layout := document.layout
	var attrs string
	if theme.ID != "" {
		attrs = ` data-preview-theme="` + theme.ID + `"`
		extraCSS += theme.cssVariables() + documentThemeElements + document.pdfFontFaces(theme)
		fontSize := math.Round(float64(theme.BaseFontSize) * layout.FontScale / 100)
		extraCSS += fmt.Sprintf(`
html[data-preview-theme] body {
  font-size: %.2fpt;
  print-color-adjust: exact; -webkit-print-color-adjust: exact;
}
@page { background-color: %s; }
`, fontSize, theme.BackgroundColor)
		if spacing := previewLineSpacing(document.source, layout.LineSpacing); spacing > 0 {
			extraCSS += fmt.Sprintf(`html[data-preview-theme] { --preview-line-height: %g; }`, spacing)
		}
	}
	extraCSS += pdfDocumentCSS
	policy, stylesheet := standaloneCustomStylesheet(policy, document.customCSS)
	return fmt.Sprintf(`<!doctype html>
<html%s>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="%s">
<meta http-equiv="x-dns-prefetch-control" content="off">
<title>%s</title>
<style>@layer builtin {
:root { color-scheme: light; }
body { margin: 0; color: #202124; background: #fff; font: %.2fpx/%.2f %s; overflow-wrap: anywhere; }
h1, h2, h3, h4, h5, h6 { line-height: 1.3; break-after: avoid; }
pre, code { font-family: ui-monospace, monospace; background: #f4f4f5; }
pre { padding: .75em; white-space: pre-wrap; tab-size: 4; }
blockquote { margin-inline: 0; padding-inline-start: 1em; border-inline-start: 3px solid #b7b7bd; }
table { border-collapse: collapse; }
th, td { padding: .4em .65em; border: 1px solid #ccc; text-align: left; }
th { background: #f4f4f5; }
.center { text-align: center; }
.right { text-align: right; }
.raw-markup { white-space: pre-wrap; }
.image-description { font-style: italic; }
@media screen { .page-break { margin: 1.5em 0; border-top: 1px dashed #b7b7bd; } }
@media print {
  tr { break-inside: avoid; }
  .page-break { break-after: page; page-break-after: always; }
}
%s
}</style>%s
</head>
<body><article class="markdown-body">%s</article></body>
</html>`, attrs, html.EscapeString(policy), html.EscapeString(title), layout.FontScale*0.15, layout.LineHeight(),
		layout.FontFamily.CSSStack(), extraCSS, stylesheet, body)
}

func printExportPDF(parent *mygo.Window, doc exportDocument) ([]byte, error) {
	out, err := generateExportPDF(parent, doc)
	return out.Data, err
}

func generateExportPDF(parent *mygo.Window, doc exportDocument) (pdfOutput, error) {
	return generateExportPDFWithFigures(parent, doc, nil)
}

func generateExportPDFWithFigures(parent *mygo.Window, doc exportDocument, figureCounter *int) (pdfOutput, error) {
	if parent.IsDestroyed() {
		return pdfOutput{}, errors.New("the export window closed")
	}
	if !mygo.WebViewAvailable() {
		return pdfOutput{}, errors.New("PDF export requires the system WebView")
	}
	dir, err := os.MkdirTemp("", "cyberwriter-print-")
	if err != nil {
		return pdfOutput{}, err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "document.html")
	document, body := renderPDFHTMLWithFigures(doc, figureCounter)
	if err := os.WriteFile(file, []byte(document), 0o600); err != nil {
		return pdfOutput{}, err
	}
	width, height := doc.layout.Dimensions()
	contentWidth := max(1, int(width-doc.layout.Left-doc.layout.Right))
	contentHeight := max(1, int(height-doc.layout.Top-doc.layout.Bottom))
	window := mygo.NewWindow(mygo.WindowOptions{
		Title: "Export PDF", Parent: parent, Hidden: true, UseContentSize: true,
		Width:  contentWidth,
		Height: max(contentHeight, pdfInitialViewportHeight),
		Page:   mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled},
	})
	defer window.Destroy()
	page := window.Page()
	loaded := make(chan error, 1)
	notify := func(err error) {
		select {
		case loaded <- err:
		default:
		}
	}
	page.OnDidFinishLoad(func() { notify(nil) })
	page.OnDidFailLoad(func(err *mygo.LoadError) { notify(err) })
	page.OnRenderProcessGone(func(reason string) { notify(fmt.Errorf("PDF renderer stopped. %s", reason)) })
	window.OnClosed(func() { notify(errors.New("PDF renderer closed")) })
	if err := page.LoadFile(file); err != nil {
		return pdfOutput{}, err
	}
	select {
	case err := <-loaded:
		if err != nil {
			return pdfOutput{}, err
		}
	case <-time.After(15 * time.Second):
		return pdfOutput{}, errors.New("timed out loading the document for PDF export")
	}
	if err := preparePDFPage(page, body, doc); err != nil {
		return pdfOutput{}, err
	}
	geometry, err := measurePDFGeometry(window, contentWidth, contentHeight)
	if err != nil {
		return pdfOutput{}, err
	}
	if runtime.GOOS != "darwin" && doc.customCSS != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = page.EvalContext(ctx, pdfStylesheetMedia)
		cancel()
		if err != nil {
			return pdfOutput{}, fmt.Errorf("apply PDF styles: %w", err)
		}
	}
	sources, err := capturePDFContent(window, float64(contentWidth), geometry.Height)
	if err == nil {
		var out pdfOutput
		out, err = composePDF(window, doc, float64(contentWidth), geometry, sources)
		if err == nil {
			return out, nil
		}
	}
	if runtime.GOOS == "darwin" || doc.pagination.Continuous {
		return pdfOutput{}, err
	}
	// Some print backends impose their own limits on custom paper height.
	// In paginated mode, retain their ordinary output and apply number/range options.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, resetErr := page.EvalContext(ctx, `
document.getElementById("pdf-capture-markers")?.remove();
document.getElementById("pdf-long-print")?.remove();`); resetErr != nil {
		return pdfOutput{}, errors.Join(err, resetErr)
	}
	l := doc.layout
	data, printErr := printPDFPage(page, mygo.PDFOptions{
		PageSize:  mygo.PageSize{Width: l.PaperWidth / 72, Height: l.PaperHeight / 72},
		Landscape: l.Landscape, Background: true,
		Margins: &mygo.Margins{Top: l.Top / 72, Right: l.Right / 72, Bottom: l.Bottom / 72, Left: l.Left / 72},
	})
	if printErr != nil {
		return pdfOutput{}, errors.Join(err, printErr)
	}
	return finishSystemPrint(page, doc, data)
}

func printPDFPage(page *mygo.Page, options mygo.PDFOptions) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	printed := make(chan result, 1)
	go func() {
		data, err := page.PrintToPDF(options)
		printed <- result{data, err}
	}()
	select {
	case out := <-printed:
		if out.err != nil {
			return nil, out.err
		}
		if !bytes.HasPrefix(out.data, []byte("%PDF-")) {
			return nil, errors.New("the system WebView returned an invalid PDF")
		}
		return out.data, nil
	case <-time.After(45 * time.Second):
		return nil, errors.New("timed out generating the PDF")
	}
}

type workspacePDFPreview struct {
	enabled      bool
	cleanupAdded bool
	returnTab    *workspaceTab
	returnMode   int
	form         exportPageForm
	viewer       pdfViewer
}

func (app *workspaceWindow) togglePDFPreview() {
	if app.draft == nil || app.modalOpen {
		return
	}
	if app.multiFile.active {
		app.problem = multiFileMarkdownOnly
		return
	}
	if app.previewDocumentType() == documentHTML {
		app.problem = htmlPDFExportProblem
		return
	}
	p := &app.pdfPreview
	mode := app.displayedMode()
	if mode != splitMode && mode != previewMode {
		p.returnTab, p.returnMode = app.workspaceTab, app.mode
		app.setMode(splitMode)
	} else if p.enabled {
		p.enabled = false
		p.viewer.clear()
		if p.returnTab == app.workspaceTab && app.mode == splitMode {
			app.setMode(p.returnMode)
		}
		p.returnTab = nil
		return
	}
	p.enabled = true
	if p.form.paper == "" {
		p.form = app.documentPDFForm()
	}
	if !p.cleanupAdded {
		app.win.OnClosed(p.viewer.clear)
		p.cleanupAdded = true
	}
}

func (app *workspaceWindow) refreshDocumentPreview() {
	if app.pdfPreview.enabled && app.previewDocumentType() != documentHTML {
		app.pdfPreview.viewer.reload(app.win)
	} else {
		app.previewPane.returnToDocument(app)
	}
	if app.win != nil {
		app.win.Invalidate()
	}
}

func (p *workspacePDFPreview) draw(c *mygoui.Context, app *workspaceWindow) {
	p.form.layout.LineSpacing = app.preferences.Export.spacing()
	_, width := app.paneToolWidths(c)
	p.viewer.setGridLayout(app.preferences.PDFPreviewGridLayout)
	document := app.exportDocument(p.form.layout)
	document.pagination = p.form.pagination
	document.appearance = p.form.appearance
	p.viewer.draw(c, app.win, document, app.previewZoom, width)
}

// pdfViewer shows exactly the bytes produced by the export pipeline.
type pdfViewer struct {
	view                    *previewViewport
	requested, generated    exportDocument
	payload                 []byte
	directory               string
	pendingFile, activeFile string
	grid                    bool
	thumbnailStyle          pdfThumbnailStyle
	gridFile, gridProblem   string
	gridCancel              context.CancelFunc
	loadTimer, debounce     *time.Timer
	refitTimer              *time.Timer
	generation              uint64
	running, placed, ready  bool
	fitWidth                float32
	zoom                    int
	pages                   int
	figureCounter           int
	problem                 string
}

func (p *pdfViewer) draw(c *mygoui.Context, win *mygo.Window, doc exportDocument, zoom int, width float32) {
	p.requestIfChanged(win, doc)
	problem := p.problem
	if p.problem == "" && p.requested == p.generated && p.view != nil && !p.view.IsDestroyed() {
		file := p.layoutFile(win)
		if file == "" && p.gridProblem == "" {
			file = p.activeFile
		}
		if file != "" {
			p.drawLayout(c, win, file, zoom, width)
			return
		}
		problem = p.gridProblem
	}
	mygoui.Column(c).Grow(1).MinHeight(0).MinWidth(0).Center().Gap(8).Padding(16).Background(c.Theme().Surface).Children(func() {
		if problem != "" {
			mygoui.Text(c, problem).TextColor(c.Theme().TextMuted)
		} else {
			mygoui.Spinner(c).Size(20, 20)
			mygoui.Text(c, "Generating preview…").TextColor(c.Theme().TextMuted)
		}
	})
}

func (p *pdfViewer) requestIfChanged(win *mygo.Window, doc exportDocument) {
	if p.requested != doc {
		p.request(win, doc)
	}
}

func (p *pdfViewer) drawLayout(c *mygoui.Context, win *mygo.Window, file string, zoom int, width float32) {
	if p.loadTimer == nil && p.pendingFile != file && (p.activeFile != file || p.pendingFile != "") {
		if p.refitTimer != nil {
			p.refitTimer.Stop()
			p.refitTimer = nil
		}
		p.pendingFile, p.ready, p.placed = file, false, false
		if p.zoom > 0 && p.zoom != previewZoomDefault {
			p.view.setZoom(previewZoomDefault)
		}
		p.zoom = 0
	}

	slot := mygoui.WebView(c.Key("pdf-document"), p.view).Grow(1).MinWidth(0).MinHeight(0).Label("PDF Preview")
	if p.thumbnailStyle.layout != pdfThumbnailsGrid {
		slot.Label("Pages")
	}
	if p.pendingFile != "" {
		// The column viewer fits its document to the viewport on first load.
		if !p.placed {
			p.placed = true
			c.Invalidate()
		} else if bounds := slot.Bounds(); bounds.W > 0 && bounds.H > 0 {
			p.fitWidth = bounds.W
			p.loadPlacedPDF(win)
		}
		return
	}
	if !p.ready {
		return
	}
	if zoom > 0 && p.zoom != zoom {
		p.zoom = zoom
		p.view.setZoom(zoom)
	}
	if width <= 0 {
		width = slot.Bounds().W
	}
	if !p.grid && width > 0 && p.fitWidth != width {
		p.refitAfterResize(win, width)
	}
}

func (p *pdfViewer) request(win *mygo.Window, doc exportDocument) {
	p.requested, p.problem = doc, ""
	if p.debounce != nil {
		p.debounce.Stop()
	}
	generation := p.generation
	p.debounce = time.AfterFunc(250*time.Millisecond, func() {
		win.Update(func() {
			if generation != p.generation || p.requested != doc {
				return
			}
			p.debounce = nil
			if !p.running {
				p.render(win, doc)
			}
		})
	})
}

func (p *pdfViewer) render(win *mygo.Window, doc exportDocument) {
	p.running = true
	generation := p.generation
	counter := p.figureCounter
	retainCounter := p.thumbnailStyle.layout != pdfThumbnailsGrid && doc.numberFigures
	go func() {
		var target *int
		if retainCounter {
			target = &counter
		}
		output, printErr := generateExportPDFWithFigures(win, doc, target)
		win.Update(func() {
			if win.IsDestroyed() || generation != p.generation {
				return
			}
			p.running = false
			if retainCounter {
				p.figureCounter = counter
			}
			if p.requested != doc {
				if p.debounce == nil {
					p.render(win, p.requested)
				}
				return
			}
			if printErr != nil {
				p.problem = printErr.Error()
				return
			}
			p.generated, p.payload, p.pages = doc, output.Data, output.Pages
			if err := p.load(win, output.Data); err != nil {
				p.problem = err.Error()
			}
		})
	}()
}

func (p *pdfViewer) reload(win *mygo.Window) {
	p.problem = ""
	if len(p.payload) == 0 || p.requested != p.generated {
		p.requested = exportDocument{}
		return
	}
	if win != nil && !win.IsDestroyed() {
		if err := p.load(win, p.payload); err != nil {
			p.problem = err.Error()
		}
	}
}

func (p *pdfViewer) load(win *mygo.Window, data []byte) error {
	p.clearView()
	dir, err := os.MkdirTemp("", "cyberwriter-pdf-preview-")
	if err != nil {
		return err
	}
	p.directory = dir
	path := filepath.Join(dir, "preview.pdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		p.clearView()
		return err
	}
	view, err := newPreviewViewport(win, mygo.WebViewOptions{Page: mygo.PageOptions{
		Untrusted: true, DevTools: mygo.DevToolsDisabled,
	}})
	if err != nil {
		p.clearView()
		return err
	}
	p.view = view
	p.pendingFile, p.zoom = path, 0
	page := view.Page()
	page.OnWillNavigate(func(event *mygo.NavigateEvent) {
		if event.UserInitiated {
			event.PreventDefault()
		}
	})
	page.OnDidFinishLoad(func() {
		if p.view == view && p.loadTimer != nil {
			p.loadTimer.Stop()
			p.loadTimer = nil
			p.ready = true
			p.zoom = 0
			win.Invalidate()
		}
	})
	page.OnDidFailLoad(func(err *mygo.LoadError) {
		if p.view == view && p.loadTimer != nil {
			p.loadTimer.Stop()
			p.loadTimer = nil
			p.activeFile = ""
			p.ready, p.problem = false, "Could not display the PDF preview. "+err.Error()
			win.Invalidate()
		}
	})
	return nil
}

func (p *pdfViewer) loadPlacedPDF(win *mygo.Window) {
	view, path := p.view, p.pendingFile
	p.pendingFile = ""
	p.activeFile = path
	var timeout *time.Timer
	timeout = time.AfterFunc(12*time.Second, func() {
		win.Update(func() {
			if p.view == view && p.loadTimer == timeout && !p.ready && p.problem == "" {
				p.loadTimer = nil
				p.activeFile = ""
				p.problem = "The system WebView could not display this PDF preview."
			}
		})
	})
	p.loadTimer = timeout
	if err := view.Page().LoadFile(path); err != nil {
		p.clearView()
		p.problem = "Could not display the PDF preview. " + err.Error()
		win.Invalidate()
	}
}

func (p *pdfViewer) refitAfterResize(win *mygo.Window, width float32) {
	if p.refitTimer != nil {
		p.refitTimer.Stop()
	}
	p.fitWidth = width
	view := p.view
	p.refitTimer = time.AfterFunc(180*time.Millisecond, func() {
		win.Update(func() {
			if p.view != view || p.grid || !p.ready || p.requested != p.generated || p.fitWidth != width {
				return
			}
			// Reload the current PDF once the pane settles. Printing again would
			// change the document while only its viewport has changed.
			p.pendingFile = filepath.Join(p.directory, "preview.pdf")
			if p.zoom != 100 {
				view.setZoom(previewZoomDefault)
			}
			p.ready, p.zoom = false, 0
			p.loadPlacedPDF(win)
		})
	})
}

func (p *pdfViewer) clearView() {
	if p.refitTimer != nil {
		p.refitTimer.Stop()
		p.refitTimer = nil
	}
	if p.loadTimer != nil {
		p.loadTimer.Stop()
		p.loadTimer = nil
	}
	p.clearThumbnails()
	if p.view != nil {
		view := p.view
		p.view = nil
		view.Destroy()
	}
	if p.directory != "" {
		_ = os.RemoveAll(p.directory)
		p.directory = ""
	}
	p.activeFile = ""
	p.placed, p.ready, p.zoom, p.pendingFile = false, false, 0, ""
	p.fitWidth = 0
}

func (p *pdfViewer) clear() {
	if p.debounce != nil {
		p.debounce.Stop()
	}
	p.clearView()
	*p = pdfViewer{generation: p.generation + 1}
}
