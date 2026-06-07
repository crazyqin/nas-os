// Package ransomwareguard 勒索软件防护
// 文件行为监控 + 蜜罐检测 + 自动快照保护
package ransomwareguard

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ThreatLevel 威胁级别
type ThreatLevel string

const (
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// AlertType 告警类型
type AlertType string

const (
	AlertMassRename    AlertType = "mass_rename"    // 批量重命名
	AlertMassDelete    AlertType = "mass_delete"    // 批量删除
	AlertHoneypotHit   AlertType = "honeypot_hit"   // 蜜罐触发
	AlertSuspiciousExt AlertType = "suspicious_ext" // 可疑扩展名
	AlertRapidEncrypt  AlertType = "rapid_encrypt"  // 快速加密模式
	AlertAbnormalIO    AlertType = "abnormal_io"    // 异常IO模式
)

// SuspiciousExt 可疑扩展名列表
var SuspiciousExt = []string{
	".encrypted", ".locked", ".crypto", ".crypt", ".enc",
	".aaa", ".abc", ".ccc", ".vvv", ".xxx", ".zzz",
	".locky", ".cerber", ".wncry", ".wncryt", ".thor",
	".zepto", ".odin", ".aesir", ".shit", ".fuck",
}

// HoneypotFile 蜜罐文件
type HoneypotFile struct {
	Path      string    `json:"path"`
	RealPath  string    `json:"real_path"` // 实际存储位置
	CreatedAt time.Time `json:"created_at"`
	Triggered bool      `json:"triggered"`
}

// SecurityAlert 安全告警
type SecurityAlert struct {
	ID        string      `json:"id"`
	Type      AlertType   `json:"type"`
	Level     ThreatLevel `json:"level"`
	Message   string      `json:"message"`
	SourceIP  string      `json:"source_ip,omitempty"`
	FilePath  string      `json:"file_path,omitempty"`
	Details   string      `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Resolved  bool        `json:"resolved"`
}

// ProtectionStatus 防护状态
type ProtectionStatus struct {
	Enabled         bool       `json:"enabled"`
	MonitoredPaths  []string   `json:"monitored_paths"`
	HoneypotCount   int        `json:"honeypot_count"`
	ActiveAlerts    int        `json:"active_alerts"`
	TotalBlocked    int64      `json:"total_blocked"`
	LastSnapshot    *time.Time `json:"last_snapshot,omitempty"`
	ProtectionSince *time.Time `json:"protection_since,omitempty"`
}

// FileEvent 文件事件
type FileEvent struct {
	Path      string    `json:"path"`
	Operation string    `json:"operation"` // create/modify/delete/rename
	Size      int64     `json:"size"`
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id,omitempty"`
}

// Manager 勒索软件防护管理器
type Manager struct {
	mu             sync.RWMutex
	enabled        bool
	monitoredPaths []string
	honeypots      map[string]*HoneypotFile
	alerts         map[string]*SecurityAlert
	recentEvents   []FileEvent
	windowSec      int // 检测时间窗口（秒）
	threshold      int // 触发阈值
	protectedSince *time.Time
	totalBlocked   int64
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		enabled:        true,
		monitoredPaths: []string{"/data", "/shared"},
		honeypots:      make(map[string]*HoneypotFile),
		alerts:         make(map[string]*SecurityAlert),
		recentEvents:   make([]FileEvent, 0, 1000),
		windowSec:      60,
		threshold:      50,
	}
}

// Enable 启用防护
func (m *Manager) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
	now := time.Now()
	m.protectedSince = &now
}

// Disable 禁用防护
func (m *Manager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// AddMonitoredPath 添加监控路径
func (m *Manager) AddMonitoredPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.monitoredPaths {
		if p == path {
			return
		}
	}
	m.monitoredPaths = append(m.monitoredPaths, path)
}

// RemoveMonitoredPath 移除监控路径
func (m *Manager) RemoveMonitoredPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, 0, len(m.monitoredPaths))
	for _, p := range m.monitoredPaths {
		if p != path {
			result = append(result, p)
		}
	}
	m.monitoredPaths = result
}

// DeployHoneypots 部署蜜罐文件
func (m *Manager) DeployHoneypots(basePath string, count int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := []string{
		"budget_2026.xlsx", "passwords.txt", "family_photos.zip",
		"tax_return.pdf", "crypto_wallet.dat", "important_docs.rar",
		"backup_codes.txt", "bank_statement.pdf", "passport_scan.jpg",
		"private_keys.pem",
	}

	deployed := 0
	for i := 0; i < count && i < len(names); i++ {
		path := filepath.Join(basePath, names[i])
		if _, exists := m.honeypots[path]; exists {
			continue
		}
		m.honeypots[path] = &HoneypotFile{
			Path:      path,
			RealPath:  filepath.Join(basePath, ".honeypot_"+names[i]),
			CreatedAt: time.Now(),
		}
		deployed++
	}
	return deployed
}

// ProcessEvent 处理文件事件
func (m *Manager) ProcessEvent(event FileEvent) *SecurityAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	// 检查蜜罐
	if hp, exists := m.honeypots[event.Path]; exists {
		if !hp.Triggered {
			hp.Triggered = true
			alert := m.createAlert(AlertHoneypotHit, ThreatCritical,
				fmt.Sprintf("蜜罐文件被访问: %s", event.Path), event)
			return alert
		}
	}

	// 检查可疑扩展名
	if event.Operation == "rename" || event.Operation == "create" {
		ext := strings.ToLower(filepath.Ext(event.Path))
		for _, se := range SuspiciousExt {
			if ext == se {
				alert := m.createAlert(AlertSuspiciousExt, ThreatHigh,
					fmt.Sprintf("检测到可疑扩展名: %s", ext), event)
				return alert
			}
		}
	}

	// 添加到最近事件
	m.recentEvents = append(m.recentEvents, event)
	if len(m.recentEvents) > 1000 {
		m.recentEvents = m.recentEvents[len(m.recentEvents)-1000:]
	}

	// 检测批量操作
	alert := m.detectMassOperation(event)
	return alert
}

func (m *Manager) detectMassOperation(event FileEvent) *SecurityAlert {
	cutoff := time.Now().Add(-time.Duration(m.windowSec) * time.Second)
	recentCount := 0
	deleteCount := 0
	renameCount := 0

	for _, e := range m.recentEvents {
		if e.Timestamp.After(cutoff) {
			recentCount++
			if e.Operation == "delete" {
				deleteCount++
			}
			if e.Operation == "rename" {
				renameCount++
			}
		}
	}

	if deleteCount > m.threshold {
		return m.createAlert(AlertMassDelete, ThreatCritical,
			fmt.Sprintf("检测到批量删除: %d 次/%d 秒", deleteCount, m.windowSec), event)
	}
	if renameCount > m.threshold {
		return m.createAlert(AlertMassRename, ThreatHigh,
			fmt.Sprintf("检测到批量重命名: %d 次/%d 秒", renameCount, m.windowSec), event)
	}
	return nil
}

func (m *Manager) createAlert(alertType AlertType, level ThreatLevel, msg string, event FileEvent) *SecurityAlert {
	alert := &SecurityAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Type:      alertType,
		Level:     level,
		Message:   msg,
		FilePath:  event.Path,
		Timestamp: time.Now(),
	}
	m.alerts[alert.ID] = alert
	m.totalBlocked++
	return alert
}

// GetStatus 获取防护状态
func (m *Manager) GetStatus() ProtectionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeAlerts := 0
	for _, a := range m.alerts {
		if !a.Resolved {
			activeAlerts++
		}
	}

	return ProtectionStatus{
		Enabled:         m.enabled,
		MonitoredPaths:  m.monitoredPaths,
		HoneypotCount:   len(m.honeypots),
		ActiveAlerts:    activeAlerts,
		TotalBlocked:    m.totalBlocked,
		LastSnapshot:    nil,
		ProtectionSince: m.protectedSince,
	}
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(resolved bool) []SecurityAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SecurityAlert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			result = append(result, *a)
		}
	}
	return result
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("告警不存在: %s", alertID)
	}
	alert.Resolved = true
	return nil
}

// GetHoneypots 获取蜜罐列表
func (m *Manager) GetHoneypots() []HoneypotFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]HoneypotFile, 0, len(m.honeypots))
	for _, hp := range m.honeypots {
		result = append(result, *hp)
	}
	return result
}

// ResetHoneypots 重置蜜罐状态
func (m *Manager) ResetHoneypots() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, hp := range m.honeypots {
		hp.Triggered = false
	}
}
