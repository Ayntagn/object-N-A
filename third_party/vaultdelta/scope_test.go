// spec: EV-UI-189 / EV-MYGO-253

package vaultdelta

import (
	"slices"
	"testing"
)

func TestMergeKeepsSiblingNamesAndCollapsesDescendants(t *testing.T) {
	first := Hints("Folder/Leaf", "Folder-Archive/Leaf")
	merged := first.Merge(Hints("Folder", "Elsewhere/Sub", "Folder/Leaf"))
	want := []string{"Elsewhere/Sub", "Folder", "Folder-Archive/Leaf"}
	if got := merged.Directories(); !slices.Equal(got, want) {
		t.Fatalf("directories = %q, want %q", got, want)
	}
	if first.Empty() || first.Full() || !slices.Equal(first.Directories(), []string{"Folder-Archive/Leaf", "Folder/Leaf"}) {
		t.Fatalf("merging changed the earlier scope: %v", first)
	}
	if !merged.Merge(Hints(".")).Full() || !Hints("Folder/Leaf", "../outside").Full() {
		t.Fatal("unknown and invalid notices must request the whole tree")
	}
}
