// spec: EV-UI-309
package ui

import (
	"strings"
	"testing"

	"cyberwriter/internal/core/vaultcontext"
)

func TestVaultContextExcerptBudgetBoundary(t *testing.T) {
	const limit = 5_000
	const firstTitle = "### [[a.md]]\n\n"
	const secondTitle = "### [[e\u0301.md]]\n\n"
	const cutoff = "\n\n_[chunk truncated to fit context budget]_\n\n"
	const cluster = "e\u0301"
	for _, remaining := range []int{199, 200} {
		first := strings.Repeat("a", limit-len(firstTitle)-2-remaining)
		got := vaultContextExcerpts([]vaultcontext.Match{
			{Path: "a.md", Text: first},
			{Path: "e\u0301.md", Text: strings.Repeat(cluster, 250)},
			{Path: "c.md", Text: "third result"},
		}, limit)
		want := first + "\n\n"
		if remaining == 200 {
			// The two-codepoint path letter and each source cluster count as one.
			want += secondTitle + strings.Repeat(cluster, remaining-len(firstTitle)-30) + cutoff
		}
		_, body, ok := strings.Cut(got, "\n\n"+firstTitle)
		if !ok || body != want {
			t.Errorf("with %d graphemes left: found first heading %t, body length %d bytes, want %d", remaining, ok, len(body), len(want))
		}
	}
}
