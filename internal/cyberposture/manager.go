package cyberposture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ThreatLevel represents threat levels
type ThreatLevel string

const (
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// PostureScore represents security posture score
type PostureScore struct {
	Overall    int            `json:"overall"`
	Network    int            `json:"network"`
	System     int            `json:"system"`
	Data       int            `json:"data"`
	Access     int            `json:"access"`
	Compliance int            `json:"compliance"`
	Trend      string         `json:"trend"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Breakdown  map[string]int `json:"breakdown"`
}

// Threat represents a detected threat
type Threat struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Level       ThreatLevel `json:"level"`
	Source      string      `json:"source"`
	Target      string      `json:"target"`
	AttackType  string      `json:"attack_type"`
	Indicators  []string    `json:"indicators"`
	DetectedAt  time.Time   `json:"detected_at"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	Status      string      `json:"status"`
	Mitigation  string      `json:"mitigation,omitempty"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string     `json:"id"`
	CVE         string     `json:"cve"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Severity    string     `json:"severity"`
	Score       float64    `json:"score"`
	Affected    []string   `json:"affected"`
	Fix         string     `json:"fix"`
	DetectedAt  time.Time  `json:"detected_at"`
	FixedAt     *time.Time `json:"fixed_at,omitempty"`
	Status      string     `json:"status"`
}

// AttackSurface represents attack surface analysis
type AttackSurface struct {
	OpenPorts       []Port     `json:"open_ports"`
	Services        []Service  `json:"services"`
	Endpoints       []Endpoint `json:"endpoints"`
	ExposedData     []string   `json:"exposed_data"`
	RiskScore       int        `json:"risk_score"`
	Recommendations []string   `json:"recommendations"`
	AnalyzedAt      time.Time  `json:"analyzed_at"`
}

// Port represents an open port
type Port struct {
	Number   int    `json:"number"`
	Protocol string `json:"protocol"`
	Service  string `json:"service"`
	Risk     string `json:"risk"`
}

// Service represents a running service
type Service struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Port    int    `json:"port"`
	Status  string `json:"status"`
	Risk    string `json:"risk"`
}

// Endpoint represents a network endpoint
type Endpoint struct {
	URL     string `json:"url"`
	Type    string `json:"type"`
	Auth    bool   `json:"auth"`
	Exposed bool   `json:"exposed"`
	Risk    string `json:"risk"`
}

// SecurityEvent represents a security event
type SecurityEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Level     ThreatLevel            `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Manager manages cybersecurity posture
type Manager struct {
	mu              sync.RWMutex
	threats         map[string]*Threat
	vulnerabilities map[string]*Vulnerability
	events          []*SecurityEvent
	surface         *AttackSurface
	score           *PostureScore
	config          *Config
}

// Config represents manager configuration
type Config struct {
	ScanInterval   time.Duration `json:"scan_interval"`
	AlertThreshold ThreatLevel   `json:"alert_threshold"`
	AutoMitigate   bool          `json:"auto_mitigate"`
	MaxEvents      int           `json:"max_events"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		ScanInterval:   1 * time.Hour,
		AlertThreshold: ThreatMedium,
		AutoMitigate:   true,
		MaxEvents:      10000,
	}
}

// NewManager creates a new cyber posture manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		threats:         make(map[string]*Threat),
		vulnerabilities: make(map[string]*Vulnerability),
		events:          make([]*SecurityEvent, 0),
		config:          config,
		score: &PostureScore{
			Breakdown: make(map[string]int),
		},
	}
}

// ScanThreats performs threat scanning
func (m *Manager) ScanThreats(ctx context.Context) ([]*Threat, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate threat scanning
	threats := make([]*Threat, 0)

	// Add sample threats if none exist
	if len(m.threats) == 0 {
		threats = append(threats, &Threat{
			ID:          "threat-1",
			Name:        "Brute Force Attempt",
			Description: "Multiple failed login attempts detected",
			Level:       ThreatMedium,
			Source:      "192.168.1.100",
			Target:      "SSH Service",
			AttackType:  "brute_force",
			Indicators:  []string{"50 failed logins in 5 minutes"},
			DetectedAt:  time.Now(),
			Status:      "active",
		})

		for _, t := range threats {
			m.threats[t.ID] = t
		}
	}

	for _, t := range m.threats {
		threats = append(threats, t)
	}

	return threats, nil
}

// ScanVulnerabilities performs vulnerability scanning
func (m *Manager) ScanVulnerabilities(ctx context.Context) ([]*Vulnerability, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	vulns := make([]*Vulnerability, 0)

	// Add sample vulnerabilities if none exist
	if len(m.vulnerabilities) == 0 {
		vulns = append(vulns, &Vulnerability{
			ID:          "vuln-1",
			CVE:         "CVE-2026-1234",
			Title:       "OpenSSL Buffer Overflow",
			Description: "Buffer overflow in OpenSSL 1.1.1",
			Severity:    "high",
			Score:       8.5,
			Affected:    []string{"openssl 1.1.1"},
			Fix:         "Update to OpenSSL 3.0",
			DetectedAt:  time.Now(),
			Status:      "open",
		})

		for _, v := range vulns {
			m.vulnerabilities[v.ID] = v
		}
	}

	for _, v := range m.vulnerabilities {
		vulns = append(vulns, v)
	}

	return vulns, nil
}

// AnalyzeAttackSurface performs attack surface analysis
func (m *Manager) AnalyzeAttackSurface(ctx context.Context) (*AttackSurface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.surface != nil {
		return m.surface, nil
	}

	m.surface = &AttackSurface{
		OpenPorts: []Port{
			{Number: 22, Protocol: "tcp", Service: "SSH", Risk: "medium"},
			{Number: 80, Protocol: "tcp", Service: "HTTP", Risk: "low"},
			{Number: 443, Protocol: "tcp", Service: "HTTPS", Risk: "low"},
			{Number: 445, Protocol: "tcp", Service: "SMB", Risk: "high"},
		},
		Services: []Service{
			{Name: "nginx", Version: "1.24.0", Port: 80, Status: "running", Risk: "low"},
			{Name: "smbd", Version: "4.18.0", Port: 445, Status: "running", Risk: "high"},
		},
		Endpoints: []Endpoint{
			{URL: "/api/v1/auth", Type: "api", Auth: true, Exposed: true, Risk: "low"},
			{URL: "/admin", Type: "web", Auth: true, Exposed: true, Risk: "medium"},
		},
		ExposedData: []string{"Public IP", "Server version header"},
		RiskScore:   65,
		Recommendations: []string{
			"Disable SMB if not needed",
			"Implement rate limiting",
			"Remove version headers",
		},
		AnalyzedAt: time.Now(),
	}

	return m.surface, nil
}

// GetScore gets security posture score
func (m *Manager) GetScore(ctx context.Context) (*PostureScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.score.Overall == 0 {
		m.score = &PostureScore{
			Overall:    75,
			Network:    70,
			System:     80,
			Data:       75,
			Access:     72,
			Compliance: 78,
			Trend:      "improving",
			UpdatedAt:  time.Now(),
			Breakdown: map[string]int{
				"firewall":       80,
				"encryption":     75,
				"authentication": 70,
				"patching":       85,
				"monitoring":     65,
			},
		}
	}

	return m.score, nil
}

// AddEvent adds a security event
func (m *Manager) AddEvent(event *SecurityEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("event-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.events = append(m.events, event)

	// Trim events if too many
	if len(m.events) > m.config.MaxEvents {
		m.events = m.events[len(m.events)-m.config.MaxEvents:]
	}

	return nil
}

// GetEvents gets security events
func (m *Manager) GetEvents(level ThreatLevel, limit int) []*SecurityEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*SecurityEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if level != "" && m.events[i].Level != level {
			continue
		}
		events = append(events, m.events[i])
		if limit > 0 && len(events) >= limit {
			break
		}
	}

	return events
}

// ResolveThreat resolves a threat
func (m *Manager) ResolveThreat(threatID, mitigation string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	threat, exists := m.threats[threatID]
	if !exists {
		return fmt.Errorf("threat %s not found", threatID)
	}

	now := time.Now()
	threat.Status = "resolved"
	threat.ResolvedAt = &now
	threat.Mitigation = mitigation

	return nil
}

// FixVulnerability marks a vulnerability as fixed
func (m *Manager) FixVulnerability(vulnID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vuln, exists := m.vulnerabilities[vulnID]
	if !exists {
		return fmt.Errorf("vulnerability %s not found", vulnID)
	}

	now := time.Now()
	vuln.Status = "fixed"
	vuln.FixedAt = &now

	return nil
}

// GetThreats gets all threats
func (m *Manager) GetThreats(level ThreatLevel) []*Threat {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threats := make([]*Threat, 0)
	for _, t := range m.threats {
		if level != "" && t.Level != level {
			continue
		}
		threats = append(threats, t)
	}

	return threats
}

// GetVulnerabilities gets all vulnerabilities
func (m *Manager) GetVulnerabilities(status string) []*Vulnerability {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vulns := make([]*Vulnerability, 0)
	for _, v := range m.vulnerabilities {
		if status != "" && v.Status != status {
			continue
		}
		vulns = append(vulns, v)
	}

	return vulns
}

// GenerateReport generates a security report
func (m *Manager) GenerateReport(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := map[string]interface{}{
		"generated_at": time.Now(),
		"score":        m.score,
		"threats": map[string]int{
			"active":   0,
			"resolved": 0,
		},
		"vulnerabilities": map[string]int{
			"open":  0,
			"fixed": 0,
		},
		"recommendations": []string{
			"Enable 2FA for all admin accounts",
			"Implement network segmentation",
			"Regular security awareness training",
		},
	}

	activeThreats := 0
	resolvedThreats := 0
	for _, t := range m.threats {
		if t.Status == "active" {
			activeThreats++
		} else {
			resolvedThreats++
		}
	}
	report["threats"].(map[string]int)["active"] = activeThreats
	report["threats"].(map[string]int)["resolved"] = resolvedThreats

	openVulns := 0
	fixedVulns := 0
	for _, v := range m.vulnerabilities {
		if v.Status == "open" {
			openVulns++
		} else {
			fixedVulns++
		}
	}
	report["vulnerabilities"].(map[string]int)["open"] = openVulns
	report["vulnerabilities"].(map[string]int)["fixed"] = fixedVulns

	return report, nil
}

// HandleHTTP registers HTTP handlers
func (m *Manager) HandleHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/cyber/score", m.handleScore)
	mux.HandleFunc("/api/v1/cyber/threats", m.handleThreats)
	mux.HandleFunc("/api/v1/cyber/vulnerabilities", m.handleVulnerabilities)
	mux.HandleFunc("/api/v1/cyber/surface", m.handleSurface)
	mux.HandleFunc("/api/v1/cyber/events", m.handleEvents)
	mux.HandleFunc("/api/v1/cyber/report", m.handleReport)
}

func (m *Manager) handleScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	score, _ := m.GetScore(r.Context())
	json.NewEncoder(w).Encode(score)
}

func (m *Manager) handleThreats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	level := ThreatLevel(r.URL.Query().Get("level"))
	threats := m.GetThreats(level)
	json.NewEncoder(w).Encode(threats)
}

func (m *Manager) handleVulnerabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")
	vulns := m.GetVulnerabilities(status)
	json.NewEncoder(w).Encode(vulns)
}

func (m *Manager) handleSurface(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	surface, _ := m.AnalyzeAttackSurface(r.Context())
	json.NewEncoder(w).Encode(surface)
}

func (m *Manager) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	level := ThreatLevel(r.URL.Query().Get("level"))
	limit := 50
	events := m.GetEvents(level, limit)
	json.NewEncoder(w).Encode(events)
}

func (m *Manager) handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report, _ := m.GenerateReport(r.Context())
	json.NewEncoder(w).Encode(report)
}
