package snapdiffviz

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SnapDiffSignal represents the difference between two storage snapshots
// for a single dataset. It captures file-level and byte-level change counts
// along with a textual diff tree and per-change-type breakdown.
type SnapDiffSignal struct {
	SnapshotBefore  string
	SnapshotAfter   string
	Dataset         string
	AddedFiles      int64
	ModifiedFiles   int64
	DeletedFiles    int64
	AddedBytes      int64
	ModifiedBytes   int64
	DeletedBytes    int64
	DiffTree        string
	CreatedAt       time.Time
	ChangesByType   map[string]int64
}

// DiffEntry represents a single file or directory change in the diff tree.
type DiffEntry struct {
	Path      string
	ChangeType string // "added", "modified", "deleted"
	IsDir     bool
	Children  []DiffEntry
}

// GenerateDiff compares two snapshots and returns a populated SnapDiffSignal
// with aggregate statistics and a diff tree.
//
// The before and after maps use file paths as keys and byte sizes as values.
// A path present in "after" but not "before" is an addition; present in
// "before" but not "after" is a deletion; present in both with differing
// sizes is a modification.
func GenerateDiff(snapshotBefore, snapshotAfter string, dataset string,
	beforeFiles, afterFiles map[string]int64) SnapDiffSignal {

	signal := SnapDiffSignal{
		SnapshotBefore: snapshotBefore,
		SnapshotAfter:  snapshotAfter,
		Dataset:        dataset,
		CreatedAt:      time.Now(),
		ChangesByType:  make(map[string]int64),
	}

	// Collect all paths
	allPaths := make(map[string]struct{})
	for p := range beforeFiles {
		allPaths[p] = struct{}{}
	}
	for p := range afterFiles {
		allPaths[p] = struct{}{}
	}

	var entries []DiffEntry

	for p := range allPaths {
		beforeSize, inBefore := beforeFiles[p]
		afterSize, inAfter := afterFiles[p]

		switch {
		case inAfter && !inBefore:
			signal.AddedFiles++
			signal.AddedBytes += afterSize
			signal.ChangesByType["added"]++
			entries = append(entries, DiffEntry{
				Path:      p,
				ChangeType: "added",
				Children:  nil,
			})
		case inBefore && !inAfter:
			signal.DeletedFiles++
			signal.DeletedBytes += beforeSize
			signal.ChangesByType["deleted"]++
			entries = append(entries, DiffEntry{
				Path:      p,
				ChangeType: "deleted",
				Children:  nil,
			})
		case inBefore && inAfter && beforeSize != afterSize:
			signal.ModifiedFiles++
			signal.ModifiedBytes += afterSize - beforeSize
			signal.ChangesByType["modified"]++
			entries = append(entries, DiffEntry{
				Path:      p,
				ChangeType: "modified",
				Children:  nil,
			})
		}
	}

	// Sort entries for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	signal.DiffTree = VisualizeDiff(entries)
	return signal
}

// VisualizeDiff generates a tree-structured textual representation of
// the diff entries using indentation and +/−/~ symbols to indicate
// added, deleted, and modified files respectively.
func VisualizeDiff(entries []DiffEntry) string {
	var b strings.Builder

	if len(entries) == 0 {
		b.WriteString("(no changes)\n")
		return b.String()
	}

	for _, e := range entries {
		writeDiffEntry(&b, e, 0)
	}

	return b.String()
}

// writeDiffEntry recursively writes a diff entry with proper indentation.
func writeDiffEntry(b *strings.Builder, e DiffEntry, depth int) {
	indent := strings.Repeat("  ", depth)
	symbol := diffSymbol(e.ChangeType)

	fmt.Fprintf(b, "%s%s %s", indent, symbol, e.Path)
	if e.IsDir {
		b.WriteString("/")
	}
	b.WriteString("\n")

	// Sort children for deterministic output
	children := e.Children
	sort.Slice(children, func(i, j int) bool {
		return children[i].Path < children[j].Path
	})

	for _, c := range children {
		writeDiffEntry(b, c, depth+1)
	}
}

// diffSymbol returns the visual symbol for a change type.
func diffSymbol(changeType string) string {
	switch changeType {
	case "added":
		return "+"
	case "deleted":
		return "−"
	case "modified":
		return "~"
	default:
		return "?"
	}
}

// ChangeSummary represents a summary of snapshot diff changes.
type ChangeSummary struct {
	TotalChanges      int64
	TotalBytesChanged int64
	LargestChangeDir   string
	LargestChangeBytes int64
	ChangeTrend        string // "growth", "shrinkage", "mixed", "stable"
	Details            string
}

// SummarizeChanges analyzes a SnapDiffSignal and returns a human-readable
// summary including total change volume, the largest changed directory,
// and an overall trend classification.
func SummarizeChanges(s SnapDiffSignal) ChangeSummary {
	totalChanges := s.AddedFiles + s.ModifiedFiles + s.DeletedFiles
	totalBytesChanged := s.AddedBytes + absInt64(s.ModifiedBytes) + s.DeletedBytes

	// Determine trend
	var trend string
	switch {
	case s.AddedFiles > 0 && s.DeletedFiles == 0 && s.ModifiedFiles == 0:
		trend = "growth"
	case s.DeletedFiles > 0 && s.AddedFiles == 0 && s.ModifiedFiles == 0:
		trend = "shrinkage"
	case totalChanges == 0:
		trend = "stable"
	default:
		trend = "mixed"
	}

	// Find largest change directory from the diff tree
	largestDir, largestBytes := findLargestChangeDir(s)

	summary := ChangeSummary{
		TotalChanges:       totalChanges,
		TotalBytesChanged: totalBytesChanged,
		LargestChangeDir:   largestDir,
		LargestChangeBytes: largestBytes,
		ChangeTrend:       trend,
	}

	summary.Details = fmt.Sprintf(
		"Dataset %q: %d files changed (%d added, %d modified, %d deleted), "+
			"%d bytes affected. Trend: %s. Largest change in %s (%d bytes).",
		s.Dataset, totalChanges, s.AddedFiles, s.ModifiedFiles, s.DeletedFiles,
		totalBytesChanged, trend, largestDir, largestBytes,
	)

	return summary
}

// findLargestChangeDir parses the diff tree to find the directory with
// the largest byte-level changes. Falls back to the dataset root if the
// tree is empty or unparseable.
func findLargestChangeDir(s SnapDiffSignal) (string, int64) {
	totalChanges := s.AddedFiles + s.ModifiedFiles + s.DeletedFiles
	totalBytes := s.AddedBytes + absInt64(s.ModifiedBytes) + s.DeletedBytes

	if s.DiffTree == "" || s.DiffTree == "(no changes)\n" {
		return s.Dataset, 0
	}

	// Aggregate bytes by top-level directory
	dirBytes := make(map[string]int64)

	for _, line := range strings.Split(s.DiffTree, "\n") {
		if line == "" || line == "(no changes)" {
			continue
		}
		// Extract the path from the diff tree line
		trimmed := strings.TrimLeft(line, " ")
		if len(trimmed) < 2 {
			continue
		}
		// Skip the symbol character
		pathPart := trimmed[2:]
		if pathPart == "" {
			continue
		}
		// Extract top-level directory
		parts := strings.SplitN(pathPart, "/", 2)
		dir := parts[0]
		if dir == "" {
			dir = "/"
		}
		// Distribute bytes evenly as approximation
		dirBytes[dir] += totalBytes / maxInt64(totalChanges, 1)
	}

	if len(dirBytes) == 0 {
		return s.Dataset, totalBytes
	}

	// Find the directory with the most bytes
	var maxDir string
	var maxBytes int64
	for dir, b := range dirBytes {
		if b > maxBytes {
			maxBytes = b
			maxDir = dir
		}
	}

	if maxDir == "" {
		maxDir = s.Dataset
	}

	return maxDir, maxBytes
}

// RetentionRecommendation represents a snapshot retention policy recommendation.
type RetentionRecommendation struct {
	ID          string
	Title       string
	Priority    string
	Action      string
	Reason      string
	KeepDays    int
	KeepWeekly  int
	KeepMonthly int
}

// RecommendRetention analyzes a series of SnapDiffSignals to recommend
// a snapshot retention policy based on observed change frequency.
//
// history is a slice of past SnapDiffSignals ordered from oldest to newest.
// The function evaluates how frequently significant changes occur and
// recommends retention periods accordingly.
func RecommendRetention(history []SnapDiffSignal) []RetentionRecommendation {
	if len(history) == 0 {
		return []RetentionRecommendation{
			{
				ID:       "default-retention",
				Title:    "Use Default Retention Policy",
				Priority: "low",
				Action:   "Keep 7 daily, 4 weekly, 3 monthly snapshots as a safe baseline.",
				Reason:   "No snapshot history available; using conservative default retention.",
				KeepDays: 7,
				KeepWeekly: 4,
				KeepMonthly: 3,
			},
		}
	}

	// Count snapshots with significant changes
	significantChanges := 0
	totalChanges := int64(0)
	for _, s := range history {
		changeCount := s.AddedFiles + s.ModifiedFiles + s.DeletedFiles
		totalChanges += changeCount
		if changeCount > 10 {
			significantChanges++
		}
	}

	// Compute average changes per snapshot
	avgChanges := totalChanges / int64(len(history))

	var recs []RetentionRecommendation

	switch {
	case avgChanges == 0:
		recs = append(recs, RetentionRecommendation{
			ID:          "reduce-retention-stable",
			Title:       "Reduce Retention — Stable Dataset",
			Priority:    "low",
			Action:      "Dataset shows no changes. Reduce to 3 daily, 2 weekly, 1 monthly snapshots.",
			Reason:      "No file changes detected across snapshots; minimal retention is sufficient.",
			KeepDays:    3,
			KeepWeekly:  2,
			KeepMonthly: 1,
		})

	case avgChanges < 5:
		recs = append(recs, RetentionRecommendation{
			ID:          "standard-retention-low-change",
			Title:       "Standard Retention — Low Change Rate",
			Priority:    "medium",
			Action:      "Keep 7 daily, 4 weekly, 3 monthly snapshots.",
			Reason:      "Low change frequency detected; standard retention provides adequate history.",
			KeepDays:    7,
			KeepWeekly:  4,
			KeepMonthly: 3,
		})

	case avgChanges < 50:
		recs = append(recs, RetentionRecommendation{
			ID:          "enhanced-retention-moderate-change",
			Title:       "Enhanced Retention — Moderate Change Rate",
			Priority:    "high",
			Action:      "Keep 14 daily, 6 weekly, 3 monthly snapshots for faster recovery.",
			Reason:      "Moderate change frequency detected; enhanced retention reduces risk of data loss.",
			KeepDays:    14,
			KeepWeekly:  6,
			KeepMonthly: 3,
		})

	default:
		recs = append(recs, RetentionRecommendation{
			ID:          "aggressive-retention-high-change",
			Title:       "Aggressive Retention — High Change Rate",
			Priority:    "critical",
			Action:      "Keep 30 daily, 8 weekly, 6 monthly snapshots; consider hourly snapshots during peak hours.",
			Reason:      "High change frequency detected; aggressive retention minimizes recovery point objective.",
			KeepDays:    30,
			KeepWeekly:  8,
			KeepMonthly: 6,
		})
	}

	// If more than half the snapshots have significant changes, add a warning
	if len(history) > 0 && significantChanges > len(history)/2 {
		recs = append(recs, RetentionRecommendation{
			ID:          "increase-snapshot-frequency",
			Title:       "Increase Snapshot Frequency",
			Priority:    "high",
			Action:      "Consider taking snapshots more frequently (every 1-2 hours) during business hours.",
			Reason:      "More than half of recent snapshots had significant changes; frequent snapshots reduce potential data loss.",
			KeepDays:    14,
			KeepWeekly:  6,
			KeepMonthly: 3,
		})
	}

	return recs
}

// absInt64 returns the absolute value of an int64.
func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// maxInt64 returns the larger of two int64 values.
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}