package vaultcontext

import "context"

// BuildPhase identifies the unit counted by an indexing progress update.
type BuildPhase uint8

const (
	BuildWordStatistics BuildPhase = iota + 1
	BuildChunks
)

// BuildProgress counts participating notes in BuildWordStatistics and newly
// processed chunks in BuildChunks.
type BuildProgress struct {
	Phase            BuildPhase
	Processed, Total int
}

type buildProgressKey struct{}

// WithBuildProgress attaches an optional observer to a single build or update.
// The build calls report synchronously on its worker goroutine.
func WithBuildProgress(ctx context.Context, report func(BuildProgress)) context.Context {
	return context.WithValue(ctx, buildProgressKey{}, report)
}

func buildProgressObserver(ctx context.Context) func(BuildProgress) {
	report, _ := ctx.Value(buildProgressKey{}).(func(BuildProgress))
	return report
}

// startChunkProgress reports the initial count and each new integer percentage.
func startChunkProgress(ctx context.Context, total int) func(int) {
	report := buildProgressObserver(ctx)
	if report == nil || total == 0 {
		return func(int) {}
	}
	lastPercent := -1
	update := func(processed int) {
		percent := int(uint64(processed) * 100 / uint64(total))
		if percent != lastPercent && ctx.Err() == nil {
			lastPercent = percent
			report(BuildProgress{Phase: BuildChunks, Processed: processed, Total: total})
		}
	}
	update(0)
	return update
}
