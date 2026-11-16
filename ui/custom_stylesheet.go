package ui

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"cyberwriter/internal/core/exportdoc"
	"github.com/egoist/mygo"
	mygoui "github.com/egoist/mygo/ui"
)

// EV-UI-126 and EV-UI-127.
const starterStylesheet = `/* Add document styles below. Uncomment a declaration to try it. */

/* Reading width and page spacing */
body {
  /* max-width: 44rem; */
  /* padding: 1.5rem; */
}

/* Heading colors */
h1 { /* color: #286fa6; */ }
h2 { /* color: #517e62; */ }

/* Paragraph rhythm */
.markdown-body p {
  /* margin-block-end: 1rem; */
  /* line-height: 1.65; */
}

/* Links, callouts, code and tables */
.wikilink { /* text-decoration-style: dotted; */ }
.callout-title { /* font-weight: 700; */ }
pre, code { /* font-size: 0.9em; */ }
table { /* font-size: 0.95em; */ }

/* Styles for narrow document viewports */
@media screen and (max-width: 540px) {
  h1 { /* font-size: 1.8em; */ }
}`

type workspaceStylesheet struct {
	text, disk string
	problem    string
	dirty      bool
	conflicted bool
	savedUntil time.Time
	pending    *time.Timer
}

func customStylesheetPath() (string, error) {
	file, err := preferenceFile()
	return filepath.Join(filepath.Dir(file), "custom.css"), err
}

func readCustomStylesheet(file string) (string, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err == nil && !utf8.Valid(data) {
		err = errors.New("custom.css is not UTF-8")
	}
	return string(data), err
}

func (app *workspaceWindow) activeCustomStylesheet() string {
	return app.preferences.activeCustomStylesheet()
}

func (prefs *workspacePreferences) activeCustomStylesheet() string {
	if prefs == nil || !prefs.UserStylesheetEnabled {
		return ""
	}
	file, err := customStylesheetPath()
	if err != nil {
		return ""
	}
	css, err := readCustomStylesheet(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(css)
}

func (app *workspaceWindow) invalidateStylesheet() {
	for _, win := range app.documentWindows() {
		if win.win != nil && !win.win.IsDestroyed() {
			win.win.Invalidate()
		}
		if win.settings != nil && !win.settings.IsDestroyed() {
			win.settings.Invalidate()
		}
	}
	if app.application != nil {
		if guide := app.application.featureGuide.win; guide != nil && !guide.IsDestroyed() {
			guide.Invalidate()
		}
	}
}

func (style *workspaceStylesheet) change(app *workspaceWindow) {
	style.dirty = true
	style.conflicted = false
	style.problem = ""
	style.savedUntil = time.Time{}
	if style.pending != nil {
		style.pending.Stop()
	}
	var pending *time.Timer
	pending = time.AfterFunc(600*time.Millisecond, func() {
		app.win.Update(func() {
			if style.pending == pending {
				style.flush(app)
			}
		})
	})
	style.pending = pending
}

func (style *workspaceStylesheet) flush(app *workspaceWindow) bool {
	if style.pending != nil {
		style.pending.Stop()
		style.pending = nil
	}
	if !style.dirty {
		return true
	}
	if style.conflicted {
		return false
	}
	file, err := customStylesheetPath()
	if err == nil {
		err = os.MkdirAll(filepath.Dir(file), 0o700)
	}
	if err == nil {
		var current string
		current, err = readCustomStylesheet(file)
		if err == nil && current != style.disk && current != style.text {
			style.disk = current
			style.conflicted = true
			err = errors.New(app.tr("the file changed outside Settings. Review your draft and edit it again to save",
				"文件已在设置以外被修改。检查草稿后，再次编辑以保存。"))
		}
	}
	if err == nil {
		err = exportdoc.WriteFile(file, []byte(style.text))
	}
	if err != nil {
		style.problem = app.tr("Could not save custom.css. ", "无法保存 custom.css。") + err.Error()
		app.invalidateStylesheet()
		return false
	}
	style.disk, style.dirty, style.problem = style.text, false, ""
	style.savedUntil = time.Now().Add(1200 * time.Millisecond)
	app.invalidateStylesheet()
	time.AfterFunc(time.Until(style.savedUntil), func() {
		app.win.Update(app.invalidateStylesheet)
	})
	return true
}

func (style *workspaceStylesheet) reveal(app *workspaceWindow) {
	file, err := customStylesheetPath()
	if err != nil {
		style.problem = err.Error()
		return
	}
	if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
		style.dirty = true
		if !style.flush(app) {
			return
		}
	} else if err != nil {
		style.problem = app.tr("Could not locate custom.css. ", "找不到 custom.css。") + err.Error()
		return
	}
	mygo.Shell.ShowItemInFolder(file)
}

func (app *workspaceWindow) customStylesheetSettings(c *mygoui.Context) {
	if app.application == nil {
		return
	}
	style := &app.application.stylesheet
	tr := app.tr
	if !style.dirty {
		if file, err := customStylesheetPath(); err == nil {
			if css, err := readCustomStylesheet(file); err == nil && css != style.disk {
				style.text, style.disk = css, css
			}
		}
	}
	theme := c.Theme()
	editorFill, editorEdge, buttonFill, helpInk := theme.Surface, theme.Border, theme.SurfaceHover, theme.Accent
	if app.blackout && !c.Preferences().HighContrast {
		editorFill, editorEdge, buttonFill = mygoui.Hex("#09090a"), mygoui.Hex("#161718"), mygoui.Hex("#131313")
	}
	mygoui.Column(c.Key("custom stylesheet")).FillWidth().Gap(8).Margin(12, 0, 0).Children(func() {
		mygoui.Row(c).FillWidth().Height(20).Gap(9).AlignItems(mygoui.Center).Children(func() {
			mygoui.Text(c, tr("Custom Stylesheet", "自定义样式表")).FontSize(13).Bold().Role(mygoui.RoleHeading)
			chromeButtonIcon(c, "questionmark.circle", tr("Custom Stylesheet Help", "自定义样式表帮助"), func() {
				app.openFeatureGuide("custom-css", app.settings)
			}, 12, helpInk).Size(12, 12)
			mygoui.Spacer(c)
			mygoui.Switch(c, &app.preferences.UserStylesheetEnabled).Label(tr("Enable Custom Stylesheet", "启用自定义样式表")).
				Size(36, 20).OnChange(func() {
				app.preferencesChanged()
				app.invalidateStylesheet()
			})
		})
		mygoui.Text(c, tr("Your own CSS, applied to every note's preview and to PDF and HTML export. It loads after the theme, so your rules win.",
			"自定义 CSS 适用于所有笔记的预览以及 PDF 和 HTML 导出。它在主题之后加载，按层叠规则覆盖主题样式。")).
			FontSize(11).Wrap().TextColor(theme.TextMuted)
		rows := strings.Count(style.text, "\n") + 1
		height := max(float32(220), float32(rows)*16.5)
		mygoui.TextArea(c.Key("stylesheet editor"), &style.text).Label(tr("Custom CSS", "自定义 CSS")).
			FillWidth().Height(height).Shrink(0).Font("monospace").FontSize(12).FixedLineHeight(16.5).
			Padding(6).Radius(6).Background(editorFill).Border(1, editorEdge).
			OnChange(func() { style.change(app) })
		mygoui.Row(c).FillWidth().Height(20).Gap(8).AlignItems(mygoui.Center).Children(func() {
			mygoui.Button(c, tr("Insert Starter Template", "插入示例模板")).Size(141, 20).FontSize(11).Padding(0).
				Background(buttonFill).Disabled(strings.TrimSpace(style.text) != "").OnClick(func() {
				if strings.TrimSpace(style.text) != "" {
					return
				}
				style.text = starterStylesheet
				style.change(app)
			})
			reveal := tr("Reveal in File Manager", "在文件管理器中显示")
			if runtime.GOOS == "darwin" {
				reveal = tr("Reveal in Finder", "在访达中显示")
			}
			revealButton := mygoui.Button(c, reveal).Height(20).FontSize(11).Padding(0, 10).
				Background(buttonFill).OnClick(func() { style.reveal(app) })
			if runtime.GOOS == "darwin" {
				revealButton.Width(103.5).Padding(0)
			}
			mygoui.Spacer(c)
			if time.Now().Before(style.savedUntil) {
				mygoui.Text(c, tr("Saved", "已保存")).FontSize(11).TextColor(theme.TextMuted).Role(mygoui.RoleStatus)
			}
		})
		if style.problem != "" {
			mygoui.Text(c, style.problem).FontSize(11).Wrap().TextColor(theme.Danger).Role(mygoui.RoleAlert)
		}
		mygoui.Text(c, tr("Edits here and edits made in another editor both take effect on the next render.",
			"在此处或其他编辑器中所做的修改，会在下次渲染时生效。")).
			FontSize(11).Wrap().TextColor(theme.TextMuted)
	})
}

// A standalone document carries the stylesheet as CSS data. Its existing
// script, resource and navigation policies still apply.
func standaloneCustomStylesheet(policy, css string) (string, string) {
	if css == "" {
		return policy, ""
	}
	policy = strings.Replace(policy, "style-src 'unsafe-inline'", "style-src 'unsafe-inline' data:", 1)
	href := base64.StdEncoding.EncodeToString([]byte(css))
	return policy, `<link id="custom-stylesheet" rel="stylesheet" href="data:text/css;base64,` + href + `">`
}
