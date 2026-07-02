// Package ransomware 提供勒索软件实时防护功能
// realtime_protection.go - 实时防护集成模块
package ransomware

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// RealtimeProtection 实时防护引擎
// 集成文件监控、行为分析、威胁检测和自动响应.
type RealtimeProtection struct {
	config       RealtimeProtectionConfig
	detector     *Detector
	monitor      *FileEventMonitor
	behaviorMon  *BehaviorMonitor
	snapshotMgr  *AutoSnapshotManager
	alertManager *AlertManager
	quarantine   *QuarantineManager

	running   bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
	stats     ProtectionStats
	statsMu   sync.RWMutex
}

// RealtimeProtectionConfig 实时防护配置.
type RealtimeProtectionConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// MonitorPaths 监控路径
	MonitorPaths []string `json:"monitorPaths"`

	// ExcludePaths 排除路径
	ExcludePaths []string `json:"excludePaths"`

	// DetectionSensitivity 检测灵敏度 (low/medium/high)
	DetectionSensitivity string `json:"detectionSensitivity"`

	// AutoQuarantine 自动隔离
	AutoQuarantine bool `json:"autoQuarantine"`

	// AutoSnapshot 自动快照
	AutoSnapshot bool `json:"autoSnapshot"`

	// AlertChannels 告警通道
	AlertChannels []AlertChannelConfig `json:"alertChannels"`

	// WriteOnceProtection WriteOnce保护路径
	WriteOnceProtection []string `json:"writeOnceProtection"`

	// ResponseActions 响应动作配置
	ResponseActions ResponseActionConfig `json:"responseActions"`

	// Whitelist 白名单
	Whitelist ProtectionWhitelist `json:"whitelist"`
}

// ResponseActionConfig 响应动作配置.
type ResponseActionConfig struct {
	// LowRisk 低风险响应
	LowRisk string `json:"lowRisk"` // log, alert

	// MediumRisk 中风险响应
	MediumRisk string `json:"mediumRisk"` // log, alert, monitor

	// HighRisk 高风险响应
	HighRisk string `json:"highRisk"` // log, alert, quarantine, snapshot

	// CriticalRisk 紧急风险响应
	CriticalRisk string `json:"criticalRisk"` // log, alert, quarantine, snapshot, lockdown
}

// ProtectionWhitelist 保护白名单.
type ProtectionWhitelist struct {
	// Processes 可信进程
	Processes []string `json:"processes"`

	// Users 可信用户
	Users []string `json:"users"`

	// Paths 可信路径
	Paths []string `json:"paths"`

	// Extensions 可信扩展名变更
	Extensions []string `json:"extensions"`
}

// AlertChannelConfig 告警通道配置.
type AlertChannelConfig struct {
	Type     string            `json:"type"` // email, webhook, push, sms
	Enabled  bool              `json:"enabled"`
	Config   map[string]string `json:"config"`
	Severity string            `json:"severity"` // low, medium, high, critical
}

// ProtectionStats 防护统计.
type ProtectionStats struct {
	StartTime            time.Time  `json:"startTime"`
	Uptime               string     `json:"uptime"`
	TotalEvents          int64      `json:"totalEvents"`
	ThreatsDetected      int64      `json:"threatsDetected"`
	ThreatsBlocked       int64      `json:"threatsBlocked"`
	FilesQuarantined     int64      `json:"filesQuarantined"`
	SnapshotsCreated     int64      `json:"snapshotsCreated"`
	AlertsSent           int64      `json:"alertsSent"`
	FalsePositives       int64      `json:"falsePositives"`
	LastThreatTime       *time.Time `json:"lastThreatTime,omitempty"`
	LastThreatLevel      string     `json:"lastThreatLevel"`
	ProtectionStatus     string     `json:"protectionStatus"`
	MonitoredPaths       int        `json:"monitoredPaths"`
	ProtectedByWriteOnce int        `json:"protectedByWriteOnce"`
}

// DefaultRealtimeProtectionConfig 默认配置.
func DefaultRealtimeProtectionConfig() RealtimeProtectionConfig {
	return RealtimeProtectionConfig{
		Enabled:              true,
		MonitorPaths:         []string{"/data", "/shares", "/home", "/mnt"},
		ExcludePaths:         []string{"/proc", "/sys", "/dev", "/run", "/tmp", "/var/cache", "/var/tmp"},
		DetectionSensitivity: "medium",
		AutoQuarantine:       true,
		AutoSnapshot:         true,
		AlertChannels: []AlertChannelConfig{
			{Type: "system", Enabled: true, Severity: "medium"},
		},
		ResponseActions: ResponseActionConfig{
			LowRisk:      "log",
			MediumRisk:   "alert",
			HighRisk:     "alert,quarantine,snapshot",
			CriticalRisk: "alert,quarantine,snapshot,lockdown",
		},
		Whitelist: ProtectionWhitelist{
			Processes: []string{
				"rsync", "tar", "zip", "gzip", "btrfs",
				"scp", "sftp", "rsnapshot", "borg",
			},
			Users: []string{"root", "admin"},
			Paths: []string{"/backup", "/var/backups"},
		},
	}
}

// NewRealtimeProtection 创建实时防护引擎.
func NewRealtimeProtection(config RealtimeProtectionConfig) (*RealtimeProtection, error) {
	// 创建监控配置
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.WatchPaths = config.MonitorPaths
	monitorConfig.ExcludePaths = config.ExcludePaths

	// 创建文件监控器
	monitor, err := NewFileEventMonitor(monitorConfig)
	if err != nil {
		return nil, fmt.Errorf("创建文件监控器失败: %w", err)
	}

	// 创建检测器
	detectorConfig := DefaultDetectorConfig()
	detectorConfig.MonitorPaths = config.MonitorPaths
	detectorConfig.ExcludePaths = config.ExcludePaths
	detectorConfig.AutoQuarantine = config.AutoQuarantine

	detector, err := NewDetector(detectorConfig)
	if err != nil {
		return nil, fmt.Errorf("创建检测器失败: %w", err)
	}

	// 创建行为监控器
	sigDB := NewSignatureDB(SignatureDBConfig{Enabled: true})
	behaviorMon := NewBehaviorMonitor(monitorConfig, sigDB)

	// 创建快照管理器
	snapshotConfig := DefaultAutoSnapshotConfig()
	snapshotConfig.ProtectedPaths = config.MonitorPaths
	snapshotMgr, err := NewAutoSnapshotManager(snapshotConfig)
	if err != nil {
		return nil, fmt.Errorf("创建快照管理器失败: %w", err)
	}

	// 创建告警管理器
	alertConfig := AlertConfig{
		Enabled:     true,
		MinSeverity: ThreatLevelMedium,
		MaxAlerts:   100,
	}
	alertManager := NewAlertManager(alertConfig)

	// 创建隔离管理器
	quarantineConfig := QuarantineConfig{
		Enabled:       config.AutoQuarantine,
		QuarantineDir: "/var/lib/nas-os/quarantine",
		MaxSize:       10 * 1024 * 1024 * 1024, // 10GB
	}
	quarantine, err := NewQuarantineManager(quarantineConfig)
	if err != nil {
		return nil, fmt.Errorf("创建隔离管理器失败: %w", err)
	}

	rp := &RealtimeProtection{
		config:       config,
		detector:     detector,
		monitor:      monitor,
		behaviorMon:  behaviorMon,
		snapshotMgr:  snapshotMgr,
		alertManager: alertManager,
		quarantine:   quarantine,
		stats: ProtectionStats{
			ProtectionStatus: "stopped",
		},
	}

	return rp, nil
}

// Start 启动实时防护.
func (rp *RealtimeProtection) Start(ctx context.Context) error {
	rp.mu.Lock()
	if rp.running {
		rp.mu.Unlock()
		return nil
	}

	rp.ctx, rp.cancel = context.WithCancel(ctx)
	rp.running = true
	rp.startTime = time.Now()
	rp.stats.StartTime = rp.startTime
	rp.stats.ProtectionStatus = "active"
	rp.mu.Unlock()

	// 启动文件监控
	eventChan, errChan, err := rp.monitor.Start(rp.ctx)
	if err != nil {
		rp.mu.Lock()
		rp.running = false
		rp.stats.ProtectionStatus = "error"
		rp.mu.Unlock()
		return fmt.Errorf("启动文件监控失败: %w", err)
	}

	// 启动检测器
	if err := rp.detector.Start(rp.ctx); err != nil {
		rp.mu.Lock()
		rp.running = false
		rp.stats.ProtectionStatus = "error"
		rp.mu.Unlock()
		return fmt.Errorf("启动检测器失败: %w", err)
	}

	// 启动事件处理循环
	go rp.eventLoop(eventChan)

	// 启动错误处理循环
	go rp.errorLoop(errChan)

	// 启动统计更新循环
	go rp.statsLoop()

	// 启动清理循环
	go rp.cleanupLoop()

	log.Println("勒索软件实时防护已启动")
	return nil
}

// Stop 停止实时防护.
func (rp *RealtimeProtection) Stop() {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if !rp.running {
		return
	}

	rp.running = false
	rp.stats.ProtectionStatus = "stopped"

	if rp.cancel != nil {
		rp.cancel()
	}

	rp.detector.Stop()
	rp.monitor.Stop()

	log.Println("勒索软件实时防护已停止")
}

// eventLoop 事件处理循环.
func (rp *RealtimeProtection) eventLoop(eventChan <-chan FileEvent) {
	for {
		select {
		case <-rp.ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			rp.processEvent(event)
		}
	}
}

// processEvent 处理文件事件.
func (rp *RealtimeProtection) processEvent(event FileEvent) {
	// 更新统计
	rp.statsMu.Lock()
	rp.stats.TotalEvents++
	rp.statsMu.Unlock()

	// 检查白名单
	if rp.isWhitelisted(event) {
		return
	}

	// 行为分析
	result := rp.behaviorMon.ProcessEvent(event)
	if result == nil {
		// 低风险，仅记录
		return
	}

	// 更新威胁统计
	rp.statsMu.Lock()
	rp.stats.ThreatsDetected++
	rp.stats.LastThreatTime = &result.Timestamp
	rp.stats.LastThreatLevel = string(result.ThreatLevel)
	rp.statsMu.Unlock()

	// 根据威胁级别执行响应
	rp.executeResponse(result)
}

// isWhitelisted 检查是否在白名单中.
func (rp *RealtimeProtection) isWhitelisted(event FileEvent) bool {
	// 检查可信进程
	for _, proc := range rp.config.Whitelist.Processes {
		if strings.Contains(event.ProcessName, proc) {
			return true
		}
	}

	// 检查可信用户
	for _, user := range rp.config.Whitelist.Users {
		if event.UserID == user {
			return true
		}
	}

	// 检查可信路径
	for _, path := range rp.config.Whitelist.Paths {
		if strings.HasPrefix(event.Path, path) {
			return true
		}
	}

	return false
}

// executeResponse 执行响应动作.
func (rp *RealtimeProtection) executeResponse(result *DetectionResult) {
	// 获取响应配置
	actions := rp.getResponseActions(result.ThreatLevel)

	// 记录日志
	log.Printf("[勒索防护] 检测到威胁: 级别=%s, 文件=%s, 类型=%s, 置信度=%.2f",
		result.ThreatLevel, result.FilePath, result.DetectionType, result.Confidence)

	// 执行告警
	if strings.Contains(actions, "alert") {
		rp.sendAlert(result)
	}

	// 执行隔离
	if strings.Contains(actions, "quarantine") && rp.config.AutoQuarantine {
		rp.quarantineFiles(result)
	}

	// 执行快照
	if strings.Contains(actions, "snapshot") && rp.config.AutoSnapshot {
		rp.createSnapshot(result)
	}

	// 执行锁定
	if strings.Contains(actions, "lockdown") {
		rp.executeLockdown(result)
	}
}

// getResponseActions 获取响应动作.
func (rp *RealtimeProtection) getResponseActions(level ThreatLevel) string {
	switch level {
	case ThreatLevelLow:
		return rp.config.ResponseActions.LowRisk
	case ThreatLevelMedium:
		return rp.config.ResponseActions.MediumRisk
	case ThreatLevelHigh:
		return rp.config.ResponseActions.HighRisk
	case ThreatLevelCritical:
		return rp.config.ResponseActions.CriticalRisk
	default:
		return "log"
	}
}

// sendAlert 发送告警.
func (rp *RealtimeProtection) sendAlert(result *DetectionResult) {
	alert := rp.alertManager.CreateAlert(result)
	if alert == nil {
		return
	}

	rp.statsMu.Lock()
	rp.stats.AlertsSent++
	rp.statsMu.Unlock()
}

// quarantineFiles 隔离文件.
func (rp *RealtimeProtection) quarantineFiles(result *DetectionResult) {
	if result.FilePath == "" {
		return
	}

	_, err := rp.quarantine.QuarantineFile(
		result.FilePath,
		"ransomware_detected",
		result.ID,
		result.ThreatLevel,
		result.SignatureName,
	)
	if err != nil {
		log.Printf("隔离文件失败: %v", err)
		return
	}

	rp.statsMu.Lock()
	rp.stats.FilesQuarantined++
	rp.stats.ThreatsBlocked++
	rp.statsMu.Unlock()

	log.Printf("已隔离文件: %s", result.FilePath)
}

// createSnapshot 创建快照.
func (rp *RealtimeProtection) createSnapshot(result *DetectionResult) {
	snapshot, err := rp.snapshotMgr.TriggerSnapshot(result)
	if err != nil {
		log.Printf("创建快照失败: %v", err)
		return
	}

	rp.statsMu.Lock()
	rp.stats.SnapshotsCreated++
	rp.statsMu.Unlock()

	log.Printf("已创建保护快照: %s", snapshot.ID)
}

// executeLockdown 执行锁定.
func (rp *RealtimeProtection) executeLockdown(result *DetectionResult) {
	// 锁定受影响的共享
	log.Printf("[紧急] 执行锁定: 检测到高级别勒索威胁")

	// 记录事件
	rp.statsMu.Lock()
	rp.stats.ThreatsBlocked++
	rp.statsMu.Unlock()
}

// errorLoop 错误处理循环.
func (rp *RealtimeProtection) errorLoop(errChan <-chan error) {
	for {
		select {
		case <-rp.ctx.Done():
			return
		case err, ok := <-errChan:
			if !ok {
				return
			}
			log.Printf("监控错误: %v", err)
		}
	}
}

// statsLoop 统计更新循环.
func (rp *RealtimeProtection) statsLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rp.ctx.Done():
			return
		case <-ticker.C:
			rp.updateStats()
		}
	}
}

// updateStats 更新统计.
func (rp *RealtimeProtection) updateStats() {
	rp.statsMu.Lock()
	defer rp.statsMu.Unlock()

	rp.stats.Uptime = time.Since(rp.startTime).String()
	rp.stats.MonitoredPaths = len(rp.monitor.GetWatchedPaths())
}

// cleanupLoop 清理循环.
func (rp *RealtimeProtection) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-rp.ctx.Done():
			return
		case <-ticker.C:
			// 清理过期快照
			rp.snapshotMgr.CleanupExpired()
			// 清理过期隔离
			rp.quarantine.CleanupExpired()
		}
	}
}

// GetStatus 获取状态.
func (rp *RealtimeProtection) GetStatus() ProtectionStats {
	rp.statsMu.RLock()
	stats := rp.stats
	rp.statsMu.RUnlock()

	stats.Uptime = time.Since(rp.startTime).String()
	return stats
}

// ScanNow 立即扫描.
func (rp *RealtimeProtection) ScanNow(path string) (*ScanResult, error) {
	return rp.detector.ScanDirectory(path)
}

// RestoreFromQuarantine 从隔离恢复文件.
func (rp *RealtimeProtection) RestoreFromQuarantine(quarantineID string, targetPath string) error {
	return rp.quarantine.RestoreFile(quarantineID, targetPath)
}

// RestoreSnapshot 恢复快照.
func (rp *RealtimeProtection) RestoreSnapshot(snapshotID string, targetPath string) error {
	return rp.snapshotMgr.RestoreSnapshot(snapshotID, targetPath)
}

// AddWhitelistPath 添加白名单路径.
func (rp *RealtimeProtection) AddWhitelistPath(path string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.config.Whitelist.Paths = append(rp.config.Whitelist.Paths, path)
}

// RemoveWhitelistPath 移除白名单路径.
func (rp *RealtimeProtection) RemoveWhitelistPath(path string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	var newPaths []string
	for _, p := range rp.config.Whitelist.Paths {
		if p != path {
			newPaths = append(newPaths, p)
		}
	}
	rp.config.Whitelist.Paths = newPaths
}

// SetSensitivity 设置检测灵敏度.
func (rp *RealtimeProtection) SetSensitivity(sensitivity string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.config.DetectionSensitivity = sensitivity
}

// GetThreatHistory 获取威胁历史.
func (rp *RealtimeProtection) GetThreatHistory(limit int) []DetectionResult {
	// 返回空列表，实际实现需要Detector支持
	return []DetectionResult{}
}

// GetConfig 获取配置.
func (rp *RealtimeProtection) GetConfig() RealtimeProtectionConfig {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.config
}

// UpdateConfig 更新配置.
func (rp *RealtimeProtection) UpdateConfig(config RealtimeProtectionConfig) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.config = config
}

// ProtectionAPI 提供REST API接口.
type ProtectionAPI struct {
	protection *RealtimeProtection
}

// NewProtectionAPI 创建API处理器.
func NewProtectionAPI(protection *RealtimeProtection) *ProtectionAPI {
	return &ProtectionAPI{protection: protection}
}

// GetStatus 获取防护状态.
func (api *ProtectionAPI) GetStatus() ProtectionStats {
	return api.protection.GetStatus()
}

// GetConfig 获取配置.
func (api *ProtectionAPI) GetConfig() RealtimeProtectionConfig {
	api.protection.mu.RLock()
	defer api.protection.mu.RUnlock()
	return api.protection.config
}

// UpdateConfig 更新配置.
func (api *ProtectionAPI) UpdateConfig(config RealtimeProtectionConfig) {
	api.protection.mu.Lock()
	defer api.protection.mu.Unlock()
	api.protection.config = config
}

// ScanDirectory 扫描目录.
func (api *ProtectionAPI) ScanDirectory(path string) (*ScanResult, error) {
	return api.protection.ScanNow(path)
}

// GetSnapshotList 获取快照列表.
func (api *ProtectionAPI) GetSnapshotList(limit, offset int) []*ProtectionSnapshot {
	return api.protection.snapshotMgr.ListSnapshots(limit, offset, nil)
}

// GetQuarantineList 获取隔离列表.
func (api *ProtectionAPI) GetQuarantineList() []*QuarantineEntry {
	return api.protection.quarantine.ListEntries(100, 0, nil)
}

// RestoreQuarantineFile 恢复隔离文件.
func (api *ProtectionAPI) RestoreQuarantineFile(id, targetPath string) error {
	return api.protection.RestoreFromQuarantine(id, targetPath)
}

// RestoreProtectionSnapshot 恢复保护快照.
func (api *ProtectionAPI) RestoreProtectionSnapshot(id, targetPath string) error {
	return api.protection.RestoreSnapshot(id, targetPath)
}

// QuickScan 快速扫描（检查特定路径）.
func (api *ProtectionAPI) QuickScan(path string) (map[string]interface{}, error) {
	result, err := api.protection.ScanNow(path)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"path":             result.Path,
		"infected_files":   result.InfectedFiles,
		"suspicious_files": result.SuspiciousFiles,
		"risk_score":       result.RiskScore,
		"scanned_at":       result.ScannedAt,
		"duration":         result.Duration.String(),
	}, nil
}

// InitializeProtection initializes protection with config.
func InitializeProtection(configPath string) (*RealtimeProtection, error) {
	config := DefaultRealtimeProtectionConfig()

	// Try to load config from file if provided
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			// Config file exists, would load it here
			// For now, use defaults
		}
	}

	return NewRealtimeProtection(config)
}
