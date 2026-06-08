package ransomai

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var alertCounter int64

// ThreatLevel 威胁等级
type ThreatLevel int

const (
	ThreatNone     ThreatLevel = 0
	ThreatLow      ThreatLevel = 1
	ThreatMedium   ThreatLevel = 2
	ThreatHigh     ThreatLevel = 3
	ThreatCritical ThreatLevel = 4
)

func (t ThreatLevel) String() string {
	switch t {
	case ThreatLow:
		return "low"
	case ThreatMedium:
		return "medium"
	case ThreatHigh:
		return "high"
	case ThreatCritical:
		return "critical"
	default:
		return "none"
	}
}

// FileEvent 文件事件
type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // create, modify, delete, rename, encrypt
	Size      int64     `json:"size"`
	Entropy   float64   `json:"entropy"` // 信息熵
	Process   string    `json:"process"`
	UserID    string    `json:"userId"`
	Timestamp time.Time `json:"timestamp"`
}

// ThreatAlert 威胁告警
type ThreatAlert struct {
	ID          string      `json:"id"`
	Level       ThreatLevel `json:"level"`
	Type        string      `json:"type"` // entropy_spike, mass_rename, rapid_delete, extension_change, honeypot_trigger
	Description string      `json:"description"`
	Source      string      `json:"source"`
	Events      []FileEvent `json:"events"`
	Score       float64     `json:"score"` // 0-1
	Blocked     bool        `json:"blocked"`
	Actions     []string    `json:"actions"` // 建议操作
	Timestamp   time.Time   `json:"timestamp"`
	Resolved    bool        `json:"resolved"`
}

// HoneypotFile 蜜罐文件
type HoneypotFile struct {
	Path      string    `json:"path"`
	Extension string    `json:"extension"`
	Triggered bool      `json:"triggered"`
	TriggerAt time.Time `json:"triggerAt,omitempty"`
	Source    string    `json:"source"` // 触发来源
}

// RansomAIConfig 配置
type RansomAIConfig struct {
	EntropyThreshold   float64 `json:"entropyThreshold"`   // 信息熵阈值 (>7.5 = 高熵/加密)
	RateThreshold      int     `json:"rateThreshold"`      // 每分钟最大操作数
	MassRenameCount    int     `json:"massRenameCount"`    // 批量重命名阈值
	RapidDeleteCount   int     `json:"rapidDeleteCount"`   // 快速删除阈值
	BlockThreshold     float64 `json:"blockThreshold"`     // 自动阻断阈值
	AutoBlock          bool    `json:"autoBlock"`
	HoneypotEnabled    bool    `json:"honeypotEnabled"`
}

// DefaultConfig 默认配置
func DefaultConfig() RansomAIConfig {
	return RansomAIConfig{
		EntropyThreshold: 7.5,
		RateThreshold:    100,
		MassRenameCount:  50,
		RapidDeleteCount: 30,
		BlockThreshold:   0.85,
		AutoBlock:        true,
		HoneypotEnabled:  true,
	}
}

// RansomAI AI勒索软件检测增强
// 对标 TrueNAS 勒索软件检测 + 飞牛安全能力
type RansomAI struct {
	mu        sync.RWMutex
	config    RansomAIConfig
	events    []FileEvent
	alerts    []ThreatAlert
	honeypots map[string]*HoneypotFile
	window    []FileEvent // 滑动窗口
	stopCh    chan struct{}
	running   bool
}

// NewRansomAI 创建勒索AI检测器
func NewRansomAI() *RansomAI {
	return &RansomAI{
		config:    DefaultConfig(),
		events:    make([]FileEvent, 0),
		alerts:    make([]ThreatAlert, 0),
		honeypots: make(map[string]*HoneypotFile),
		window:    make([]FileEvent, 0),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动
func (r *RansomAI) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()
	go r.analysisLoop()
	log.Println("[RansomAI] AI勒索检测引擎已启动")
}

// Stop 停止
func (r *RansomAI) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	close(r.stopCh)
	r.running = false
}

// RecordEvent 记录文件事件
func (r *RansomAI) RecordEvent(event FileEvent) *ThreatAlert {
	r.mu.Lock()
	defer r.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	r.events = append(r.events, event)
	r.window = append(r.window, event)

	// 清理窗口（保留最近1分钟）
	cutoff := time.Now().Add(-time.Minute)
	filtered := make([]FileEvent, 0)
	for _, e := range r.window {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	r.window = filtered

	// 检查蜜罐
	if r.config.HoneypotEnabled {
		if hp, ok := r.honeypots[event.Path]; ok && !hp.Triggered {
			hp.Triggered = true
			hp.TriggerAt = time.Now()
			hp.Source = event.Process
			alert := r.createAlert(ThreatCritical, "honeypot_trigger",
				"蜜罐文件被触发: "+event.Path, event)
			r.alerts = append(r.alerts, alert)
			return &alert
		}
	}

	// 分析威胁
	threat := r.analyzeThreat(event)
	if threat != nil {
		r.alerts = append(r.alerts, *threat)
		return threat
	}
	return nil
}

// analyzeThreat 分析威胁
func (r *RansomAI) analyzeThreat(event FileEvent) *ThreatAlert {
	// 高熵检测（可能是加密）
	if event.Entropy > r.config.EntropyThreshold {
		score := (event.Entropy - 7.0) / 1.0
		if score > 1.0 {
			score = 1.0
		}
		if score > r.config.BlockThreshold {
			return r.createAlertPtr(ThreatCritical, "entropy_spike",
				"检测到高熵写入（疑似加密）: "+event.Path, event, score)
		}
		return r.createAlertPtr(ThreatHigh, "entropy_spike",
			"检测到高熵数据: "+event.Path, event, score)
	}

	// 批量操作检测
	if len(r.window) > r.config.RateThreshold {
		return r.createAlertPtr(ThreatHigh, "rate_spike",
			"检测到异常高频文件操作", event, 0.8)
	}

	// 批量删除
	deleteCount := 0
	for _, e := range r.window {
		if e.Operation == "delete" {
			deleteCount++
		}
	}
	if deleteCount >= r.config.RapidDeleteCount {
		return r.createAlertPtr(ThreatCritical, "rapid_delete",
			"检测到批量删除操作", event, 0.9)
	}

	// 批量重命名
	renameCount := 0
	for _, e := range r.window {
		if e.Operation == "rename" {
			renameCount++
		}
	}
	if renameCount >= r.config.MassRenameCount {
		return r.createAlertPtr(ThreatHigh, "mass_rename",
			"检测到批量重命名操作", event, 0.75)
	}

	// 扩展名变更检测
	if event.Operation == "rename" && isSuspiciousRename(event.Path) {
		return r.createAlertPtr(ThreatHigh, "extension_change",
			"检测到可疑扩展名变更: "+event.Path, event, 0.7)
	}

	return nil
}

func isSuspiciousRename(path string) bool {
	suspicious := []string{".encrypted", ".locked", ".crypto", ".vault", ".ransom", ".wncry"}
	for _, s := range suspicious {
		if len(path) >= len(s) && path[len(path)-len(s):] == s {
			return true
		}
	}
	return false
}

func (r *RansomAI) createAlert(level ThreatLevel, alertType, desc string, event FileEvent) ThreatAlert {
	return ThreatAlert{
		ID:          fmt.Sprintf("alert-%s-%04d", time.Now().Format("20060102150405.000"), atomic.AddInt64(&alertCounter, 1)),
		Level:       level,
		Type:        alertType,
		Description: desc,
		Source:      event.Process,
		Events:      []FileEvent{event},
		Score:       r.calculateScore(level),
		Blocked:     r.config.AutoBlock && level >= ThreatHigh,
		Actions:     r.suggestActions(level, alertType),
		Timestamp:   time.Now(),
	}
}

func (r *RansomAI) createAlertPtr(level ThreatLevel, alertType, desc string, event FileEvent, score float64) *ThreatAlert {
	alert := r.createAlert(level, alertType, desc, event)
	alert.Score = score
	return &alert
}

func (r *RansomAI) calculateScore(level ThreatLevel) float64 {
	switch level {
	case ThreatCritical:
		return 0.95
	case ThreatHigh:
		return 0.75
	case ThreatMedium:
		return 0.5
	case ThreatLow:
		return 0.25
	default:
		return 0
	}
}

func (r *RansomAI) suggestActions(level ThreatLevel, alertType string) []string {
	actions := []string{"记录日志"}
	if level >= ThreatMedium {
		actions = append(actions, "通知管理员")
	}
	if level >= ThreatHigh {
		actions = append(actions, "隔离可疑进程", "暂停文件操作")
	}
	if level >= ThreatCritical {
		actions = append(actions, "阻断网络", "创建快照", "启动恢复流程")
	}
	return actions
}

func (r *RansomAI) analysisLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCh:
			return
		}
	}
}

func (r *RansomAI) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// 保留最近 24 小时事件
	cutoff := time.Now().Add(-24 * time.Hour)
	filtered := make([]FileEvent, 0)
	for _, e := range r.events {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	r.events = filtered
}

// AddHoneypot 添加蜜罐文件
func (r *RansomAI) AddHoneypot(path, ext string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.honeypots[path] = &HoneypotFile{
		Path:      path,
		Extension: ext,
	}
}

// GetAlerts 获取告警列表
func (r *RansomAI) GetAlerts() []ThreatAlert {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ThreatAlert, len(r.alerts))
	copy(result, r.alerts)
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp.After(result[j].Timestamp) })
	return result
}

// GetHoneypots 获取蜜罐列表
func (r *RansomAI) GetHoneypots() []HoneypotFile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]HoneypotFile, 0, len(r.honeypots))
	for _, hp := range r.honeypots {
		result = append(result, *hp)
	}
	return result
}

// GetConfig 获取配置
func (r *RansomAI) GetConfig() RansomAIConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// suppress unused
var _ = math.Log
