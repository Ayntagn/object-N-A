package wysiwyg

import (
	"crypto/rand"
	"strings"

	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"github.com/egoist/mygo/ui/richtext"
)

const QueryBlockLabel = "Query Block"
const queryAttachmentScheme = "cyberwriter-query:"
const queryCaretID = "cyberwriter-query-caret"

// QueryBlock keeps the saved fence independent of the rendered Vault results.
type QueryBlock struct {
	Source       string
	markdown, id string
	object       *richtext.Attachment
}

func newQueryBlock(source, markdown, id string) *QueryBlock {
	query := &QueryBlock{Source: source, markdown: markdown, id: id}
	query.object = &richtext.Attachment{
		Name: QueryBlockLabel, URL: queryAttachmentScheme + id + ":" + rand.Text(),
		Width: 1, Height: 70, Data: query,
		// The native card paints over the reserved space.
		Draw: func(*mygoui.Painter, mygoui.Rect) {},
	}
	return query
}

func IsQueryFence(block *parser.Block) bool {
	return block.Kind == parser.CodeBlock && block.Fenced && !block.Open && strings.EqualFold(block.Language(), "query")
}

func isRootQueryFence(block *parser.Block) bool { return block.Parent == 0 && IsQueryFence(block) }

func (q *QueryBlock) Height() float32         { return q.object.Height }
func (q *QueryBlock) ReserveHeight(h float32) { q.object.Height = h }

func (s *Session) HasQueries() bool {
	return strings.Contains(s.lastMarkdown, "!["+QueryBlockLabel+"]("+queryAttachmentScheme)
}

func (s *Session) QueryAt(offset int) (*QueryBlock, bool) {
	if s.Document == nil {
		return nil, false
	}
	return queryAt(s.Document.Text(), offset)
}

func queryAt(text richtext.Text, offset int) (*QueryBlock, bool) {
	if offset < 0 || offset >= text.Len() {
		return nil, false
	}
	attachment := text.AttributesAt(offset).Attachment
	if attachment == nil {
		return nil, false
	}
	query, ok := attachment.Data.(*QueryBlock)
	return query, ok
}

func (s *Session) InsertQuery(source string) bool {
	query := newQueryBlock(source, fencedDiagramMarkdown(source, "query"), rand.Text())
	return s.insertStandalone(query.object, queryCaretID, false)
}

func (s *Session) ChangeQuery(offset int, previous *QueryBlock, source string) bool {
	query, ok := s.QueryAt(offset)
	if !ok || query != previous || query.Source == source || !s.Current(s.draft) {
		return false
	}
	next := newQueryBlock(source, replaceFenceBody(query.markdown, source), previous.id)
	next.ReserveHeight(previous.Height())
	s.Document.Edit(func(edit *richtext.Edit) {
		edit.ReplaceString(offset, offset+1, string(richtext.ObjectReplacement), richtext.Attributes{Attachment: next.object})
	})
	return true
}

// Keep the fence marker, indentation, language, and meta as written. Grow
// the delimiters if a new source line could close the old fence.
func replaceFenceBody(markdown, source string) string {
	open, _, _ := strings.Cut(markdown, "\n")
	close := markdown[strings.LastIndexByte(markdown, '\n')+1:]
	if source == "" {
		return open + "\n" + close
	}
	indent := open[:len(open)-len(strings.TrimLeft(open, " \t"))]
	marker := open[len(indent) : len(indent)+1]
	openLen := len(open) - len(indent) - len(strings.TrimLeft(open[len(indent):], marker))
	size := openLen
	var body strings.Builder
	for line := range strings.SplitSeq(source, "\n") {
		candidate := strings.TrimSpace(line)
		if candidate != "" && strings.Trim(candidate, marker) == "" {
			size = max(size, len(candidate)+1)
		}
		body.WriteString(indent)
		body.WriteString(line)
		body.WriteByte('\n')
	}
	if size > openLen {
		open = indent + strings.Repeat(marker, size) + open[len(indent)+openLen:]
		closeIndent := close[:len(close)-len(strings.TrimLeft(close, " \t"))]
		fence := strings.TrimLeft(close[len(closeIndent):], marker)
		close = closeIndent + strings.Repeat(marker, max(size, len(close)-len(closeIndent)-len(fence))) + fence
	}
	return open + "\n" + body.String() + close
}
