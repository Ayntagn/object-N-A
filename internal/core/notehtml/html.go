// Package notehtml filters HTML authored in a Markdown note for rendering.
package notehtml

import (
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Policy resolves safe user-authored links and images through the caller's
// document resource policy. Print exposes disclosures and replaces frames
// with links.
type Policy struct {
	Link  func(string) string
	Image func(string) string
	Print bool
}

// InlineTag describes one raw HTML token in a Markdown inline stream.
// DropContent identifies elements whose contents must be omitted.
type InlineTag struct {
	Name, Markup             string
	Close, Void, DropContent bool
}

var inlineTags = names("a abbr audio b bdi bdo br button cite code del dfn em font i iframe img input ins kbd label mark meter output progress q rp rt ruby s samp small source span strong sub sup time track u var video wbr")
var blockTags = names("address article aside blockquote caption center col colgroup dd details dialog div dl dt fieldset figcaption figure footer form h1 h2 h3 h4 h5 h6 header hgroup hr legend li main nav ol optgroup option p picture pre section select summary table tbody td textarea tfoot th thead tr ul")
var svgTags = names("svg circle defs desc ellipse g line linearGradient path polygon polyline radialGradient rect stop text title tspan")
var mathTags = names("math maction menclose merror mfenced mfrac mi mmultiscripts mn mo mover mpadded mphantom mprescripts mroot mrow ms mspace msqrt mstyle msub msubsup msup mtable mtd mtext mtr munder munderover none semantics")
var voidTags = names("br col hr img input source track wbr")
var droppedTags = names("applet base embed frame frameset link meta noembed noframes noscript object plaintext script style template xmp")

var generalAttrs = names("dir hidden lang role title")
var htmlAttrs = map[string]string{
	"audio": "controls loop muted preload", "blockquote": "cite",
	"col": "align span valign width", "colgroup": "align span valign width",
	"del": "cite datetime", "details": "name open", "dialog": "open",
	"fieldset": "disabled", "font": "color face size", "img": "alt height width",
	"input": "checked disabled max maxlength min minlength placeholder readonly size step value",
	"ins":   "cite datetime", "label": "for", "li": "value", "meter": "high low max min optimum value",
	"ol": "reversed start type", "optgroup": "disabled label", "option": "disabled label selected value",
	"output": "for", "progress": "max value", "q": "cite", "select": "disabled multiple size",
	"source": "media type", "table": "align bgcolor border cellpadding cellspacing frame rules summary width",
	"tbody": "align valign", "td": "abbr align bgcolor colspan headers height rowspan valign width",
	"textarea": "cols disabled maxlength minlength placeholder readonly rows",
	"tfoot":    "align valign", "th": "abbr align bgcolor colspan headers height rowspan scope valign width",
	"thead": "align valign", "time": "datetime", "tr": "align bgcolor valign",
	"track": "default kind label srclang", "video": "controls height loop muted playsinline preload width",
}
var htmlAlign = names("div h1 h2 h3 h4 h5 h6 hr p")
var svgAttrs = names("cx cy d dx dy fill-rule font-family font-size font-style font-weight fx fy gradientTransform gradientUnits height offset opacity preserveAspectRatio points r rx ry spreadMethod stop-opacity stroke-dasharray stroke-dashoffset stroke-linecap stroke-linejoin stroke-miterlimit stroke-opacity stroke-width text-anchor transform viewBox width x x1 x2 y y1 y2")
var svgPaint = names("fill stroke stop-color")
var mathAttrs = names("accent accentunder align columnalign columnlines columnspacing display displaystyle fence frame largeop linethickness lspace mathbackground mathcolor mathsize mathvariant maxsize minsize notation rowalign rowlines rowspacing rowspan rspace scriptlevel stretchy symmetric width")
var inputTypes = names("button checkbox color date datetime-local email month number radio range search tel text time url week")

// Keep authored anchors and classes separate from selectors used by the
// preview's own controls, charts, and document shell.
var reservedClasses = names("api-run-btn api-result-toggle markdown-body math-display math-inline mermaid preview-code-copy snippet-link vega vega-lite")
var reservedIDs = names("__mygo cw-snippet-hover custom-stylesheet mygo preview preview-theme-fonts")
var reservedData = names("data-callout data-color-emphasis data-contents-anchor data-cw-more data-cw-scroll-past-end data-document data-kind data-language data-mermaid-background data-offset data-preview-theme data-snippet-card data-snippet-template data-source-line data-vega-done data-vega-preview data-version")

func names(list string) map[string]bool {
	set := make(map[string]bool)
	for _, name := range strings.Fields(list) {
		set[name] = true
	}
	return set
}

// Fragment renders an HTML block as a self-contained fragment.
func (p Policy) Fragment(source string) (string, bool) {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(source), context)
	if err != nil {
		return "", false
	}

	cleaned := &html.Node{Type: html.DocumentNode}
	for _, node := range nodes {
		p.appendNode(cleaned, node)
	}
	var markup strings.Builder
	if err := html.Render(&markup, cleaned); err != nil {
		return "", false
	}
	return markup.String(), true
}

func (p Policy) appendNode(parent, node *html.Node) {
	switch node.Type {
	case html.TextNode:
		parent.AppendChild(&html.Node{Type: html.TextNode, Data: node.Data})
	case html.ElementNode:
		if droppedTags[node.Data] {
			return
		}
		if node.Namespace == "" && node.Data == "iframe" {
			if frame := p.frame(node.Attr); frame != nil {
				parent.AppendChild(frame)
			}
			return
		}
		if p.Print && node.Namespace == "" && mediaElement(node.Data) {
			src := webURL(attributeValue(node.Attr, "src"))
			for child := node.FirstChild; src == "" && child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && child.Namespace == "" && child.Data == "source" {
					src = webURL(attributeValue(child.Attr, "src"))
				}
			}
			if link := resourceLink(src, attributeValue(node.Attr, "title")); link != nil {
				parent.AppendChild(link)
			}
			return
		}
		if allowedElement(node.Namespace, node.Data) {
			cleaned := &html.Node{Type: html.ElementNode, DataAtom: node.DataAtom, Data: node.Data, Namespace: node.Namespace, Attr: p.attributes(node.Namespace, node.Data, node.Attr)}
			parent.AppendChild(cleaned)
			parent = cleaned
		} else if node.Namespace != "" {
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			p.appendNode(parent, child)
		}
	}
}

func allowedElement(namespace, tag string) bool {
	switch namespace {
	case "":
		return inlineTags[tag] || blockTags[tag]
	case "svg":
		return svgTags[tag]
	case "math":
		return mathTags[tag]
	default:
		return false
	}
}

// InlineTag filters a single token. The caller pairs normal start and end
// tags within the current Markdown span.
func (p Policy) InlineTag(source string) InlineTag {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	kind := tokenizer.Next()
	if kind != html.StartTagToken && kind != html.EndTagToken && kind != html.SelfClosingTagToken {
		return InlineTag{}
	}
	token := tokenizer.Token()
	if len(tokenizer.Raw()) != len(source) {
		return InlineTag{}
	}
	tag := InlineTag{Name: token.Data, Close: kind == html.EndTagToken, Void: voidTags[token.Data]}
	if droppedTags[tag.Name] {
		switch tag.Name {
		case "applet", "noembed", "noframes", "noscript", "object", "plaintext", "script", "style", "template", "xmp":
			tag.DropContent = true
			return tag
		default:
			return InlineTag{}
		}
	}
	if !inlineTags[tag.Name] || p.Print && (tag.Name == "source" || tag.Name == "track") {
		return InlineTag{}
	}
	if tag.Name == "iframe" || p.Print && mediaElement(tag.Name) {
		// Frames and print links are atomic. Raw frame children never
		// become markup.
		tag.Void = true
		if !tag.Close {
			var element *html.Node
			if tag.Name == "iframe" {
				element = p.frame(token.Attr)
			} else {
				element = resourceLink(webURL(attributeValue(token.Attr, "src")), attributeValue(token.Attr, "title"))
			}
			if element != nil {
				var markup strings.Builder
				if html.Render(&markup, element) == nil {
					tag.Markup = markup.String()
				}
			}
		}
		return tag
	}
	if !tag.Close {
		token.Type = html.StartTagToken
		token.Attr = p.attributes("", tag.Name, token.Attr)
		tag.Markup = token.String()
		if kind == html.SelfClosingTagToken && !tag.Void {
			tag.Markup += "</" + tag.Name + ">"
			tag.Void = true
		}
	}
	return tag
}

func (p Policy) attributes(namespace, tag string, attrs []html.Attribute) []html.Attribute {
	var cleaned []html.Attribute
	for _, attr := range attrs {
		if attr.Namespace != "" {
			continue
		}
		value, keep := p.attribute(namespace, tag, attr.Key, attr.Val)
		if keep {
			cleaned = append(cleaned, html.Attribute{Key: attr.Key, Val: value})
		}
	}
	switch tag {
	case "button":
		cleaned = append(cleaned, html.Attribute{Key: "type", Val: "button"})
	case "details":
		if p.Print && !hasAttribute(cleaned, "open") {
			cleaned = append(cleaned, html.Attribute{Key: "open"})
		}
	case "img":
		if webURL(attributeValue(cleaned, "src")) != "" {
			cleaned = append(cleaned, html.Attribute{Key: "referrerpolicy", Val: "no-referrer"})
		}
	}
	return cleaned
}

func (p Policy) attribute(namespace, tag, key, value string) (string, bool) {
	switch key {
	case "class":
		var kept []string
		for _, part := range strings.Fields(value) {
			if !reservedClasses[part] {
				kept = append(kept, part)
			}
		}
		return strings.Join(kept, " "), len(kept) > 0
	case "id":
		return value, value != "" && !reservedIDs[value] && !strings.ContainsAny(value, " \t\r\n\f")
	case "style":
		cleaned := cleanStyle(value)
		return cleaned, cleaned != ""
	}
	if generalAttrs[key] || customAttribute(key, "aria-") || customAttribute(key, "data-") && !reservedData[key] {
		return value, true
	}
	switch namespace {
	case "svg":
		return value, svgAttrs[key] || svgPaint[key] && passivePaint(value)
	case "math":
		return value, mathAttrs[key]
	}
	switch {
	case tag == "a" && key == "href":
		href := ordinaryURL(value, true)
		if href != "" && p.Link != nil {
			href = p.Link(href)
		}
		return href, href != ""
	case tag == "img" && key == "src", tag == "video" && key == "poster":
		src := p.image(value)
		return src, src != ""
	case key == "src" && (mediaElement(tag) || tag == "source" || tag == "track"):
		src := webURL(value)
		if src == "" {
			src = dataURL(value, "audio/mpeg audio/mp4 audio/ogg audio/wav audio/webm audio/x-wav video/mp4 video/ogg video/webm text/vtt")
		}
		return src, src != "" && !p.Print
	case tag == "input" && key == "type":
		kind := strings.ToLower(value)
		return kind, inputTypes[kind]
	case tag == "input" && key == "name":
		// Names keep radio sets functional without exposing form actions.
		return value, true
	case key == "cite":
		cite := ordinaryURL(value, false)
		return cite, cite != "" && includes(htmlAttrs[tag], key)
	case key == "align" && htmlAlign[tag]:
		return value, true
	default:
		return value, includes(htmlAttrs[tag], key)
	}
}

func (p Policy) frame(attrs []html.Attribute) *html.Node {
	src := webURL(attributeValue(attrs, "src"))
	if src == "" {
		return nil
	}
	if p.Print {
		return resourceLink(src, attributeValue(attrs, "title"))
	}
	sandbox := "allow-scripts allow-presentation"
	if hasAttribute(attrs, "sandbox") {
		var permitted []string
		for _, item := range strings.Fields(strings.ToLower(attributeValue(attrs, "sandbox"))) {
			if item == "allow-scripts" || item == "allow-presentation" {
				permitted = append(permitted, item)
			}
		}
		sandbox = strings.Join(permitted, " ")
	}
	cleaned := []html.Attribute{{Key: "src", Val: src}, {Key: "sandbox", Val: sandbox}, {Key: "referrerpolicy", Val: "no-referrer"}}
	for _, attr := range attrs {
		switch attr.Key {
		case "allowfullscreen", "height", "title", "width":
			cleaned = append(cleaned, attr)
		}
	}
	return &html.Node{Type: html.ElementNode, DataAtom: atom.Iframe, Data: "iframe", Attr: cleaned}
}

func mediaElement(tag string) bool {
	return tag == "audio" || tag == "video"
}

func resourceLink(src, title string) *html.Node {
	if src == "" {
		return nil
	}
	if title == "" {
		title = src
	}
	link := &html.Node{Type: html.ElementNode, DataAtom: atom.A, Data: "a", Attr: []html.Attribute{{Key: "href", Val: src}}}
	link.AppendChild(&html.Node{Type: html.TextNode, Data: title})
	return link
}

func hasAttribute(attrs []html.Attribute, key string) bool {
	for _, attr := range attrs {
		if attr.Key == key && attr.Namespace == "" {
			return true
		}
	}
	return false
}

func attributeValue(attrs []html.Attribute, key string) string {
	for _, attr := range attrs {
		if attr.Key == key && attr.Namespace == "" {
			return attr.Val
		}
	}
	return ""
}

func customAttribute(key, prefix string) bool {
	tail, ok := strings.CutPrefix(key, prefix)
	if !ok || tail == "" {
		return false
	}
	for _, r := range tail {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

func includes(list, name string) bool {
	for candidate := range strings.FieldsSeq(list) {
		if candidate == name {
			return true
		}
	}
	return false
}

func passivePaint(value string) bool {
	value = strings.TrimSpace(value)
	paint := strings.ToLower(value)
	if !strings.Contains(paint, "(") {
		return !strings.ContainsAny(value, "\\")
	}
	for _, function := range []string{"rgb(", "rgba(", "hsl(", "hsla("} {
		if strings.HasPrefix(paint, function) && strings.HasSuffix(paint, ")") {
			return !strings.ContainsAny(paint[len(function):len(paint)-1], "()\\")
		}
	}
	if id, local := strings.CutPrefix(value, "url(#"); local && strings.HasSuffix(id, ")") {
		id = strings.TrimSuffix(id, ")")
		return id != "" && !strings.ContainsAny(id, " ()'\"\\\t\r\n")
	}
	return false
}

func (p Policy) image(raw string) string {
	if src := dataURL(raw, "image/avif image/bmp image/gif image/jpeg image/png image/webp image/x-icon"); src != "" {
		return src
	}
	src := ordinaryURL(raw, false)
	if src == "" || p.Image == nil {
		return ""
	}
	return p.Image(src)
}

func dataURL(raw, mimeList string) string {
	raw = strings.TrimSpace(raw)
	meta, _, ok := strings.Cut(raw, ",")
	if !ok {
		return ""
	}
	mime, ok := strings.CutPrefix(strings.ToLower(meta), "data:")
	if !ok {
		return ""
	}
	if media, ok := strings.CutSuffix(mime, ";base64"); ok && includes(mimeList, media) {
		return raw
	}
	return ""
}

func parseURL(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsRune(raw, '\\') || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return nil
	}
	target, err := url.Parse(raw)
	if err != nil || target.User != nil {
		return nil
	}
	return target
}

func ordinaryURL(raw string, mailto bool) string {
	target := parseURL(raw)
	if target == nil {
		return ""
	}
	if (target.Scheme == "http" || target.Scheme == "https") && target.Hostname() != "" ||
		target.Scheme == "" && target.Host == "" && !strings.HasPrefix(target.Path, "/") ||
		mailto && target.Scheme == "mailto" && target.Opaque != "" {
		return target.String()
	}
	return ""
}

func webURL(raw string) string {
	target := parseURL(raw)
	if target == nil || target.Hostname() == "" || target.Scheme != "http" && target.Scheme != "https" {
		return ""
	}
	return target.String()
}
