package ui

import (
	"encoding/base64"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"path"
	"strconv"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	previewNoteScheme = "cyberwriter-note"
	previewPDFScheme  = "cyberwriter-pdf"
)

func previewNoteURL(source, target, anchor string) string {
	return previewNoteDestination(source, target, anchor, false)
}

func previewNoteDestination(source, target, anchor string, currentTab bool) string {
	if target == source {
		return (&url.URL{Fragment: anchor}).String()
	}
	query := url.Values{"path": {target}}
	if anchor != "" {
		query.Set("anchor", anchor)
	}
	if currentTab {
		query.Set("tab", "current")
	}
	return (&url.URL{Scheme: previewNoteScheme, Host: "open", RawQuery: query.Encode()}).String()
}

func previewNoteNavigation(raw string) (relative, anchor string, currentTab, ok bool) {
	link, err := url.Parse(raw)
	if err != nil || link.Scheme != previewNoteScheme || link.Host != "open" || link.User != nil {
		return "", "", false, false
	}
	query := link.Query()
	relative = query.Get("path")
	if !fs.ValidPath(relative) || !workspace.IsMarkdownPath(relative) {
		return "", "", false, false
	}
	return relative, query.Get("anchor"), query.Get("tab") == "current", true
}

func previewPDFDestination(relative string, page int) string {
	query := url.Values{"path": {relative}}
	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}
	return (&url.URL{Scheme: previewPDFScheme, Host: "open", RawQuery: query.Encode()}).String()
}

func previewPDFNavigation(raw string) (relative string, page int, ok bool) {
	link, err := url.Parse(raw)
	if err != nil || link.Scheme != previewPDFScheme || link.Host != "open" || link.User != nil ||
		link.Path != "" && link.Path != "/" || link.Fragment != "" {
		return "", 0, false
	}
	query, err := url.ParseQuery(link.RawQuery)
	relative = query.Get("path")
	if err != nil || len(query["path"]) != 1 || !fs.ValidPath(relative) ||
		strings.ContainsAny(relative, "\\\x00\r\n") || !workspace.IsPDFPath(relative) {
		return "", 0, false
	}
	if pages := query["page"]; len(pages) != 0 {
		page, err = strconv.Atoi(pages[0])
		if len(pages) != 1 || err != nil || page < 1 {
			return "", 0, false
		}
	}
	return relative, page, true
}

func (app *workspaceWindow) followPreviewAnchor(anchor string) {
	if anchor == "" {
		return
	}
	if !app.multiFile.active && (app.mode == sourceMode || app.mode == wysiwygMode) {
		app.setMode(splitMode)
	}
	app.previewAnchor = anchor
}

type previewResources struct {
	index             *vaultindex.Index
	vault             *workspace.Vault
	wikiImages        *wikiImageNames
	wikiPDFs          *wikiPDFIndex
	resolveReportNote func(string) (string, bool)
	images            map[string]string
	fileImages        *previewImageFrame
	remaining         int64
	embeds            int
	chartFiles        *previewChartFiles
	chartContents     map[string]string
	chartAssets       *previewChartAssetFrame
}

type previewResourceOrigin struct {
	vault      *workspace.Vault
	index      *vaultindex.Index
	wikiImages *wikiImageNames
	wikiPDFs   *wikiPDFIndex
	note       string
}

func (app *workspaceWindow) previewOrigin() previewResourceOrigin {
	return app.tabPreviewOrigin(app.workspaceTab)
}

func (app *workspaceWindow) tabPreviewOrigin(tab *workspaceTab) previewResourceOrigin {
	if tab.storage != nil {
		return previewResourceOrigin{vault: tab.storage, note: tab.document.Path}
	}
	return previewResourceOrigin{vault: app.vault, index: app.index.data, wikiImages: app.index.wikiImages,
		wikiPDFs: app.index.wikiPDFs, note: tab.document.Path}
}

func (origin previewResourceOrigin) resources() *previewResources {
	if origin.vault == nil || origin.note == "" {
		return nil
	}
	return &previewResources{vault: origin.vault, index: origin.index, wikiImages: origin.wikiImages, wikiPDFs: origin.wikiPDFs,
		remaining: 12 << 20, images: make(map[string]string)}
}

// wikiImageNames is an immutable snapshot of the visible Vault attachments.
// An empty value identifies a name shared by more than one attachment.
type wikiImageNames map[string]string

func (origin previewResourceOrigin) wikiImageCandidates(target string) []string {
	if !workspace.IsRasterImagePath(target) || strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\\x00") {
		return nil
	}
	nearby := path.Join(path.Dir(origin.note), target)
	first, second := nearby, ""
	switch {
	case strings.HasPrefix(target, "./"), strings.HasPrefix(target, "../"):
	case strings.ContainsRune(target, '/'):
		// Match an explicit Vault path first, keeping the earlier same-note-folder fallback.
		first, second = target, nearby
	case origin.wikiImages != nil:
		second = (*origin.wikiImages)[target]
	default:
		second = target
	}
	var candidates []string
	if fs.ValidPath(first) {
		candidates = append(candidates, first)
	}
	if second != first && fs.ValidPath(second) {
		candidates = append(candidates, second)
	}
	return candidates
}

func (m *previewMarkup) rawInline(n parser.Inline) string {
	source := m.doc.Source()
	if n.Start >= 0 && n.End <= len(source) && n.End > n.Start {
		return source[n.Start:n.End]
	}
	return ""
}

func (m *previewMarkup) prose(n parser.Inline) {
	if (m.resources == nil && m.standalone == htmlInApp && m.snippets == nil) || m.inLink || !strings.Contains(n.Text, "[[") {
		m.out.WriteString(html.EscapeString(n.Text))
		return
	}
	raw := m.rawInline(n)
	links := vaultindex.Wikilinks(raw)
	if len(links) == 0 {
		m.out.WriteString(html.EscapeString(n.Text))
		return
	}
	start := 0
	for _, link := range links {
		m.out.WriteString(html.EscapeString(previewLiteral(raw[start:link.Start])))
		if !m.wikilink(link) {
			m.out.WriteString(html.EscapeString(previewLiteral(raw[link.Start:link.End])))
		}
		start = link.End
	}
	m.out.WriteString(html.EscapeString(previewLiteral(raw[start:])))
}

// Preserve the Markdown parser's handling of escapes and entities on either
// side of a Wikilink. A Text inline has no nested emphasis or links.
func previewLiteral(raw string) string {
	var text strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '&' {
			if n, entity := parser.Entity(raw[i:]); n > 0 {
				text.WriteString(entity)
				i += n - 1
				continue
			}
		}
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune(`!"#$%&'()*+,-./:;<=>?@[\]^_`+"\x60"+`{|}~`, rune(raw[i+1])) {
			i++
		}
		text.WriteByte(raw[i])
	}
	return text.String()
}

func (m *previewMarkup) wikilink(link vaultindex.Wikilink) bool {
	if m.writeSnippetLink(link) {
		return true
	}
	if name, _, _ := strings.Cut(link.Target, "#"); !link.Embed && workspace.IsPDFPath(strings.TrimSpace(name)) {
		return m.writePDFWikilink(link)
	}
	if link.Embed {
		if src := m.imageSource(link.Target, true); src != "" {
			alt := link.Label
			if m.standalone != htmlInApp {
				alt = link.Target
			}
			m.writeImage(src, alt, "", 0)
			return true
		}
	}
	if m.standalone != htmlInApp {
		if link.Embed {
			return false
		}
		target := link.Target
		if !strings.HasSuffix(strings.ToLower(target), ".md") {
			target += ".md"
		}
		if dest := standaloneRelativeURL(target); dest != "" {
			fmt.Fprintf(&m.out, `<a class="wikilink" href="%s">%s</a>`, html.EscapeString(dest), standaloneWikilinkLabel(link.Label))
			return true
		}
		return false
	}
	if m.resources == nil {
		return false
	}
	var relative string
	var resolved bool
	fromHealthReport := m.resources.resolveReportNote != nil && m.relative == workspace.VaultHealthFile && !link.Embed
	if fromHealthReport {
		relative, resolved = m.resources.resolveReportNote(link.Target)
	}
	if !resolved && m.resources.index != nil {
		relative, resolved = m.resources.index.Resolve(m.relative, link.Target)
	}
	if resolved {
		_, section, _ := strings.Cut(link.Target, "#")
		dest := previewNoteDestination(m.relative, relative, markdown.Slug(section), fromHealthReport)
		if dest == "" {
			dest = "#"
		}
		fmt.Fprintf(&m.out, `<a class="wikilink" href="%s">%s</a>`, html.EscapeString(dest), html.EscapeString(link.Label))
		return true
	}
	return false
}

func (m *previewMarkup) writePDFWikilink(link vaultindex.Wikilink) bool {
	relative, page, ok := m.resources.resolveWikiPDF(m.relative, link.Target)
	if !ok {
		return false
	}
	label := html.EscapeString(link.Label)
	if m.figures != nil && m.figures.pdf {
		// Printed PDFs use the alias as text. The export has no Vault opener.
		m.out.WriteString(label)
		return true
	}
	var dest string
	if m.standalone == htmlInApp {
		dest = previewPDFDestination(relative, page)
	} else {
		target := url.URL{Path: relative}
		if page > 0 {
			target.Fragment = "page=" + strconv.Itoa(page)
		}
		dest = target.String()
	}
	fmt.Fprintf(&m.out, `<a class="wikilink" href="%s">%s</a>`, html.EscapeString(dest), label)
	return true
}

func (m *previewMarkup) embedParagraph(in []parser.Inline, anchor string) bool {
	res := m.resources
	if len(in) != 1 || in[0].Kind != parser.Text || res.index == nil || res.embeds >= 8 {
		return false
	}
	raw := strings.TrimSpace(m.rawInline(in[0]))
	links := vaultindex.Wikilinks(raw)
	if len(links) != 1 || !links[0].Embed || links[0].Start != 0 || links[0].End != len(raw) {
		return false
	}
	link := links[0]
	if workspace.IsRasterImagePath(link.Target) && m.imageSource(link.Target, true) != "" {
		return false
	}
	relative, ok := res.index.Resolve(m.relative, link.Target)
	if !ok || relative == m.relative {
		return false
	}
	doc, err := res.vault.ReadDocument(relative)
	if err != nil {
		return false
	}
	res.embeds++
	if m.standalone != htmlInApp {
		child := previewMarkup{doc: parser.New(m.doc.Extensions()), resources: res, relative: relative, embedded: true,
			charts: m.charts, figures: m.figures, standalone: m.standalone, codeTheme: m.codeTheme,
			calloutStyles: m.calloutStyles}
		body, _ := child.render(doc.Text, "", true)
		body = child.fillWikiFigures(body)
		fmt.Fprintf(&m.out, `<div class="embed-note"%s><header>%s</header>%s</div>`, anchor, html.EscapeString(link.Target), body)
		return true
	}
	_, _, body := workspace.SplitFrontmatter(doc.Text)
	child := previewMarkup{doc: parser.New(m.doc.Extensions()), resources: res, relative: relative, embedded: true,
		charts: m.charts, figures: m.figures, codeTheme: m.codeTheme, codeCopies: m.codeCopies, calloutStyles: m.calloutStyles}
	child.setSource(body)
	child.blocks(0)
	fmt.Fprintf(&m.out, `<aside class="note-embed"%s>`, anchor)
	m.wikilink(link)
	m.out.WriteString(child.out.String())
	m.out.WriteString("</aside>")
	return true
}

func (m *previewMarkup) imageSource(destination string, wiki bool) string {
	res := m.resources
	if res == nil {
		return ""
	}
	var candidates []string
	if wiki {
		origin := previewResourceOrigin{note: m.relative, wikiImages: res.wikiImages}
		candidates = origin.wikiImageCandidates(destination)
	} else {
		relative, ok := workspace.ResolveImagePath(m.relative, destination)
		if !ok {
			return ""
		}
		candidates = append(candidates, relative)
	}
	for _, relative := range candidates {
		if cached, exists := res.images[relative]; exists {
			if cached != "" {
				return cached
			}
			continue
		}
		limit := min(res.remaining, 4<<20)
		if limit <= 0 {
			break
		}
		if res.fileImages != nil {
			asset, ok := res.fileImages.add(res.vault, relative, limit)
			if !ok {
				res.images[relative] = ""
				continue
			}
			res.remaining -= asset.bytes
			src := previewImageOrigin + asset.route
			res.images[relative] = src
			return src
		}
		image, mime, err := res.vault.ReadPreviewImage(relative, limit)
		if err != nil {
			res.images[relative] = ""
			continue
		}
		res.remaining -= int64(len(image))
		src := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image)
		res.images[relative] = src
		return src
	}
	return ""
}
