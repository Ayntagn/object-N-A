// spec: FEAT-001 / Markdown Shortcuts
package mdshortcuts

import "testing"

func TestMatchesTypedMathAndTabTextInMarkdown(t *testing.T) {
	p, err := Compile([]Rule{
		{Trigger: "ff", Replacement: `\frac{${0:num}}{${1:den}}`, Scope: Math, Enabled: true, Automatic: true},
		{Trigger: ";note", Replacement: "## ${0:Heading}", Scope: Text, Enabled: true},
		{Trigger: "off", Replacement: "not active", Scope: Text, Enabled: false, Automatic: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		input, typed string
		start, stops int
		template     string
	}{
		{`$f`, `f`, 1, 2, `\\frac{${1:num}\}{${2:den}\}`},
		{"$$\nf", `f`, 3, 2, `\\frac{${1:num}\}{${2:den}\}`},
		{"🙂 ;note", "", len("🙂 "), 1, "## ${1:Heading}"},
		{`ff`, "", 0, 0, ""},
		{`f`, `f`, 0, 0, ""},
		{"```md\n$f", `f`, 0, 0, ""},
		{"`$f", `f`, 0, 0, ""},
		{`\$f`, `f`, 0, 0, ""},
		{`$;note`, "", 0, 0, ""},
		{`of`, `f`, 0, 0, ""},
	} {
		got, ok := p.Match(Input{Markdown: tt.input, Start: len(tt.input), End: len(tt.input), Typed: tt.typed})
		if ok != (tt.template != "") || got.Start != tt.start || got.Template != tt.template || got.TabStops != tt.stops {
			t.Errorf("%q typed %q → %+v (match %t)", tt.input, tt.typed, got, ok)
		}
	}
}

func TestRegexAndVisualCaptureStayLiteral(t *testing.T) {
	p, err := Compile([]Rule{
		{Trigger: `([[:alpha:]])(\d)`, Replacement: `[[0]]_{[[1]]}$0`, Enabled: true, Automatic: true, Regex: true},
		{Trigger: "U", Replacement: `\underbrace{${VISUAL}}_{${0:name}}`, Enabled: true, Automatic: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.Match(Input{Typed: "β x2"})
	if !ok || got.Template != `β x_{2\}${1}` || got.Start != 0 || got.End != 0 {
		t.Fatalf("regex in committed text discarded earlier input: %+v", got)
	}
	got, ok = p.Match(Input{Markdown: `$5`, Start: 0, End: 2, Typed: "U"})
	if !ok || got.Template != `\\underbrace{\$5\}_{${1:name}\}` || got.End != 2 || got.TabStops != 1 {
		t.Fatalf("literal selection did not survive expansion: %+v", got)
	}
}
