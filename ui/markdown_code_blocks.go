package ui

import (
	"strings"
	"time"

	"cyberwriter/internal/core/liveapi"
	"cyberwriter/internal/core/mermaid"
	"cyberwriter/ui/wysiwyg"
	mygoui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
)

func withMarkdownCodeBadges(options *markdown.Options) *markdown.Options {
	options.CodeBlock = func(c *mygoui.Context, block *markdown.CodeBlockView) {
		if block.Open || block.Language == "" || markdownSpecialFence(block) {
			markdown.DefaultCodeBlock(c, block)
			return
		}
		drawMarkdownCodeBlock(c, block, options)
	}
	return options
}

func markdownSpecialFence(block *markdown.CodeBlockView) bool {
	if strings.EqualFold(block.Language, "query") || strings.EqualFold(block.Language, "api-result") {
		return true
	}
	_, diagram := mermaid.Background(block.Info)
	_, request := liveapi.ParseOptions(block.Info)
	return diagram || request || wysiwyg.VegaMode(block.Info) != ""
}

func drawMarkdownCodeBlock(c *mygoui.Context, block *markdown.CodeBlockView, options *markdown.Options) {
	window := c.Theme()
	background, ink := window.Text.Alpha(0.04), window.TextMuted
	if style := options.Theme; style != nil {
		background, ink = style.CodeBlockBackground, style.Text.Alpha(0.74)
	}
	box := mygoui.Column(c).Direction(mygoui.LeftToRight).AlignSelf(mygoui.Stretch).MinWidth(0).
		Background(background).Border(1, window.Border).Radius(window.Radius + 2).Clip().
		Role(mygoui.RoleGroup).Label("Code, " + block.Language)
	box.Children(func() {
		mygoui.Row(c).Justify(mygoui.End).Padding(5, 6, 0).Children(func() {
			label := "Copy " + block.Language + " code"
			badge := mygoui.ButtonBase(c.Key("code language")).Height(20).MaxWidth(140).
				Padding(0, 6).Center().Radius(4).Label(label).Background(ink.Alpha(0.10))
			until := mygoui.Local(badge, "copied until", func() time.Time { return time.Time{} })
			caption := block.Language
			if remaining := until.Sub(c.Now()); remaining > 0 {
				caption, label = "Copied", "Copied"
				c.After(remaining)
			}
			if badge.Hovered() {
				badge.Background(ink.Alpha(0.18))
			}
			badge.Tooltip(label).OnClick(func() {
				c.WriteClipboard(block.Code())
				*until = c.Now().Add(1500 * time.Millisecond)
			}).Children(func() {
				mygoui.Text(c, caption).Font("monospace").FontSize(11).TextColor(ink).SingleLine().
					Ellipsis("…").Role(mygoui.RoleNone)
			})
		})
		if !drawAdditionalFenceBody(c, box, block, options) {
			block.Body(c)
		}
	})
}

type additionalFenceBody struct {
	language, source string
	runs             []editor.Token
}

func drawAdditionalFenceBody(c *mygoui.Context, box mygoui.Element, block *markdown.CodeBlockView, options *markdown.Options) bool {
	if nativeFenceTokenizer(block.Language) != nil {
		return false
	}
	lexer := additionalFenceLexer(block.Language)
	if lexer == nil {
		return false
	}
	source := strings.TrimSuffix(block.Code(), "\n")
	cache := mygoui.Local(box, "code syntax", func() additionalFenceBody { return additionalFenceBody{} })
	if cache.language != block.Language || cache.source != source {
		*cache = additionalFenceBody{language: block.Language, source: source, runs: tokenizeAdditionalFence(lexer, source)}
	}
	if len(cache.runs) == 0 {
		return false
	}
	style := markdown.DefaultTheme(c.Theme())
	if custom := options.Theme; custom != nil {
		if custom.FontSize > 0 {
			style.FontSize = custom.FontSize
		}
		if custom.CodeFont != "" {
			style.CodeFont = custom.CodeFont
		}
		if custom.CodeSize > 0 {
			style.CodeSize = custom.CodeSize
		}
		if custom.CodeLineHeight > 0 {
			style.CodeLineHeight = custom.CodeLineHeight
		}
		if custom.CodeBlockPadding > 0 {
			style.CodeBlockPadding = custom.CodeBlockPadding
		}
		if custom.CodeColors != nil {
			style.CodeColors = custom.CodeColors
		}
	}
	spans := make([]mygoui.Span, 0, len(cache.runs))
	start := 0
	for _, run := range cache.runs {
		tokenStyle := style.CodeColors.Tokens[run.Kind]
		span := mygoui.Span{Text: cache.source[start:run.End], Color: style.CodeColors.TokenColor(run.Kind), Italic: tokenStyle.Italic}
		if tokenStyle.Bold {
			span.Weight = 600
		}
		spans = append(spans, span)
		start = run.End
	}
	var outer mygoui.Element
	if options.WrapCode {
		outer = mygoui.Column(c)
	} else {
		outer = mygoui.ScrollHorizontal(c)
	}
	outer.AlignSelf(mygoui.Stretch).MinWidth(0).Font(style.CodeFont).
		FontSize(style.FontSize * style.CodeSize).LineHeight(style.CodeLineHeight).Children(func() {
		pad := style.FontSize * style.CodeBlockPadding
		column := mygoui.Column(c).Padding(style.FontSize*0.25, pad, pad, pad)
		if !options.WrapCode {
			column.Shrink(0)
		}
		column.Children(func() {
			body := mygoui.RichText(c, spans...)
			if !options.WrapCode {
				body.NoWrap()
			}
		})
	})
	return true
}
