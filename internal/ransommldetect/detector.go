// Package ransommldetect provides ML-based ransomware detection with
// entropy analysis, write-frequency anomaly detection, batch extension-change
// detection, YARA rule matching interface, and automated incident response.
//
// Inspired by TrueNAS 26 ransomware detection capabilities.
package ransommldetect

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
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

// ============================================================
// Threat levels
// ============================================================

// ThreatLevel represents the severity of a detected threat.
type ThreatLevel int

const (
	ThreatLevelLow      ThreatLevel = iota + 1 // Low: suspicious but unconfirmed
	ThreatLevelMedium                          // Medium: likely malicious activity
	ThreatLevelHigh                            // High: confirmed attack in progress
	ThreatLevelCritical                        // Critical: active encryption / data loss imminent
)

// String returns a human-readable threat level.
func (l ThreatLevel) String() string {
	switch l {
	case ThreatLevelCritical:
		return "critical"
	case ThreatLevelHigh:
		return "high"
	case ThreatLevelMedium:
		return "medium"
	case ThreatLevelLow:
		return "low"
	default:
		return "unknown"
	}
}

// MarshalText implements encoding.TextMarshaler.
func (l ThreatLevel) MarshalText() ([]byte, error) {
	return []byte(l.String()), nil
}

// ThreatLevelFromString parses a threat level from its string representation.
func ThreatLevelFromString(s string) ThreatLevel {
	switch strings.ToLower(s) {
	case "critical":
		return ThreatLevelCritical
	case "high":
		return ThreatLevelHigh
	case "medium":
		return ThreatLevelMedium
	case "low":
		return ThreatLevelLow
	default:
		return ThreatLevelLow
	}
}

// ============================================================
// Feature extraction – file entropy
// ============================================================

// EntropyResult holds the result of an entropy analysis on a single file.
type EntropyResult struct {
	Path      string    `json:"path"`
	Entropy   float64   `json:"entropy"` // Shannon entropy 0-8
	IsHigh    bool      `json:"is_high"` // entropy > threshold
	FileSize  int64     `json:"file_size"`
	SampledAt time.Time `json:"sampled_at"`
}

// CalculateEntropy computes Shannon entropy for a byte slice.
// Returns a value in [0, 8] where 8 indicates maximum randomness (encryption).
func CalculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	freq := make(map[byte]int, 256)
	for _, b := range data {
		freq[b]++
	}
	total := float64(len(data))
	var entropy float64
	for _, count := range freq {
		if count == 0 {
			continue
		}
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// ExtractEntropyFeatures analyses a batch of file byte-samples and returns
// entropy results for each.  sampleSize caps per-file bytes read (0 = all).
func ExtractEntropyFeatures(samples map[string][]byte, threshold float64, sampleSize int) []EntropyResult {
	results := make([]EntropyResult, 0, len(samples))
	for path, data := range samples {
		if sampleSize > 0 && len(data) > sampleSize {
			data = data[:sampleSize]
		}
		e := CalculateEntropy(data)
		results = append(results, EntropyResult{
			Path:      path,
			Entropy:   e,
			IsHigh:    e >= threshold,
			FileSize:  int64(len(data)),
			SampledAt: time.Now(),
		})
	}
	return results
}

// ============================================================
// Feature extraction – write-frequency anomaly
// ============================================================

// WriteFrequencyStats holds per-directory write frequency statistics.
type WriteFrequencyStats struct {
	Directory    string  `json:"directory"`
	OpsPerMinute float64 `json:"ops_per_minute"`
	Baseline     float64 `json:"baseline"`  // historical average ops/min
	Deviation    float64 `json:"deviation"` // (current - baseline) / baseline
	IsAnomaly    bool    `json:"is_anomaly"`
}

// WriteFrequencyAnalyzer tracks write rates per directory and detects spikes.
type WriteFrequencyAnalyzer struct {
	mu       sync.RWMutex
	windows  map[string][]time.Time // directory -> list of recent op timestamps
	baseline map[string]float64     // directory -> historical ops/min baseline
	window   time.Duration          // sliding window size
	maxDev   float64                // deviation ratio threshold for anomaly
}

// NewWriteFrequencyAnalyzer creates a new analyzer.
// window controls the sliding-window duration; maxDeviation is the
// (current/baseline) ratio above which activity is flagged anomalous.
func NewWriteFrequencyAnalyzer(window time.Duration, maxDeviation float64) *WriteFrequencyAnalyzer {
	return &WriteFrequencyAnalyzer{
		windows:  make(map[string][]time.Time),
		baseline: make(map[string]float64),
		window:   window,
		maxDev:   maxDeviation,
	}
}

// Record records a file operation in the given directory.
func (a *WriteFrequencyAnalyzer) Record(directory string, t time.Time) {
	a.mu.Lock()
	a.windows[directory] = append(a.windows[directory], t)
	a.mu.Unlock()
}

// SetBaseline sets the historical baseline ops/min for a directory.
func (a *WriteFrequencyAnalyzer) SetBaseline(directory string, opsPerMin float64) {
	a.mu.Lock()
	a.baseline[directory] = opsPerMin
	a.mu.Unlock()
}

// Analyze computes current write-frequency stats and prunes old timestamps.
func (a *WriteFrequencyAnalyzer) Analyze() []WriteFrequencyStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-a.window)
	result := make([]WriteFrequencyStats, 0, len(a.windows))

	for dir, times := range a.windows {
		// prune old entries
		pruned := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				pruned = append(pruned, t)
			}
		}
		a.windows[dir] = pruned

		currentRate := float64(len(pruned)) / a.window.Minutes()
		baseline := a.baseline[dir]
		var deviation float64
		if baseline > 0 {
			deviation = (currentRate - baseline) / baseline
		} else if currentRate > 0 {
			deviation = currentRate // no baseline, any activity is deviation
		}

		isAnomaly := deviation > a.maxDev

		result = append(result, WriteFrequencyStats{
			Directory:    dir,
			OpsPerMinute: currentRate,
			Baseline:     baseline,
			Deviation:    deviation,
			IsAnomaly:    isAnomaly,
		})
	}
	return result
}

// ============================================================
// Feature extraction – batch extension-change detection
// ============================================================

// ExtensionChangeEvent represents a single file rename / extension change.
type ExtensionChangeEvent struct {
	Path     string    `json:"path"`
	OldExt   string    `json:"old_ext"`
	NewExt   string    `json:"new_ext"`
	Process  string    `json:"process"`
	UnixTime int64     `json:"unix_time"`
	Time     time.Time `json:"time"`
}

// ExtensionChangeResult holds the analysis result for batch extension changes.
type ExtensionChangeResult struct {
	NewExtension string                 `json:"new_extension"`
	Count        int                    `json:"count"`
	Events       []ExtensionChangeEvent `json:"events"`
	IsSuspicious bool                   `json:"is_suspicious"`
	Confidence   float64                `json:"confidence"`
}

// KnownRansomwareExtensions is a list of extensions commonly used by ransomware.
var KnownRansomwareExtensions = map[string]bool{
	".encrypted": true, ".locked": true, ".crypto": true, ".crypt": true,
	".locky": true, ".cerber": true, ".wncry": true, ".wanna": true,
	".petya": true, ".ryuk": true, ".maze": true, ".sage": true,
	".globe": true, ".dharma": true, ".phobos": true, ".harma": true,
	".stop": true, ".nmore": true, ".vesad": true, ".robe": true,
	".enc": true, ".cry": true, ".zzzzz": true, ".abc": true,
	".xyz": true, ".lockedby": true, ".decrypt": true,
}

// ExtensionChangeDetector detects batch file-extension changes.
type ExtensionChangeDetector struct {
	mu        sync.RWMutex
	events    []ExtensionChangeEvent
	window    time.Duration
	threshold int // number of same-new-ext events in window to trigger
}

// NewExtensionChangeDetector creates a new detector.
func NewExtensionChangeDetector(window time.Duration, threshold int) *ExtensionChangeDetector {
	return &ExtensionChangeDetector{
		events:    make([]ExtensionChangeEvent, 0, 256),
		window:    window,
		threshold: threshold,
	}
}

// Record records an extension change event.
func (d *ExtensionChangeDetector) Record(ev ExtensionChangeEvent) {
	d.mu.Lock()
	d.events = append(d.events, ev)
	d.mu.Unlock()
}

// Analyze checks the current window for suspicious batch extension changes.
func (d *ExtensionChangeDetector) Analyze() []ExtensionChangeResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-d.window)

	// prune old events and group by new extension
	pruned := d.events[:0]
	grouped := make(map[string][]ExtensionChangeEvent)

	for _, ev := range d.events {
		if ev.Time.After(cutoff) {
			pruned = append(pruned, ev)
			grouped[ev.NewExt] = append(grouped[ev.NewExt], ev)
		}
	}
	d.events = pruned

	var results []ExtensionChangeResult
	for newExt, events := range grouped {
		isSuspicious := len(events) >= d.threshold || KnownRansomwareExtensions[strings.ToLower(newExt)]
		confidence := 0.0
		if KnownRansomwareExtensions[strings.ToLower(newExt)] {
			confidence = 0.9
		} else if len(events) >= d.threshold {
			confidence = math.Min(1.0, float64(len(events))/float64(d.threshold*2))
		}
		if isSuspicious || confidence > 0.3 {
			results = append(results, ExtensionChangeResult{
				NewExtension: newExt,
				Count:        len(events),
				Events:       events,
				IsSuspicious: isSuspicious,
				Confidence:   confidence,
			})
		}
	}
	return results
}

// ============================================================
// YARA rule matching interface
// ============================================================

// YARAMatch represents a single YARA rule match result.
type YARAMatch struct {
	RuleName  string            `json:"rule_name"`
	Namespace string            `json:"namespace,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Metas     map[string]string `json:"metas,omitempty"`
	FilePath  string            `json:"file_path"`
}

// YARAScanner is the interface that YARA rule engines must implement.
// Implementations may wrap an actual libyara binding or a simulated engine.
type YARAScanner interface {
	// ScanFile scans a single file against loaded YARA rules.
	ScanFile(filePath string) ([]YARAMatch, error)
	// ScanDir recursively scans a directory.
	ScanDir(dirPath string) ([]YARAMatch, error)
	// AddRule adds a YARA rule from source text.
	AddRule(ruleSource string) error
	// LoadRulesFile loads YARA rules from a file path.
	LoadRulesFile(path string) error
}

// NoOpYARAScanner is a placeholder implementation that always returns no matches.
// Replace with a real YARA binding in production.
type NoOpYARAScanner struct{}

func (n *NoOpYARAScanner) ScanFile(_ string) ([]YARAMatch, error) { return nil, nil }
func (n *NoOpYARAScanner) ScanDir(_ string) ([]YARAMatch, error)  { return nil, nil }
func (n *NoOpYARAScanner) AddRule(_ string) error                 { return nil }
func (n *NoOpYARAScanner) LoadRulesFile(_ string) error           { return nil }

// ============================================================
// MLModel – feature extraction orchestrator
// ============================================================

// FeatureVector is the combined feature set produced by MLModel for a window.
type FeatureVector struct {
	Timestamp         time.Time            `json:"timestamp"`
	HighEntropyCount  int                  `json:"high_entropy_count"`
	HighEntropyRatio  float64              `json:"high_entropy_ratio"` // high-entropy files / total scanned
	WriteAnomalyCount int                  `json:"write_anomaly_count"`
	TopWriteAnomaly   *WriteFrequencyStats `json:"top_write_anomaly,omitempty"`
	ExtChangeCount    int                  `json:"ext_change_count"`
	SuspiciousExts    []string             `json:"suspicious_exts"`
	YARAMatchCount    int                  `json:"yara_match_count"`
	YARAMatches       []YARAMatch          `json:"yara_matches,omitempty"`
}

// MLModel orchestrates feature extraction and prediction.
type MLModel struct {
	mu             sync.RWMutex
	logger         *zap.Logger
	entropyThresh  float64
	sampleSize     int
	freqAnalyzer   *WriteFrequencyAnalyzer
	extDetector    *ExtensionChangeDetector
	yaraScanner    YARAScanner
	entropySamples map[string][]byte // path -> recent bytes snapshot
}

// MLModelConfig configures the MLModel.
type MLModelConfig struct {
	EntropyThreshold  float64       `json:"entropy_threshold"`
	EntropySampleSize int           `json:"entropy_sample_size"` // bytes to sample per file (0=all)
	FreqWindow        time.Duration `json:"freq_window"`
	FreqMaxDeviation  float64       `json:"freq_max_deviation"`
	ExtWindow         time.Duration `json:"ext_window"`
	ExtThreshold      int           `json:"ext_threshold"`
}

// DefaultMLModelConfig returns sensible defaults.
func DefaultMLModelConfig() MLModelConfig {
	return MLModelConfig{
		EntropyThreshold:  7.5,
		EntropySampleSize: 65536, // 64 KB
		FreqWindow:        5 * time.Minute,
		FreqMaxDeviation:  3.0, // 3× baseline = anomaly
		ExtWindow:         5 * time.Minute,
		ExtThreshold:      10,
	}
}

// NewMLModel creates a new ML model with the given config.
func NewMLModel(cfg MLModelConfig, logger *zap.Logger) *MLModel {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MLModel{
		logger:         logger,
		entropyThresh:  cfg.EntropyThreshold,
		sampleSize:     cfg.EntropySampleSize,
		freqAnalyzer:   NewWriteFrequencyAnalyzer(cfg.FreqWindow, cfg.FreqMaxDeviation),
		extDetector:    NewExtensionChangeDetector(cfg.ExtWindow, cfg.ExtThreshold),
		yaraScanner:    &NoOpYARAScanner{},
		entropySamples: make(map[string][]byte),
	}
}

// SetYARAScanner sets the YARA rule scanner implementation.
func (m *MLModel) SetYARAScanner(scanner YARAScanner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.yaraScanner = scanner
}

// FeedEntropySample feeds a raw byte sample for a file path.
func (m *MLModel) FeedEntropySample(path string, data []byte) {
	m.mu.Lock()
	m.entropySamples[path] = data
	m.mu.Unlock()
}

// RecordFileActivity records a file activity event for frequency analysis.
func (m *MLModel) RecordFileActivity(act FileActivity) {
	dir := getDir(act.Path)
	m.freqAnalyzer.Record(dir, act.Timestamp)
}

// RecordExtensionChange records an extension change event.
func (m *MLModel) RecordExtensionChange(ev ExtensionChangeEvent) {
	m.extDetector.Record(ev)
}

// ExtractFeatures computes the current feature vector from all sources.
func (m *MLModel) ExtractFeatures() FeatureVector {
	m.mu.RLock()
	samples := m.entropySamples
	scanner := m.yaraScanner
	m.mu.RUnlock()

	// Entropy features
	entropyResults := ExtractEntropyFeatures(samples, m.entropyThresh, m.sampleSize)
	highCount := 0
	for _, r := range entropyResults {
		if r.IsHigh {
			highCount++
		}
	}
	highRatio := 0.0
	if len(entropyResults) > 0 {
		highRatio = float64(highCount) / float64(len(entropyResults))
	}

	// Write-frequency features
	freqStats := m.freqAnalyzer.Analyze()
	anomalyCount := 0
	var topAnomaly *WriteFrequencyStats
	for i := range freqStats {
		if freqStats[i].IsAnomaly {
			anomalyCount++
			if topAnomaly == nil || freqStats[i].Deviation > topAnomaly.Deviation {
				topAnomaly = &freqStats[i]
			}
		}
	}

	// Extension-change features
	extResults := m.extDetector.Analyze()
	extChangeCount := 0
	var suspiciousExts []string
	for _, r := range extResults {
		if r.IsSuspicious {
			extChangeCount += r.Count
			suspiciousExts = append(suspiciousExts, r.NewExtension)
		}
	}

	// YARA matches (run on high-entropy files only to save resources)
	var yaraMatches []YARAMatch
	for _, r := range entropyResults {
		if r.IsHigh {
			matches, err := scanner.ScanFile(r.Path)
			if err != nil {
				m.logger.Warn("YARA scan failed", zap.String("path", r.Path), zap.Error(err))
				continue
			}
			yaraMatches = append(yaraMatches, matches...)
		}
	}

	return FeatureVector{
		Timestamp:         time.Now(),
		HighEntropyCount:  highCount,
		HighEntropyRatio:  highRatio,
		WriteAnomalyCount: anomalyCount,
		TopWriteAnomaly:   topAnomaly,
		ExtChangeCount:    extChangeCount,
		SuspiciousExts:    suspiciousExts,
		YARAMatchCount:    len(yaraMatches),
		YARAMatches:       yaraMatches,
	}
}

// ============================================================
// RansomwarePredictor – weighted scoring
// ============================================================

// Prediction represents the output of the ransomware predictor.
type Prediction struct {
	ThreatLevel ThreatLevel   `json:"threat_level"`
	Score       float64       `json:"score"`      // 0–1 composite score
	Confidence  float64       `json:"confidence"` // 0–1 how confident
	Reason      string        `json:"reason"`
	FeatureVec  FeatureVector `json:"feature_vector"`
}

// PredictorWeights controls the relative weight of each feature in the score.
type PredictorWeights struct {
	EntropyWeight   float64 `json:"entropy_weight"`
	FreqWeight      float64 `json:"freq_weight"`
	ExtChangeWeight float64 `json:"ext_change_weight"`
	YARAWeight      float64 `json:"yara_weight"`
}

// DefaultPredictorWeights returns default weights (sum = 1.0).
func DefaultPredictorWeights() PredictorWeights {
	return PredictorWeights{
		EntropyWeight:   0.30,
		FreqWeight:      0.25,
		ExtChangeWeight: 0.25,
		YARAWeight:      0.20,
	}
}

// RansomwarePredictor predicts the likelihood of an ongoing ransomware attack
// using weighted statistical scoring (no external ML library required).
type RansomwarePredictor struct {
	weights PredictorWeights
	logger  *zap.Logger
}

// NewRansomwarePredictor creates a new predictor.
func NewRansomwarePredictor(weights PredictorWeights, logger *zap.Logger) *RansomwarePredictor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RansomwarePredictor{weights: weights, logger: logger}
}

// Predict evaluates a feature vector and returns a prediction.
func (p *RansomwarePredictor) Predict(fv FeatureVector) Prediction {
	// Sub-scores in [0, 1]
	entropyScore := fv.HighEntropyRatio // ratio of high-entropy files

	freqScore := 0.0
	if fv.TopWriteAnomaly != nil && fv.TopWriteAnomaly.Deviation > 0 {
		freqScore = math.Min(1.0, fv.TopWriteAnomaly.Deviation/10.0)
	}

	extScore := 0.0
	if fv.ExtChangeCount > 0 {
		extScore = math.Min(1.0, float64(fv.ExtChangeCount)/100.0)
		// Boost if known ransomware extensions
		for _, ext := range fv.SuspiciousExts {
			if KnownRansomwareExtensions[strings.ToLower(ext)] {
				extScore = math.Min(1.0, extScore+0.3)
				break
			}
		}
	}

	yaraScore := 0.0
	if fv.YARAMatchCount > 0 {
		yaraScore = math.Min(1.0, float64(fv.YARAMatchCount)/5.0)
	}

	composite := p.weights.EntropyWeight*entropyScore +
		p.weights.FreqWeight*freqScore +
		p.weights.ExtChangeWeight*extScore +
		p.weights.YARAWeight*yaraScore
	composite = math.Min(1.0, composite)

	// Confidence: more signals → higher confidence
	signals := 0
	if entropyScore > 0.3 {
		signals++
	}
	if freqScore > 0.3 {
		signals++
	}
	if extScore > 0.3 {
		signals++
	}
	if yaraScore > 0.1 {
		signals++
	}
	confidence := float64(signals) / 4.0

	var level ThreatLevel
	switch {
	case composite >= 0.9:
		level = ThreatLevelCritical
	case composite >= 0.7:
		level = ThreatLevelHigh
	case composite >= 0.4:
		level = ThreatLevelMedium
	default:
		level = ThreatLevelLow
	}

	reason := fmt.Sprintf(
		"entropy=%.2f freq=%.2f ext=%.2f yara=%.2f → composite=%.3f signals=%d",
		entropyScore, freqScore, extScore, yaraScore, composite, signals,
	)

	return Prediction{
		ThreatLevel: level,
		Score:       composite,
		Confidence:  confidence,
		Reason:      reason,
		FeatureVec:  fv,
	}
}

// ============================================================
// Enhanced Detector
// ============================================================

// ThreatStatus represents the current overall threat status.
type ThreatStatus struct {
	Level       ThreatLevel `json:"level"`
	Score       float64     `json:"score"`
	LastAlert   *Alert      `json:"last_alert,omitempty"`
	ActiveSince *time.Time  `json:"active_since,omitempty"`
	EventsCount int         `json:"events_count"`
	Uptime      int64       `json:"uptime"`
	Running     bool        `json:"running"`
}

// DetectorConfig holds enhanced detector configuration.
type DetectorConfig struct {
	WindowSizeSec    int     `json:"window_size_sec"`
	EntropyThreshold float64 `json:"entropy_threshold"`
	RateThreshold    int     `json:"rate_threshold"`
	BlockThreshold   float64 `json:"block_threshold"`
	Enabled          bool    `json:"enabled"`

	// Extended config
	MLModelConfig    MLModelConfig    `json:"ml_model_config"`
	PredictorWeights PredictorWeights `json:"predictor_weights"`
}

// DefaultDetectorConfig returns default config.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		WindowSizeSec:    60,
		EntropyThreshold: 7.5,
		RateThreshold:    100,
		BlockThreshold:   0.85,
		Enabled:          true,
		MLModelConfig:    DefaultMLModelConfig(),
		PredictorWeights: DefaultPredictorWeights(),
	}
}

// Detector provides ML-based ransomware detection.
type Detector struct {
	mu         sync.RWMutex
	config     DetectorConfig
	activities []FileActivity
	alerts     []Alert
	stopCh     chan struct{}
	running    bool
	startTime  time.Time

	// Enhanced components
	logger    *zap.Logger
	mlModel   *MLModel
	predictor *RansomwarePredictor

	// Event history for threat status queries
	threatEvents []ThreatEventRecord
}

// ThreatEventRecord is a persisted threat event.
type ThreatEventRecord struct {
	ID          string      `json:"id"`
	ThreatLevel ThreatLevel `json:"threat_level"`
	Score       float64     `json:"score"`
	Confidence  float64     `json:"confidence"`
	Reason      string      `json:"reason"`
	Source      string      `json:"source"`
	Pattern     string      `json:"pattern"`
	Blocked     bool        `json:"blocked"`
	CreatedAt   time.Time   `json:"created_at"`
}

// NewDetector creates a new ransomware detector.
func NewDetector(config DetectorConfig, logger *zap.Logger) *Detector {
	if logger == nil {
		logger = zap.NewNop()
	}
	mlModel := NewMLModel(config.MLModelConfig, logger)
	predictor := NewRansomwarePredictor(config.PredictorWeights, logger)
	return &Detector{
		config:       config,
		stopCh:       make(chan struct{}),
		logger:       logger,
		mlModel:      mlModel,
		predictor:    predictor,
		threatEvents: make([]ThreatEventRecord, 0, 1024),
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
	d.startTime = time.Now()
	d.mu.Unlock()

	go d.analysisLoop()
	d.logger.Info("ML勒索检测引擎已启动")
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
	d.logger.Info("ML勒索检测引擎已停止")
}

// RecordActivity records a file activity for analysis.
func (d *Detector) RecordActivity(activity FileActivity) {
	d.mu.Lock()
	d.activities = append(d.activities, activity)
	d.mu.Unlock()

	// Feed into ML model components
	d.mlModel.RecordFileActivity(activity)
	if activity.Operation == "rename" {
		// Try to detect extension change from the path
		oldExt := ""
		newExt := getExtension(activity.Path)
		if oldExt != newExt && newExt != "" {
			d.mlModel.RecordExtensionChange(ExtensionChangeEvent{
				Path:    activity.Path,
				OldExt:  oldExt,
				NewExt:  newExt,
				Process: activity.Process,
				Time:    activity.Timestamp,
			})
		}
	}
}

// FeedEntropySample feeds raw bytes for entropy analysis on a file.
func (d *Detector) FeedEntropySample(path string, data []byte) {
	d.mlModel.FeedEntropySample(path, data)
}

// GetAlerts returns all detected alerts.
func (d *Detector) GetAlerts() []Alert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]Alert, len(d.alerts))
	copy(result, d.alerts)
	return result
}

// GetThreatEvents returns threat event history (paginated).
func (d *Detector) GetThreatEvents(page, perPage int) []ThreatEventRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()

	total := len(d.threatEvents)
	start := (page - 1) * perPage
	if start >= total {
		return nil
	}
	end := start + perPage
	if end > total {
		end = total
	}
	result := make([]ThreatEventRecord, end-start)
	copy(result, d.threatEvents[start:end])
	return result
}

// GetThreatStatus returns the current overall threat status.
func (d *Detector) GetThreatStatus() ThreatStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	status := ThreatStatus{
		Level:       ThreatLevelLow,
		Running:     d.running,
		EventsCount: len(d.threatEvents),
	}

	if d.running {
		status.Uptime = int64(time.Since(d.startTime).Seconds())
	}

	if len(d.alerts) > 0 {
		last := d.alerts[len(d.alerts)-1]
		status.LastAlert = &last
		status.Score = last.Score
		status.Level = ThreatLevelFromString(last.Severity)
		if status.Level >= ThreatLevelHigh {
			ts := last.Timestamp
			status.ActiveSince = &ts
		}
	}

	return status
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

// MLModel returns the underlying ML model for direct interaction.
func (d *Detector) MLModel() *MLModel {
	return d.mlModel
}

// Predictor returns the underlying predictor.
func (d *Detector) Predictor() *RansomwarePredictor {
	return d.predictor
}

// ScanNow triggers an immediate scan and returns the prediction.
func (d *Detector) ScanNow() Prediction {
	fv := d.mlModel.ExtractFeatures()
	pred := d.predictor.Predict(fv)

	if pred.Score >= d.config.BlockThreshold {
		alert := Alert{
			ID:        fmt.Sprintf("ransom-%d", time.Now().UnixNano()),
			Severity:  pred.ThreatLevel.String(),
			Source:    "ml_predictor",
			Pattern:   pred.Reason,
			Score:     pred.Score,
			Timestamp: time.Now(),
			Blocked:   true,
		}
		d.mu.Lock()
		d.alerts = append(d.alerts, alert)
		d.threatEvents = append(d.threatEvents, ThreatEventRecord{
			ID:          alert.ID,
			ThreatLevel: pred.ThreatLevel,
			Score:       pred.Score,
			Confidence:  pred.Confidence,
			Reason:      pred.Reason,
			Source:      "ml_predictor",
			Pattern:     describePatternFromFV(fv),
			Blocked:     true,
			CreatedAt:   time.Now(),
		})
		d.mu.Unlock()
		d.logger.Warn("勒索检测告警",
			zap.Float64("score", pred.Score),
			zap.String("threat_level", pred.ThreatLevel.String()),
			zap.String("reason", pred.Reason),
		)
	}
	return pred
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
	cutoff := time.Now().Add(-time.Duration(d.config.WindowSizeSec) * time.Second)
	var window []FileActivity
	var remaining []FileActivity
	for _, a := range d.activities {
		if a.Timestamp.After(cutoff) {
			window = append(window, a)
		}
		if a.Timestamp.After(time.Now().Add(-5 * time.Minute)) {
			remaining = append(remaining, a)
		}
	}
	d.activities = remaining
	d.mu.Unlock()

	if len(window) == 0 {
		return
	}

	// Legacy scoring
	score := d.calculateRiskScore(window)

	// ML prediction
	pred := d.ScanNow()

	// Take the higher of legacy and ML scores
	finalScore := math.Max(score, pred.Score)

	if finalScore >= d.config.BlockThreshold {
		alert := Alert{
			ID:        fmt.Sprintf("ransom-%d", time.Now().UnixNano()),
			Severity:  severityFromScore(finalScore),
			Source:    mostActivePath(window),
			Pattern:   describePattern(window),
			Score:     finalScore,
			Timestamp: time.Now(),
			Blocked:   true,
		}

		d.mu.Lock()
		d.alerts = append(d.alerts, alert)
		d.mu.Unlock()

		d.logger.Warn("勒索检测告警",
			zap.Float64("score", finalScore),
			zap.String("pattern", alert.Pattern),
			zap.Bool("blocked", true),
		)
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

	// Factor 4: Known ransomware extensions
	extensions := make(map[string]int)
	for _, a := range activities {
		ext := getExtension(a.Path)
		extensions[ext]++
	}
	if extensions[".encrypted"] > 0 || extensions[".locked"] > 0 || extensions[".crypto"] > 0 {
		score += 0.4
	}

	return math.Min(1.0, score)
}

func describePatternFromFV(fv FeatureVector) string {
	parts := []string{}
	if fv.HighEntropyCount > 0 {
		parts = append(parts, fmt.Sprintf("high_entropy=%d", fv.HighEntropyCount))
	}
	if fv.WriteAnomalyCount > 0 {
		parts = append(parts, fmt.Sprintf("write_anomaly=%d", fv.WriteAnomalyCount))
	}
	if fv.ExtChangeCount > 0 {
		parts = append(parts, fmt.Sprintf("ext_change=%d", fv.ExtChangeCount))
	}
	if fv.YARAMatchCount > 0 {
		parts = append(parts, fmt.Sprintf("yara_match=%d", fv.YARAMatchCount))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

// ============================================================
// Shared helpers (package-level)
// ============================================================

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

// ext returns the file extension (lower-cased) using filepath.Ext.
func ext(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return "."
}
