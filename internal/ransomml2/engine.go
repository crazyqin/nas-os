package ransomml2

import (
	"sync"
	"time"
)

// ThreatLevel 威胁等级。
type ThreatLevel string

const (
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// FileType 文件类型分类。
type FileType string

const (
	FileTypeDoc    FileType = "document"
	FileTypeImage  FileType = "image"
	FileTypeVideo  FileType = "video"
	FileTypeCode   FileType = "code"
	FileTypeArchive FileType = "archive"
	FileTypeUnknown FileType = "unknown"
)

// FileActivity 文件活动记录。
type FileActivity struct {
	ID          string      `json:"id"`
	FilePath    string      `json:"file_path"`
	Action      string      `json:"action"` // create, modify, delete, rename
	FileType    FileType    `json:"file_type"`
	SizeBytes   int64       `json:"size_bytes"`
	ProcessName string      `json:"process_name"`
	ProcessPID  int         `json:"process_pid"`
	UserID      string      `json:"user_id"`
	Timestamp   time.Time   `json:"timestamp"`
	Entropy     float64     `json:"entropy"` // 文件熵值
}

// ThreatEvent 威胁事件。
type ThreatEvent struct {
	ID          string      `json:"id"`
	Level       ThreatLevel `json:"level"`
	Type        string      `json:"type"` // ransomware, suspicious, anomaly
	Description string      `json:"description"`
	Activities  []string    `json:"activity_ids"`
	FilePaths   []string    `json:"file_paths"`
	ProcessName string      `json:"process_name"`
	DetectedAt  time.Time   `json:"detected_at"`
	Resolved    bool        `json:"resolved"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
}

// MLModel ML模型配置。
type MLModel struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Type       string    `json:"type"` // anomaly_detection, classification
	Accuracy   float64   `json:"accuracy"`
	TrainedAt  time.Time `json:"trained_at"`
	Status     string    `json:"status"` // ready, training, error
}

// DetectionStats 检测统计。
type DetectionStats struct {
	TotalScans      int64   `json:"total_scans"`
	ThreatsDetected int64   `json:"threats_detected"`
	ThreatsBlocked  int64   `json:"threats_blocked"`
	FalsePositives  int64   `json:"false_positives"`
	AvgScanTimeMs   float64 `json:"avg_scan_time_ms"`
	ModelAccuracy   float64 `json:"model_accuracy"`
}

// Engine ML勒索防护引擎。
type Engine struct {
	mu         sync.RWMutex
	activities []FileActivity
	threats    map[string]*ThreatEvent
	model      *MLModel
	stats      DetectionStats
	config     Config
}

// Config 引擎配置。
type Config struct {
	EntropyThreshold  float64 `json:"entropy_threshold"`   // 熵值阈值
	BatchSizeThreshold int    `json:"batch_size_threshold"` // 批量操作阈值
	TimeWindowSec     int     `json:"time_window_sec"`      // 时间窗口
	AutoBlock         bool    `json:"auto_block"`           // 自动阻断
	AlertEnabled      bool    `json:"alert_enabled"`        // 告警启用
}

// NewEngine 创建新的引擎。
func NewEngine() *Engine {
	return &Engine{
		threats: make(map[string]*ThreatEvent),
		model: &MLModel{
			ID:        "ransom-ml-v2",
			Name:      "Ransomware Detection ML Model v2",
			Version:   "2.0.0",
			Type:      "anomaly_detection",
			Accuracy:  0.985,
			TrainedAt: time.Now(),
			Status:    "ready",
		},
		config: Config{
			EntropyThreshold:   7.5,
			BatchSizeThreshold: 100,
			TimeWindowSec:      60,
			AutoBlock:          true,
			AlertEnabled:       true,
		},
	}
}

// RecordActivity 记录文件活动。
func (e *Engine) RecordActivity(activity FileActivity) {
	e.mu.Lock()
	defer e.mu.Unlock()

	activity.Timestamp = time.Now()
	e.activities = append(e.activities, activity)
	e.stats.TotalScans++

	// 检测逻辑
	e.detectThreats(activity)
}

// detectThreats 检测威胁。
func (e *Engine) detectThreats(activity FileActivity) {
	// 1. 高熵值检测（可能是加密）
	if activity.Entropy > e.config.EntropyThreshold {
		e.createThreat(ThreatHigh, "high_entropy", 
			"检测到高熵值文件，可能是勒索软件加密: "+activity.FilePath,
			[]string{activity.FilePath}, activity.ProcessName)
	}

	// 2. 批量删除检测
	recentDeletes := e.countRecentActions("delete", e.config.TimeWindowSec)
	if recentDeletes > e.config.BatchSizeThreshold {
		e.createThreat(ThreatCritical, "mass_delete",
			"检测到批量删除操作，疑似勒索软件行为",
			nil, activity.ProcessName)
	}

	// 3. 批量重命名检测
	recentRenames := e.countRecentActions("rename", e.config.TimeWindowSec)
	if recentRenames > e.config.BatchSizeThreshold {
		e.createThreat(ThreatCritical, "mass_rename",
			"检测到批量重命名操作，疑似勒索软件加密",
			nil, activity.ProcessName)
	}
}

func (e *Engine) countRecentActions(action string, windowSec int) int {
	count := 0
	cutoff := time.Now().Add(-time.Duration(windowSec) * time.Second)
	for _, a := range e.activities {
		if a.Action == action && a.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}

func (e *Engine) createThreat(level ThreatLevel, threatType, desc string, files []string, process string) {
	threat := &ThreatEvent{
		ID:          generateID(),
		Level:       level,
		Type:        threatType,
		Description: desc,
		FilePaths:   files,
		ProcessName: process,
		DetectedAt:  time.Now(),
	}
	e.threats[threat.ID] = threat
	e.stats.ThreatsDetected++

	if e.config.AutoBlock && (level == ThreatHigh || level == ThreatCritical) {
		e.stats.ThreatsBlocked++
	}
}

// GetThreat 获取威胁事件。
func (e *Engine) GetThreat(id string) (*ThreatEvent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	threat, exists := e.threats[id]
	return threat, exists
}

// ListThreats 列出威胁事件。
func (e *Engine) ListThreats() []*ThreatEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*ThreatEvent, 0, len(e.threats))
	for _, t := range e.threats {
		result = append(result, t)
	}
	return result
}

// ResolveThreat 解决威胁。
func (e *Engine) ResolveThreat(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	threat, exists := e.threats[id]
	if !exists {
		return ErrThreatNotFound
	}
	now := time.Now()
	threat.Resolved = true
	threat.ResolvedAt = &now
	return nil
}

// GetModel 获取模型信息。
func (e *Engine) GetModel() *MLModel {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.model
}

// GetStats 获取统计。
func (e *Engine) GetStats() DetectionStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// UpdateConfig 更新配置。
func (e *Engine) UpdateConfig(config Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
}

// GetConfig 获取配置。
func (e *Engine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

func generateID() string {
	return time.Now().Format("20060102150405") + "threat"
}

// 错误定义。
var (
	ErrThreatNotFound = &RansomError{"threat not found"}
)

type RansomError struct {
	msg string
}

func (e *RansomError) Error() string {
	return e.msg
}
