// spec: FEAT-001 / EV-OBSIDIAN-001. Reading view language tags, syntax and copying.
package ui

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func TestReadingPreviewCodeFences(t *testing.T) {
	const shellHeredoc = "<<'END'\n# literal UTF-8 中文\nif test = note\nEND"
	colors := regexp.MustCompile(`style="color:#[0-9a-f]{6}"`)
	spanTags := regexp.MustCompile(`</?span(?: [^>]*)?>`)
	grammarTokens := map[string]struct {
		text string
		kind editor.TokenKind
	}{
		"jsx":  {"article", editor.TokenTag},
		"tsx":  {"data-kind", editor.TokenAttribute},
		"objc": {"@interface", editor.TokenKeyword},
		"scss": {"$accent", editor.TokenVariable},
		"bash": {shellHeredoc, editor.TokenString},
	}
	for _, fixture := range []struct {
		language   string
		source     string
		classified bool
	}{
		{"Go", "package example\nconst greeting = \"hello\"\n", true},
		{"py", "def greet(name):\n    return \"hi\" + name\n", true},
		{"js", "const answer = 42 // comment\n", true},
		{"ts", "const answer: number = 42\n", true},
		{"jsx", "const badge = <article title=\"note\" />\n", true},
		{"tsx", "const badge: JSX.Element = <article data-kind=\"note\" />\n", true},
		{"objc", "@interface Note : NSObject\n@end\n", true},
		{"dotnet", "var message = \"note\"; // comment\n", true},
		{"html", "<p class=\"example\">中文</p>\n", true},
		{"scss", "$accent: #79a;\narticle { color: $accent; }\n", true},
		{"bash", "cat " + shellHeredoc + "\nprintf '%s' \"$HOME\"\n", true},
		{"json", "{\"greeting\": 42, \"enabled\": true}\n", true},
		{"yaml", "name: \"hello\"\nenabled: true\n", true},
		{"sql", "SELECT name FROM notes WHERE id = 42;\n", true},
		{"powershell", "$notes = Get-ChildItem -Path \"$HOME\" # note\n", true},
		{"pwsh", "$notes = Get-ChildItem -Path \"$HOME\" # note\n", true},
		{"ps1", "$notes = Get-ChildItem -Path \"$HOME\" # note\n", true},
		{"tex", "\\textbf{Note} % comment\n", true},
		{"latex", "\\textbf{Note} % comment\n", true},
		{"markup", "<strong class=\"note\">原文 & code</strong>\n", true},
		{"mathml", "<math><mi>β</mi><mn>42</mn></math>\n", true},
		{"shell-session", "$ printf '%s' \"$HOME\"\nHello\n", true},
		{"shellsession", "$ printf '%s' \"$HOME\"\nHello\n", true},
		{"jinja2", "{% if note %}{{ note }}{% endif %}\n", true},
		{"diff", "-old value\n+new value\n", true},
		{"plaintext", "const word = 42\n", false},
		{"custom-lang", "<p>原样复制</p>\n", false},
	} {
		t.Run(fixture.language, func(t *testing.T) {
			copies := previewCodeCopies{revision: 12}
			markup := previewMarkup{doc: parser.New(webPreviewExtensions), charts: true,
				codeTheme: previewCodeTheme(true), codeCopies: &copies}
			source := "\x60\x60\x60" + fixture.language + " metadata\n" + fixture.source + "\x60\x60\x60\n"
			body, _ := markup.render(source, "", true)
			href := "cyberwriter-copy-code://block/12/1"
			visible := fmt.Sprintf(`href="%s" aria-label="Copy %s code" title="Copy code"><span aria-live="polite">%s</span>`,
				href, fixture.language, fixture.language)
			if !strings.Contains(body, visible) || !strings.Contains(body, `data-source-line="1"`) {
				t.Fatalf("preview lost the language tag or scroll anchor: %s", body)
			}
			if got, ok := copies.codeForURL(href); !ok || got != fixture.source {
				t.Fatalf("clipboard source = %q, want %q", got, fixture.source)
			}
			if _, ok := copies.codeForURL("cyberwriter-copy-code://block/11/1"); ok {
				t.Fatal("copy accepted a link from a previous render")
			}
			distinct := make(map[string]bool)
			for _, color := range colors.FindAllString(body, -1) {
				distinct[color] = true
			}
			if fixture.classified && len(distinct) < 2 || !fixture.classified && len(distinct) != 0 {
				t.Fatalf("unexpected syntax colors for %s in %s", fixture.language, body)
			}
			if token, ok := grammarTokens[fixture.language]; ok {
				color := markup.codeTheme.TokenColor(token.kind)
				span := fmt.Sprintf(`<span style="color:#%02x%02x%02x">%s</span>`, color.R, color.G, color.B, html.EscapeString(token.text))
				if !strings.Contains(body, span) {
					t.Fatalf("%s lost the syntax category for %q: %s", fixture.language, token.text, body)
				}
			}
			_, code, _ := strings.Cut(body, "<code")
			_, code, _ = strings.Cut(code, ">")
			inside, _, _ := strings.Cut(code, "</code>")
			inside = html.UnescapeString(spanTags.ReplaceAllString(inside, ""))
			want := fixture.source
			if fixture.classified {
				want = strings.TrimSuffix(want, "\n")
			}
			if inside != want {
				t.Fatalf("rendered code = %q, want %q", inside, want)
			}
		})
	}
}

func TestReadingPreviewDistinguishesPropertiesFromValues(t *testing.T) {
	for _, fixture := range []struct {
		language, code string
		roles          []string
	}{
		{"html", `<p class="note">Example</p>`, []string{"class", `"note"`}},
		{"css", `article { width: 12px; }`, []string{"article", "width", "12px"}},
		{"json", `{"count": 42}`, []string{`"count"`, "42"}},
	} {
		for _, mode := range []struct {
			name string
			dark bool
		}{{"light", false}, {"dark", true}} {
			t.Run(fixture.language+"/"+mode.name, func(t *testing.T) {
				markup := previewMarkup{doc: parser.New(webPreviewExtensions), codeTheme: previewCodeTheme(mode.dark)}
				body, _ := markup.render("```"+fixture.language+"\n"+fixture.code+"\n```\n", "", true)
				colors := make(map[string]string, len(fixture.roles))
				for _, role := range fixture.roles {
					pattern := `style="color:(#[0-9a-f]{6})">` + regexp.QuoteMeta(html.EscapeString(role)) + `</span>`
					match := regexp.MustCompile(pattern).FindStringSubmatch(body)
					if len(match) != 2 {
						t.Fatalf("missing token for %q in %s", role, body)
					}
					if previous, exists := colors[match[1]]; exists {
						t.Fatalf("%q and %q both render in %s", previous, role, match[1])
					}
					colors[match[1]] = role
				}
			})
		}
	}
}
