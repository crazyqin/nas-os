// Package antiRansomHoneypot 提供防勒索蜜罐功能
// 通过部署诱饵文件和监控异常行为来检测勒索软件攻击
package antiRansomHoneypot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// 蜜罐文件类型
const (
	HoneypotTypeDocument = "document" // 文档类诱饵
	HoneypotTypeImage    = "image"    // 图片类诱饵
	HoneypotTypeDatabase = "database" // 数据库类诱饵
	HoneypotTypeArchive  = "archive"  // 压缩包类诱饵
	HoneypotTypeCode     = "code"     // 代码类诱饵
	HoneypotTypeConfig   = "config"   // 配置文件诱饵
)

// 威胁级别
const (
	ThreatLevelLow      = "low"      // 低威胁：单文件异常
	ThreatLevelMedium   = "medium"   // 中威胁：多文件异常
	ThreatLevelHigh     = "high"     // 高威胁：批量加密行为
	ThreatLevelCritical = "critical" // 严重：勒索软件确认
)

// 防护动作
const (
	ActionAlert      = "alert"      // 仅告警
	ActionQuarantine = "quarantine" // 隔离进程
	ActionBlockIP    = "block_ip"   // 封锁IP
	ActionLockShare  = "lock_share" // 锁定共享
	ActionShutdown   = "shutdown"   // 紧急关机
)

// 错误定义
var (
	ErrHoneypotNotFound = errors.New("蜜罐不存在")
	ErrHoneypotExists   = errors.New("蜜罐已存在")
	ErrInvalidPath      = errors.New("无效路径")
	ErrPolicyNotFound   = errors.New("策略不存在")
	ErrAlreadyTriggered = errors.New("已触发告警")
)

// HoneypotFile 蜜罐文件
type HoneypotFile struct {
	ID           string    `json:"id"`            // 蜜罐ID
	Path         string    `json:"path"`          // 文件路径
	Type         string    `json:"type"`          // 蜜罐类型
	FileName     string    `json:"file_name"`     // 文件名
	FileSize     int64     `json:"file_size"`     // 文件大小
	ContentHash  string    `json:"content_hash"`  // 内容哈希
	Enabled      bool      `json:"enabled"`       // 是否启用
	ShareName    string    `json:"share_name"`    // 所属共享名
	Tags         []string  `json:"tags"`          // 标签
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
	LastChecked  time.Time `json:"last_checked"`  // 最后检查时间
	TriggerCount int64     `json:"trigger_count"` // 触发次数
}

// ThreatEvent 威胁事件
type ThreatEvent struct {
	ID            string    `json:"id"`             // 事件ID
	HoneypotID    string    `json:"honeypot_id"`    // 触发的蜜罐ID
	ThreatLevel   string    `json:"threat_level"`   // 威胁级别
	SourceIP      string    `json:"source_ip"`      // 来源IP
	SourceUser    string    `json:"source_user"`    // 来源用户
	ProcessName   string    `json:"process_name"`   // 进程名
	ProcessPID    int       `json:"process_pid"`    // 进程PID
	Operation     string    `json:"operation"`      // 操作类型（read/write/delete/rename/encrypt）
	FilePath      string    `json:"file_path"`      // 被操作文件路径
	OldHash       string    `json:"old_hash"`       // 原始哈希
	NewHash       string    `json:"new_hash"`       // 新哈希
	EntropyBefore float64   `json:"entropy_before"` // 操作前熵值
	EntropyAfter  float64   `json:"entropy_after"`  // 操作后熵值
	ActionTaken   string    `json:"action_taken"`   // 采取的动作
	Timestamp     time.Time `json:"timestamp"`      // 发生时间
	Details       string    `json:"details"`        // 详细描述
}

// ProtectionPolicy 防护策略
type ProtectionPolicy struct {
	ID                 string    `json:"id"`                   // 策略ID
	Name               string    `json:"name"`                 // 策略名称
	Description        string    `json:"description"`          // 描述
	Enabled            bool      `json:"enabled"`              // 是否启用
	EntropyThreshold   float64   `json:"entropy_threshold"`    // 熵值阈值（检测加密行为）
	FileChangeRateMax  int       `json:"file_change_rate_max"` // 每分钟最大文件变更数
	BatchOperationSize int       `json:"batch_operation_size"` // 批量操作阈值
	MonitorExtensions  []string  `json:"monitor_extensions"`   // 监控的文件扩展名
	ExemptUsers        []string  `json:"exempt_users"`         // 豁免用户
	ExemptIPs          []string  `json:"exempt_ips"`           // 豁免IP
	DefaultAction      string    `json:"default_action"`       // 默认动作
	AutoResponse       bool      `json:"auto_response"`        // 自动响应
	AlertChannels      []string  `json:"alert_channels"`       // 告警通道
	QuarantinePath     string    `json:"quarantine_path"`      // 隔离路径
	BackupOnTrigger    bool      `json:"backup_on_trigger"`    // 触发时自动备份
	RecoveryPointLimit int       `json:"recovery_point_limit"` // 恢复点保留数量
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// DetectionStats 检测统计
type DetectionStats struct {
	TotalHoneypots    int            `json:"total_honeypots"`   // 蜜罐总数
	ActiveHoneypots   int            `json:"active_honeypots"`  // 活跃蜜罐数
	TotalEvents       int64          `json:"total_events"`      // 总事件数
	BlockedAttacks    int64          `json:"blocked_attacks"`   // 拦截攻击数
	LastDetection     *time.Time     `json:"last_detection"`    // 最近检测时间
	AvgResponseTimeMs float64        `json:"avg_response_time"` // 平均响应时间
	TopThreatSources  []ThreatSource `json:"top_sources"`       // 主要威胁来源
}

// ThreatSource 威胁来源
type ThreatSource struct {
	IP       string    `json:"ip"`
	Count    int64     `json:"count"`
	LastSeen time.Time `json:"last_seen"`
	Level    string    `json:"level"`
}

// HoneypotManager 蜜罐管理器
type HoneypotManager struct {
	mu           sync.RWMutex
	honeypots    map[string]*HoneypotFile
	events       []*ThreatEvent
	policies     map[string]*ProtectionPolicy
	stats        *DetectionStats
	eventCounter int64
}

// NewHoneypotManager 创建蜜罐管理器
func NewHoneypotManager() *HoneypotManager {
	return &HoneypotManager{
		honeypots: make(map[string]*HoneypotFile),
		events:    make([]*ThreatEvent, 0),
		policies:  make(map[string]*ProtectionPolicy),
		stats:     &DetectionStats{},
	}
}

// CreateHoneypot 创建蜜罐文件
func (m *HoneypotManager) CreateHoneypot(hp *HoneypotFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hp.ID == "" {
		hp.ID = generateID()
	}
	if _, exists := m.honeypots[hp.ID]; exists {
		return ErrHoneypotExists
	}
	if hp.Path == "" {
		return ErrInvalidPath
	}

	hp.CreatedAt = time.Now()
	hp.UpdatedAt = time.Now()
	hp.LastChecked = time.Now()
	m.honeypots[hp.ID] = hp
	m.stats.TotalHoneypots++
	if hp.Enabled {
		m.stats.ActiveHoneypots++
	}
	return nil
}

// RemoveHoneypot 移除蜜罐
func (m *HoneypotManager) RemoveHoneypot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hp, exists := m.honeypots[id]
	if !exists {
		return ErrHoneypotNotFound
	}
	if hp.Enabled {
		m.stats.ActiveHoneypots--
	}
	m.stats.TotalHoneypots--
	delete(m.honeypots, id)
	return nil
}

// ReportThreat 上报威胁事件
func (m *HoneypotManager) ReportThreat(event *ThreatEvent) (*ThreatEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.honeypots[event.HoneypotID]; !exists {
		return nil, ErrHoneypotNotFound
	}

	m.eventCounter++
	event.ID = fmt.Sprintf("evt-%d-%s", m.eventCounter, generateShortID())
	event.Timestamp = time.Now()

	// 根据熵值变化判断威胁级别
	if event.EntropyAfter > 7.0 && event.EntropyBefore < 5.0 {
		event.ThreatLevel = ThreatLevelCritical
	} else if event.EntropyAfter > 6.0 {
		event.ThreatLevel = ThreatLevelHigh
	} else if event.Operation == "delete" || event.Operation == "rename" {
		event.ThreatLevel = ThreatLevelMedium
	} else {
		event.ThreatLevel = ThreatLevelLow
	}

	m.events = append(m.events, event)
	m.stats.TotalEvents++

	// 更新蜜罐触发计数
	if hp, ok := m.honeypots[event.HoneypotID]; ok {
		hp.TriggerCount++
		hp.LastChecked = time.Now()
	}

	// 更新威胁来源统计
	m.updateThreatSource(event)

	now := time.Now()
	m.stats.LastDetection = &now

	return event, nil
}

// SetProtectionPolicy 设置防护策略
func (m *HoneypotManager) SetProtectionPolicy(policy *ProtectionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateID()
	}
	policy.UpdatedAt = time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	m.policies[policy.ID] = policy
	return nil
}

// GetStats 获取检测统计
func (m *HoneypotManager) GetStats() *DetectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	events := int64(len(m.events))
	blocked := int64(0)
	for _, e := range m.events {
		if e.ActionTaken != ActionAlert {
			blocked++
		}
	}
	stats.TotalEvents = events
	stats.BlockedAttacks = blocked
	return &stats
}

// GetEvents 获取事件列表
func (m *HoneypotManager) GetEvents(limit int, level string) []*ThreatEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ThreatEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if level != "" && m.events[i].ThreatLevel != level {
			continue
		}
		result = append(result, m.events[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ListHoneypots 列出所有蜜罐
func (m *HoneypotManager) ListHoneypots() []*HoneypotFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*HoneypotFile, 0, len(m.honeypots))
	for _, hp := range m.honeypots {
		list = append(list, hp)
	}
	return list
}

// AnalyzeEntropy 分析文件熵值（用于检测加密行为）
func AnalyzeEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int64)
	for _, b := range data {
		freq[b]++
	}

	var entropy float64
	length := float64(len(data))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// updateThreatSource 更新威胁来源统计
func (m *HoneypotManager) updateThreatSource(event *ThreatEvent) {
	if event.SourceIP == "" {
		return
	}
	for i, src := range m.stats.TopThreatSources {
		if src.IP == event.SourceIP {
			m.stats.TopThreatSources[i].Count++
			m.stats.TopThreatSources[i].LastSeen = event.Timestamp
			if event.ThreatLevel > src.Level {
				m.stats.TopThreatSources[i].Level = event.ThreatLevel
			}
			return
		}
	}
	m.stats.TopThreatSources = append(m.stats.TopThreatSources, ThreatSource{
		IP:       event.SourceIP,
		Count:    1,
		LastSeen: event.Timestamp,
		Level:    event.ThreatLevel,
	})
}

// ExportEvents 导出事件为JSON
func (m *HoneypotManager) ExportEvents() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.events, "", "  ")
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
