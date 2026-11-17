package wysiwyg

import (
	"crypto/rand"
	"path"
	"strconv"
	"strings"

	"cyberwriter/internal/core/vaultindex"
	"cyberwriter/internal/core/workspace"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const imageScheme = "cyberwriter-image:"
const imageLabel = "Image"
const imageCaretID = "cyberwriter-image-caret"

// Image is one undoable version of a Markdown image. Keep its original
// source independently from the dimensions and bitmap used by RichText.
type Image struct {
	ID, Source, Caption, Alt string
	ImageTransform
	Block, Wiki bool
	WikiWidth   int
	token       string
	markdown    string
	object      *richtext.Attachment
}

func newImage(id string, block bool, source, caption, alt, token, suffix string, transform ImageTransform) *Image {
	if id == "" {
		id = rand.Text()
	}
	picture := &Image{ID: id, Source: source, Caption: caption, Alt: alt, Block: block,
		ImageTransform: transform, token: token, markdown: token + suffix}
	picture.object = &richtext.Attachment{
		Name: imageLabel, URL: imageScheme + id + ":" + rand.Text(), Data: picture,
		Width: 1, Height: 18, Draw: func(*mygoui.Painter, mygoui.Rect) {},
	}
	if block {
		picture.object.Height = 100
		if source == "" {
			picture.object.Height = 56
		}
	}
	return picture
}

func (p *Image) HasTransform() bool { return len(p.markdown) > len(p.token) }

func ImageAt(text richtext.Text, offset int) (*Image, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	at := text.AttributesAt(offset).Attachment
	if at == nil {
		return nil, false
	}
	picture, ok := at.Data.(*Image)
	if ok {
		_, end := text.ParagraphRange(offset)
		ok = offset < end
	}
	return picture, ok
}

func (p *Image) SetVisual(width, height float32, draw func(*mygoui.Painter, mygoui.Rect)) bool {
	changed := p.object.Width != width || p.object.Height != height
	p.object.Width, p.object.Height = width, height
	if draw != nil {
		p.object.Draw = draw
	}
	return changed
}

type imageRegion struct {
	start, tokenEnd, end int
	block, wiki          bool
	width                int
	src, alt, tip        string
	transform            ImageTransform
}

func imageRegions(doc *parser.Document) []imageRegion {
	var regions []imageRegion
	for i := 1; i < doc.Len(); i++ {
		block := doc.Block(i)
		// RichText tables use their own cell serializer. A reference image
		// is kept with its reference definition by the source editor.
		if block.Kind != parser.Paragraph && block.Kind != parser.Heading {
			continue
		}
		inlines := doc.Inlines(i)
		isRootParagraph := block.Parent == 0 && block.Kind == parser.Paragraph
		if isRootParagraph {
			if link, transform, end, ok := ReadWikiBlockImage(doc.Source(), inlines); ok {
				region := wikiImageRegion(link, true)
				region.end, region.transform = end, transform
				regions = append(regions, region)
				continue
			}
		}
		for j := 0; j < len(inlines); {
			inline := inlines[j]
			if inline.Kind == parser.Text && inline.Start >= block.Start && inline.End <= block.End &&
				strings.Contains(inline.Text, "![[") {
				raw := doc.Source()[inline.Start:inline.End]
				for _, link := range vaultindex.Wikilinks(raw) {
					if !link.Embed || !workspace.IsRasterImagePath(link.Target) {
						continue
					}
					blockImage := isRootParagraph && len(inlines) == 1 &&
						strings.TrimSpace(raw[:link.Start]) == "" && strings.TrimSpace(raw[link.End:]) == ""
					link.Start += inline.Start
					link.End += inline.Start
					regions = append(regions, wikiImageRegion(link, blockImage))
				}
			}
			if inline.Kind == parser.Image && !inline.Pending && inline.Start >= block.Start && inline.End <= block.End {
				if inline.End > inline.Start && doc.Source()[inline.End-1] == ')' {
					alt := parser.PlainText(inlines[j+1 : inline.Next])
					region := imageRegion{start: inline.Start, tokenEnd: inline.End, end: inline.End,
						src: inline.Dest, alt: alt, tip: inline.Title, transform: ImageTransform{Ratio: 1}}
					if isRootParagraph && j == 0 {
						if transform, end, ok := ReadBlockImageTransform(doc.Source(), inlines); ok {
							region.block, region.end, region.transform = true, end, transform
						} else if inline.Next == len(inlines) {
							region.block = true
							region.transform.Ratio = legacyImageRatio(alt)
						}
					}
					regions = append(regions, region)
				}
			}
			j = inline.Next
		}
	}
	return regions
}

func wikiImageRegion(link vaultindex.Wikilink, block bool) imageRegion {
	region := imageRegion{
		start: link.Start, tokenEnd: link.End, end: link.End,
		block: block, wiki: true, src: link.Target, alt: path.Base(link.Target),
		transform: ImageTransform{Ratio: 1},
	}
	if link.Label != link.Target {
		if width, err := strconv.Atoi(link.Label); err == nil && width > 0 {
			region.width = width
		} else {
			region.tip, region.alt = link.Label, link.Label
		}
	}
	return region
}

func wikiImageTokenRegion(token string) (imageRegion, bool) {
	doc := parser.New(Extensions)
	doc.Set(token)
	regions := imageRegions(doc)
	if len(regions) != 1 {
		return imageRegion{}, false
	}
	region := regions[0]
	return region, region.wiki && region.block && region.start == 0 && region.tokenEnd == len(token)
}

func projectImages(doc *parser.Document, source string, formulas map[string]*Formula) (string, map[string]*Image) {
	regions := imageRegions(doc)
	if len(regions) == 0 {
		return source, nil
	}
	images := make(map[string]*Image, len(regions))
	var out strings.Builder
	start := 0
	for _, region := range regions {
		if formulas[region.src] != nil {
			continue
		}
		picture := newImage("", region.block, region.src, region.tip, region.alt,
			source[region.start:region.tokenEnd], source[region.tokenEnd:region.end], region.transform)
		picture.Wiki, picture.WikiWidth = region.wiki, region.width
		images[picture.object.URL] = picture
		out.WriteString(source[start:region.start])
		out.WriteString(attachmentMarkdown(picture.object))
		start = region.end
	}
	out.WriteString(source[start:])
	return out.String(), images
}

var imageTitleEscapes = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func imageMarkdown(source, caption, alt string) string {
	if strings.ContainsAny(source, " ()<>") {
		source = "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(source) + ">"
	}
	title := ""
	if caption != "" {
		title = ` "` + imageTitleEscapes.Replace(caption) + `"`
	}
	return "![" + wikiLabelEscapes.Replace(alt) + "](" + source + title + ")"
}

// InsertImage places a root image before the surviving suffix. The block
// menu keeps the text caret in that suffix. The toolbar selects an image
// inserted into an empty paragraph.
func (s *Session) InsertImage(source string, fromToolbar bool) bool {
	picture := newImage("", true, source, "", "1.00", imageMarkdown(source, "", "1.00"), "", ImageTransform{Ratio: 1})
	var empty bool
	if s.Document != nil {
		a, z := s.Document.Selection()
		start, end := s.Document.Text().ParagraphRange(z)
		empty = a == z && start == end
	}
	return s.insertStandalone(picture.object, imageCaretID, fromToolbar && empty)
}

// FillWikiImage replaces an empty image card with a saved attachment in
// the same undo step and keeps any existing sizing.
func (s *Session) FillWikiImage(offset int, previous *Image, token string) bool {
	picture, ok := s.currentImage(offset, previous)
	if !ok || picture.Source != "" {
		return false
	}
	region, ok := wikiImageTokenRegion(token)
	if !ok {
		return false
	}
	transform := picture.ImageTransform
	next := newImage(picture.ID, true, region.src, region.tip, region.alt, token, transform.suffix(""), transform)
	next.Wiki, next.WikiWidth = true, region.width
	s.replaceImage(offset, next)
	return true
}

// ChangeImage updates a card with a single RichText undo step.
func (s *Session) ChangeImage(offset int, previous *Image, source, caption string, ratio float64) (*Image, bool) {
	picture, ok := s.currentImage(offset, previous)
	if !ok || picture.Source == source && picture.Caption == caption && picture.Ratio == ratio {
		return nil, false
	}
	transform := ImageTransform{Ratio: ratio, Rotation: picture.Rotation}
	if !transform.valid() {
		return nil, false
	}
	alt, token, suffix := picture.Alt, picture.token, picture.markdown[len(picture.token):]
	if picture.Wiki {
		if source != picture.Source {
			return nil, false
		}
		width := picture.WikiWidth
		if caption != picture.Caption {
			caption = strings.TrimSpace(caption)
			token = "![[" + source
			if caption != "" {
				token += "|" + caption
			}
			token += "]]"
			region, ok := wikiImageTokenRegion(token)
			if !ok || region.src != source || region.tip != caption {
				return nil, false
			}
			alt, width = region.alt, region.width
		}
		if ratio != picture.Ratio {
			suffix = transform.suffix("")
		}
		next := newImage(picture.ID, true, source, caption, alt, token, suffix, transform)
		next.Wiki, next.WikiWidth = true, width
		return s.replaceImage(offset, next), true
	}
	if ratio != picture.Ratio {
		if picture.HasTransform() {
			suffix = transform.suffix(alt)
		} else {
			alt = strconv.FormatFloat(ratio, 'f', 2, 64)
		}
	}
	if source != picture.Source || caption != picture.Caption || alt != picture.Alt {
		token = imageMarkdown(source, caption, alt)
	}
	return s.replaceImage(offset, newImage(picture.ID, true, source, caption, alt, token, suffix, transform)), true
}

// TransformImage changes the display without rewriting the asset or its
// original Markdown token, in one RichText undo step.
func (s *Session) TransformImage(offset int, previous *Image, transform ImageTransform) (*Image, bool) {
	picture, ok := s.currentImage(offset, previous)
	if !ok || picture.Source == "" || !transform.valid() || transform == picture.ImageTransform {
		return nil, false
	}
	alt := picture.Alt
	if picture.Wiki {
		alt = ""
	}
	next := newImage(picture.ID, true, picture.Source, picture.Caption, picture.Alt, picture.token, transform.suffix(alt), transform)
	next.Wiki, next.WikiWidth = picture.Wiki, picture.WikiWidth
	return s.replaceImage(offset, next), true
}

func (s *Session) currentImage(offset int, previous *Image) (*Image, bool) {
	if s.Document == nil || !s.Current(s.draft) {
		return nil, false
	}
	picture, ok := ImageAt(s.Document.Text(), offset)
	return picture, ok && picture == previous && picture.Block
}

func (s *Session) replaceImage(offset int, next *Image) *Image {
	s.Document.Edit(func(edit *richtext.Edit) {
		a, z := edit.Selection()
		edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: next.object})
		edit.Select(a, z)
	})
	return next
}

// patchImageSource replaces one changed image token while keeping adjacent
// Markdown, including the original syntax of other images, byte-for-byte.
func (s *Session) patchImageSource(markdown string, text richtext.Text) (string, bool) {
	const prefix = "![" + imageLabel + "](" + imageScheme
	if !strings.Contains(markdown, prefix) || !strings.Contains(s.lastMarkdown, prefix) {
		return "", false
	}
	change := changedRange(s.lastMarkdown, markdown)
	for _, run := range text.Runs(0, text.Len()) {
		picture := imageFromAttributes(run.Attributes)
		if picture == nil {
			continue
		}
		marker := attachmentMarkdown(picture.object)
		start := strings.Index(markdown, marker)
		if start < 0 || change.Start < start || change.Start+len(change.Text) > start+len(marker) {
			continue
		}
		end := start + len(marker) - len(change.Text) + change.End - change.Start
		if end <= start || end > len(s.lastMarkdown) || !strings.HasPrefix(s.lastMarkdown[start:end], prefix+picture.ID+":") {
			return "", false
		}
		head, _, body := workspace.SplitFrontmatter(s.source)
		doc := parser.New(Extensions)
		doc.Set(body)
		regions := imageRegions(doc)
		index := strings.Count(markdown[:start], prefix)
		if len(regions) != strings.Count(s.lastMarkdown, prefix) || index >= len(regions) {
			return "", false
		}
		region := regions[index]
		return head + body[:region.start] + picture.markdown + body[region.end:], true
	}
	return "", false
}

func imageFromAttributes(attrs richtext.Attributes) *Image {
	if attrs.Attachment != nil {
		picture, _ := attrs.Attachment.Data.(*Image)
		return picture
	}
	return nil
}
