// Package aidatadedup 提供 AI 驱动的数据去重管理器
package aidatadedup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 数据去重管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *DedupConfig
	scanResults map[string]*ScanResult
	groups      map[string]*DuplicateGroup
	reports     []*DedupReport
	stopChan    chan struct{}
	running     bool
}

// NewManager 创建数据去重管理器
func NewManager(logger *zap.Logger, config *DedupConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultDedupConfig()
	}

	return &Manager{
		logger:      logger,
		config:      config,
		scanResults: make(map[string]*ScanResult),
		groups:      make(map[string]*DuplicateGroup),
		reports:     make([]*DedupReport, 0),
		stopChan:    make(chan struct{}),
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager already running")
	}

	if !m.config.Enabled {
		return fmt.Errorf("dedup manager is disabled")
	}

	m.running = true
	m.stopChan = make(chan struct{})

	go m.runAutoScan(ctx)

	m.logger.Info("dedup manager started",
		zap.String("strategy", string(m.config.DefaultStrategy)),
		zap.Float64("threshold", m.config.SimilarityThreshold))

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("manager not running")
	}

	close(m.stopChan)
	m.running = false

	m.logger.Info("dedup manager stopped")
	return nil
}

// IsRunning 检查是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// computeHash 计算文件哈希
func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// runAutoScan 自动扫描协程
func (m *Manager) runAutoScan(ctx context.Context) {
	interval := time.Duration(m.config.ScanIntervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			if len(m.config.ExcludedPaths) > 0 {
				m.logger.Debug("auto scan triggered")
				// 自动扫描逻辑 - 实际扫描需要文件系统访问
			}
		}
	}
}

// Scan 扫描路径查找重复文件
func (m *Manager) Scan(ctx context.Context, req *DedupRequest) (*ScanResult, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("manager not running")
	}

	startTime := time.Now()
	resultID := generateID()

	result := &ScanResult{
		ID:                  resultID,
		ScanPath:            req.Paths[0],
		SimilarityThreshold: req.SimilarityThreshold,
		StartTime:           startTime,
		Status:              StatusAnalyzing,
		DuplicateGroups:     make([]*DuplicateGroup, 0),
	}

	if result.SimilarityThreshold == 0 {
		result.SimilarityThreshold = m.config.SimilarityThreshold
	}

	m.mu.Lock()
	m.scanResults[resultID] = result
	m.mu.Unlock()

	m.logger.Info("scan started",
		zap.String("id", resultID),
		zap.Strings("paths", req.Paths),
		zap.String("strategy", string(req.Strategy)))

	// 模拟扫描 - 实际实现需要遍历文件系统
	go func() {
		time.Sleep(100 * time.Millisecond) // 模拟扫描耗时

		endTime := time.Now()
		m.mu.Lock()
		result.EndTime = endTime
		result.Duration = endTime.Sub(startTime)
		result.Status = StatusMerged
		m.mu.Unlock()

		m.logger.Info("scan completed",
			zap.String("id", resultID),
			zap.Duration("duration", result.Duration))
	}()

	return result, nil
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult(id string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.scanResults[id]
	if !ok {
		return nil, fmt.Errorf("scan result not found: %s", id)
	}
	return result, nil
}

// ListScanResults 列出所有扫描结果
func (m *Manager) ListScanResults() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*ScanResult, 0, len(m.scanResults))
	for _, r := range m.scanResults {
		results = append(results, r)
	}
	return results
}

// GetDuplicateGroup 获取重复组
func (m *Manager) GetDuplicateGroup(id string) (*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[id]
	if !ok {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return group, nil
}

// ListDuplicateGroups 列出所有重复组
func (m *Manager) ListDuplicateGroups() []*DuplicateGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*DuplicateGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// MergeFiles 合并重复文件
func (m *Manager) MergeFiles(ctx context.Context, req *MergeRequest) (*DedupReport, error) {
	if !m.IsRunning() {
		return nil, fmt.Errorf("manager not running")
	}

	m.mu.RLock()
	group, ok := m.groups[req.GroupID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("group not found: %s", req.GroupID)
	}

	report := &DedupReport{
		ID:           generateID(),
		ScanResultID: req.GroupID,
		StartTime:    time.Now(),
		TotalFiles:   len(group.Files),
	}

	// 选择保留的文件
	var keepFile *FileEntry
	if req.KeepFileID != "" {
		for _, f := range group.Files {
			if f.ID == req.KeepFileID {
				keepFile = f
				break
			}
		}
	}

	if keepFile == nil {
		keepFile = m.selectBestFile(group.Files, req.Strategy)
	}

	if keepFile == nil {
		return nil, fmt.Errorf("no file to keep")
	}

	// 模拟合并操作
	m.mu.Lock()
	group.Status = StatusMerged
	group.Recommended = keepFile
	group.UpdatedAt = time.Now()

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)
	report.FilesMerged = 1
	report.FilesDeleted = len(group.Files) - 1
	report.SpaceSaved = group.SaveableSize

	m.reports = append(m.reports, report)
	m.mu.Unlock()

	m.logger.Info("files merged",
		zap.String("group_id", req.GroupID),
		zap.String("kept_file", keepFile.Path),
		zap.Int64("space_saved", report.SpaceSaved))

	return report, nil
}

// selectBestFile 选择最佳保留文件
func (m *Manager) selectBestFile(files []*FileEntry, strategy MergeStrategy) *FileEntry {
	if len(files) == 0 {
		return nil
	}

	best := files[0]
	for _, f := range files[1:] {
		switch strategy {
		case MergeKeepNewest:
			if f.ModTime.After(best.ModTime) {
				best = f
			}
		case MergeKeepOldest:
			if f.ModTime.Before(best.ModTime) {
				best = f
			}
		case MergeKeepLargest:
			if f.Size > best.Size {
				best = f
			}
		default:
			if f.ModTime.After(best.ModTime) {
				best = f
			}
		}
	}
	return best
}

// AnalyzeFile 分析文件相似度
func (m *Manager) AnalyzeFile(ctx context.Context, file *FileEntry) (*AIAnalysisResult, error) {
	if !m.config.EnableAI {
		return nil, fmt.Errorf("AI analysis is disabled")
	}

	result := &AIAnalysisResult{
		FileID:     file.ID,
		AnalyzedAt: time.Now(),
		Features:   make(map[string]float64),
	}

	// 模拟 AI 分析
	result.ContentType = string(file.FileType)
	result.Confidence = 0.95
	result.IsDuplicate = false

	m.logger.Debug("file analyzed",
		zap.String("file_id", file.ID),
		zap.String("type", result.ContentType),
		zap.Float64("confidence", result.Confidence))

	return result, nil
}

// GetReport 获取去重报告
func (m *Manager) GetReport(id string) (*DedupReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.reports {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, fmt.Errorf("report not found: %s", id)
}

// ListReports 列出所有报告
func (m *Manager) ListReports() []*DedupReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*DedupReport, len(m.reports))
	copy(reports, m.reports)
	return reports
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *DedupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *DedupConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSaved := int64(0)
	for _, r := range m.reports {
		totalSaved += r.SpaceSaved
	}

	return map[string]interface{}{
		"scan_results":      len(m.scanResults),
		"duplicate_groups":  len(m.groups),
		"reports":           len(m.reports),
		"total_space_saved": totalSaved,
		"running":           m.running,
	}
}

// AddFileEntry 添加文件条目（用于测试）
func (m *Manager) AddFileEntry(entry *FileEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.GroupID != "" {
		group, ok := m.groups[entry.GroupID]
		if ok {
			group.Files = append(group.Files, entry)
			group.TotalSize += entry.Size
			return
		}
	}
}

// AddDuplicateGroup 添加重复组（用于测试）
func (m *Manager) AddDuplicateGroup(group *DuplicateGroup) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups[group.ID] = group
}

// ResolveDuplicateGroup 解决重复组
func (m *Manager) ResolveDuplicateGroup(groupID string, keepFileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found: %s", groupID)
	}

	found := false
	for _, f := range group.Files {
		if f.ID == keepFileID {
			found = true
			group.Recommended = f
			break
		}
	}

	if !found {
		return fmt.Errorf("file not found in group: %s", keepFileID)
	}

	group.Status = StatusDuplicate
	group.UpdatedAt = time.Now()
	return nil
}

// 为避免 unused import 错误
var _ = filepath.Join
