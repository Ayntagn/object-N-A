package ui

import (
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"math"
	"strconv"
	"strings"

	"cyberwriter/internal/core/exportdoc"
	"cyberwriter/ui/wysiwyg"
)

func (m *previewMarkup) writeTransformedFigure(src, alt, caption, title, anchor string, transform wysiwyg.ImageTransform) {
	naturalW, naturalH, ok := previewImageDimensions(src)
	if !ok {
		if caption != "" {
			m.writeCaptionedFigure(src, alt, caption, title, anchor)
		} else {
			if src == "" {
				src = "data:,"
			}
			fmt.Fprintf(&m.out, "<p%s>", anchor)
			m.writeImage(src, alt, title, 0)
			m.out.WriteString("</p>")
		}
		return
	}

	width, _ := transform.DisplaySize(float64(naturalW), float64(naturalH), math.Inf(1))
	orientedW, orientedH := naturalW, naturalH
	if transform.SwapsAxes() {
		orientedW, orientedH = naturalH, naturalW
	}
	widthCSS := strconv.FormatFloat(width, 'f', -1, 64)
	imagePercent := 100 * float64(naturalW) / float64(orientedW)

	// The flow box uses the rotated dimensions; only the bitmap is rotated.
	// It can shrink with its column while retaining the same proportions.
	fmt.Fprintf(&m.out, `<figure class="md-figure"%s><span style="display:block;position:relative;overflow:hidden;margin-inline:auto;width:min(100%%,%spx);aspect-ratio:%d/%d">`,
		anchor, widthCSS, orientedW, orientedH)
	fmt.Fprintf(&m.out, `<img src="%s" alt="%s" style="position:absolute;left:50%%;top:50%%;width:%g%%;height:auto;max-width:none;transform:translate(-50%%,-50%%) rotate(%ddeg)"`,
		src, html.EscapeString(alt), imagePercent, transform.Rotation)
	if title != "" {
		fmt.Fprintf(&m.out, ` title="%s"`, html.EscapeString(title))
	}
	m.out.WriteString("></span>")
	if caption != "" {
		label, caption := exportdoc.FigureCaption(caption, m.figures.numbered, &m.figures.counter)
		m.out.WriteString("<figcaption>")
		if label != "" {
			fmt.Fprintf(&m.out, `<span class="figure-label">%s</span> `, label)
		}
		m.out.WriteString(html.EscapeString(caption))
		m.out.WriteString("</figcaption>")
	}
	m.out.WriteString("</figure>")
}

func previewImageDimensions(src string) (width, height int, ok bool) {
	mime, encoded, found := strings.Cut(src, ",")
	if !found || !strings.HasPrefix(mime, "data:image/") || !strings.HasSuffix(mime, ";base64") {
		return 0, 0, false
	}
	data := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
	config, format, err := image.DecodeConfig(data)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0, false
	}
	// Both the browser and MyGo apply JPEG orientation when showing the
	// bitmap. DecodeConfig supplies the encoded axes for the same image.
	if format == "jpeg" && jpegOrientationSwapsAxes(base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))) {
		return config.Height, config.Width, true
	}
	return config.Width, config.Height, true
}
