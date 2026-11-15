package presentation

import (
	"strconv"
	"time"
)

type Rating int

const (
	Again Rating = iota + 1
	Hard
	Good
	Easy
)

type Review struct {
	Ease        float64  `json:"easeFactor"`
	Interval    int      `json:"interval"`
	Repetitions int      `json:"repetitions"`
	Next        float64  `json:"nextReviewDate"`
	Last        *float64 `json:"lastReviewDate,omitempty"`
}

type Progress map[string]Review

// Key follows the rolling 64-bit identifier in the native review files.
func (card Card) Key() string {
	var hash uint64
	for i := range len(card.Question) {
		hash = hash*31 + uint64(card.Question[i])
	}
	return strconv.FormatUint(hash, 16)
}

const foundationEpoch = 978307200 // 2001-01-01 UTC, as Unix seconds.
const daySeconds = 24 * 60 * 60
const maxInt = int(^uint(0) >> 1)

func reviewTime(now time.Time) float64 {
	return float64(now.Unix()-foundationEpoch) + float64(now.Nanosecond())/1e9
}

func (p Progress) Due(cards []Card, now time.Time) int {
	seconds := reviewTime(now)
	due := 0
	for _, card := range cards {
		state, exists := p[card.Key()]
		if !exists || state.Next <= seconds {
			due++
		}
	}
	return due
}

func (p Progress) Rate(card Card, rating Rating, now time.Time) {
	key := card.Key()
	state, exists := p[key]
	if !exists {
		state.Ease = 2.5
	}
	if state.Repetitions < maxInt {
		state.Repetitions++
	}
	switch rating {
	case Again:
		state.Ease = max(1.3, state.Ease-0.2)
		state.Interval, state.Repetitions = 1, 0
	case Hard:
		state.Ease = max(1.3, state.Ease-0.15)
		state.Interval = reviewDays(float64(max(1, state.Interval)) * 1.2)
	case Good:
		switch state.Repetitions {
		case 1:
			state.Interval = 1
		case 2:
			state.Interval = 6
		default:
			state.Interval = reviewDays(float64(state.Interval) * state.Ease)
		}
	case Easy:
		switch state.Repetitions {
		case 1:
			state.Interval = 4
		case 2:
			state.Interval = 10
		default:
			state.Interval = reviewDays(float64(state.Interval) * state.Ease * 1.3)
		}
		state.Ease += 0.15
	default:
		return
	}
	last := reviewTime(now)
	state.Last = &last
	state.Next = last + float64(state.Interval)*daySeconds
	p[key] = state
}

func reviewDays(days float64) int {
	if days >= float64(maxInt) {
		return maxInt
	}
	return max(1, int(days))
}
