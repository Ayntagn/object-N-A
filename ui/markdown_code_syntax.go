package ui

import (
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/egoist/mygo/ui/editor"
)

func nativeFenceTokenizer(language string) editor.Tokenizer {
	// Use the full-block grammars for dialect syntax and shell here-documents.
	switch strings.ToLower(language) {
	case "jsx", "tsx", "objc", "objective-c", "objectivec", "sass", "scss", "bash", "sh", "shell", "zsh", "ksh":
		return nil
	}
	if lang := editor.LanguageNamed(language); lang != nil {
		return lang.Tokenizer
	}
	return nil
}

// Only explicit names and aliases select a grammar. An unrecognized label
// leaves its text plain and keeps the original language badge for copying.
var additionalFenceLexers = sync.OnceValue(func() map[string]chroma.Lexer {
	registered := make(map[string]chroma.Lexer)
	for _, name := range lexers.Names(true) {
		lexer := lexers.Get(name)
		if lexer != nil && lexer.Config().Name != "plaintext" {
			registered[strings.ToLower(name)] = lexer
		}
	}
	registered["markup"] = registered["html"]
	registered["mathml"] = registered["xml"]
	registered["ssml"] = registered["xml"]
	registered["pwsh"] = registered["powershell"]
	registered["sh-session"] = registered["shell-session"]
	// Prism accepts these names as well. Keep them visible on the badge.
	registered["shellsession"] = registered["shell-session"]
	registered["dotnet"] = registered["csharp"]
	registered["jinja2"] = registered["django"]
	return registered
})

func additionalFenceLexer(language string) chroma.Lexer {
	return additionalFenceLexers()[strings.ToLower(language)]
}

// Chroma reads the entire block so multiline comments, strings and prompts
// retain their state. Validate its runs against the original bytes, including
// when a grammar expects a synthetic final newline.
func tokenizeAdditionalFence(lexer chroma.Lexer, source string) []editor.Token {
	if source == "" {
		return nil
	}
	input := source
	if !strings.HasSuffix(input, "\n") {
		input += "\n"
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root", Nested: true}, input)
	if err != nil {
		return nil
	}
	var runs []editor.Token
	position := 0
	for position < len(source) {
		token := iterator()
		if token == chroma.EOF {
			return nil
		}
		if token.Value == "" {
			continue
		}
		if !strings.HasPrefix(input[position:], token.Value) {
			return nil
		}
		position += len(token.Value)
		end := min(position, len(source))
		kind := additionalFenceKind(token.Type)
		if last := len(runs) - 1; last >= 0 && runs[last].Kind == kind {
			runs[last].End = end
		} else {
			runs = append(runs, editor.Token{End: end, Kind: kind})
		}
	}
	return runs
}

func additionalFenceKind(kind chroma.TokenType) editor.TokenKind {
	switch kind {
	case chroma.KeywordType, chroma.NameClass, chroma.NameException, chroma.NameNamespace:
		return editor.TokenType
	case chroma.KeywordConstant, chroma.NameConstant:
		return editor.TokenConstant
	case chroma.NameAttribute:
		return editor.TokenAttribute
	case chroma.NameTag:
		return editor.TokenTag
	case chroma.NameProperty:
		return editor.TokenProperty
	case chroma.NameDecorator, chroma.CommentPreproc, chroma.CommentPreprocFile, chroma.GenericPrompt:
		return editor.TokenMeta
	case chroma.LiteralStringEscape, chroma.LiteralStringInterpol, chroma.NameEntity:
		return editor.TokenEscape
	case chroma.LiteralStringRegex:
		return editor.TokenRegexp
	case chroma.GenericHeading, chroma.GenericSubheading:
		return editor.TokenHeading
	case chroma.GenericInserted:
		return editor.TokenInserted
	case chroma.GenericDeleted:
		return editor.TokenDeleted
	case chroma.Error, chroma.GenericError:
		return editor.TokenInvalid
	}
	switch kind.SubCategory() {
	case chroma.NameBuiltin, chroma.NameFunction:
		return editor.TokenFunction
	case chroma.NameVariable:
		return editor.TokenVariable
	case chroma.LiteralString:
		return editor.TokenString
	case chroma.LiteralNumber:
		return editor.TokenNumber
	}
	switch kind.Category() {
	case chroma.Keyword:
		return editor.TokenKeyword
	case chroma.Comment:
		return editor.TokenComment
	case chroma.Operator:
		return editor.TokenOperator
	case chroma.Punctuation:
		return editor.TokenPunctuation
	}
	return editor.TokenText
}
