package workspace

import "unicode"

type TextMetrics struct {
	Words          int
	Characters     int
	ReadingMinutes int
}

// MeasureText counts the Markdown draft. ReadingMinutes is an estimate.
func MeasureText(markdown string) TextMetrics {
	var metrics TextMetrics
	inWord := false
	for _, r := range markdown {
		metrics.Characters++
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_':
			if !inWord {
				metrics.Words++
				inWord = true
			}
		case !unicode.IsMark(r):
			inWord = false
		}
	}
	const wordsPerMinute = 200
	metrics.ReadingMinutes = max(1, (metrics.Words+wordsPerMinute-1)/wordsPerMinute)
	return metrics
}
