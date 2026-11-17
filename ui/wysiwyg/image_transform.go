package wysiwyg

import (
	"math"
	"strconv"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const imageTransformPrefix = "<!-- object-na-image: scale="
const imageTransformEnd = " -->"

// ImageTransform holds the display multiplier and clockwise rotation.
type ImageTransform struct {
	Ratio    float64
	Rotation int
}

func (t ImageTransform) valid() bool {
	return t.Ratio > 0 && !math.IsInf(t.Ratio, 0) && t.Rotation >= 0 && t.Rotation < 360 && t.Rotation%90 == 0
}

func (t ImageTransform) SwapsAxes() bool { return t.Rotation == 90 || t.Rotation == 270 }

// DisplaySize fits the rotated extent to the available width while preserving
// the aspect ratio. Large images share the visual editor's height limit.
func (t ImageTransform) DisplaySize(width, height, available float64) (float64, float64) {
	if t.SwapsAxes() {
		width, height = height, width
	}
	factor := min(max(t.Ratio, 1/max(width, height)), available/width, 4096/height)
	return width * factor, height * factor
}

func legacyImageRatio(alt string) float64 {
	ratio, err := strconv.ParseFloat(strings.TrimSpace(alt), 64)
	if err == nil && (ImageTransform{Ratio: ratio}).valid() {
		return ratio
	}
	return 1
}

func (t ImageTransform) suffix(alt string) string {
	if t.Rotation == 0 && t.Ratio == legacyImageRatio(alt) {
		return ""
	}
	scale := strconv.FormatFloat(t.Ratio, 'f', -1, 64)
	_, places, decimal := strings.Cut(scale, ".")
	if !decimal {
		scale += ".00"
	} else if len(places) == 1 {
		scale += "0"
	}
	return " " + imageTransformPrefix + scale + " rotate=" + strconv.Itoa(t.Rotation) + imageTransformEnd
}

// ReadBlockImageTransform recognizes a single trailing comment on a
// standalone inline-style image. Callers supply the inlines of a root
// Markdown paragraph. The end offset includes the comment for splicing.
func ReadBlockImageTransform(source string, in []parser.Inline) (ImageTransform, int, bool) {
	if len(in) < 3 || in[0].Kind != parser.Image || in[0].Pending {
		return ImageTransform{}, 0, false
	}
	image := in[0]
	comment := in[len(in)-1]
	if image.Next != len(in)-2 || in[image.Next].Kind != parser.Text ||
		comment.Kind != parser.HTML || comment.Start <= image.End ||
		strings.Trim(source[image.End:comment.Start], " \t") != "" ||
		image.End <= image.Start || source[image.End-1] != ')' {
		return ImageTransform{}, 0, false
	}
	t, ok := parseImageTransform(comment.Text)
	return t, comment.End, ok
}

// ReadWikiBlockImage reads a standalone bitmap embed with a trailing image
// transform comment. Link offsets and the returned end index refer to source.
func ReadWikiBlockImage(source string, in []parser.Inline) (vaultindex.Wikilink, ImageTransform, int, bool) {
	if len(in) != 2 || in[0].Kind != parser.Text || in[1].Kind != parser.HTML {
		return vaultindex.Wikilink{}, ImageTransform{}, 0, false
	}
	text, comment := in[0], in[1]
	transform, ok := parseImageTransform(comment.Text)
	if !ok {
		return vaultindex.Wikilink{}, ImageTransform{}, 0, false
	}
	raw := source[text.Start:comment.Start]
	links := vaultindex.Wikilinks(raw)
	if len(links) != 1 {
		return vaultindex.Wikilink{}, ImageTransform{}, 0, false
	}
	link := links[0]
	if !link.Embed || !workspace.IsRasterImagePath(link.Target) ||
		strings.Trim(raw[:link.Start], " \t") != "" || strings.Trim(raw[link.End:], " \t") != "" {
		return vaultindex.Wikilink{}, ImageTransform{}, 0, false
	}
	link.Start += text.Start
	link.End += text.Start
	return link, transform, comment.End, true
}

func parseImageTransform(text string) (ImageTransform, bool) {
	if !strings.HasPrefix(text, imageTransformPrefix) || !strings.HasSuffix(text, imageTransformEnd) {
		return ImageTransform{}, false
	}
	scale, degrees, ok := strings.Cut(text[len(imageTransformPrefix):len(text)-len(imageTransformEnd)], " rotate=")
	if !ok || len(scale) > 350 {
		return ImageTransform{}, false
	}
	for _, digit := range scale {
		if digit != '.' && (digit < '0' || digit > '9') {
			return ImageTransform{}, false
		}
	}
	var rotation int
	switch degrees {
	case "0":
	case "90":
		rotation = 90
	case "180":
		rotation = 180
	case "270":
		rotation = 270
	default:
		return ImageTransform{}, false
	}
	ratio, err := strconv.ParseFloat(scale, 64)
	t := ImageTransform{Ratio: ratio, Rotation: rotation}
	return t, err == nil && t.valid()
}
