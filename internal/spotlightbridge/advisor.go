package spotlightbridge

import (
	"sort"
	"time"
)

// Signal represents the current state of SMB Spotlight indexing.
type Signal struct {
	SpotlightEnabled         bool
	SMBShareCount            int
	SharesWithSpotlight       int
	IndexSizeGB              float64
	IndexAge                 time.Duration
	MacosClients             int
	SearchLatencyMs          int
	ContentIndexingEnabled   bool
	EncryptedSharesExcluded  bool
	MaxIndexSizeGB           float64
	IndexCorruptionDetected  bool
	LastReindexAge           time.Duration
}

// Recommendation is a single advisory produced by Analyze.
type Recommendation struct {
	ID       string
	Title    string
	Priority string
	Action   string
	Reason   string
}

// Analyze inspects a Signal and returns a sorted list of recommendations.
func Analyze(s Signal) []Recommendation {
	recs := make([]Recommendation, 0)

	if !s.SpotlightEnabled && s.SMBShareCount > 0 && s.MacosClients > 0 {
		recs = append(recs, Recommendation{
			ID:       "enable-spotlight",
			Title:    "Enable SMB Spotlight",
			Priority: "critical",
			Action:   "Enable the Spotlight service for SMB shares to allow macOS clients to search shared files natively.",
			Reason:   "Spotlight is disabled but there are SMB shares and macOS clients that would benefit from Spotlight search.",
		})
	}

	if s.SpotlightEnabled && s.SharesWithSpotlight < s.SMBShareCount {
		recs = append(recs, Recommendation{
			ID:       "extend-spotlight",
			Title:    "Extend Spotlight to All SMB Shares",
			Priority: "high",
			Action:   "Enable Spotlight indexing on every SMB share so all shares are searchable.",
			Reason:   "Only some SMB shares have Spotlight indexing enabled; extend it to all shares for consistent search.",
		})
	}

	if s.SpotlightEnabled && !s.ContentIndexingEnabled {
		recs = append(recs, Recommendation{
			ID:       "enable-content-indexing",
			Title:    "Enable Content Indexing",
			Priority: "high",
			Action:   "Turn on content indexing so file contents—not just metadata—are searchable.",
			Reason:   "Content indexing is disabled; macOS Spotlight results will be limited to file names and metadata.",
		})
	}

	if s.SpotlightEnabled && !s.EncryptedSharesExcluded {
		recs = append(recs, Recommendation{
			ID:       "exclude-encrypted-shares",
			Title:    "Exclude Encrypted Shares from Indexing",
			Priority: "high",
			Action:   "Configure Spotlight to skip encrypted SMB shares so their contents are not indexed.",
			Reason:   "Encrypted shares are not excluded from Spotlight indexing; indexing them may expose sensitive data.",
		})
	}

	if s.SpotlightEnabled && s.SearchLatencyMs > 500 {
		recs = append(recs, Recommendation{
			ID:       "optimize-index",
			Title:    "Optimize Spotlight Index Performance",
			Priority: "medium",
			Action:   "Rebuild the Spotlight index and consider increasing index cache or tuning I/O priority.",
			Reason:   "Search latency exceeds 500 ms; the index may be fragmented or undersized.",
		})
	}

	if s.SpotlightEnabled && s.IndexSizeGB > s.MaxIndexSizeGB && s.MaxIndexSizeGB > 0 {
		recs = append(recs, Recommendation{
			ID:       "limit-index-size",
			Title:    "Limit Spotlight Index Size",
			Priority: "medium",
			Action:   "Restrict indexed paths or exclude large media directories to keep the index within the configured limit.",
			Reason:   "Index size exceeds the configured maximum; this can consume excessive storage and slow searches.",
		})
	}

	if s.SpotlightEnabled && s.IndexCorruptionDetected {
		recs = append(recs, Recommendation{
			ID:       "rebuild-corrupt-index",
			Title:    "Rebuild Corrupted Index",
			Priority: "critical",
			Action:   "Delete the existing Spotlight index and trigger a full reindex immediately.",
			Reason:   "Index corruption has been detected; search results may be inaccurate or missing.",
		})
	}

	if s.SpotlightEnabled && s.IndexAge > 24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "refresh-stale-index",
			Title:    "Refresh Stale Index",
			Priority: "medium",
			Action:   "Trigger an incremental reindex to bring the Spotlight index up to date.",
			Reason:   "The index has not been updated in over 24 hours; search results may be stale.",
		})
	}

	if s.SpotlightEnabled && s.LastReindexAge > 7*24*time.Hour {
		recs = append(recs, Recommendation{
			ID:       "schedule-periodic-reindex",
			Title:    "Schedule Periodic Reindex",
			Priority: "low",
			Action:   "Set up a scheduled weekly full reindex of all Spotlight-enabled SMB shares.",
			Reason:   "No full reindex has occurred in over 7 days; periodic reindexing prevents index drift.",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return priorityRank(recs[i].Priority) < priorityRank(recs[j].Priority)
	})

	return recs
}

// priorityRank maps priority strings to sort weights (lower = more urgent).
func priorityRank(p string) int {
	switch p {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}