package wysiwyg

import (
	"crypto/rand"
	"strings"
	"unicode"

	"cyberwriter/internal/core/liveapi"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const apiAttachmentScheme = "cyberwriter-live-api:"
const apiCaretID = "cyberwriter-live-api-caret"

// APIBlock is the source and raw fence of one request or result.
type APIBlock struct {
	Source, Info string
	Result       bool
	markdown, id string
	object       *richtext.Attachment
}

func newAPIBlock(source, info, markdown, id string) *APIBlock {
	lang := strings.TrimSpace(info)
	if end := strings.IndexFunc(lang, unicode.IsSpace); end >= 0 {
		lang = lang[:end]
	}
	block := &APIBlock{Source: source, Info: info, Result: strings.EqualFold(lang, "api-result"), markdown: markdown, id: id}
	label := "API Request"
	if block.Result {
		label = "API Result"
	}
	block.object = &richtext.Attachment{
		Name: label, URL: apiAttachmentScheme + id + ":" + rand.Text(),
		Width: 1, Height: 74, Data: block,
		Draw: func(*mygoui.Painter, mygoui.Rect) {},
	}
	return block
}

func isRootAPIRequest(block *parser.Block) bool {
	if block.Parent != 0 || block.Kind != parser.CodeBlock || !block.Fenced || block.Open {
		return false
	}
	_, ok := liveapi.ParseOptions(block.Info())
	return ok
}

func isRootAPIResult(block *parser.Block) bool {
	return block.Parent == 0 && block.Kind == parser.CodeBlock && block.Fenced && !block.Open &&
		strings.EqualFold(block.Language(), "api-result")
}

func (b *APIBlock) Height() float32         { return b.object.Height }
func (b *APIBlock) ReserveHeight(h float32) { b.object.Height = h }

func (s *Session) HasAPIBlocks() bool {
	return s.hasAPIBlocks
}

func (s *Session) APIAt(offset int) (*APIBlock, bool) {
	if s.Document == nil {
		return nil, false
	}
	return apiAt(s.Document.Text(), offset)
}

func apiAt(text richtext.Text, offset int) (*APIBlock, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	object := text.AttributesAt(offset).Attachment
	if object == nil {
		return nil, false
	}
	block, ok := object.Data.(*APIBlock)
	return block, ok
}

func (s *Session) InsertAPIRequest(info, source string) bool {
	if s.Document == nil {
		return false
	}
	block := newAPIBlock(source, info, fencedDiagramMarkdown(source, info), rand.Text())
	a, z := s.Document.Selection()
	return s.insertStandalone(block.object, apiCaretID, max(a, z) == s.Document.Text().Len())
}

func (s *Session) ChangeAPI(offset int, block *APIBlock, source string) bool {
	current, ok := s.APIAt(offset)
	if !ok || current != block || block.Source == source || !s.Current(s.draft) {
		return false
	}
	next := newAPIBlock(source, block.Info, replaceFenceBody(block.markdown, source), block.id)
	next.ReserveHeight(block.Height())
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: next.object})
	})
	return true
}

// APIRequestOrdinal maps the selected visual request to its root-level
// source fence, including when two requests have the same text.
func (s *Session) APIRequestOrdinal(at int, block *APIBlock) int {
	if s.Document == nil || block.Result {
		return -1
	}
	ordinal := 0
	text := s.Document.Text()
	for offset := range text.Paragraphs() {
		if offset > at {
			break
		}
		request, ok := apiAt(text, offset)
		if !ok || request.Result {
			continue
		}
		if offset == at && request == block {
			return ordinal
		}
		ordinal++
	}
	return -1
}
