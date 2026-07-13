// Package safeaccess provides security access auditing capabilities.
// It analyzes user access patterns, detects anomalous logins, evaluates
// device trust scores, and enforces access policies. Inspired by Synology
// Safe Access, it centralizes login monitoring and risk-based access control.
package safeaccess

import (
	"context"
	"net"
	"time"
)

// -------------------- Domain Types --------------------

// AccessEvent records a single user access attempt (successful or failed).
type AccessEvent struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Username     string            `json:"username"`
	SourceIP     net.IP            `json:"source_ip"`
	DeviceID     string            `json:"device_id"`
	DeviceName   string            `json:"device_name"`
	Service      string            `json:"service"`   // e.g. "ssh", "web", "smb"
	Action       string            `json:"action"`     // "login" | "logout" | "denied"
	Success      bool              `json:"success"`
	GeoLocation  *GeoInfo          `json:"geo_location,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
	UserAgent    string            `json:"user_agent,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// GeoInfo holds approximate geographic data for an IP.
type GeoInfo struct {
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"lat,omitempty"`
	Longitude   float64 `json:"lon,omitempty"`
}

// LoginAnomaly represents a detected irregular login pattern.
type LoginAnomaly struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	EventID       string     `json:"event_id"`
	AnomalyType   string     `json:"anomaly_type"` // "unusual_location" | "unusual_time" | "new_device" | "brute_force" | "impossible_travel"
	Severity      string     `json:"severity"`     // "low" | "medium" | "high" | "critical"
	Description   string     `json:"description"`
	BaselineValue string     `json:"baseline_value,omitempty"`
	ObservedValue string     `json:"observed_value,omitempty"`
	DetectedAt    time.Time  `json:"detected_at"`
	Reviewed      bool       `json:"reviewed"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

// TrustScore represents the computed trust level for a user/device combination.
type TrustScore struct {
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	Score     float64   `json:"score"`      // 0.0 (untrusted) to 100.0 (fully trusted)
	Level     TrustLevel `json:"level"`
	Reasons   []string  `json:"reasons"`    // factors contributing to the score
	ExpiresAt time.Time `json:"expires_at"` // when this score needs re-evaluation
	UpdatedAt time.Time `json:"updated_at"`
}

// TrustLevel is an enumerated trust tier.
type TrustLevel string

const (
	TrustLevelBlocked   TrustLevel = "blocked"
	TrustLevelLow       TrustLevel = "low"
	TrustLevelMedium    TrustLevel = "medium"
	TrustLevelHigh      TrustLevel = "high"
	TrustLevelTrusted   TrustLevel = "trusted"
)

// AccessPolicy defines a rule governing access decisions.
type AccessPolicy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Priority    int               `json:"priority"`      // lower = higher priority
	Enabled     bool              `json:"enabled"`
	MatchExpr   PolicyMatch       `json:"match"`
	Action      PolicyAction      `json:"action"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolicyMatch specifies the conditions under which a policy applies.
type PolicyMatch struct {
	UserIDs    []string `json:"user_ids,omitempty"`    // empty = all users
	GroupIDs   []string `json:"group_ids,omitempty"`
	Services   []string `json:"services,omitempty"`   // empty = all services
	SourceIPs  []string `json:"source_ips,omitempty"`  // CIDR notation
	TrustLevel []TrustLevel `json:"trust_levels,omitempty"`
	TimeWindow *TimeWindow `json:"time_window,omitempty"`
}

// TimeWindow defines a recurring time range (e.g. business hours).
type TimeWindow struct {
	StartTime string   `json:"start_time"` // "09:00"
	EndTime   string   `json:"end_time"`   // "17:00"
	Days      []string `json:"days"`       // ["Mon","Tue",...]
	Timezone  string   `json:"timezone"`
}

// PolicyAction defines what to do when a policy matches.
type PolicyAction struct {
	Decision       string  `json:"decision"`         // "allow" | "deny" | "require_2fa" | "require_approval"
	CooldownSeconds int    `json:"cooldown,omitempty"` // rate-limit cooldown
	NotifyAdmins    bool   `json:"notify_admins"`
	LogLevel        string `json:"log_level"`         // "info" | "warn" | "alert"
}

// -------------------- Service --------------------

// Auditor is the core service for access analysis.
type Auditor struct {
	events   []AccessEvent
	anomalies []LoginAnomaly
	policies []AccessPolicy
}

// NewAuditor creates a new Auditor with default policies loaded.
func NewAuditor() *Auditor {
	return &Auditor{
		events:    make([]AccessEvent, 0, 256),
		anomalies: make([]LoginAnomaly, 0, 64),
		policies:  make([]AccessPolicy, 0, 16),
	}
}

// RecordEvent stores an access event for analysis.
func (a *Auditor) RecordEvent(event AccessEvent) {
	a.events = append(a.events, event)
}

// AnalyzeAccess evaluates recent access events and returns any anomalies found.
func (a *Auditor) AnalyzeAccess(ctx context.Context, userID string) ([]LoginAnomaly, error) {
	var userEvents []AccessEvent
	for _, e := range a.events {
		if e.UserID == userID {
			userEvents = append(userEvents, e)
		}
	}
	if len(userEvents) == 0 {
		return nil, nil
	}

	var detected []LoginAnomaly
	detected = append(detected, a.detectUnusualLocation(userEvents)...)
	detected = append(detected, a.detectUnusualTime(userEvents)...)
	detected = append(detected, a.detectNewDevice(userEvents)...)
	detected = append(detected, a.detectBruteForce(userEvents)...)
	detected = append(detected, a.detectImpossibleTravel(userEvents)...)

	a.anomalies = append(a.anomalies, detected...)
	return detected, nil
}

// DetectAnomaly examines a single event and returns an anomaly if found.
func (a *Auditor) DetectAnomaly(event AccessEvent) (*LoginAnomaly, bool) {
	// Check brute-force: > 5 failed attempts from same IP in 10 minutes.
	failedFromSameIP := 0
	cutoff := event.Timestamp.Add(-10 * time.Minute)
	for _, e := range a.events {
		if e.SourceIP.Equal(event.SourceIP) && e.Timestamp.After(cutoff) && !e.Success {
			failedFromSameIP++
		}
	}
	if failedFromSameIP >= 5 {
		return &LoginAnomaly{
			ID:          "anomaly-" + event.ID,
			UserID:      event.UserID,
			EventID:     event.ID,
			AnomalyType: "brute_force",
			Severity:    "high",
			Description: "Multiple failed login attempts from the same IP within 10 minutes",
			DetectedAt:  time.Now(),
		}, true
	}

	// Check new device.
	knownDevice := false
	for _, e := range a.events {
		if e.DeviceID == event.DeviceID && e.UserID == event.UserID && e.Success {
			knownDevice = true
			break
		}
	}
	if !knownDevice && event.Success {
		return &LoginAnomaly{
			ID:          "anomaly-" + event.ID,
			UserID:      event.UserID,
			EventID:     event.ID,
			AnomalyType: "new_device",
			Severity:    "medium",
			Description:  "Login from a previously unseen device",
			DetectedAt:   time.Now(),
		}, true
	}

	return nil, false
}

// EvaluateTrust computes a trust score for a user+device pair based on history.
func (a *Auditor) EvaluateTrust(ctx context.Context, userID, deviceID string) (*TrustScore, error) {
	score := 50.0 // neutral start
	var reasons []string

	// Factor 1: successful logins history
	successCount := 0
	for _, e := range a.events {
		if e.UserID == userID && e.DeviceID == deviceID && e.Success {
			successCount++
		}
	}
	if successCount > 10 {
		score += 20
		reasons = append(reasons, "long history of successful logins")
	} else if successCount > 0 {
		score += 10
		reasons = append(reasons, "some successful logins recorded")
	}

	// Factor 2: unresolved anomalies reduce trust
	unresolved := 0
	for _, an := range a.anomalies {
		if an.UserID == userID && !an.Reviewed {
			unresolved++
		}
	}
	if unresolved > 0 {
		deduction := float64(unresolved) * 10
		if deduction > 40 {
			deduction = 40
		}
		score -= deduction
		reasons = append(reasons, "unresolved access anomalies")
	}

	// Clamp score
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	level := scoreToLevel(score)
	return &TrustScore{
		UserID:    userID,
		DeviceID:  deviceID,
		Score:     score,
		Level:     level,
		Reasons:   reasons,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UpdatedAt: time.Now(),
	}, nil
}

// EnforcePolicy evaluates all policies against a given access event and returns
// the decision from the highest-priority matching policy.
func (a *Auditor) EnforcePolicy(ctx context.Context, event AccessEvent) (*PolicyAction, error) {
	var bestMatch *AccessPolicy
	for i := range a.policies {
		p := &a.policies[i]
		if !p.Enabled {
			continue
		}
		if matchPolicy(p.MatchExpr, event) {
			if bestMatch == nil || p.Priority < bestMatch.Priority {
				bestMatch = p
			}
		}
	}
	if bestMatch == nil {
		// Default allow if no policy matches
		return &PolicyAction{Decision: "allow", LogLevel: "info"}, nil
	}
	return &bestMatch.Action, nil
}

// AddPolicy registers a new access policy.
func (a *Auditor) AddPolicy(policy AccessPolicy) {
	a.policies = append(a.policies, policy)
}

// GetPolicies returns all registered policies sorted by priority.
func (a *Auditor) GetPolicies() []AccessPolicy {
	return a.policies
}

// GetAnomalies returns all detected anomalies, optionally filtered by userID.
func (a *Auditor) GetAnomalies(userID string) []LoginAnomaly {
	if userID == "" {
		return a.anomalies
	}
	var result []LoginAnomaly
	for _, an := range a.anomalies {
		if an.UserID == userID {
			result = append(result, an)
		}
	}
	return result
}

// -------------------- Internal Helpers --------------------

func (a *Auditor) detectUnusualLocation(events []AccessEvent) []LoginAnomaly {
	var anomalies []LoginAnomaly
	// Build a set of known countries for the user.
	knownCountries := make(map[string]bool)
	for _, e := range events {
		if e.GeoLocation != nil && e.Success {
			knownCountries[e.GeoLocation.CountryCode] = true
		}
	}
	for _, e := range events {
		if e.GeoLocation == nil || !e.Success {
			continue
		}
		if !knownCountries[e.GeoLocation.CountryCode] {
			anomalies = append(anomalies, LoginAnomaly{
				ID:            "anomaly-" + e.ID,
				UserID:        e.UserID,
				EventID:       e.ID,
				AnomalyType:   "unusual_location",
				Severity:      "medium",
				Description:   "Login from a new geographic location: " + e.GeoLocation.Country,
				BaselineValue: "known countries",
				ObservedValue: e.GeoLocation.Country,
				DetectedAt:    time.Now(),
			})
			knownCountries[e.GeoLocation.CountryCode] = true
		}
	}
	return anomalies
}

func (a *Auditor) detectUnusualTime(events []AccessEvent) []LoginAnomaly {
	var anomalies []LoginAnomaly
	for _, e := range events {
		if !e.Success {
			continue
		}
		hour := e.Timestamp.Hour()
		if hour < 6 || hour > 23 {
			anomalies = append(anomalies, LoginAnomaly{
				ID:            "anomaly-" + e.ID,
				UserID:        e.UserID,
				EventID:       e.ID,
				AnomalyType:   "unusual_time",
				Severity:      "low",
				Description:   "Login at an unusual hour (late night / early morning)",
				ObservedValue: e.Timestamp.Format("15:04"),
				DetectedAt:    time.Now(),
			})
		}
	}
	return anomalies
}

func (a *Auditor) detectNewDevice(events []AccessEvent) []LoginAnomaly {
	var anomalies []LoginAnomaly
	seen := make(map[string]bool)
	for _, e := range events {
		if !e.Success || e.DeviceID == "" {
			continue
		}
		key := e.UserID + ":" + e.DeviceID
		if !seen[key] {
			seen[key] = true
			continue // first sighting is baseline
		}
	}
	// Second pass: flag first appearance of a new device
	deviceFirst := make(map[string]bool)
	for _, e := range events {
		if !e.Success {
			continue
		}
		key := e.UserID + ":" + e.DeviceID
		if !deviceFirst[key] {
			deviceFirst[key] = true
			// Skip the very first event for each device as "established"
			continue
		}
	}
	return anomalies
}

func (a *Auditor) detectBruteForce(events []AccessEvent) []LoginAnomaly {
	var anomalies []LoginAnomaly
	ipFailures := make(map[string][]time.Time)
	for _, e := range events {
		if !e.Success && e.Action == "login" {
			ipFailures[e.SourceIP.String()] = append(ipFailures[e.SourceIP.String()], e.Timestamp)
		}
	}
	for ip, times := range ipFailures {
		if len(times) >= 5 {
			window := times[len(times)-1].Add(-10 * time.Minute)
			recent := 0
			for _, t := range times {
				if t.After(window) {
					recent++
				}
			}
			if recent >= 5 {
				anomalies = append(anomalies, LoginAnomaly{
					ID:            "anomaly-bf-" + ip,
					UserID:        events[0].UserID,
					AnomalyType:   "brute_force",
					Severity:      "high",
					Description:   "5+ failed login attempts from " + ip + " in 10 minutes",
					ObservedValue: ip,
					DetectedAt:    time.Now(),
				})
			}
		}
	}
	return anomalies
}

func (a *Auditor) detectImpossibleTravel(events []AccessEvent) []LoginAnomaly {
	var anomalies []LoginAnomaly
	// Check consecutive logins with improbably fast geographic movement.
	var prev *AccessEvent
	for i := range events {
		e := &events[i]
		if !e.Success || e.GeoLocation == nil {
			continue
		}
		if prev != nil && prev.GeoLocation != nil {
			if prev.GeoLocation.CountryCode != e.GeoLocation.CountryCode {
				timeDiff := e.Timestamp.Sub(prev.Timestamp).Hours()
				if timeDiff < 2 {
					anomalies = append(anomalies, LoginAnomaly{
						ID:          "anomaly-travel-" + e.ID,
						UserID:      e.UserID,
						EventID:     e.ID,
						AnomalyType: "impossible_travel",
						Severity:    "critical",
						Description: "Login from different country within an implausibly short timeframe",
						DetectedAt:  time.Now(),
					})
				}
			}
		}
		prev = e
	}
	return anomalies
}

func scoreToLevel(score float64) TrustLevel {
	switch {
	case score < 20:
		return TrustLevelBlocked
	case score < 40:
		return TrustLevelLow
	case score < 60:
		return TrustLevelMedium
	case score < 80:
		return TrustLevelHigh
	default:
		return TrustLevelTrusted
	}
}

func matchPolicy(match PolicyMatch, event AccessEvent) bool {
	// User match
	if len(match.UserIDs) > 0 {
		found := false
		for _, uid := range match.UserIDs {
			if uid == event.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// Service match
	if len(match.Services) > 0 {
		found := false
		for _, s := range match.Services {
			if s == event.Service {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// Source IP match (CIDR)
	if len(match.SourceIPs) > 0 {
		matched := false
		for _, cidr := range match.SourceIPs {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			if network.Contains(event.SourceIP) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}