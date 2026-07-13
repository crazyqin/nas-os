// Package datasovereignty2 provides data sovereignty enforcement capabilities
// for the NAS OS. It implements data residency policy engines, cross-border
// transfer compliance checks, regional regulation mapping (GDPR/PIPL/CCPA),
// and data classification labeling to ensure data handling complies with
// applicable legal frameworks.
package datasovereignty2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Core Structs
// ---------------------------------------------------------------------------

// SovereigntyPolicy represents a top-level data sovereignty policy that
// governs how data is stored, transferred, and classified within the system.
type SovereigntyPolicy struct {
	ID            string           // Unique policy identifier
	Name          string           // Human-readable policy name
	Description   string           // Detailed description
	ResidencyRules []ResidencyRule // Rules governing data residency
	Regulations   []RegulationMapping // Applicable regulations
	DataLabels    map[string]string // Default classification labels
	CreatedAt     time.Time
	UpdatedAt     time.Time
	mu            sync.RWMutex
}

// ResidencyRule defines where data must physically reside.
type ResidencyRule struct {
	ID              string   // Rule identifier
	Name            string   // Rule name
	AllowedRegions  []string // Regions where data may reside (e.g., "EU", "CN", "US")
	ProhibitedRegions []string // Regions where data must NOT reside
	DataTypeScope   []string // Data types this rule applies to (e.g., "PII", "financial")
	EnforceReplica  bool     // Whether replicas must also respect residency
}

// CrossBorderCheck represents the result and context of a cross-border
// data transfer compliance check.
type CrossBorderCheck struct {
	ID               string    // Check identifier
	SourceRegion     string    // Region the data currently resides in
	DestinationRegion string   // Region the data is being transferred to
	DataType         string    // Classification of the data being transferred
	PolicyID         string    // Reference to the governing SovereigntyPolicy
	Approved         bool      // Whether the transfer is approved
	Reason           string    // Explanation for the decision
	Violations       []string  // List of regulation violations, if any
	Timestamp        time.Time // When the check was performed
}

// RegulationMapping maps a data type and region to applicable regulations
// such as GDPR, PIPL, or CCPA.
type RegulationMapping struct {
	ID           string            // Mapping identifier
	Regulation   string            // Regulation name (e.g., "GDPR", "PIPL", "CCPA")
	Region       string            // Region this mapping applies to
	DataTypeTags []string          // Data types covered (e.g., "PII", "sensitive")
	Level        string            // Compliance level: "strict", "moderate", "light"
	Requirements []string          // List of compliance requirements
	Metadata     map[string]string // Additional regulation-specific metadata
}

// ---------------------------------------------------------------------------
// SovereigntyPolicy Methods
// ---------------------------------------------------------------------------

// CheckResidency evaluates whether the given data type is allowed to reside
// in the specified region under the policy's residency rules.
func (p *SovereigntyPolicy) CheckResidency(dataType string, region string) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, rule := range p.ResidencyRules {
		if !matchDataType(rule.DataTypeScope, dataType) {
			continue
		}
		// Check if region is prohibited
		for _, prohibited := range rule.ProhibitedRegions {
			if prohibited == region {
				return false, fmt.Sprintf("region %s is prohibited for data type %s by rule %s", region, dataType, rule.Name)
			}
		}
		// Check if region is explicitly allowed
		allowed := len(rule.AllowedRegions) == 0 // No allowed list means all non-prohibited are fine
		for _, a := range rule.AllowedRegions {
			if a == region {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Sprintf("region %s is not in the allowed list for data type %s under rule %s", region, dataType, rule.Name)
		}
	}
	return true, "residency check passed"
}

// ValidateTransfer performs a cross-border transfer compliance check for
// moving data of the given type from sourceRegion to destinationRegion.
func (p *SovereigntyPolicy) ValidateTransfer(sourceRegion, destinationRegion, dataType string) CrossBorderCheck {
	p.mu.RLock()
	defer p.mu.RUnlock()

	check := CrossBorderCheck{
		ID:                fmt.Sprintf("xcb-%d", time.Now().UnixNano()),
		SourceRegion:      sourceRegion,
		DestinationRegion: destinationRegion,
		DataType:          dataType,
		PolicyID:          p.ID,
		Timestamp:         time.Now(),
		Approved:          true,
	}

	// Verify source residency
	if ok, reason := p.CheckResidency(dataType, sourceRegion); !ok {
		check.Approved = false
		check.Reason = "source region residency violation: " + reason
		return check
	}

	// Verify destination residency
	if ok, reason := p.CheckResidency(dataType, destinationRegion); !ok {
		check.Approved = false
		check.Reason = "destination region residency violation: " + reason
		return check
	}

	// Verify all regulation mappings are satisfied
	for _, reg := range p.Regulations {
		if !matchRegion(reg.Region, destinationRegion) {
			continue
		}
		if !matchDataType(reg.DataTypeTags, dataType) {
			continue
		}
		switch reg.Level {
		case "strict":
			// Strict requires explicit approval for each requirement
			for _, req := range reg.Requirements {
				if req == "explicit-consent" || req == "government-approval" {
					check.Violations = append(check.Violations,
						fmt.Sprintf("regulation %s requires %s for data type %s in region %s",
							reg.Regulation, req, dataType, destinationRegion))
				}
			}
			if len(check.Violations) > 0 {
				check.Approved = false
				check.Reason = "strict regulation requirements not met"
			}
		case "moderate":
			// Moderate allows transfer but logs for review
			check.Reason = "moderate regulation: transfer allowed with logging"
		case "light":
			// Light allows transfer without additional requirements
		}
	}

	if check.Approved {
		check.Reason = "transfer approved: all residency and regulation checks passed"
	}
	return check
}

// MapRegulation finds the regulation mappings that apply to the given data
// type and region, returning a slice of matching RegulationMapping.
func (p *SovereigntyPolicy) MapRegulation(dataType, region string) []RegulationMapping {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var matched []RegulationMapping
	for _, reg := range p.Regulations {
		if matchDataType(reg.DataTypeTags, dataType) && matchRegion(reg.Region, region) {
			matched = append(matched, reg)
		}
	}
	return matched
}

// ClassifyData assigns classification labels to data based on the policy's
// default data labels and returns the computed label set.
func (p *SovereigntyPolicy) ClassifyData(dataContent string, dataType string) map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	labels := make(map[string]string)
	// Start with default labels from the policy
	for k, v := range p.DataLabels {
		labels[k] = v
	}
	// Add dynamic classification based on data type
	labels["data_type"] = dataType
	labels["classification_timestamp"] = time.Now().Format(time.RFC3339)
	labels["content_hash"] = hashContent(dataContent)

	// Determine sensitivity level from regulation mappings
	for _, reg := range p.Regulations {
		if matchDataType(reg.DataTypeTags, dataType) {
			if reg.Level == "strict" {
				labels["sensitivity"] = "high"
			} else if reg.Level == "moderate" {
				if labels["sensitivity"] != "high" {
					labels["sensitivity"] = "medium"
				}
			} else {
				if labels["sensitivity"] == "" {
					labels["sensitivity"] = "low"
				}
			}
		}
	}
	if labels["sensitivity"] == "" {
		labels["sensitivity"] = "unclassified"
	}

	return labels
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// matchDataType checks whether a data type falls within the given scope/tags.
func matchDataType(scope []string, dataType string) bool {
	if len(scope) == 0 {
		return true // empty scope matches all
	}
	for _, s := range scope {
		if s == dataType || s == "*" {
			return true
		}
	}
	return false
}

// matchRegion checks whether a region matches the regulation's region field.
func matchRegion(regRegion, target string) bool {
	return regRegion == target || regRegion == "*" || regRegion == ""
}

// hashContent returns a SHA-256 hex digest of the given content.
func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}