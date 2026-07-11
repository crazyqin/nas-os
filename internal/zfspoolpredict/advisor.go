package zfspoolpredict

import (
	"math"
	"time"
)

// PoolHealthSignal contains all parameters needed for ZFS pool health prediction.
type PoolHealthSignal struct {
	PoolName        string
	VdevType        string // mirror, raidz1, raidz2, raidz3, stripe
	DiskCount       int
	HealthyDisks    int
	DegradedDisks   int
	FailedDisks     int
	ScrubErrors     int64
	ChecksumErrors  int64
	ReadErrors      int64
	WriteErrors     int64
	SMARTWarnings   int
	PoolCapacity    float64 // 0-100 percent used
	Fragmentation   float64 // 0-100 percent
	LastScrubTime   time.Time
	HealthScore     float64 // 0-100, computed by Predict
	PredictedFailure bool
	RiskLevel       string // Low, Medium, High, Critical
}

// Recommendation is a single actionable suggestion for pool maintenance.
type Recommendation struct {
	Action  string // ScrubNow, ReplaceDisk, CheckCables, NoAction
	Reason  string
	Target  string // disk path or pool name
}

// Predict computes the HealthScore (0-100), RiskLevel, and PredictedFailure
// from the raw metrics in the signal.
func Predict(s *PoolHealthSignal) {
	score := 100.0

	// --- Disk health penalties ---

	// Failed disks are the most severe: each costs 30 points
	score -= float64(s.FailedDisks) * 30

	// Degraded disks cost 15 points each
	score -= float64(s.DegradedDisks) * 15

	// SMART warnings cost 8 points each
	score -= float64(s.SMARTWarnings) * 8

	// --- Error penalties ---

	// Scrub errors indicate media problems; scale logarithmically
	if s.ScrubErrors > 0 {
		score -= math.Log10(float64(s.ScrubErrors)+1) * 10
	}

	// Checksum errors indicate silent corruption; scale logarithmically
	if s.ChecksumErrors > 0 {
		score -= math.Log10(float64(s.ChecksumErrors)+1) * 12
	}

	// Read errors
	if s.ReadErrors > 0 {
		score -= math.Log10(float64(s.ReadErrors)+1) * 6
	}

	// Write errors
	if s.WriteErrors > 0 {
		score -= math.Log10(float64(s.WriteErrors)+1) * 6
	}

	// --- Capacity penalty ---
	// Pools above 80% capacity start to degrade; above 90% is critical
	if s.PoolCapacity > 90 {
		score -= 15
	} else if s.PoolCapacity > 80 {
		score -= 8
	}

	// --- Fragmentation penalty ---
	if s.Fragmentation > 70 {
		score -= 5
	}

	// Clamp to [0, 100]
	score = math.Max(0, math.Min(100, score))
	s.HealthScore = score

	// --- Risk level ---
	switch {
	case score >= 80:
		s.RiskLevel = "Low"
	case score >= 60:
		s.RiskLevel = "Medium"
	case score >= 40:
		s.RiskLevel = "High"
	default:
		s.RiskLevel = "Critical"
	}

	// --- Predicted failure ---
	// Predict failure when there are degraded disks, or a significant
	// checksum-error trend (more than 10 checksum errors), or any failed disks.
	if s.DegradedDisks > 0 || s.FailedDisks > 0 || s.ChecksumErrors > 10 {
		s.PredictedFailure = true
	} else {
		s.PredictedFailure = false
	}
}

// Recommend produces a prioritised list of maintenance recommendations
// based on the evaluated signal.  The caller should invoke Predict first
// (or rely on the fact that Recommend calls it internally).
func Recommend(s PoolHealthSignal) []Recommendation {
	Predict(&s)
	var recs []Recommendation

	// Failed disks: immediate replacement required.
	if s.FailedDisks > 0 {
		recs = append(recs, Recommendation{
			Action: "ReplaceDisk",
			Reason: "one or more disks have failed and the pool is at risk of data loss",
			Target: s.PoolName,
		})
	}

	// Degraded disks: recommend replacement before failure.
	if s.DegradedDisks > 0 && s.FailedDisks == 0 {
		recs = append(recs, Recommendation{
			Action: "ReplaceDisk",
			Reason: "degraded disks detected; replace proactively to avoid pool failure",
			Target: s.PoolName,
		})
	}

	// High checksum errors: possible cable/controller issue.
	if s.ChecksumErrors > 10 {
		recs = append(recs, Recommendation{
			Action: "CheckCables",
			Reason: "elevated checksum errors suggest possible cable, HBA, or controller problems",
			Target: s.PoolName,
		})
	}

	// Scrub errors or stale scrub: run a scrub now.
	scrubStale := false
	if !s.LastScrubTime.IsZero() && time.Since(s.LastScrubTime) > 35*24*time.Hour {
		scrubStale = true
	}
	if s.ScrubErrors > 0 || scrubStale {
		recs = append(recs, Recommendation{
			Action: "ScrubNow",
			Reason: "scrub errors detected or last scrub is more than 35 days old; run a scrub to identify and repair latent issues",
			Target: s.PoolName,
		})
	}

	// SMART warnings: replace affected disks.
	if s.SMARTWarnings > 0 {
		recs = append(recs, Recommendation{
			Action: "ReplaceDisk",
			Reason: "SMART warnings indicate impending disk failure",
			Target: s.PoolName,
		})
	}

	// If nothing is wrong, say so.
	if len(recs) == 0 {
		recs = append(recs, Recommendation{
			Action: "NoAction",
			Reason: "pool is healthy; no maintenance required at this time",
			Target: s.PoolName,
		})
	}

	return recs
}