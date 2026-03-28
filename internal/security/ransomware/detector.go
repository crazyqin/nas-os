// Package ransomware provides ransomware detection and protection
package ransomware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Detector detects ransomware activity based on file behavior patterns
type Detector struct {
	config       DetectorConfig
	monitor      *FileMonitor
	analyzer     *BehaviorAnalyzer
	alerts       chan DetectorAlert
	eventLog     []DetectorFileEvent
	ransomwareSigns map[string]bool
	mu           sync.RWMutex
	running      bool
	// 增强功能组件
	entropyAnalyzer    *EntropyAnalyzer
	rapidChangeTracker *RapidChangeTracker
	processMonitor     *ProcessActivityMonitor
	patternMatcher     *AdvancedPatternMatcher
	snapshotManager    *AutoSnapshotManager
}

// DetectorConfig for ransomware detection (internal use)
type DetectorConfig struct {
	EnableDetection     bool          `json:"enableDetection"`
	MonitorPaths        []string      `json:"monitorPaths"`
	ExcludePaths        []string      `json:"excludePaths"`
	MaxEventLogSize     int           `json:"maxEventLogSize"`
	AlertThreshold      int           `json:"alertThreshold"`
	AlertWindow         time.Duration `json:"alertWindow"`
	AutoQuarantine      bool          `json:"autoQuarantine"`
	QuarantinePath      string        `json:"quarantinePath"`
	ProtectedExtensions []string      `json:"protectedExtensions"`
	SuspiciousExtensions []string     `json:"suspiciousExtensions"`
	// 增强检测配置
	EntropyThreshold       float64       `json:"entropyThreshold"`       // 熵值阈值（默认7.5）
	RapidChangeThreshold   int           `json:"rapidChangeThreshold"`   // 快速变更阈值（文件数）
	RapidChangeWindow      time.Duration `json:"rapidChangeWindow"`      // 快速变更时间窗口
	EnableEntropyAnalysis  bool          `json:"enableEntropyAnalysis"`  // 启用熵值分析
	EnableProcessMonitor   bool          `json:"enableProcessMonitor"`   // 启用进程监控
	EnableAutoSnapshot     bool          `json:"enableAutoSnapshot"`     // 启用自动快照
	SnapshotConfig         AutoSnapshotConfig `json:"snapshotConfig"`     // 快照配置
	MaxFileSizeToAnalyze   int64         `json:"maxFileSizeToAnalyze"`   // 最大分析文件大小
}

// DefaultDetectorConfig returns default detector configuration
func DefaultDetectorConfig() *DetectorConfig {
	return &DetectorConfig{
		EnableDetection:  true,
		MonitorPaths:     []string{"/data", "/home"},
		MaxEventLogSize:  10000,
		AlertThreshold:   50,
		AlertWindow:      time.Minute * 5,
		AutoQuarantine:   false,
		QuarantinePath:   "/var/lib/nas-os/quarantine",
		ProtectedExtensions: []string{
			".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			".pdf", ".jpg", ".jpeg", ".png", ".gif", ".mp4",
			".mp3", ".zip", ".tar", ".gz", ".sql", ".db",
		},
		SuspiciousExtensions: []string{
			".encrypted", ".locked", ".crypto", ".ransom",
			".enc", ".cerber", ".locky", ".wannacry",
			".cryptolocker", ".zepto", ".sage", ".globe",
		},
	}
}

// DetectorFileEvent represents a file system event
type DetectorFileEvent struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Path         string    `json:"path"`
	OldPath      string    `json:"oldPath,omitempty"`
	Size         int64     `json:"size"`
	OldSize      int64     `json:"oldSize,omitempty"`
	Extension    string    `json:"extension"`
	OldExtension string    `json:"oldExtension,omitempty"`
	User         string    `json:"user"`
	Process      string    `json:"process"`
	PID          int       `json:"pid"`
	Timestamp    time.Time `json:"timestamp"`
	Suspicious   bool      `json:"suspicious"`
	Reason       string    `json:"reason,omitempty"`
}

// DetectorAlert represents a ransomware alert
type DetectorAlert struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	Title         string              `json:"title"`
	Message       string              `json:"message"`
	Events        []DetectorFileEvent `json:"events"`
	RiskScore     int                 `json:"riskScore"`
	ThreatType    string              `json:"threatType"`
	Recommendations []string          `json:"recommendations"`
	Timestamp     time.Time           `json:"timestamp"`
	Acknowledged  bool                `json:"acknowledged"`
}

// NewDetector creates a new ransomware detector
func NewDetector(config *DetectorConfig) (*Detector, error) {
	if config == nil {
		config = DefaultDetectorConfig()
	}

	if config.AutoQuarantine && config.QuarantinePath != "" {
		if err := os.MkdirAll(config.QuarantinePath, 0700); err != nil {
			return nil, fmt.Errorf("failed to create quarantine path: %w", err)
		}
	}

	d := &Detector{
		config:          *config,
		alerts:          make(chan DetectorAlert, 100),
		eventLog:        make([]DetectorFileEvent, 0),
		ransomwareSigns: make(map[string]bool),
	}

	d.loadSignatures()
	d.monitor = NewFileMonitor(config.MonitorPaths, config.ExcludePaths)
	d.analyzer = NewBehaviorAnalyzer(d.config.AlertThreshold, d.config.AlertWindow)

	return d, nil
}

// loadSignatures loads known ransomware signatures
func (d *Detector) loadSignatures() {
	for _, ext := range d.config.SuspiciousExtensions {
		d.ransomwareSigns[strings.ToLower(ext)] = true
	}

	ransomNotes := []string{
		"readme.txt", "decrypt_instructions.html", "restore_files.txt",
		"_readme_.txt", "help_decrypt.html", "!readme!.txt",
		"de_crypt_readme.txt", "restore_files_.txt", "how_to_decrypt.html",
	}
	for _, note := range ransomNotes {
		d.ransomwareSigns[strings.ToLower(note)] = true
	}
}

// Start begins monitoring for ransomware activity
func (d *Detector) Start(ctx context.Context) error {
	d.mu.Lock()
	d.running = true
	d.mu.Unlock()

	events, err := d.monitor.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start file monitor: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				d.processEvent(event)
			}
		}
	}()

	return nil
}

// Stop stops the detector
func (d *Detector) Stop() {
	d.mu.Lock()
	d.running = false
	d.mu.Unlock()
	d.monitor.Stop()
}

// processEvent processes a file event
func (d *Detector) processEvent(event DetectorFileEvent) {
	event = d.analyzeEvent(event)
	d.logEvent(event)
	alert := d.analyzer.Analyze(event)
	if alert != nil {
		d.handleAlert(*alert)
	}
}

// analyzeEvent determines if an event is suspicious
func (d *Detector) analyzeEvent(event DetectorFileEvent) DetectorFileEvent {
	event.Suspicious = false
	event.Reason = ""

	if d.isRansomwareExtension(event.Extension) {
		event.Suspicious = true
		event.Reason = fmt.Sprintf("Ransomware extension detected: %s", event.Extension)
		return event
	}

	if event.OldExtension != "" && d.isRansomwareExtension(event.Extension) {
		event.Suspicious = true
		event.Reason = fmt.Sprintf("File renamed to ransomware extension: %s -> %s", event.OldExtension, event.Extension)
		return event
	}

	if event.Type == "modify" && d.isProtectedExtension(event.Extension) {
		if event.Size > 0 && event.OldSize > 0 {
			sizeChange := float64(abs(event.Size-event.OldSize)) / float64(event.OldSize)
			if sizeChange > 0.1 {
				event.Suspicious = true
				event.Reason = fmt.Sprintf("Suspicious file modification: %.1f%% size change", sizeChange*100)
				return event
			}
		}
	}

	if event.Type == "delete" && d.isProtectedExtension(event.Extension) {
		event.Suspicious = true
		event.Reason = "Protected file deleted"
		return event
	}

	filename := strings.ToLower(filepath.Base(event.Path))
	if d.ransomwareSigns[filename] {
		event.Suspicious = true
		event.Reason = "Possible ransom note file created"
		return event
	}

	return event
}

// isRansomwareExtension checks if extension is a known ransomware extension
func (d *Detector) isRansomwareExtension(ext string) bool {
	return d.ransomwareSigns[strings.ToLower(ext)]
}

// isProtectedExtension checks if extension should be protected
func (d *Detector) isProtectedExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, protected := range d.config.ProtectedExtensions {
		if ext == strings.ToLower(protected) {
			return true
		}
	}
	return false
}

// logEvent logs an event
func (d *Detector) logEvent(event DetectorFileEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.eventLog = append(d.eventLog, event)
	if len(d.eventLog) > d.config.MaxEventLogSize {
		d.eventLog = d.eventLog[1:]
	}
}

// handleAlert handles a ransomware alert
func (d *Detector) handleAlert(alert DetectorAlert) {
	select {
	case d.alerts <- alert:
	default:
	}
	if d.config.AutoQuarantine {
		d.quarantineFiles(alert.Events)
	}
}

// quarantineFiles moves suspicious files to quarantine
func (d *Detector) quarantineFiles(events []DetectorFileEvent) {
	for _, event := range events {
		if !event.Suspicious {
			continue
		}
		src := event.Path
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		hash := sha256.Sum256([]byte(src + time.Now().String()))
		dst := filepath.Join(d.config.QuarantinePath, hex.EncodeToString(hash[:16])+filepath.Ext(src))
		if err := os.Rename(src, dst); err != nil {
			continue
		}
		d.logEvent(DetectorFileEvent{
			Type:       "quarantine",
			Path:       dst,
			OldPath:    src,
			Timestamp:  time.Now(),
			Suspicious: true,
			Reason:     "Auto-quarantined due to ransomware detection",
		})
	}
}

// Alerts returns the alert channel
func (d *Detector) Alerts() <-chan DetectorAlert {
	return d.alerts
}

// GetEventLog returns recent events
func (d *Detector) GetEventLog(limit int) []DetectorFileEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if limit <= 0 || limit > len(d.eventLog) {
		limit = len(d.eventLog)
	}
	start := len(d.eventLog) - limit
	if start < 0 {
		start = 0
	}
	result := make([]DetectorFileEvent, limit)
	copy(result, d.eventLog[start:])
	return result
}

// ScanResult represents the result of a ransomware scan
type ScanResult struct {
	Path            string        `json:"path"`
	InfectedFiles   []string      `json:"infectedFiles"`
	SuspiciousFiles []string      `json:"suspiciousFiles"`
	RiskScore       int           `json:"riskScore"`
	ScannedAt       time.Time     `json:"scannedAt"`
	Duration        time.Duration `json:"duration"`
}

// ScanDirectory scans a directory for ransomware
func (d *Detector) ScanDirectory(path string) (*ScanResult, error) {
	result := &ScanResult{
		Path:      path,
		ScannedAt: time.Now(),
	}

	start := time.Now()

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(filePath))

		if d.isRansomwareExtension(ext) {
			result.InfectedFiles = append(result.InfectedFiles, filePath)
			result.RiskScore += 20
			return nil
		}

		filename := strings.ToLower(info.Name())
		if d.ransomwareSigns[filename] {
			result.SuspiciousFiles = append(result.SuspiciousFiles, filePath)
			result.RiskScore += 15
			return nil
		}

		if d.isProtectedExtension(ext) {
			if file, err := os.Open(filePath); err == nil {
				defer file.Close()
				header := make([]byte, 512)
				if _, err := file.Read(header); err == nil {
					if d.looksEncrypted(header) {
						result.SuspiciousFiles = append(result.SuspiciousFiles, filePath)
						result.RiskScore += 10
					}
				}
			}
		}

		return nil
	})

	result.Duration = time.Since(start)

	if result.RiskScore > 100 {
		result.RiskScore = 100
	}

	return result, err
}

// looksEncrypted checks if data appears to be encrypted
func (d *Detector) looksEncrypted(data []byte) bool {
	entropy := calculateEntropy(data)
	return entropy > 7.5
}

// calculateEntropy calculates Shannon entropy
func calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]float64)
	for _, b := range data {
		freq[b]++
	}

	var entropy float64
	size := float64(len(data))
	for _, count := range freq {
		p := count / size
		if p > 0 {
			entropy -= p * logBase2(p)
		}
	}

	return entropy
}

func logBase2(x float64) float64 {
	if x <= 0 {
		return 0
	}
	const ln2 = 0.6931471805599453
	return nativeLog(x) / ln2
}

func nativeLog(x float64) float64 {
	// Simple approximation for log
	if x <= 0 {
		return 0
	}
	return float64(int(1e9 * (x - 1) / (x + 1)))
}

var ransomNotePattern = regexp.MustCompile(`(?i)(decrypt|bitcoin|ransom|payment|encrypted|\.onion)`)

// CheckRansomNoteContent checks if file content looks like a ransom note
func CheckRansomNoteContent(r io.Reader) bool {
	data, err := io.ReadAll(io.LimitReader(r, 10240))
	if err != nil {
		return false
	}
	content := string(data)
	matches := ransomNotePattern.FindAllString(content, -1)
	return len(matches) >= 3
}

// FileMonitor monitors file system for changes
type FileMonitor struct {
	paths    []string
	excludes []string
	events   chan DetectorFileEvent
	running  bool
	mu       sync.Mutex
}

// NewFileMonitor creates a new file monitor
func NewFileMonitor(paths, excludes []string) *FileMonitor {
	return &FileMonitor{
		paths:    paths,
		excludes: excludes,
		events:   make(chan DetectorFileEvent, 1000),
	}
}

// Start begins monitoring
func (m *FileMonitor) Start(ctx context.Context) (<-chan DetectorFileEvent, error) {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop(ctx)

	return m.events, nil
}

// Stop stops monitoring
func (m *FileMonitor) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	close(m.events)
}

// monitorLoop is the main monitoring loop
func (m *FileMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Scan for changes (simplified)
		}
	}
}

// isExcluded checks if a path is excluded
func (m *FileMonitor) isExcluded(path string) bool {
	for _, exclude := range m.excludes {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}
	return false
}

// BehaviorAnalyzer analyzes file events for ransomware patterns
type BehaviorAnalyzer struct {
	threshold int
	window    time.Duration
	events    []DetectorFileEvent
	mu        sync.Mutex
}

// NewBehaviorAnalyzer creates a new behavior analyzer
func NewBehaviorAnalyzer(threshold int, window time.Duration) *BehaviorAnalyzer {
	return &BehaviorAnalyzer{
		threshold: threshold,
		window:    window,
		events:    make([]DetectorFileEvent, 0),
	}
}

// Analyze analyzes events and returns alerts if ransomware activity detected
func (ba *BehaviorAnalyzer) Analyze(event DetectorFileEvent) *DetectorAlert {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	ba.events = append(ba.events, event)

	cutoff := time.Now().Add(-ba.window)
	newEvents := make([]DetectorFileEvent, 0)
	for _, e := range ba.events {
		if e.Timestamp.After(cutoff) {
			newEvents = append(newEvents, e)
		}
	}
	ba.events = newEvents

	suspiciousCount := 0
	suspiciousEvents := make([]DetectorFileEvent, 0)
	for _, e := range ba.events {
		if e.Suspicious {
			suspiciousCount++
			suspiciousEvents = append(suspiciousEvents, e)
		}
	}

	if suspiciousCount >= ba.threshold {
		riskScore := ba.calculateRiskScore(suspiciousEvents)
		return &DetectorAlert{
			ID:              generateID(),
			Type:            ba.getAlertType(riskScore),
			Title:           "Possible Ransomware Activity Detected",
			Message:         fmt.Sprintf("%d suspicious file operations detected in the last %v", suspiciousCount, ba.window),
			Events:          suspiciousEvents,
			RiskScore:       riskScore,
			ThreatType:      ba.identifyThreatType(suspiciousEvents),
			Recommendations: ba.getRecommendations(riskScore),
			Timestamp:       time.Now(),
		}
	}

	return nil
}

// calculateRiskScore calculates a risk score (0-100)
func (ba *BehaviorAnalyzer) calculateRiskScore(events []DetectorFileEvent) int {
	if len(events) == 0 {
		return 0
	}

	score := 0

	for _, e := range events {
		if strings.HasPrefix(e.Reason, "Ransomware extension") {
			score += 30
		}
		if e.Type == "modify" {
			score += 5
		}
		if e.Type == "delete" {
			score += 10
		}
		if strings.Contains(e.Reason, "ransom note") {
			score += 25
		}
	}

	if score > 100 {
		score = 100
	}

	return score
}

// getAlertType returns alert type based on risk score
func (ba *BehaviorAnalyzer) getAlertType(score int) string {
	if score >= 70 {
		return "emergency"
	} else if score >= 40 {
		return "critical"
	}
	return "warning"
}

// identifyThreatType identifies the likely threat type
func (ba *BehaviorAnalyzer) identifyThreatType(events []DetectorFileEvent) string {
	for _, e := range events {
		if strings.Contains(e.Reason, "Ransomware extension") {
			return "encryption_ransomware"
		}
		if strings.Contains(e.Reason, "ransom note") {
			return "ransomware_infection"
		}
		if e.Type == "delete" {
			return "destructive_attack"
		}
	}
	return "suspicious_activity"
}

// getRecommendations returns recommended actions
func (ba *BehaviorAnalyzer) getRecommendations(score int) []string {
	recommendations := []string{
		"Review suspicious file events immediately",
		"Check for unauthorized processes or users",
		"Consider isolating affected shares",
	}

	if score >= 70 {
		recommendations = append(recommendations,
			"Disconnect from network immediately",
			"Restore from known good backup",
			"Contact security team",
		)
	} else if score >= 40 {
		recommendations = append(recommendations,
			"Enable read-only mode for affected shares",
			"Review recent user activity",
		)
	}

	return recommendations
}

// Helper functions

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func generateID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}