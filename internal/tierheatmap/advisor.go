package tierheatmap

import (
	"fmt"
	"sort"
	"time"
)

// TierType constants for storage tier classification.
const (
	TierSSD  = "SSD"
	TierHDD  = "HDD"
	TierNVMe = "NVMe"
)

// RecommendedAction constants for migration suggestions.
const (
	ActionKeep        = "Keep"
	ActionMoveToCold  = "MoveToCold"
	ActionMoveToHot   = "MoveToHot"
	ActionArchive     = "Archive"
)

// TierHeatmapSignal holds the current state of a storage tier for heatmap analysis.
type TierHeatmapSignal struct {
	LayerName        string
	TierType         string // SSD, HDD, NVMe
	Temperature      float64
	FileCount        int64
	AccessFrequency  float64
	LastAccessTime   time.Time
	MigrationScore   float64
	RecommendedAction string
}

// HeatmapResult contains the overall analysis output.
type HeatmapResult struct {
	Signals       []TierHeatmapSignal
	HotSignals    []TierHeatmapSignal
	ColdSignals   []TierHeatmapSignal
	ArchiveSignals []TierHeatmapSignal
	Summary       string
}

// Analyze examines a list of tier heatmap signals, computes MigrationScore,
// assigns RecommendedAction, and returns a structured heatmap result.
func Analyze(signals []TierHeatmapSignal) HeatmapResult {
	result := HeatmapResult{}

	if len(signals) == 0 {
		result.Summary = "no tier data available for analysis"
		return result
	}

	// Use wall-clock time as reference for recency calculations.
	refTime := time.Now()

	// Frequency factor uses an absolute scale: access frequency of 100+ is high.
	// This avoids the problem where all-cold tiers still get high relative scores.

	// Compute MigrationScore and RecommendedAction for each signal.
	for i := range signals {
		s := &signals[i]

		// Recency factor: 1.0 if just accessed, decays over 30 days.
		daysSinceAccess := refTime.Sub(s.LastAccessTime).Hours() / 24
		recencyFactor := 1.0 - daysSinceAccess/30.0
		if recencyFactor < 0 {
			recencyFactor = 0
		}

		// Frequency factor: absolute scale (100+ = full), avoids relative-only normalization.
		freqFactor := s.AccessFrequency / 100.0
		if freqFactor > 1.0 {
			freqFactor = 1.0
		}

		// Temperature factor: higher temperature means more active.
		tempFactor := s.Temperature / 100.0
		if tempFactor > 1.0 {
			tempFactor = 1.0
		}
		if tempFactor < 0 {
			tempFactor = 0
		}

		// MigrationScore: weighted combination of frequency, recency, and temperature.
		// Higher score = hotter data = should be on faster tier.
		score := (freqFactor*0.5 + recencyFactor*0.3 + tempFactor*0.2) * 100
		if score > 100 {
			score = 100
		}
		if score < 0 {
			score = 0
		}
		s.MigrationScore = score

		// Determine recommended action based on score and tier type.
		s.RecommendedAction = recommendAction(s.TierType, score, daysSinceAccess)
	}

	result.Signals = signals

	// Categorize signals.
	for _, s := range signals {
		switch s.RecommendedAction {
		case ActionMoveToHot:
			result.HotSignals = append(result.HotSignals, s)
		case ActionMoveToCold:
			result.ColdSignals = append(result.ColdSignals, s)
		case ActionArchive:
			result.ArchiveSignals = append(result.ArchiveSignals, s)
		}
	}

	// Sort signals by MigrationScore descending (hottest first).
	sort.Slice(result.Signals, func(i, j int) bool {
		return result.Signals[i].MigrationScore > result.Signals[j].MigrationScore
	})

	// Build summary.
	result.Summary = buildSummary(result)

	return result
}

// recommendAction determines the best migration action for a signal.
func recommendAction(tierType string, score, daysSinceAccess float64) string {
	// Tier-specific logic for hot/cold promotion/demotion.
	switch tierType {
	case TierNVMe, TierSSD:
		// Hot tier (NVMe/SSD): if score is low, data is cold → demote.
		if score < 30 {
			return ActionMoveToCold
		}
		return ActionKeep

	case TierHDD:
		// Cold tier (HDD): if score is high, data is hot → promote.
		if score >= 70 {
			return ActionMoveToHot
		}
		// If very cold and stale, suggest archive.
		if score < 10 && daysSinceAccess > 90 {
			return ActionArchive
		}
		return ActionKeep

	default:
		return ActionKeep
	}
}

// buildSummary creates a human-readable summary of the heatmap analysis.
func buildSummary(r HeatmapResult) string {
	total := len(r.Signals)
	hot := len(r.HotSignals)
	cold := len(r.ColdSignals)
	archive := len(r.ArchiveSignals)
	keep := total - hot - cold - archive

	return fmt.Sprintf(
		"analyzed %d tier signals: %d keep, %d promote-to-hot, %d demote-to-cold, %d archive",
		total, keep, hot, cold, archive,
	)
}