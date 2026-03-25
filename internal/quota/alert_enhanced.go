// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 预警配置增强 ==========

// AlertThresholdConfig 预警阈值配置.
type AlertThresholdConfig struct {
	// 多级预警阈值
	WarningThreshold   float64 `json:"warning_threshold"`   // 警告级别（默认 70%）
	CriticalThreshold  float64 `json:"critical_threshold"`  // 严重级别（默认 85%）
	EmergencyThreshold float64 `json:"emergency_threshold"` // 紧急级别（默认 95%）

	// 预警冷却时间
	CooldownDuration time.Duration `json:"cooldown_duration"` // 同一配额告警冷却时间

	// 重复告警配置
	RepeatAlert      bool          `json:"repeat_alert"`       // 是否重复发送告警
	RepeatInterval   time.Duration `json:"repeat_interval"`    // 重复告警间隔
	MaxRepeatCount   int           `json:"max_repeat_count"`   // 最大重复次数
	EscalateOnRepeat bool          `json:"escalate_on_repeat"` // 重复告警是否升级

	// 告警聚合
	AggregateAlerts   bool          `json:"aggregate_alerts"`    // 是否聚合告警
	AggregateInterval time.Duration `json:"aggregate_interval"`  // 聚合间隔
	AggregateMaxCount int           `json:"aggregate_max_count"` // 聚合最大数量
}

// DefaultAlertThresholdConfig 默认预警阈值配置.
func DefaultAlertThresholdConfig() AlertThresholdConfig {
	return AlertThresholdConfig{
		WarningThreshold:   70,
		CriticalThreshold:  85,
		EmergencyThreshold: 95,
		CooldownDuration:   30 * time.Minute,
		RepeatAlert:        true,
		RepeatInterval:     1 * time.Hour,
		MaxRepeatCount:     3,
		EscalateOnRepeat:   true,
		AggregateAlerts:    true,
		AggregateInterval:  5 * time.Minute,
		AggregateMaxCount:  10,
	}
}

// NotificationChannel 通知渠道配置.
type NotificationChannel struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"` // email, webhook, slack, discord, telegram
	Enabled  bool                   `json:"enabled"`
	Config   map[string]interface{} `json:"config"`
	Severity []AlertSeverity        `json:"severity"` // 哪些严重级别使用此渠道
}

// AlertNotificationManager 预警通知管理器.
type AlertNotificationManager struct {
	mu              sync.RWMutex
	channels        map[string]*NotificationChannel
	thresholdConfig AlertThresholdConfig
	cooldownTracker map[string]time.Time // quotaID -> last alert time
	repeatTracker   map[string]int       // quotaID -> repeat count
	aggregateBuffer map[string][]*Alert  // severity -> pending alerts
	lastAggregate   time.Time
}

// NewAlertNotificationManager 创建预警通知管理器.
func NewAlertNotificationManager(thresholdConfig AlertThresholdConfig) *AlertNotificationManager {
	return &AlertNotificationManager{
		channels:        make(map[string]*NotificationChannel),
		thresholdConfig: thresholdConfig,
		cooldownTracker: make(map[string]time.Time),
		repeatTracker:   make(map[string]int),
		aggregateBuffer: make(map[string][]*Alert),
		lastAggregate:   time.Now(),
	}
}

// AddChannel 添加通知渠道.
func (m *AlertNotificationManager) AddChannel(channel *NotificationChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[channel.ID] = channel
}

// RemoveChannel 移除通知渠道.
func (m *AlertNotificationManager) RemoveChannel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

// GetChannels 获取所有通知渠道.
func (m *AlertNotificationManager) GetChannels() []*NotificationChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*NotificationChannel, 0, len(m.channels))
	for _, c := range m.channels {
		result = append(result, c)
	}
	return result
}

// ShouldAlert 判断是否应该发送告警.
func (m *AlertNotificationManager) ShouldAlert(quotaID string, severity AlertSeverity) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 检查冷却时间
	if lastAlert, exists := m.cooldownTracker[quotaID]; exists {
		if now.Sub(lastAlert) < m.thresholdConfig.CooldownDuration {
			return false
		}
	}

	// 检查重复次数
	if count, exists := m.repeatTracker[quotaID]; exists {
		if count >= m.thresholdConfig.MaxRepeatCount {
			return false
		}
	}

	return true
}

// RecordAlert 记录告警发送.
func (m *AlertNotificationManager) RecordAlert(quotaID string, severity AlertSeverity) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.cooldownTracker[quotaID] = now

	// 更新重复计数
	m.repeatTracker[quotaID]++
}

// ResetRepeatCounter 重置重复计数器.
func (m *AlertNotificationManager) ResetRepeatCounter(quotaID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.repeatTracker, quotaID)
}

// CleanupCooldown 清理过期的冷却记录.
func (m *AlertNotificationManager) CleanupCooldown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, t := range m.cooldownTracker {
		if t.Before(cutoff) {
			delete(m.cooldownTracker, id)
			delete(m.repeatTracker, id)
		}
	}
}

// DetermineSeverity 根据使用率确定告警严重级别.
func (m *AlertNotificationManager) DetermineSeverity(usagePercent float64) AlertSeverity {
	if usagePercent >= m.thresholdConfig.EmergencyThreshold {
		return AlertSeverityEmergency
	}
	if usagePercent >= m.thresholdConfig.CriticalThreshold {
		return AlertSeverityCritical
	}
	if usagePercent >= m.thresholdConfig.WarningThreshold {
		return AlertSeverityWarning
	}
	return AlertSeverityInfo
}

// ========== 自动清理增强 ==========

// LargeFileRule 大文件检测规则.
type LargeFileRule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	VolumeName   string        `json:"volume_name"`
	Path         string        `json:"path"`
	MinSize      uint64        `json:"min_size"`      // 最小文件大小
	FileTypes    []string      `json:"file_types"`    // 文件类型过滤（如 .log, .tmp）
	ExcludePaths []string      `json:"exclude_paths"` // 排除路径
	Action       CleanupAction `json:"action"`        // 处理动作
	ArchivePath  string        `json:"archive_path"`  // 归档路径（动作为 archive 时使用）
	NotifyBefore bool          `json:"notify_before"` // 执行前通知
	NotifyEmail  string        `json:"notify_email"`  // 通知邮箱
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// LargeFileResult 大文件检测结果.
type LargeFileResult struct {
	RuleID       string        `json:"rule_id"`
	RuleName     string        `json:"rule_name"`
	Path         string        `json:"path"`
	Files        []CleanupFile `json:"files"`
	TotalSize    uint64        `json:"total_size"`
	FileCount    int           `json:"file_count"`
	TopLargest   []CleanupFile `json:"top_largest"` // 最大的 N 个文件
	ScannedAt    time.Time     `json:"scanned_at"`
	ScanDuration time.Duration `json:"scan_duration"`
}

// ExpiredFileRule 过期文件清理规则.
type ExpiredFileRule struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	VolumeName      string        `json:"volume_name"`
	Path            string        `json:"path"`
	MaxAge          int           `json:"max_age"`          // 最大保留天数
	AccessType      string        `json:"access_type"`      // modification, access
	FileTypes       []string      `json:"file_types"`       // 文件类型过滤
	ExcludePaths    []string      `json:"exclude_paths"`    // 排除路径
	ExcludePatterns []string      `json:"exclude_patterns"` // 排除模式
	Action          CleanupAction `json:"action"`           // 处理动作
	ArchivePath     string        `json:"archive_path"`     // 归档路径
	DryRun          bool          `json:"dry_run"`          // 仅预览不执行
	Schedule        string        `json:"schedule"`         // cron 表达式
	Enabled         bool          `json:"enabled"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// ExpiredFileResult 过期文件检测结果.
type ExpiredFileResult struct {
	RuleID       string        `json:"rule_id"`
	RuleName     string        `json:"rule_name"`
	Path         string        `json:"path"`
	Files        []CleanupFile `json:"files"`
	TotalSize    uint64        `json:"total_size"`
	FileCount    int           `json:"file_count"`
	OldestFile   *CleanupFile  `json:"oldest_file,omitempty"`
	ScannedAt    time.Time     `json:"scanned_at"`
	ScanDuration time.Duration `json:"scan_duration"`
}

// CleanupRuleSet 清理规则集.
type CleanupRuleSet struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	VolumeName   string             `json:"volume_name"`
	LargeFiles   []*LargeFileRule   `json:"large_files"`
	ExpiredFiles []*ExpiredFileRule `json:"expired_files"`
	Enabled      bool               `json:"enabled"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// CleanupEnhancedManager 增强的清理管理器.
type CleanupEnhancedManager struct {
	quotaMgr         *Manager
	largeFileRules   map[string]*LargeFileRule
	expiredFileRules map[string]*ExpiredFileRule
	ruleSets         map[string]*CleanupRuleSet
	scanResults      map[string]interface{} // 最近扫描结果
	mu               sync.RWMutex
}

// NewCleanupEnhancedManager 创建增强清理管理器.
func NewCleanupEnhancedManager(quotaMgr *Manager) *CleanupEnhancedManager {
	return &CleanupEnhancedManager{
		quotaMgr:         quotaMgr,
		largeFileRules:   make(map[string]*LargeFileRule),
		expiredFileRules: make(map[string]*ExpiredFileRule),
		ruleSets:         make(map[string]*CleanupRuleSet),
		scanResults:      make(map[string]interface{}),
	}
}

// ========== 大文件检测 ==========

// CreateLargeFileRule 创建大文件检测规则.
func (m *CleanupEnhancedManager) CreateLargeFileRule(rule LargeFileRule) (*LargeFileRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.largeFileRules[rule.ID] = &rule
	return &rule, nil
}

// GetLargeFileRule 获取大文件检测规则.
func (m *CleanupEnhancedManager) GetLargeFileRule(id string) (*LargeFileRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.largeFileRules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}
	return rule, nil
}

// ListLargeFileRules 列出大文件检测规则.
func (m *CleanupEnhancedManager) ListLargeFileRules(volumeName string) []*LargeFileRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LargeFileRule, 0)
	for _, r := range m.largeFileRules {
		if volumeName == "" || r.VolumeName == volumeName {
			result = append(result, r)
		}
	}
	return result
}

// UpdateLargeFileRule 更新大文件检测规则.
func (m *CleanupEnhancedManager) UpdateLargeFileRule(id string, rule LargeFileRule) (*LargeFileRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.largeFileRules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.largeFileRules[id] = &rule
	return &rule, nil
}

// DeleteLargeFileRule 删除大文件检测规则.
func (m *CleanupEnhancedManager) DeleteLargeFileRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.largeFileRules[id]; !exists {
		return fmt.Errorf("规则不存在")
	}
	delete(m.largeFileRules, id)
	return nil
}

// ScanLargeFiles 扫描大文件.
func (m *CleanupEnhancedManager) ScanLargeFiles(ruleID string, topN int) (*LargeFileResult, error) {
	m.mu.RLock()
	rule, exists := m.largeFileRules[ruleID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}

	if !rule.Enabled {
		return nil, fmt.Errorf("规则已禁用")
	}

	startTime := time.Now()
	result := &LargeFileResult{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Path:      rule.Path,
		Files:     make([]CleanupFile, 0),
		ScannedAt: startTime,
	}

	// 获取目标路径
	targetPath := rule.Path
	if targetPath == "" && m.quotaMgr.storageMgr != nil {
		vol := m.quotaMgr.storageMgr.GetVolume(rule.VolumeName)
		if vol != nil {
			targetPath = vol.MountPoint
		}
	}

	if targetPath == "" {
		return nil, fmt.Errorf("无法确定目标路径")
	}

	// 扫描文件
	err := filepath.WalkDir(targetPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// 检查排除路径
			for _, exclude := range rule.ExcludePaths {
				if path == exclude {
					return filepath.SkipDir
				}
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		size := uint64(info.Size())
		if size < rule.MinSize {
			return nil
		}

		// 检查文件类型
		if len(rule.FileTypes) > 0 {
			ext := filepath.Ext(path)
			matched := false
			for _, ft := range rule.FileTypes {
				if ext == ft {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		file := CleanupFile{
			Path:    path,
			Size:    size,
			ModTime: info.ModTime(),
			Reason:  fmt.Sprintf("文件大小 %s 超过阈值 %s", formatBytes(size), formatBytes(rule.MinSize)),
		}

		result.Files = append(result.Files, file)
		result.TotalSize += size
		result.FileCount++

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 排序获取最大的 N 个文件
	if topN > 0 && len(result.Files) > topN {
		// 按大小降序排序
		sortedFiles := make([]CleanupFile, len(result.Files))
		copy(sortedFiles, result.Files)
		for i := 0; i < len(sortedFiles)-1; i++ {
			for j := i + 1; j < len(sortedFiles); j++ {
				if sortedFiles[j].Size > sortedFiles[i].Size {
					sortedFiles[i], sortedFiles[j] = sortedFiles[j], sortedFiles[i]
				}
			}
		}
		result.TopLargest = sortedFiles[:topN]
	} else {
		result.TopLargest = result.Files
	}

	result.ScanDuration = time.Since(startTime)

	// 缓存结果
	m.mu.Lock()
	m.scanResults["large_"+ruleID] = result
	m.mu.Unlock()

	return result, nil
}

// ========== 过期文件清理 ==========

// CreateExpiredFileRule 创建过期文件清理规则.
func (m *CleanupEnhancedManager) CreateExpiredFileRule(rule ExpiredFileRule) (*ExpiredFileRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.expiredFileRules[rule.ID] = &rule
	return &rule, nil
}

// GetExpiredFileRule 获取过期文件清理规则.
func (m *CleanupEnhancedManager) GetExpiredFileRule(id string) (*ExpiredFileRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.expiredFileRules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}
	return rule, nil
}

// ListExpiredFileRules 列出过期文件清理规则.
func (m *CleanupEnhancedManager) ListExpiredFileRules(volumeName string) []*ExpiredFileRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ExpiredFileRule, 0)
	for _, r := range m.expiredFileRules {
		if volumeName == "" || r.VolumeName == volumeName {
			result = append(result, r)
		}
	}
	return result
}

// UpdateExpiredFileRule 更新过期文件清理规则.
func (m *CleanupEnhancedManager) UpdateExpiredFileRule(id string, rule ExpiredFileRule) (*ExpiredFileRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.expiredFileRules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.expiredFileRules[id] = &rule
	return &rule, nil
}

// DeleteExpiredFileRule 删除过期文件清理规则.
func (m *CleanupEnhancedManager) DeleteExpiredFileRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.expiredFileRules[id]; !exists {
		return fmt.Errorf("规则不存在")
	}
	delete(m.expiredFileRules, id)
	return nil
}

// ScanExpiredFiles 扫描过期文件.
func (m *CleanupEnhancedManager) ScanExpiredFiles(ruleID string) (*ExpiredFileResult, error) {
	m.mu.RLock()
	rule, exists := m.expiredFileRules[ruleID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("规则不存在")
	}

	if !rule.Enabled {
		return nil, fmt.Errorf("规则已禁用")
	}

	startTime := time.Now()
	result := &ExpiredFileResult{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Path:      rule.Path,
		Files:     make([]CleanupFile, 0),
		ScannedAt: startTime,
	}

	// 获取目标路径
	targetPath := rule.Path
	if targetPath == "" && m.quotaMgr.storageMgr != nil {
		vol := m.quotaMgr.storageMgr.GetVolume(rule.VolumeName)
		if vol != nil {
			targetPath = vol.MountPoint
		}
	}

	if targetPath == "" {
		return nil, fmt.Errorf("无法确定目标路径")
	}

	now := time.Now()
	cutoffDate := now.AddDate(0, 0, -rule.MaxAge)

	// 扫描文件
	err := filepath.WalkDir(targetPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			// 检查排除路径
			for _, exclude := range rule.ExcludePaths {
				if path == exclude {
					return filepath.SkipDir
				}
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// 检查文件时间
		var fileTime time.Time
		if rule.AccessType == "access" {
			if atime, ok := getFileAccessTime(info.Sys()); ok {
				fileTime = atime
			} else {
				fileTime = info.ModTime()
			}
		} else {
			fileTime = info.ModTime()
		}

		if fileTime.After(cutoffDate) {
			return nil
		}

		// 检查文件类型
		if len(rule.FileTypes) > 0 {
			ext := filepath.Ext(path)
			matched := false
			for _, ft := range rule.FileTypes {
				if ext == ft {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// 检查排除模式
		for _, pattern := range rule.ExcludePatterns {
			matched, _ := filepath.Match(pattern, d.Name())
			if matched {
				return nil
			}
		}

		size := uint64(info.Size())
		file := CleanupFile{
			Path:    path,
			Size:    size,
			ModTime: info.ModTime(),
			Reason:  fmt.Sprintf("文件已过期 %d 天", int(now.Sub(fileTime).Hours()/24)),
		}

		// 获取访问时间
		if atime, ok := getFileAccessTime(info.Sys()); ok {
			file.AccTime = &atime
		}

		result.Files = append(result.Files, file)
		result.TotalSize += size
		result.FileCount++

		// 记录最旧文件
		if result.OldestFile == nil || file.ModTime.Before(result.OldestFile.ModTime) {
			result.OldestFile = &file
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	result.ScanDuration = time.Since(startTime)

	// 缓存结果
	m.mu.Lock()
	m.scanResults["expired_"+ruleID] = result
	m.mu.Unlock()

	return result, nil
}

// ExecuteExpiredFileCleanup 执行过期文件清理.
func (m *CleanupEnhancedManager) ExecuteExpiredFileCleanup(ruleID string) (*ExpiredFileResult, error) {
	// 先扫描
	result, err := m.ScanExpiredFiles(ruleID)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	rule := m.expiredFileRules[ruleID]
	m.mu.RUnlock()

	if rule == nil {
		return nil, fmt.Errorf("规则不存在")
	}

	// 如果是 dry-run 模式，不执行实际清理
	if rule.DryRun {
		return result, nil
	}

	// 执行清理
	for _, file := range result.Files {
		switch rule.Action {
		case CleanupActionDelete:
			_ = os.Remove(file.Path)
		case CleanupActionArchive:
			if rule.ArchivePath != "" {
				_ = os.MkdirAll(rule.ArchivePath, 0750)
				_ = os.Rename(file.Path, filepath.Join(rule.ArchivePath, filepath.Base(file.Path)))
			}
		}
	}

	return result, nil
}

// GetScanResult 获取扫描结果.
func (m *CleanupEnhancedManager) GetScanResult(resultKey string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, exists := m.scanResults[resultKey]
	if !exists {
		return nil, fmt.Errorf("结果不存在")
	}
	return result, nil
}

// ========== 规则集管理 ==========

// CreateRuleSet 创建规则集.
func (m *CleanupEnhancedManager) CreateRuleSet(ruleSet CleanupRuleSet) (*CleanupRuleSet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ruleSet.ID == "" {
		ruleSet.ID = generateID()
	}
	ruleSet.CreatedAt = time.Now()
	ruleSet.UpdatedAt = time.Now()

	m.ruleSets[ruleSet.ID] = &ruleSet
	return &ruleSet, nil
}

// GetRuleSet 获取规则集.
func (m *CleanupEnhancedManager) GetRuleSet(id string) (*CleanupRuleSet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ruleSet, exists := m.ruleSets[id]
	if !exists {
		return nil, fmt.Errorf("规则集不存在")
	}
	return ruleSet, nil
}

// ListRuleSets 列出规则集.
func (m *CleanupEnhancedManager) ListRuleSets(volumeName string) []*CleanupRuleSet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CleanupRuleSet, 0)
	for _, rs := range m.ruleSets {
		if volumeName == "" || rs.VolumeName == volumeName {
			result = append(result, rs)
		}
	}
	return result
}

// ExecuteRuleSet 执行规则集.
func (m *CleanupEnhancedManager) ExecuteRuleSet(id string) (map[string]interface{}, error) {
	m.mu.RLock()
	ruleSet, exists := m.ruleSets[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("规则集不存在")
	}

	results := make(map[string]interface{})

	// 执行大文件扫描
	for _, rule := range ruleSet.LargeFiles {
		if rule.Enabled {
			result, err := m.ScanLargeFiles(rule.ID, 10)
			if err == nil {
				results["large_"+rule.ID] = result
			}
		}
	}

	// 执行过期文件扫描
	for _, rule := range ruleSet.ExpiredFiles {
		if rule.Enabled {
			result, err := m.ScanExpiredFiles(rule.ID)
			if err == nil {
				results["expired_"+rule.ID] = result
			}
		}
	}

	return results, nil
}

// SaveRules 保存规则到文件.
func (m *CleanupEnhancedManager) SaveRules(configPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := struct {
		LargeFileRules   []*LargeFileRule   `json:"large_file_rules"`
		ExpiredFileRules []*ExpiredFileRule `json:"expired_file_rules"`
		RuleSets         []*CleanupRuleSet  `json:"rule_sets"`
	}{
		LargeFileRules:   make([]*LargeFileRule, 0),
		ExpiredFileRules: make([]*ExpiredFileRule, 0),
		RuleSets:         make([]*CleanupRuleSet, 0),
	}

	for _, r := range m.largeFileRules {
		data.LargeFileRules = append(data.LargeFileRules, r)
	}
	for _, r := range m.expiredFileRules {
		data.ExpiredFileRules = append(data.ExpiredFileRules, r)
	}
	for _, rs := range m.ruleSets {
		data.RuleSets = append(data.RuleSets, rs)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, jsonData, 0600)
}

// LoadRules 从文件加载规则.
func (m *CleanupEnhancedManager) LoadRules(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var loaded struct {
		LargeFileRules   []*LargeFileRule   `json:"large_file_rules"`
		ExpiredFileRules []*ExpiredFileRule `json:"expired_file_rules"`
		RuleSets         []*CleanupRuleSet  `json:"rule_sets"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range loaded.LargeFileRules {
		m.largeFileRules[r.ID] = r
	}
	for _, r := range loaded.ExpiredFileRules {
		m.expiredFileRules[r.ID] = r
	}
	for _, rs := range loaded.RuleSets {
		m.ruleSets[rs.ID] = rs
	}

	return nil
}
