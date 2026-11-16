package aiworkspace

import (
	"time"

	"github.com/go-text/typesetting/segmenter"
)

// ReplyMetrics records the local measurements of a completed or stopped SSE reply.
type ReplyMetrics struct {
	TimeToFirstToken time.Duration `json:"timeToFirstTokenNs"`
	TokensPerSecond  float64       `json:"tokensPerSecond"`
}

// StreamProgress is the timing and estimated token count through a text delta.
// It is zero for a reply delivered as JSON.
type StreamProgress struct {
	startedAt        time.Time
	timeToFirstToken time.Duration
	estimatedTokens  int
}

// EV-UI-257. Both stream formats estimate one token per four grapheme
// clusters in a text delta, with a minimum of one per delta.
func (p *StreamProgress) observeText(segments *segmenter.Segmenter, text string, receivedAt time.Time) {
	if p.estimatedTokens == 0 {
		p.timeToFirstToken = receivedAt.Sub(p.startedAt)
	}
	segments.InitWithString(text)
	characters := 0
	for graphemes := segments.GraphemeIterator(); graphemes.Next(); {
		characters++
	}
	p.estimatedTokens += max(1, characters/4)
}

// MetricsAt uses the duration of the stream, including the wait for its first text.
func (p StreamProgress) MetricsAt(endedAt time.Time) ReplyMetrics {
	elapsed := endedAt.Sub(p.startedAt)
	if p.estimatedTokens == 0 || elapsed <= 0 {
		return ReplyMetrics{}
	}
	return ReplyMetrics{
		TimeToFirstToken: p.timeToFirstToken,
		TokensPerSecond:  float64(p.estimatedTokens) / elapsed.Seconds(),
	}
}
