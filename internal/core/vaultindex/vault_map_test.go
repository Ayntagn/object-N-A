// spec: EV-UI-081
package vaultindex

import (
	"fmt"
	"strings"
	"testing"
)

func TestVaultMapKeepsHighestDegreeCardsAtLimit(t *testing.T) {
	files := make(map[string]string)
	var hub strings.Builder
	for i := range 61 {
		label := fmt.Sprintf("Note %02d", i)
		fmt.Fprintf(&hub, "[[%s]]\n", label)
		files[label+".md"] = ""
	}
	files["Hub.md"] = hub.String()
	plan := buildFixture(t, files).Map()
	if len(plan.Cards) != 60 || len(plan.Edges) != 59 || plan.Omitted != 2 {
		t.Fatalf("map has %d cards, %d edges, %d omitted", len(plan.Cards), len(plan.Edges), plan.Omitted)
	}
	if plan.Cards[0].Label != "Hub" || plan.Cards[59].Label != "Note 58" {
		t.Fatalf("ranked selection starts with %q and ends with %q", plan.Cards[0].Label, plan.Cards[59].Label)
	}
}
