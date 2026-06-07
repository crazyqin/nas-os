// Package ransomwarecanary 提供勒索软件金丝雀管理核心业务逻辑
package ransomwarecanary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 勒索软件金丝雀管理器.
type Manager struct {
	canaries     map[string]*CanaryFile // id -> canary
	alerts       []*CanaryAlert
	lockedShares map[string]string // share -> lock reason
	config       CanaryConfig
	lastCheck    *time.Time
	mu           sync.RWMutex
	stopCh       chan struct{}
}

// NewManager 创建金丝雀管理器.
func NewManager() *Manager {
	return &Manager{
		canaries:     make(map[string]*CanaryFile),
		alerts:       make([]*CanaryAlert, 0),
		lockedShares: make(map[string]string),
		config: CanaryConfig{
			Enabled:          true,
			CheckIntervalSec: 60,
			MonitoredPaths:   []string{},
			AutoLockEnabled:  false,
			MaxAlertsPerHour: 50,
		},
		stopCh: make(chan struct{}),
	}
}

// ========== 金丝雀部署 ==========

// DeployCanary 部署一个金丝雀文件.
func (m *Manager) DeployCanary(req DeployCanaryRequest) (*CanaryFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成文件路径
	filePath := req.FilePath
	if filePath == "" {
		filePath = filepath.Join("/shares", req.ShareName, fmt.Sprintf(".canary_%s", uuid.New().String()[:8]))
	}

	// 检查是否已存在同路径金丝雀
	for _, c := range m.canaries {
		if c.FilePath == filePath && c.Status != "disabled" {
			return nil, fmt.Errorf("canary already exists at path %s", filePath)
		}
	}

	// 生成金丝雀文件内容（伪装数据）
	content := generateCanaryContent(req.Name)
	hash := computeSHA256(content)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// 写入金丝雀文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, fmt.Errorf("write canary file: %w", err)
	}

	now := time.Now()
	canary := &CanaryFile{
		ID:          uuid.New().String(),
		Name:        req.Name,
		FilePath:    filePath,
		ContentHash: hash,
		FileSize:    int64(len(content)),
		Status:      "active",
		ShareName:   req.ShareName,
		CreatedAt:   now,
		LastChecked: now,
	}

	m.canaries[canary.ID] = canary
	return canary, nil
}

// RemoveCanary 移除金丝雀文件.
func (m *Manager) RemoveCanary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	canary, ok := m.canaries[id]
	if !ok {
		return fmt.Errorf("canary %q not found", id)
	}

	// 删除文件
	if err := os.Remove(canary.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove canary file: %w", err)
	}

	delete(m.canaries, id)
	return nil
}

// DisableCanary 禁用金丝雀.
func (m *Manager) DisableCanary(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	canary, ok := m.canaries[id]
	if !ok {
		return fmt.Errorf("canary %q not found", id)
	}

	canary.Status = "disabled"
	return nil
}

// ListCanaries 列出所有金丝雀.
func (m *Manager) ListCanaries() []*CanaryFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CanaryFile, 0, len(m.canaries))
	for _, c := range m.canaries {
		cp := *c
		result = append(result, &cp)
	}
	return result
}

// ========== 监控检测 ==========

// MonitorCanaries 检查所有活跃金丝雀文件.
func (m *Manager) MonitorCanaries() (*DetectionResult, error) {
	m.mu.RLock()
	// 收集需要检查的金丝雀
	var active []*CanaryFile
	for _, c := range m.canaries {
		if c.Status == "active" {
			cp := *c
			active = append(active, &cp)
		}
	}
	m.mu.RUnlock()

	start := time.Now()
	var alerts []*CanaryAlert

	for _, canary := range active {
		alert := m.checkCanary(canary)
		if alert != nil {
			alerts = append(alerts, alert)
		}
	}

	m.mu.Lock()
	now := time.Now()
	m.lastCheck = &now
	// 更新 last_checked 时间
	for _, c := range active {
		if mc, ok := m.canaries[c.ID]; ok {
			mc.LastChecked = now
		}
	}
	m.mu.Unlock()

	return &DetectionResult{
		TotalChecked: len(active),
		AlertCount:   len(alerts),
		Alerts:       alerts,
		Timestamp:    now,
		Duration:     time.Since(start),
	}, nil
}

// checkCanary 检查单个金丝雀文件.
func (m *Manager) checkCanary(canary *CanaryFile) *CanaryAlert {
	info, err := os.Stat(canary.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件被删除
			return m.createAlert(canary, "deleted", "critical",
				fmt.Sprintf("金丝雀文件 %s 已被删除", canary.Name))
		}
		return m.createAlert(canary, "access_denied", "high",
			fmt.Sprintf("无法访问金丝雀文件 %s: %v", canary.Name, err))
	}

	// 检查文件大小变化
	if info.Size() != canary.FileSize {
		return m.createAlert(canary, "modified", "high",
			fmt.Sprintf("金丝雀文件 %s 大小变化: %d -> %d bytes", canary.Name, canary.FileSize, info.Size()))
	}

	// 检查内容哈希
	currentHash, err := computeFileSHA256(canary.FilePath)
	if err != nil {
		return m.createAlert(canary, "access_denied", "high",
			fmt.Sprintf("无法读取金丝雀文件 %s: %v", canary.Name, err))
	}

	if currentHash != canary.ContentHash {
		// 内容变化 — 可能是加密攻击
		return m.createAlert(canary, "encrypted", "critical",
			fmt.Sprintf("金丝雀文件 %s 内容已变化（疑似加密攻击）", canary.Name))
	}

	return nil
}

// createAlert 创建告警并处理自动锁定.
func (m *Manager) createAlert(canary *CanaryFile, alertType, severity, desc string) *CanaryAlert {
	alert := &CanaryAlert{
		ID:          uuid.New().String(),
		CanaryID:    canary.ID,
		CanaryName:  canary.Name,
		AlertType:   alertType,
		Severity:    severity,
		Description: desc,
		ShareName:   canary.ShareName,
		Timestamp:   time.Now(),
	}

	m.mu.Lock()
	m.alerts = append(m.alerts, alert)

	// 更新金丝雀状态
	if mc, ok := m.canaries[canary.ID]; ok {
		mc.Status = "triggered"
		now := time.Now()
		mc.TriggeredAt = &now
	}

	// 自动锁定共享
	if m.config.AutoLockEnabled && (severity == "high" || severity == "critical") {
		locked, err := m.autoLockShareUnsafe(canary.ShareName, fmt.Sprintf("勒索软件告警: %s", desc))
		if err == nil {
			alert.ShareLocked = locked
		}
	}
	m.mu.Unlock()

	return alert
}

// ========== 告警管理 ==========

// TriggerAlert 手动触发告警.
func (m *Manager) TriggerAlert(canaryID, alertType, severity, description string) (*CanaryAlert, error) {
	m.mu.RLock()
	canary, ok := m.canaries[canaryID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("canary %q not found", canaryID)
	}
	cp := *canary
	m.mu.RUnlock()

	return m.createAlert(&cp, alertType, severity, description), nil
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(limit int) []*CanaryAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*CanaryAlert, limit)
	for i, a := range m.alerts[start:] {
		cp := *a
		result[i] = &cp
	}
	return result
}

// ClearAlerts 清除告警.
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = make([]*CanaryAlert, 0)
}

// ========== 共享锁定 ==========

// AutoLockShare 自动锁定共享.
func (m *Manager) AutoLockShare(shareName, reason string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.autoLockShareUnsafe(shareName, reason)
}

// autoLockShareUnsafe 内部锁定共享（需持锁）.
func (m *Manager) autoLockShareUnsafe(shareName, reason string) (bool, error) {
	if _, locked := m.lockedShares[shareName]; locked {
		return false, nil // 已锁定
	}

	// 执行共享锁定：创建锁定标记文件
	lockPath := filepath.Join("/shares", shareName, ".ransomware_lock")
	lockContent := fmt.Sprintf("LOCKED|%s|%s", time.Now().Format(time.RFC3339), reason)

	if err := os.WriteFile(lockPath, []byte(lockContent), 0600); err != nil {
		// 如果写入失败（如路径不存在），记录但仍标记为已锁定
		_ = err
	}

	m.lockedShares[shareName] = reason
	return true, nil
}

// UnlockShare 解锁共享.
func (m *Manager) UnlockShare(shareName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, locked := m.lockedShares[shareName]; !locked {
		return fmt.Errorf("share %q is not locked", shareName)
	}

	// 删除锁定标记文件
	lockPath := filepath.Join("/shares", shareName, ".ransomware_lock")
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		_ = err
	}

	delete(m.lockedShares, shareName)
	return nil
}

// GetLockedShares 获取已锁定的共享.
func (m *Manager) GetLockedShares() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.lockedShares))
	for k, v := range m.lockedShares {
		result[k] = v
	}
	return result
}

// ========== 配置管理 ==========

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() CanaryConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg CanaryConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetStatus 获取系统状态.
func (m *Manager) GetStatus() CanaryStatusResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount, triggeredCount := 0, 0
	for _, c := range m.canaries {
		switch c.Status {
		case "active":
			activeCount++
		case "triggered":
			triggeredCount++
		}
	}

	return CanaryStatusResponse{
		Enabled:        m.config.Enabled,
		TotalCanaries:  len(m.canaries),
		ActiveCount:    activeCount,
		TriggeredCount: triggeredCount,
		TotalAlerts:    len(m.alerts),
		LastCheckTime:  m.lastCheck,
		Config:         m.config,
	}
}

// Stop 停止监控.
func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
}

// ========== 辅助函数 ==========

// generateCanaryContent 根据文件名生成伪装内容.
func generateCanaryContent(name string) []byte {
	// 生成看起来真实的伪装数据
	header := fmt.Sprintf("[This is a canary file: %s]\n", name)
	padding := make([]byte, 1024)
	for i := range padding {
		padding[i] = byte((i % 94) + 33) // 可打印 ASCII 字符
	}
	return append([]byte(header), padding...)
}

// computeSHA256 计算数据的 SHA256 哈希.
func computeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// computeFileSHA256 计算文件的 SHA256 哈希.
func computeFileSHA256(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return computeSHA256(data), nil
}
