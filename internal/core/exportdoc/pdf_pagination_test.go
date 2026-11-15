// spec: EV-UI-131 / EV-UI-132
package exportdoc

import (
	"slices"
	"testing"
)

func TestPDFPageCuts(t *testing.T) {
	// Candidate boundaries around each cut in the 42-paragraph Letter fixture.
	ordinary := []float64{482, 498, 562, 578, 642, 658, 722,
		1138, 1202, 1218, 1281, 1297, 1361, 1537, 1743, 1759, 1823,
		2303, 2319, 2383, 2399, 2463, 2479, 2543,
		2942, 2958, 3022, 3038, 3102, 3118, 3182, 3539}
	want := []float64{0, 658, 1297, 1823, 2479, 3118, 3541}
	if got := PDFPageCuts(3541, 684, []float64{1537}, ordinary); !slices.Equal(got, want) {
		t.Fatalf("cuts = %v, want %v", got, want)
	}
}

func TestPDFPageRange(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []int
	}{
		{"2-3, 5", []int{1, 2, 4}},
		{"4,2,4", []int{1, 3}},
		{"99", nil},
		{"0,2,99", []int{1}},
		{"8-4, 1 - 2", []int{3, 4, 5}},
	} {
		if got := PDFPageRange(tc.input, 6); !slices.Equal(got, tc.want) {
			t.Errorf("range %q = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestPDFNumberLabel(t *testing.T) {
	numbering := PDFPagination{NumberFormat: PDFRomanUpper, StartAt: 3}
	got := []string{numbering.NumberLabel(1), numbering.NumberLabel(2), numbering.NumberLabel(4)}
	if want := []string{"IV", "V", "VII"}; !slices.Equal(got, want) {
		t.Errorf("range numbers = %v, want %v", got, want)
	}
	numbering = PDFPagination{NumberFormat: PDFLetterLower, StartAt: 26}
	if got := numbering.NumberLabel(1); got != "aa" {
		t.Errorf("letter after z = %q, want aa", got)
	}
}
