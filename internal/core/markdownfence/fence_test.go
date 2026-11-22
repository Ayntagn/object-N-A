// spec: FEAT-001 / EV-MYGO-298

package markdownfence

import (
	"strings"
	"testing"
)

func TestFenceReplacementPreservesMarkdown(t *testing.T) {
	const before = "\ufeff---\r\ntitle: 示例\r\n---\r\n前言\r\n\r\n"
	const opener = "> -  ```go title=demo\r\n"
	const closer = ">    ``` \t"
	const after = "\r\n\r\n后记\r\n"
	source := before + opener + ">    package old\r\n>    \n" + closer + after
	fence, ok := Find(source, strings.Index(source, "package old"))
	if !ok || fence.Language != "go" || fence.Code != "package old\n\n" {
		t.Fatalf("Find = %+v, %t", fence, ok)
	}
	if fence.Start != len(before) || fence.End != len(source)-len(after) ||
		source[fence.CodeStart:fence.CodeEnd] != ">    package old\r\n>    \n" {
		t.Fatalf("source coordinates = %+v", fence)
	}
	got, err := fence.Replacement("package new\n\nfunc f() {}")
	want := before + opener + ">    package new\r\n>    \n>    func f() {}\r\n" + closer + after
	if err != nil || source[:fence.Start]+got+source[fence.End:] != want {
		t.Fatalf("replacement = %q, %v", got, err)
	}
}

func TestFenceReplacementPreservesCodeIndentInCompactQuote(t *testing.T) {
	fence, ok := Find(">```go\n>old\n>```", 8)
	if !ok {
		t.Fatal("fence was not found")
	}
	got, err := fence.Replacement("  next\n")
	if want := ">```go\n>   next\n>```"; err != nil || got != want {
		t.Fatalf("replacement = %q, %v; want %q", got, err, want)
	}
}

func TestFenceReplacementWidensOnlyWhereNeeded(t *testing.T) {
	const original = "opening\n\n~~~text label\nsample\n~~~~~\n\nclosing"
	fence, ok := Find(original, strings.Index(original, "sample"))
	if !ok {
		t.Fatal("fence was not found")
	}
	for _, test := range []struct{ name, code, want string }{
		{"nested delimiter", "~~~~~\nsample\n", "~~~~~~text label\n~~~~~\nsample\n~~~~~~"},
		{"indented delimiter", "    ~~~~~\n", "~~~text label\n    ~~~~~\n~~~~~"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := fence.Replacement(test.code); err != nil || got != test.want {
				t.Fatalf("replacement = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestFindRequiresAnOrdinaryClosedCodeFence(t *testing.T) {
	for _, source := range []string{
		"    code", "```go\ncode", "> ```go\n> code\n\noutside",
		"```API title=demo\ncode\n```", "```vega-lite\ncode\n```", "```query\ncode\n```",
		"---\n```go\ncode\n```\n---\n",
	} {
		if fence, ok := Find(source, strings.Index(source, "code")); ok {
			t.Errorf("found in %q: %+v", source, fence)
		}
	}
	fence, ok := Find("```\n```", 4)
	if !ok || fence.Code != "" {
		t.Errorf("valid empty fence = %+v, %t", fence, ok)
	}
}
