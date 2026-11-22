package mdprojection

// spec: EV-EDITOR-004 / EV-MYGO-306

import "testing"

func TestProjectionMovesFollowingRangesAndRejectsStaleEdits(t *testing.T) {
	base := Version{Visual: 1, Source: 4}
	projection := New(base, []Span{
		{VisualStart: 0, VisualEnd: 3, SourceStart: 5, SourceEnd: 14},
		{VisualStart: 8, VisualEnd: 13, SourceStart: 20, SourceEnd: 28},
		{VisualStart: 18, VisualEnd: 21, SourceStart: 37, SourceEnd: 40},
	})
	patch, ok := projection.Patch(base, Change{Start: 2, OldEnd: 3, NewEnd: 5})
	if !ok || patch.VisualEnd() != 5 {
		t.Fatalf("first patch = %v, %v", patch, ok)
	}

	next := Version{Visual: 3, Source: 5}
	if !projection.Advance(patch, next, 10) {
		t.Fatal("valid patch did not advance the projection")
	}
	if _, ok := projection.Patch(base, Change{Start: 2, OldEnd: 3, NewEnd: 5}); ok ||
		projection.Advance(patch, Version{Visual: 4, Source: 6}, 10) {
		t.Fatal("a stale edit reused the previous coordinates")
	}
	if _, ok := projection.Patch(next, Change{Start: 4, OldEnd: 11, NewEnd: 5}); ok {
		t.Fatal("an edit across unmapped text should remain on the full writer")
	}

	patch, ok = projection.Patch(next, Change{Start: 10, OldEnd: 10, NewEnd: 11})
	want := Span{VisualStart: 10, VisualEnd: 15, SourceStart: 21, SourceEnd: 29}
	if !ok || patch.Span() != want || patch.VisualEnd() != 16 {
		t.Fatalf("shifted span = %v, %v, want %v", patch.Span(), ok, want)
	}
	next = Version{Visual: 4, Source: 6}
	if !projection.Advance(patch, next, 9) {
		t.Fatal("second patch did not advance the projection")
	}

	patch, ok = projection.Patch(next, Change{Start: 24, OldEnd: 24, NewEnd: 25})
	want = Span{VisualStart: 21, VisualEnd: 24, SourceStart: 39, SourceEnd: 42}
	if !ok || patch.Span() != want {
		t.Fatalf("last span = %v, %v, want %v", patch.Span(), ok, want)
	}
}
