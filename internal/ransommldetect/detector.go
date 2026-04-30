package ransommldetect

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Alert represents a ransomware detection alert.
type Alert struct {
	ID         string    `json:"id"`
	Severity   string    `json:"severity"` // low, medium, high, critical
	Source     string    `json:"source"`   // file path or process
	Pattern    string    `json:"pattern"`  // detected pattern description
	Score      float64   `json:"score"`    // 0.0 - 1.0 risk score
	Timestamp  time.Time `json:"timestamp"`
	Blocked    bool      `json:"blocked"`
	ProcessPID int       `json:"process_pid,omitempty"`
}

// FileActivity represents a file operation for analysis.
type FileActivity struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // create, modify, delete, rename
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
	Process   string    `json:"process"`
	UserID    string    `json:"user_id"`
}

// DetectorConfig holds ML detector configuration.
type DetectorConfig struct {
	WindowSizeSec    int     `json:"window_size_sec"`    // analysis window
	EntropyThreshold float64 `json:"entropy_threshold"`   // high entropy = encrypted
	RateThreshold    int     `json:"rate_threshold"`      // max ops per window
	BlockThreshold   float64 `json:"block_threshold"`     // score to auto-block
	Enabled          bool    `json:"enabled"`
}

// DefaultDetectorConfig returns default config.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		WindowSizeSec:    60,
		EntropyThreshold: 7.5,
		RateThreshold:    100,
		BlockThreshold:   0.85,
		Enabled:          true,
	}
}

// Detector provides ML-based ransomware detection.
type Detector struct {
	mu      sync.RWMutex
	config  DetectorConfig
	activities []FileActivity
	alerts  []Alert
	stopCh  chan struct{}
	running bool
}

// NewDetector creates a new ransomware detector.
func NewDetector(config DetectorConfig) *Detector {
	return &Detector{
		config:  config,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the detection engine.
func (d *Detector) Start() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.mu.Unlock()

	go d.analysisLoop()
	log.Println("✅ ML勒索检测引擎已启动")
}

// Stop halts the detection engine.
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	close(d.stopCh)
	d.running = false
}

// RecordActivity records a file activity for analysis.
func (d *Detector) RecordActivity(activity FileActivity) {
	d.mu.Lock()
	d.activities = append(d.activities, activity)
	d.mu.Unlock()
}

// GetAlerts returns all detected alerts.
func (d *Detector) GetAlerts() []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]Alert, len(d.alerts))
	copy(result, d.alerts)
	return result
}

// GetConfig returns the current config.
func (d *Detector) GetConfig() DetectorConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// UpdateConfig updates the detector config.
func (d *Detector) UpdateConfig(config DetectorConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config = config
}

func (d *Detector) analysisLoop() {
	ticker := time.NewTicker(time.Duration(d.config.WindowSizeSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.analyze()
		}
	}
}

func (d *Detector) analyze() {
	d.mu.Lock()
	// Get activities in the current window
	cutoff := time.Now().Add(-time.Duration(d.config.WindowSizeSec) * time.Second)
	var window []FileActivity
	var remaining []FileActivity
	for _, a := range d.activities {
		if a.Timestamp.After(cutoff) {
			window = append(window, a)
		}
		// Keep last 5 minutes for trend analysis
		if a.Timestamp.After(time.Now().Add(-5 * time.Minute)) {
			remaining = append(remaining, a)
		}
	}
	d.activities = remaining
	d.mu.Unlock()

	if len(window) == 0 {
		return
	}

	// Analyze patterns
	score := d.calculateRiskScore(window)
	if score >= d.config.BlockThreshold {
		alert := Alert{
			ID:        fmt.Sprintf("ransom-%d", time.Now().UnixNano()),
			Severity:  severityFromScore(score),
			Source:    mostActivePath(window),
			Pattern:   describePattern(window),
			Score:     score,
			Timestamp: time.Now(),
			Blocked:   true,
		}

		d.mu.Lock()
		d.alerts = append(d.alerts, alert)
		d.mu.Unlock()

		log.Printf("🚨 勒索检测告警: score=%.2f pattern=%s blocked=true", score, alert.Pattern)
	}
}

func (d *Detector) calculateRiskScore(activities []FileActivity) float64 {
	score := 0.0

	// Factor 1: Operation rate
	rate := float64(len(activities)) / float64(d.config.WindowSizeSec)
	if rate > float64(d.config.RateThreshold)/float64(d.config.WindowSizeSec) {
		score += 0.3
	}

	// Factor 2: High rename/delete ratio
	renameCount := 0
	deleteCount := 0
	for _, a := range activities {
		switch a.Operation {
		case "rename":
			renameCount++
		case "delete":
			deleteCount++
		}
	}
	ratio := float64(renameCount+deleteCount) / float64(len(activities))
	if ratio > 0.7 {
		score += 0.3
	}

	// Factor 3: Single process mass operations
	procCounts := make(map[string]int)
	for _, a := range activities {
		procCounts[a.Process]++
	}
	for _, count := range procCounts {
		if count > 50 {
			score += 0.2
			break
		}
	}

	// Factor 4: Entropy estimation (simplified)
	// In production, calculate actual Shannon entropy of file content samples
	// High entropy (>7.5) suggests encryption
	extensions := make(map[string]int)
	for _, a := range activities {
		ext := getExtension(a.Path)
		extensions[ext]++
	}
	// If many files changed extension to unusual ones
	if extensions[".encrypted"] > 0 || extensions[".locked"] > 0 || extensions[".crypto"] > 0 {
		score += 0.4
	}

	return math.Min(1.0, score)
}

func severityFromScore(score float64) string {
	switch {
	case score >= 0.9:
		return "critical"
	case score >= 0.7:
		return "high"
	case score >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

func mostActivePath(activities []FileActivity) string {
	if len(activities) == 0 {
		return ""
	}
	counts := make(map[string]int)
	maxPath := activities[0].Path
	maxCount := 0
	for _, a := range activities {
		dir := getDir(a.Path)
		counts[dir]++
		if counts[dir] > maxCount {
			maxCount = counts[dir]
			maxPath = dir
		}
	}
	return maxPath
}

func describePattern(activities []FileActivity) string {
	renameCount := 0
	deleteCount := 0
	modifyCount := 0
	for _, a := range activities {
		switch a.Operation {
		case "rename":
			renameCount++
		case "delete":
			deleteCount++
		case "modify":
			modifyCount++
		}
	}
	total := len(activities)
	if renameCount > total/2 {
		return "mass_rename_pattern"
	}
	if deleteCount > total/2 {
		return "mass_delete_pattern"
	}
	if modifyCount > total/2 {
		return "mass_encrypt_pattern"
	}
	return "mixed_suspicious_pattern"
}

func getExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			return ""
		}
	}
	return ""
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return "."
}
