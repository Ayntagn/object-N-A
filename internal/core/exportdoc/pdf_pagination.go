package exportdoc

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type PDFNumberPosition int

const (
	PDFBottomCenter PDFNumberPosition = iota
	PDFBottomRight
	PDFBottomLeft
	PDFTopCenter
	PDFTopRight
	PDFTopLeft
)

type PDFNumberFormat int

const (
	PDFDecimal PDFNumberFormat = iota
	PDFRomanLower
	PDFRomanUpper
	PDFLetterLower
	PDFLetterUpper
)

const MaxPDFStartAt = 999

// PDFPagination's zero value splits the document into numbered pages starting at 1.
type PDFPagination struct {
	Continuous     bool
	OmitNumbers    bool
	NumberPosition PDFNumberPosition
	NumberFormat   PDFNumberFormat
	StartAt        int
	Range          string
}

func (p PDFPagination) FirstNumber() int {
	return max(1, min(p.StartAt, MaxPDFStartAt))
}

// NumberLabel uses the original, zero-based index before page selection.
func (p PDFPagination) NumberLabel(index int) string {
	if p.Continuous || p.OmitNumbers {
		return ""
	}
	number := p.FirstNumber() + index
	switch p.NumberFormat {
	case PDFRomanLower, PDFRomanUpper:
		var label strings.Builder
		for _, digit := range [...]struct {
			value int
			text  string
		}{
			{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"},
			{50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
		} {
			label.WriteString(strings.Repeat(digit.text, number/digit.value))
			number %= digit.value
		}
		if p.NumberFormat == PDFRomanLower {
			return strings.ToLower(label.String())
		}
		return label.String()
	case PDFLetterLower, PDFLetterUpper:
		var letters [16]byte
		base := byte('A')
		if p.NumberFormat == PDFLetterLower {
			base = 'a'
		}
		i := len(letters)
		for number > 0 {
			number--
			i--
			letters[i] = base + byte(number%26)
			number /= 26
		}
		return string(letters[i:])
	default:
		return strconv.Itoa(number)
	}
}

// PDFPageCuts returns the source y-coordinates that bound each physical page.
// EV-UI-132: use the first in-page marker, then snap within the last 30% of the page.
func PDFPageCuts(height, pageHeight float64, manual, ordinary []float64) []float64 {
	if height <= 0 || pageHeight <= 0 || math.IsNaN(height+pageHeight) || math.IsInf(height+pageHeight, 0) {
		return nil
	}
	manual, ordinary = slices.Clone(manual), slices.Clone(ordinary)
	slices.Sort(manual)
	slices.Sort(ordinary)
	cuts := []float64{0}
	for start := 0.0; start < height; {
		nominal := start + pageHeight
		target := nominal
		for len(manual) > 0 && manual[0] <= start {
			manual = manual[1:]
		}
		if len(manual) > 0 && manual[0] < nominal {
			target = manual[0]
			manual = manual[1:]
		}
		end, best := target, math.Inf(1)
		for i := sort.SearchFloat64s(ordinary, start+0.70*pageHeight); i < len(ordinary) && ordinary[i] <= nominal; i++ {
			if distance := math.Abs(ordinary[i] - target); distance < best {
				end, best = ordinary[i], distance
			}
		}
		end = min(end, height)
		if end <= start {
			return nil
		}
		cuts = append(cuts, end)
		start = end
	}
	return cuts
}

// PDFPageRange returns sorted, zero-based pages. An empty result selects all pages.
func PDFPageRange(input string, count int) []int {
	if input == "" || count < 1 {
		return nil
	}
	pages := make(map[int]struct{})
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, "-") {
			if page, err := strconv.Atoi(part); err == nil && page >= 1 && page <= count {
				pages[page-1] = struct{}{}
			}
			continue
		}
		var endpoints []int
		for _, piece := range strings.Split(part, "-") {
			if number, err := strconv.Atoi(piece); err == nil {
				endpoints = append(endpoints, number)
			}
		}
		if len(endpoints) == 2 {
			start, end := endpoints[0], endpoints[1]
			for i := max(1, min(start, end)) - 1; i < min(count, max(start, end)); i++ {
				pages[i] = struct{}{}
			}
		}
	}
	if len(pages) == 0 {
		return nil
	}
	result := make([]int, 0, len(pages))
	for index := range pages {
		result = append(result, index)
	}
	slices.Sort(result)
	return result
}
