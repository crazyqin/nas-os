// Package behaviorransom 提供基于行为分析的勒索软件检测功能
// manager.go - 行为勒索检测管理器
package behaviorransom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 行为勒索检测管理器
type Manager struct {
	mu           sync.RWMutex
	config       DetectorConfig
	detector     *BehaviorDetector
	activities   []FileActivity
	threats      []ThreatEvent
	quarantine   []QuarantineRecord
	stats        ManagerStatus
	running      bool
	startTime    time.Time
	stopCh       chan struct{}
	activityCh   chan FileActivity
	threatCh     chan ThreatEvent
	alertHandler func(ThreatEvent)
}

// NewManager 创建新的行为勒索检测管理器
func NewManager(config DetectorConfig) *Manager {
	return &Manager{
		config:     config,
		detector:   NewBehaviorDetector(config),
		activities: make([]FileActivity, 0, 1000),
		threats:    make([]ThreatEvent, 0),
		quarantine: make([]QuarantineRecord, 0),
		stopCh:     make(chan struct{}),
		activityCh: make(chan FileActivity, 100),
		threatCh:   make(chan ThreatEvent, 50),
		stats: ManagerStatus{
			Config: config,
		},
	}
}

// SetAlertHandler 设置告警处理函数
func (m *Manager) SetAlertHandler(handler func(ThreatEvent)) {
	m.mu.Lock()
	m.alertHandler = handler
	m.mu.Unlock()
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.startTime = time.Now()
	m.stats.Running = true
	m.mu.Unlock()

	go m.activityLoop()
	go m.threatLoop()
	go m.analysisLoop()
	go m.cleanupLoop()

	log.Println("✅ 行为勒索检测管理器已启动")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.stats.Running = false
	m.mu.Unlock()

	log.Println("行为勒索检测管理器已停止")
}

// RecordActivity 记录文件活动
func (m *Manager) RecordActivity(activity FileActivity) {
	m.mu.Lock()
	m.stats.TotalActivities++
	m.mu.Unlock()

	select {
	case m.activityCh <- activity:
	default:
		log.Println("活动通道已满，丢弃事件")
	}
}

// GetStatus 获取管理器状态
func (m *Manager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := m.stats
	status.Config = m.config
	if m.running {
		status.Uptime = int64(time.Since(m.startTime).Seconds())
	}
	status.ActiveThreats = len(m.threats)
	return status
}

// GetThreats 获取威胁事件列表
func (m *Manager) GetThreats() []ThreatEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ThreatEvent, len(m.threats))
	copy(result, m.threats)
	return result
}

// GetConfig 获取配置
func (m *Manager) GetConfig() DetectorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config DetectorConfig) {
	m.mu.Lock()
	m.config = config
	m.stats.Config = config
	m.mu.Unlock()

	log.Println("行为勒索检测配置已更新")
}

// GetQuarantineRecords 获取隔离记录
func (m *Manager) GetQuarantineRecords() []QuarantineRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]QuarantineRecord, len(m.quarantine))
	copy(result, m.quarantine)
	return result
}

// GetPatterns 获取行为模式列表
func (m *Manager) GetPatterns() []BehaviorPattern {
	return m.detector.GetPatterns()
}

// activityLoop 活动处理循环
func (m *Manager) activityLoop() {
	for {
		select {
		case <-m.stopCh:
			return
		case activity := <-m.activityCh:
			m.processActivity(activity)
		}
	}
}

// processActivity 处理单个活动
func (m *Manager) processActivity(activity FileActivity) {
	m.mu.Lock()
	m.activities = append(m.activities, activity)

	// 保持缓冲区大小
	if len(m.activities) > 10000 {
		m.activities = m.activities[len(m.activities)-10000:]
	}
	m.mu.Unlock()
}

// threatLoop 威胁处理循环
func (m *Manager) threatLoop() {
	for {
		select {
		case <-m.stopCh:
			return
		case threat := <-m.threatCh:
			m.handleThreat(threat)
		}
	}
}

// handleThreat 处理威胁事件
func (m *Manager) handleThreat(threat ThreatEvent) {
	m.mu.Lock()
	m.threats = append(m.threats, threat)
	m.stats.TotalThreats++
	m.stats.BlockedThreats++
	now := time.Now()
	m.stats.LastThreatTime = &now

	// 保持威胁历史大小
	if len(m.threats) > m.config.MaxAlertHistory {
		m.threats = m.threats[len(m.threats)-m.config.MaxAlertHistory:]
	}

	handler := m.alertHandler
	m.mu.Unlock()

	// 自动隔离
	if m.config.AutoQuarantine && threat.Action == ActionQuarantine {
		m.quarantineFiles(threat)
	}

	// 调用告警处理函数
	if handler != nil {
		handler(threat)
	}

	log.Printf("🚨 威胁检测: ID=%s, 级别=%s, 分数=%d, 模式=%s",
		threat.ID, threat.ThreatLevel.String(), threat.Score, threat.Pattern)
}

// analysisLoop 定期分析循环
func (m *Manager) analysisLoop() {
	ticker := time.NewTicker(time.Duration(m.config.WindowSizeSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAnalysis()
		}
	}
}

// runAnalysis 执行分析
func (m *Manager) runAnalysis() {
	m.mu.RLock()
	if len(m.activities) == 0 {
		m.mu.RUnlock()
		return
	}

	// 获取窗口内的活动
	cutoff := time.Now().Add(-time.Duration(m.config.WindowSizeSec) * time.Second)
	var window []FileActivity
	for _, a := range m.activities {
		if a.Timestamp.After(cutoff) {
			window = append(window, a)
		}
	}
	m.mu.RUnlock()

	if len(window) == 0 {
		return
	}

	// 检测行为模式
	threats := m.detector.DetectPatterns(window)
	for _, threat := range threats {
		select {
		case m.threatCh <- threat:
		default:
			log.Println("威胁通道已满")
		}
	}

	// 分析熵值变化
	spiked, delta := m.detector.AnalyzeEntropyChange(window)
	if spiked {
		log.Printf("⚠️ 检测到熵值突增: delta=%.2f", delta)
	}
}

// cleanupLoop 定期清理循环
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup 清理过期数据
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清理超过1小时的活动
	cutoff := time.Now().Add(-1 * time.Hour)
	var remaining []FileActivity
	for _, a := range m.activities {
		if a.Timestamp.After(cutoff) {
			remaining = append(remaining, a)
		}
	}
	m.activities = remaining
}

// quarantineFiles 隔离文件
func (m *Manager) quarantineFiles(threat ThreatEvent) {
	quarantineDir := "/var/quarantine/behaviorransom"

	// 创建隔离目录
	if err := os.MkdirAll(quarantineDir, 0700); err != nil {
		log.Printf("创建隔离目录失败: %v", err)
		return
	}

	for _, filePath := range threat.AffectedFiles {
		record := QuarantineRecord{
			ID:             fmt.Sprintf("q-%d", time.Now().UnixNano()),
			OriginalPath:   filePath,
			QuarantinePath: filepath.Join(quarantineDir, filepath.Base(filePath)),
			Reason:         fmt.Sprintf("行为勒索检测: %s", threat.Pattern),
			ThreatEventID:  threat.ID,
			Timestamp:      time.Now(),
		}

		// 计算文件哈希
		hash, err := hashFile(filePath)
		if err == nil {
			record.FileHash = hash
		}

		// 获取文件大小
		info, err := os.Stat(filePath)
		if err == nil {
			record.FileSize = info.Size()
		}

		m.mu.Lock()
		m.quarantine = append(m.quarantine, record)
		m.mu.Unlock()

		log.Printf("文件已隔离: %s -> %s", filePath, record.QuarantinePath)
	}
}

// hashFile 计算文件SHA256哈希
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
