// spec: FEAT-001 / EV-MYGO-302 / Markdown Shortcuts (LaTeX Suite static snippets)
package mdshortcuts

import (
	"slices"
	"strings"
	"testing"
)

func TestParseSnippetsFeedsExistingRuleEngine(t *testing.T) {
	const source = `[
		// A case-insensitive literal regex in inline math.
		{trigger: /([a-z])(\d)/i, replacement: '[[0]]_[[1]]', options: 'nA', description: '下标'},
		/* A string regular expression, restricted to block math. */
		{"trigger": "(b)(\\d)", replacement: "block[[1]]", options: "MrA"},
		{trigger: 'U', replacement: '\\underbrace{${VISUAL}}_$0', options: 'mA'},
		{trigger: 'x', replacement: 'priority', options: 'tA', priority: 2.5},
		{trigger: 'ax', replacement: 'longest', options: 'tA'},
		{trigger: 'off', replacement: 'inactive', options: 'A', enabled: false},
	];`
	rules, err := ParseSnippets(source)
	if err != nil {
		t.Fatal(err)
	}
	program, err := Compile(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		input Input
		want  string
	}{
		{Input{Markdown: "$A$", Start: 2, End: 2, Typed: "2"}, "A_2"},
		{Input{Markdown: "$$\nb", Start: 4, End: 4, Typed: "2"}, "block2"},
		{Input{Markdown: "$x$", Start: 1, End: 2, Typed: "U"}, `\\underbrace{x\}_${1}`},
		{Input{Markdown: "a", Start: 1, End: 1, Typed: "x"}, "priority"},
		{Input{Typed: "off"}, ""},
	} {
		got, ok := program.Match(test.input)
		if ok != (test.want != "") || got.Template != test.want {
			t.Errorf("match %+v = %q (%t), want %q", test.input, got.Template, ok, test.want)
		}
	}
}

func TestFormatSnippetsRoundTripsSavedRules(t *testing.T) {
	rules := []Rule{
		{Trigger: "//", Replacement: "\\frac{$0}{$1}\n\t\"末尾\" 🙂", Scope: Math, Enabled: true, Automatic: true},
		{Trigger: "(?i)([a-z])(\\d)", Replacement: "[[0]]_[[1]]", Scope: InlineMath, Regex: true, WholeWord: true},
		{Trigger: "'note", Replacement: "", Scope: Text, Enabled: true, Description: "\"quote\"", Priority: -1},
		{Trigger: "case", Replacement: "  ", Scope: BlockMath, Enabled: true},
		{Trigger: "same", Replacement: "first", Enabled: true},
		{Trigger: "same", Replacement: "second", Enabled: true},
	}
	source := FormatSnippets(rules)
	got, err := ParseSnippets(source)
	if err != nil || !slices.Equal(got, rules) {
		t.Fatalf("round trip: got %+v, error %v\n%s", got, err, source)
	}
	p, err := Compile(got)
	if err != nil {
		t.Fatal(err)
	}
	expansion, ok := p.Match(Input{Markdown: "same", Start: 4, End: 4})
	if !ok || expansion.Template != "first" {
		t.Fatalf("saved order was lost: %+v (%t)", expansion, ok)
	}
}

func TestParseSnippetsReportsUnsupportedInputAtItsLine(t *testing.T) {
	for _, test := range []struct{ snippet, problem string }{
		{`{trigger: "a", replacement: (m) => m[0], options: "A"}`, "quoted string"},
		{`{trigger: "a", replacement: "x", options: "cA"}`, "unsupported option"},
		{`{trigger: /(?<=x)a/, replacement: "x", options: "A"}`, "regular expression"},
		{`{trigger: "a", replacement: "x", context: () => true}`, "unsupported field"},
	} {
		rules, err := ParseSnippets("[\n  {trigger: 'ok', replacement: 'copy'},\n  " + test.snippet + ",\n]")
		if rules != nil || err == nil || !strings.Contains(err.Error(), "line 3, column ") || !strings.Contains(err.Error(), test.problem) {
			t.Errorf("malformed rule %s: got %+v, error %v", test.snippet, rules, err)
		}
	}
}
