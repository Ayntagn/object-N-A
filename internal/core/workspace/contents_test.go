// spec: EV-UI-114 / EV-UI-115
package workspace

import "testing"

func TestMarkdownContentsUsesNativeHeadingRules(t *testing.T) {
	source := "# Table of Contents\n##   Café 你好 — 🙂\n  ## Indented\n" +
		"```text\n# Included sample\n```\nSetext Label\n------------\n" +
		"## Café 你好 — 🙂\n## A_B / C--D 12.34!\n## One Two\n# \t###\n"
	want := "## Table of Contents\n\n" +
		"  - [Café 你好 — 🙂](#café-你好-)\n- [Included sample](#included-sample)\n" +
		"  - [Café 你好 — 🙂](#café-你好-)\n  - [A_B / C--D 12.34!](#a_b-c-d-1234)\n" +
		"  - [One Two](#onetwo)\n- [###](#)\n"
	if got := MarkdownContents(source); got != want {
		t.Fatalf("contents = %q, want %q", got, want)
	}
	if got := MarkdownContents("plain text\n"); got != "## Table of Contents\n\n" {
		t.Fatalf("contents without headings = %q", got)
	}
}
