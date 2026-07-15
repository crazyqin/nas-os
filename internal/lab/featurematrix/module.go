// Package featurematrix provides functionality for building and analyzing
// feature comparison matrices between NAS-OS and competing NAS platforms.
// It supports automatic matrix generation, feature completeness scoring,
// gap identification, and prioritization of missing or weaker features.
package featurematrix

import (
	"sort"
)

// FeatureMatrix represents the complete comparison matrix between NAS-OS and competitors.
type FeatureMatrix struct {
	// Name of the NAS-OS product being compared.
	ProductName string
	// Version of the NAS-OS product.
	Version string
	// Entries is the list of feature entries forming the matrix rows.
	Entries []FeatureEntry
	// Competitors lists the competitor product names included in the matrix.
	Competitors []string
}

// FeatureEntry represents a single feature row in the comparison matrix.
type FeatureEntry struct {
	// Name is the human-readable feature name (e.g. "Snapshot Management").
	Name string
	// Category groups related features (e.g. "Storage", "Backup", "Security").
	Category string
	// Description explains what the feature does.
	Description string
	// NasOSSupport indicates NAS-OS support level: "full", "partial", "none".
	NasOSSupport string
	// CompetitorFeatures maps competitor product names to their support details.
	CompetitorFeatures map[string]CompetitorFeature
}

// CompetitorFeature describes a competitor's implementation of a specific feature.
type CompetitorFeature struct {
	// CompetitorName is the name of the competing product.
	CompetitorName string
	// Support level: "full", "partial", "none".
	Support string
	// Notes provides additional context about the competitor's implementation.
	Notes string
	// Version indicates which version of the competitor product has this feature.
	Version string
}

// GapAnalysis represents the analysis of feature gaps between NAS-OS and competitors.
type GapAnalysis struct {
	// FeatureName is the name of the feature with a gap.
	FeatureName string
	// Category of the feature.
	Category string
	// GapType classifies the gap: "missing", "weaker", "parity".
	GapType string
	// Severity rates the gap's importance: "critical", "high", "medium", "low".
	Severity string
	// AffectedCompetitors lists competitors that have a stronger implementation.
	AffectedCompetitors []string
	// Description explains the gap in detail.
	Description string
}

// BuildMatrix constructs a FeatureMatrix from a set of feature entries and competitor names.
// It ensures each FeatureEntry has a CompetitorFeatures map initialized for all competitors.
func BuildMatrix(productName, version string, entries []FeatureEntry, competitors []string) *FeatureMatrix {
	for i := range entries {
		if entries[i].CompetitorFeatures == nil {
			entries[i].CompetitorFeatures = make(map[string]CompetitorFeature)
		}
		for _, c := range competitors {
			if _, exists := entries[i].CompetitorFeatures[c]; !exists {
				entries[i].CompetitorFeatures[c] = CompetitorFeature{
					CompetitorName: c,
					Support:        "none",
				}
			}
		}
	}
	return &FeatureMatrix{
		ProductName: productName,
		Version:     version,
		Entries:     entries,
		Competitors: competitors,
	}
}

// ScoreFeature rates the completeness of a single feature entry on a 0–100 scale.
// Full support = 100, partial = 50, none = 0. The final score factors in competitor
// coverage so that features where NAS-OS is ahead get a bonus.
func ScoreFeature(entry FeatureEntry) float64 {
	var nasScore float64
	switch entry.NasOSSupport {
	case "full":
		nasScore = 100
	case "partial":
		nasScore = 50
	default:
		nasScore = 0
	}

	// Calculate average competitor score for comparison.
	var compTotal float64
	compCount := 0
	for _, cf := range entry.CompetitorFeatures {
		var s float64
		switch cf.Support {
		case "full":
			s = 100
		case "partial":
			s = 50
		default:
			s = 0
		}
		compTotal += s
		compCount++
	}
	if compCount == 0 {
		return nasScore
	}
	avgComp := compTotal / float64(compCount)

	// Bonus if NAS-OS is ahead of average competitor.
	if nasScore > avgComp {
		nasScore += (nasScore - avgComp) * 0.1 // 10% of the lead as bonus
		if nasScore > 100 {
			nasScore = 100
		}
	}
	return nasScore
}

// IdentifyGaps compares NAS-OS against all competitors in the matrix and returns
// a list of GapAnalysis items for features where NAS-OS is missing or weaker.
func IdentifyGaps(matrix *FeatureMatrix) []GapAnalysis {
	var gaps []GapAnalysis
	for _, entry := range matrix.Entries {
		var aheadCompetitors []string
		for _, compName := range matrix.Competitors {
			cf, ok := entry.CompetitorFeatures[compName]
			if !ok {
				continue
			}
			// Determine if competitor is ahead.
			if rankSupport(cf.Support) > rankSupport(entry.NasOSSupport) {
				aheadCompetitors = append(aheadCompetitors, compName)
			}
		}

		if len(aheadCompetitors) == 0 {
			continue
		}

		gapType := "weaker"
		if entry.NasOSSupport == "none" {
			gapType = "missing"
		}

		severity := classifySeverity(entry.Category, gapType, len(aheadCompetitors))

		gaps = append(gaps, GapAnalysis{
			FeatureName:        entry.Name,
			Category:           entry.Category,
			GapType:            gapType,
			Severity:           severity,
			AffectedCompetitors: aheadCompetitors,
			Description:        entry.Description,
		})
	}
	return gaps
}

// PrioritizeGaps sorts gap analyses by severity (critical > high > medium > low)
// and returns the sorted slice. Within the same severity, gaps affecting more
// competitors are ranked higher.
func PrioritizeGaps(gaps []GapAnalysis) []GapAnalysis {
	// Make a copy to avoid mutating the input.
	sorted := make([]GapAnalysis, len(gaps))
	copy(sorted, gaps)

	sort.SliceStable(sorted, func(i, j int) bool {
		ri := rankSeverity(sorted[i].Severity)
		rj := rankSeverity(sorted[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return len(sorted[i].AffectedCompetitors) > len(sorted[j].AffectedCompetitors)
	})
	return sorted
}

// rankSupport converts support level to a numeric rank for comparison.
func rankSupport(level string) int {
	switch level {
	case "full":
		return 3
	case "partial":
		return 2
	default:
		return 0
	}
}

// rankSeverity converts severity to a numeric rank for sorting.
func rankSeverity(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// classifySeverity determines gap severity based on category, gap type, and
// the number of competitors that are ahead.
func classifySeverity(category, gapType string, competitorsAhead int) string {
	// Critical categories that affect core NAS functionality.
	criticalCategories := map[string]bool{
		"Storage":    true,
		"Backup":     true,
		"Security":   true,
		"FileSharing": true,
	}

	if gapType == "missing" && criticalCategories[category] && competitorsAhead >= 2 {
		return "critical"
	}
	if gapType == "missing" && competitorsAhead >= 2 {
		return "high"
	}
	if gapType == "missing" {
		return "medium"
	}
	// gapType == "weaker"
	if competitorsAhead >= 3 {
		return "high"
	}
	if competitorsAhead >= 1 {
		return "medium"
	}
	return "low"
}