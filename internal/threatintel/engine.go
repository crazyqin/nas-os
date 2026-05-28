// Package threatintel - engine.go 实现威胁情报引擎核心，包括多源情报聚合、
// 威胁评分计算和告警管理功能。
package threatintel

import (
	"fmt"
	"sync"
	"time"
)

// Engine 威胁情报引擎
type Engine struct {
	config     *ThreatIntelConfig
	feeds      map[string]*ThreatFeed
	iocs       map[string]*IOC
	alerts     map[string]*Alert
	scanResults map[string]*ScanResult
	scanMgr    *ScanManager
	score      *ThreatScore
	mu         sync.RWMutex
}

// NewEngine 创建威胁情报引擎
func NewEngine(config *ThreatIntelConfig) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	return &Engine{
		config:      config,
		feeds:       make(map[string]*ThreatFeed),
		iocs:        make(map[string]*IOC),
		alerts:      make(map[string]*Alert),
		scanResults: make(map[string]*ScanResult),
		scanMgr:     NewScanManager(),
		score: &ThreatScore{
			Level:     "safe",
			UpdatedAt: time.Now(),
		},
	}
}

// ============================================================
// 情报源管理
// ============================================================

// AddFeed 添加情报源
func (e *Engine) AddFeed(feed *ThreatFeed) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.feeds[feed.ID]; exists {
		return ErrFeedExists
	}

	feed.CreatedAt = time.Now()
	if feed.Status == "" {
		feed.Status = FeedStatusActive
	}

	e.feeds[feed.ID] = feed
	return nil
}

// RemoveFeed 移除情报源
func (e *Engine) RemoveFeed(feedID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.feeds[feedID]; !exists {
		return ErrFeedNotFound
	}

	// 删除该源的所有 IOC
	for id, ioc := range e.iocs {
		if ioc.SourceID == feedID {
			delete(e.iocs, id)
		}
	}

	delete(e.feeds, feedID)
	return nil
}

// GetFeed 获取情报源信息
func (e *Engine) GetFeed(feedID string) (*ThreatFeed, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	feed, exists := e.feeds[feedID]
	if !exists {
		return nil, ErrFeedNotFound
	}

	return feed, nil
}

// ListFeeds 列出所有情报源
func (e *Engine) ListFeeds() []*ThreatFeed {
	e.mu.RLock()
	defer e.mu.RUnlock()

	feeds := make([]*ThreatFeed, 0, len(e.feeds))
	for _, f := range e.feeds {
		feeds = append(feeds, f)
	}
	return feeds
}

// UpdateFeedStatus 更新情报源状态
func (e *Engine) UpdateFeedStatus(feedID string, status FeedStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	feed, exists := e.feeds[feedID]
	if !exists {
		return ErrFeedNotFound
	}

	feed.Status = status
	feed.LastUpdate = time.Now()
	return nil
}

// ============================================================
// IOC 管理
// ============================================================

// AddIOC 添加 IOC
func (e *Engine) AddIOC(ioc *IOC) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.iocs) >= e.config.MaxIOCs {
		return NewThreatIntelError("MAX_IOCS_REACHED", "已达到 IOC 数量上限", nil)
	}

	ioc.CreatedAt = time.Now()
	ioc.FirstSeen = time.Now()
	ioc.LastSeen = time.Now()

	e.iocs[ioc.ID] = ioc

	// 更新情报源 IOC 计数
	if feed, exists := e.feeds[ioc.SourceID]; exists {
		feed.IOCCount++
	}

	return nil
}

// RemoveIOC 移除 IOC
func (e *Engine) RemoveIOC(iocID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ioc, exists := e.iocs[iocID]
	if !exists {
		return ErrIOCNotFound
	}

	// 更新情报源 IOC 计数
	if feed, exists := e.feeds[ioc.SourceID]; exists {
		feed.IOCCount--
	}

	delete(e.iocs, iocID)
	return nil
}

// GetIOC 获取 IOC 详情
func (e *Engine) GetIOC(iocID string) (*IOC, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ioc, exists := e.iocs[iocID]
	if !exists {
		return nil, ErrIOCNotFound
	}

	return ioc, nil
}

// ListIOCs 列出所有 IOC
func (e *Engine) ListIOCs() []*IOC {
	e.mu.RLock()
	defer e.mu.RUnlock()

	iocs := make([]*IOC, 0, len(e.iocs))
	for _, ioc := range e.iocs {
		iocs = append(iocs, ioc)
	}
	return iocs
}

// LookupIOC 根据类型和值查找 IOC
func (e *Engine) LookupIOC(iocType IOCType, value string) *IOC {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, ioc := range e.iocs {
		if ioc.Type == iocType && ioc.Value == value {
			return ioc
		}
	}
	return nil
}

// BlockIOC 阻断 IOC
func (e *Engine) BlockIOC(iocID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ioc, exists := e.iocs[iocID]
	if !exists {
		return ErrIOCNotFound
	}

	ioc.Blocked = true
	return nil
}

// UnblockIOC 取消阻断 IOC
func (e *Engine) UnblockIOC(iocID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ioc, exists := e.iocs[iocID]
	if !exists {
		return ErrIOCNotFound
	}

	ioc.Blocked = false
	return nil
}

// GetBlockedIOCs 获取所有已阻断的 IOC
func (e *Engine) GetBlockedIOCs() []*IOC {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var blocked []*IOC
	for _, ioc := range e.iocs {
		if ioc.Blocked {
			blocked = append(blocked, ioc)
		}
	}
	return blocked
}

// ============================================================
// 告警管理
// ============================================================

// CreateAlert 创建告警
func (e *Engine) CreateAlert(alert *Alert) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.alerts) >= e.config.MaxAlerts {
		return NewThreatIntelError("MAX_ALERTS_REACHED", "已达到告警数量上限", nil)
	}

	alert.Status = AlertStatusOpen
	alert.FirstSeen = time.Now()
	alert.LastSeen = time.Now()
	alert.CreatedAt = time.Now()

	e.alerts[alert.ID] = alert
	return nil
}

// AcknowledgeAlert 确认告警
func (e *Engine) AcknowledgeAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	alert.Status = AlertStatusAcknowledged
	now := time.Now()
	alert.AckedAt = &now
	return nil
}

// ResolveAlert 解决告警
func (e *Engine) ResolveAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	alert.Status = AlertStatusResolved
	now := time.Now()
	alert.ResolvedAt = &now
	return nil
}

// MarkFalsePositive 标记为误报
func (e *Engine) MarkFalsePositive(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return ErrAlertNotFound
	}

	alert.Status = AlertStatusFalsePositive
	now := time.Now()
	alert.ResolvedAt = &now
	return nil
}

// GetAlert 获取告警详情
func (e *Engine) GetAlert(alertID string) (*Alert, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return nil, ErrAlertNotFound
	}

	return alert, nil
}

// ListAlerts 列出所有告警
func (e *Engine) ListAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	alerts := make([]*Alert, 0, len(e.alerts))
	for _, a := range e.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// GetOpenAlerts 获取未处理告警
func (e *Engine) GetOpenAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var openAlerts []*Alert
	for _, alert := range e.alerts {
		if alert.Status == AlertStatusOpen {
			openAlerts = append(openAlerts, alert)
		}
	}
	return openAlerts
}

// ============================================================
// 扫描结果管理
// ============================================================

// SaveScanResult 保存扫描结果
func (e *Engine) SaveScanResult(result *ScanResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.scanResults[result.ID] = result
}

// GetScanResult 获取扫描结果
func (e *Engine) GetScanResult(scanID string) (*ScanResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, exists := e.scanResults[scanID]
	if !exists {
		return nil, NewThreatIntelError("SCAN_NOT_FOUND", "扫描结果不存在", nil)
	}

	return result, nil
}

// ListScanResults 列出所有扫描结果
func (e *Engine) ListScanResults() []*ScanResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*ScanResult, 0, len(e.scanResults))
	for _, r := range e.scanResults {
		results = append(results, r)
	}
	return results
}

// ============================================================
// 威胁评分
// ============================================================

// CalculateThreatScore 计算总体威胁评分
func (e *Engine) CalculateThreatScore() *ThreatScore {
	e.mu.Lock()
	defer e.mu.Unlock()

	score := &ThreatScore{
		UpdatedAt: time.Now(),
	}

	// 计算 IOC 威胁评分
	totalIOCs := len(e.iocs)
	blockedIOCs := 0
	highSeverityIOCs := 0
	for _, ioc := range e.iocs {
		if ioc.Blocked {
			blockedIOCs++
		}
		if ioc.Severity == SeverityHigh || ioc.Severity == SeverityCritical {
			highSeverityIOCs++
		}
	}

	if totalIOCs > 0 {
		score.IOCScore = (highSeverityIOCs * 100) / totalIOCs
	}

	// 计算漏洞评分
	totalVulns := 0
	criticalVulns := 0
	for _, result := range e.scanResults {
		for _, vuln := range result.Vulnerabilities {
			totalVulns++
			if vuln.Severity == SeverityCritical {
				criticalVulns++
			}
		}
	}

	if totalVulns > 0 {
		score.VulnScore = (criticalVulns * 100) / totalVulns
	}

	// 计算网络评分（基于扫描结果）
	totalOpenPorts := 0
	for _, result := range e.scanResults {
		totalOpenPorts += result.OpenPorts
	}
	if totalOpenPorts > 20 {
		score.NetworkScore = 80
	} else if totalOpenPorts > 10 {
		score.NetworkScore = 50
	} else {
		score.NetworkScore = 20
	}

	// 信誉评分（基于未阻断的高威胁 IOC）
	unblockedHigh := 0
	for _, ioc := range e.iocs {
		if !ioc.Blocked && (ioc.Severity == SeverityHigh || ioc.Severity == SeverityCritical) {
			unblockedHigh++
		}
	}
	if unblockedHigh > 0 {
		score.ReputationScore = 70
	} else {
		score.ReputationScore = 100
	}

	// 总体评分
	score.Overall = (score.IOCScore + score.VulnScore + score.NetworkScore + score.ReputationScore) / 4

	// 确定级别
	switch {
	case score.Overall >= 80:
		score.Level = "critical"
		score.Summary = "检测到严重威胁，建议立即处理"
	case score.Overall >= 60:
		score.Level = "high"
		score.Summary = "检测到高威胁，建议尽快处理"
	case score.Overall >= 40:
		score.Level = "medium"
		score.Summary = "存在中等威胁风险"
	case score.Overall >= 20:
		score.Level = "low"
		score.Summary = "威胁级别较低"
	default:
		score.Level = "safe"
		score.Summary = "系统安全状态良好"
	}

	e.score = score
	return score
}

// GetThreatScore 获取当前威胁评分
func (e *Engine) GetThreatScore() *ThreatScore {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.score
}

// ============================================================
// 统计与扫描控制
// ============================================================

// GetStats 获取统计信息
func (e *Engine) GetStats() *ThreatIntelStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &ThreatIntelStats{
		TotalFeeds:  len(e.feeds),
		TotalIOCs:   len(e.iocs),
		TotalAlerts: len(e.alerts),
		ThreatScore: e.score,
	}

	for _, feed := range e.feeds {
		if feed.Status == FeedStatusActive {
			stats.ActiveFeeds++
		}
	}

	for _, ioc := range e.iocs {
		if ioc.Blocked {
			stats.BlockedIOCs++
		}
	}

	for _, alert := range e.alerts {
		if alert.Status == AlertStatusOpen {
			stats.OpenAlerts++
		}
	}

	stats.ScansPerformed = len(e.scanResults)

	return stats
}

// StartScan 开始扫描
func (e *Engine) StartScan(scanID, scanType, target string) (*ScanResult, error) {
	if !e.scanMgr.TryStartScan() {
		return nil, ErrScanInProgress
	}

	result := &ScanResult{
		ID:        scanID,
		ScanType:  scanType,
		Status:    ScanStatusRunning,
		Target:    target,
		StartTime: time.Now(),
	}

	e.SaveScanResult(result)
	return result, nil
}

// CompleteScan 完成扫描
func (e *Engine) CompleteScan(scanID string, result *ScanResult) {
	e.scanMgr.FinishScan()
	result.Status = ScanStatusComplete
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	e.SaveScanResult(result)
}

// FailScan 标记扫描失败
func (e *Engine) FailScan(scanID string, reason string) {
	e.scanMgr.FinishScan()
	result, exists := e.scanResults[scanID]
	if exists {
		result.Status = ScanStatusFailed
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Summary = reason
	}
}

// IsScanning 是否正在扫描
func (e *Engine) IsScanning() bool {
	return e.scanMgr.IsScanning()
}

// CleanupExpiredIOCs 清理过期 IOC
func (e *Engine) CleanupExpiredIOCs() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	cleaned := 0
	now := time.Now()
	for id, ioc := range e.iocs {
		if ioc.ExpiresAt != nil && now.After(*ioc.ExpiresAt) {
			delete(e.iocs, id)
			cleaned++
		}
	}
	return cleaned
}

// FormatAlertMessage 格式化告警消息
func FormatAlertMessage(alert *Alert) string {
	return fmt.Sprintf("[%s] %s - %s (Score: %d)",
		alert.Severity, alert.Title, alert.Description, alert.Score)
}
