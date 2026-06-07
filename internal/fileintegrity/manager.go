// Package fileintegrity 提供文件完整性监控核心逻辑
package fileintegrity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 文件完整性监控管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *MonitorConfig
	baselines   map[string]*Baseline
	rules       map[string]*MonitorRule
	changes     []*FileChange
	alerts      []*Alert
	auditLog    []*AuditLogEntry
	scanResults []*ScanResult
	stopChan    chan struct{}
	running     bool
	watchers    map[string]context.CancelFunc
}

// NewManager 创建文件完整性监控管理器
func NewManager(logger *zap.Logger, config *MonitorConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultMonitorConfig()
	}
	return &Manager{
		logger:      logger,
		config:      config,
		baselines:   make(map[string]*Baseline),
		rules:       make(map[string]*MonitorRule),
		changes:     make([]*FileChange, 0),
		alerts:      make([]*Alert, 0),
		auditLog:    make([]*AuditLogEntry, 0),
		scanResults: make([]*ScanResult, 0),
		stopChan:    make(chan struct{}),
		watchers:    make(map[string]context.CancelFunc),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager is already running")
	}
	if !m.config.Enabled {
		m.logger.Info("file integrity monitoring is disabled")
		return nil
	}

	m.running = true
	m.logger.Info("file integrity monitor started",
		zap.String("algorithm", string(m.config.DefaultAlgorithm)),
		zap.Bool("realtime", m.config.RealTimeWatch))

	m.addAuditLog("start", "system", "FIM manager started")

	if m.config.RealTimeWatch {
		go m.watchLoop(ctx)
	}
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	close(m.stopChan)
	for id, cancel := range m.watchers {
		cancel()
		m.logger.Debug("stopped watcher", zap.String("rule_id", id))
	}
	m.watchers = make(map[string]context.CancelFunc)
	m.running = false
	m.logger.Info("file integrity monitor stopped")
	m.addAuditLog("stop", "system", "FIM manager stopped")
}

// IsRunning 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus 获取监控状态
func (m *Manager) GetStatus() MonitorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.running {
		return MonitorStatusIdle
	}
	if len(m.watchers) > 0 {
		return MonitorStatusWatching
	}
	return MonitorStatusIdle
}

// CreateBaseline 创建基线
func (m *Manager) CreateBaseline(ctx context.Context, name, desc string, paths []string, algo HashAlgorithm) (*Baseline, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	if algo == "" {
		algo = m.config.DefaultAlgorithm
	}

	baseline := &Baseline{
		ID:            generateID(),
		Name:          name,
		Description:   desc,
		Entries:       make(map[string]*FileEntry),
		HashAlgorithm: algo,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Metadata:      make(map[string]string),
	}

	for _, root := range paths {
		if err := m.scanPath(ctx, root, baseline, nil, nil); err != nil {
			m.logger.Warn("scan path error", zap.String("path", root), zap.Error(err))
		}
	}

	baseline.FileCount = len(baseline.Entries)
	var totalSize int64
	for _, e := range baseline.Entries {
		totalSize += e.Size
	}
	baseline.TotalSize = totalSize

	m.mu.Lock()
	m.baselines[baseline.ID] = baseline
	m.mu.Unlock()

	m.addAuditLog("baseline_create", baseline.ID,
		fmt.Sprintf("Created baseline '%s' with %d files", name, baseline.FileCount))
	m.logger.Info("baseline created",
		zap.String("id", baseline.ID),
		zap.String("name", name),
		zap.Int("files", baseline.FileCount))

	return baseline, nil
}

// GetBaseline 获取基线
func (m *Manager) GetBaseline(id string) (*Baseline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.baselines[id]
	if !ok {
		return nil, fmt.Errorf("baseline %s not found", id)
	}
	return b, nil
}

// ListBaselines 列出基线
func (m *Manager) ListBaselines(page, pageSize int) *PaginatedResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*Baseline, 0, len(m.baselines))
	for _, b := range m.baselines {
		all = append(all, b)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return &PaginatedResult{Total: total, Page: page, PageSize: pageSize, Items: []*Baseline{}}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &PaginatedResult{Total: total, Page: page, PageSize: pageSize, Items: all[start:end]}
}

// DeleteBaseline 删除基线
func (m *Manager) DeleteBaseline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.baselines[id]; !ok {
		return fmt.Errorf("baseline %s not found", id)
	}
	delete(m.baselines, id)
	m.addAuditLog("baseline_delete", id, "Deleted baseline")
	return nil
}

// AddRule 添加监控规则
func (m *Manager) AddRule(rule *MonitorRule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(rule.Paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}

	rule.ID = generateID()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	if rule.HashAlgorithm == "" {
		rule.HashAlgorithm = m.config.DefaultAlgorithm
	}
	if rule.AlertLevel == "" {
		rule.AlertLevel = AlertWarning
	}
	if rule.MaxDepth <= 0 {
		rule.MaxDepth = 10
	}

	m.mu.Lock()
	m.rules[rule.ID] = rule
	m.mu.Unlock()

	m.addAuditLog("rule_add", rule.ID, fmt.Sprintf("Added rule '%s'", rule.Name))
	m.logger.Info("rule added",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name),
		zap.Strings("paths", rule.Paths))

	if m.running && m.config.RealTimeWatch {
		m.startWatcher(rule)
	}
	return nil
}

// UpdateRule 更新监控规则
func (m *Manager) UpdateRule(rule *MonitorRule) error {
	m.mu.Lock()
	existing, ok := m.rules[rule.ID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("rule %s not found", rule.ID)
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	m.mu.Unlock()

	if m.running && m.config.RealTimeWatch {
		m.stopWatcher(rule.ID)
		if rule.Enabled {
			m.startWatcher(rule)
		}
	}
	m.addAuditLog("rule_update", rule.ID, fmt.Sprintf("Updated rule '%s'", rule.Name))
	return nil
}

// DeleteRule 删除监控规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	if _, ok := m.rules[id]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("rule %s not found", id)
	}
	delete(m.rules, id)
	m.mu.Unlock()

	m.stopWatcher(id)
	m.addAuditLog("rule_delete", id, "Deleted rule")
	return nil
}

// GetRule 获取监控规则
func (m *Manager) GetRule(id string) (*MonitorRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	return r, nil
}

// ListRules 列出监控规则
func (m *Manager) ListRules() []*MonitorRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]*MonitorRule, 0, len(m.rules))
	for _, r := range m.rules {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all
}

// RunScan 执行扫描
func (m *Manager) RunScan(ctx context.Context, req *ScanRequest) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{
		ID:        generateID(),
		Mode:      req.Mode,
		RuleIDs:   req.RuleIDs,
		StartedAt: start,
	}

	m.logger.Info("scan started",
		zap.String("id", result.ID),
		zap.String("mode", string(req.Mode)))

	// 确定要扫描的规则
	var scanRules []*MonitorRule
	m.mu.RLock()
	if len(req.RuleIDs) > 0 {
		for _, id := range req.RuleIDs {
			if r, ok := m.rules[id]; ok {
				scanRules = append(scanRules, r)
			}
		}
	} else {
		for _, r := range m.rules {
			if r.Enabled {
				scanRules = append(scanRules, r)
			}
		}
	}
	m.mu.RUnlock()

	// 确定扫描参数
	var paths []string
	var excludePaths []string
	var excludePatterns []string
	var algo HashAlgorithm

	if len(req.Paths) > 0 {
		paths = req.Paths
		algo = m.config.DefaultAlgorithm
	} else {
		for _, r := range scanRules {
			paths = append(paths, r.Paths...)
			excludePaths = append(excludePaths, r.ExcludePaths...)
			excludePatterns = append(excludePatterns, r.ExcludePatterns...)
			if algo == "" {
				algo = r.HashAlgorithm
			}
		}
	}
	if algo == "" {
		algo = m.config.DefaultAlgorithm
	}

	// 执行扫描
	changes := make([]*FileChange, 0)
	var scanErrors []string

	for _, root := range paths {
		err := m.scanForChanges(ctx, root, algo, excludePaths, excludePatterns, req.ForceRehash, &changes, &result.FilesScanned)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("%s: %v", root, err))
		}
	}

	result.ChangesFound = len(changes)
	result.Changes = changes
	result.Errors = scanErrors
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)

	for _, ch := range changes {
		m.mu.Lock()
		m.changes = append(m.changes, ch)
		m.mu.Unlock()
		m.triggerAlerts(ch)
	}

	m.mu.Lock()
	m.scanResults = append(m.scanResults, result)
	if len(m.scanResults) > 100 {
		m.scanResults = m.scanResults[len(m.scanResults)-100:]
	}
	m.mu.Unlock()

	m.addAuditLog("scan", result.ID,
		fmt.Sprintf("Scan completed: %d files scanned, %d changes found", result.FilesScanned, result.ChangesFound))
	m.logger.Info("scan completed",
		zap.String("id", result.ID),
		zap.Int("scanned", result.FilesScanned),
		zap.Int("changes", result.ChangesFound),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// GenerateReport 生成完整性校验报告
func (m *Manager) GenerateReport(ctx context.Context, baselineID string) (*IntegrityReport, error) {
	start := time.Now()

	m.mu.RLock()
	baseline, ok := m.baselines[baselineID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("baseline %s not found", baselineID)
	}

	report := &IntegrityReport{
		ID:           generateID(),
		BaselineID:   baselineID,
		BaselineName: baseline.Name,
		TotalFiles:   baseline.FileCount,
		GeneratedAt:  start,
	}

	changes := make([]*FileChange, 0)
	modified, deleted, permChanges, verified := 0, 0, 0, 0

	for path, entry := range baseline.Entries {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			deleted++
			changes = append(changes, &FileChange{
				ID:           generateID(),
				Path:         path,
				ChangeType:   ChangeDeleted,
				BaselineHash: entry.Hash,
				DetectedAt:   time.Now(),
				AlertLevel:   AlertCritical,
			})
			continue
		}
		if err != nil {
			continue
		}

		if entry.Mode != 0 && uint32(info.Mode()) != entry.Mode {
			permChanges++
			changes = append(changes, &FileChange{
				ID:         generateID(),
				Path:       path,
				ChangeType: ChangePermission,
				OldMode:    entry.Mode,
				NewMode:    uint32(info.Mode()),
				DetectedAt: time.Now(),
				AlertLevel: AlertWarning,
			})
		}

		if !info.IsDir() {
			currentHash, err := m.hashFile(path, entry.HashAlgorithm)
			if err != nil {
				continue
			}
			if currentHash != entry.Hash {
				modified++
				changes = append(changes, &FileChange{
					ID:           generateID(),
					Path:         path,
					ChangeType:   ChangeModified,
					BaselineHash: entry.Hash,
					CurrentHash:  currentHash,
					DetectedAt:   time.Now(),
					AlertLevel:   AlertCritical,
				})
			} else {
				verified++
			}
		} else {
			verified++
		}
	}

	report.ModifiedFiles = modified
	report.DeletedFiles = deleted
	report.PermissionChanges = permChanges
	report.VerifiedFiles = verified
	report.Changes = changes

	if report.TotalFiles > 0 {
		report.IntegrityScore = float64(verified) / float64(report.TotalFiles) * 100
	}
	report.Duration = time.Since(start)

	m.addAuditLog("report", report.ID,
		fmt.Sprintf("Integrity report generated: score=%.1f%%", report.IntegrityScore))
	return report, nil
}

// ListChanges 列出变更
func (m *Manager) ListChanges(req *ListChangesRequest) *PaginatedResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := make([]*FileChange, 0)
	for _, ch := range m.changes {
		if req.Level != "" && ch.AlertLevel != req.Level {
			continue
		}
		if req.Since != nil && ch.DetectedAt.Before(*req.Since) {
			continue
		}
		if req.Until != nil && ch.DetectedAt.After(*req.Until) {
			continue
		}
		if req.Acked != nil && ch.Acknowledged != *req.Acked {
			continue
		}
		filtered = append(filtered, ch)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DetectedAt.After(filtered[j].DetectedAt)
	})

	total := len(filtered)
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return &PaginatedResult{Total: total, Page: page, PageSize: pageSize, Items: []*FileChange{}}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &PaginatedResult{Total: total, Page: page, PageSize: pageSize, Items: filtered[start:end]}
}

// AcknowledgeChange 确认变更
func (m *Manager) AcknowledgeChange(changeID, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.changes {
		if ch.ID == changeID {
			ch.Acknowledged = true
			ch.Notes = notes
			m.addAuditLog("ack_change", changeID, fmt.Sprintf("Change acknowledged: %s", notes))
			return nil
		}
	}
	return fmt.Errorf("change %s not found", changeID)
}

// GetRepairSuggestions 获取修复建议
func (m *Manager) GetRepairSuggestions(changeID string) ([]*RepairSuggestion, error) {
	m.mu.RLock()
	var change *FileChange
	for _, ch := range m.changes {
		if ch.ID == changeID {
			change = ch
			break
		}
	}
	m.mu.RUnlock()

	if change == nil {
		return nil, fmt.Errorf("change %s not found", changeID)
	}

	suggestions := make([]*RepairSuggestion, 0)

	switch change.ChangeType {
	case ChangeModified:
		suggestions = append(suggestions, &RepairSuggestion{
			ID:          generateID(),
			ChangeID:    changeID,
			Path:        change.Path,
			ChangeType:  ChangeModified,
			Suggestion:  "文件已被修改。如非预期变更，建议从备份恢复原始文件。",
			Action:      "restore",
			Risk:        AlertCritical,
			Commands:    []string{fmt.Sprintf("# 从备份恢复: cp /backup%s %s", change.Path, change.Path)},
			RestoreHash: change.BaselineHash,
		})
		suggestions = append(suggestions, &RepairSuggestion{
			ID:         generateID(),
			ChangeID:   changeID,
			Path:       change.Path,
			ChangeType: ChangeModified,
			Suggestion: "如变更合法，更新基线以记录新状态。",
			Action:     "update_baseline",
			Risk:       AlertInfo,
			Automated:  true,
		})
	case ChangeDeleted:
		suggestions = append(suggestions, &RepairSuggestion{
			ID:         generateID(),
			ChangeID:   changeID,
			Path:       change.Path,
			ChangeType: ChangeDeleted,
			Suggestion: "文件已被删除。如非预期操作，建议从备份恢复。",
			Action:     "restore",
			Risk:       AlertCritical,
			Commands:   []string{fmt.Sprintf("# 从备份恢复: cp /backup%s %s", change.Path, change.Path)},
		})
	case ChangePermission:
		suggestions = append(suggestions, &RepairSuggestion{
			ID:         generateID(),
			ChangeID:   changeID,
			Path:       change.Path,
			ChangeType: ChangePermission,
			Suggestion: fmt.Sprintf("权限已变更 (%o -> %o)。如非预期操作，建议恢复原始权限。", change.OldMode, change.NewMode),
			Action:     "restore_permission",
			Risk:       AlertWarning,
			Automated:  true,
			Commands:   []string{fmt.Sprintf("chmod %o %s", change.OldMode, change.Path)},
		})
	case ChangeOwnership:
		suggestions = append(suggestions, &RepairSuggestion{
			ID:         generateID(),
			ChangeID:   changeID,
			Path:       change.Path,
			ChangeType: ChangeOwnership,
			Suggestion: fmt.Sprintf("所有者已变更 (uid:%d->%d, gid:%d->%d)。", change.OldUID, change.NewUID, change.OldGID, change.NewGID),
			Action:     "restore_ownership",
			Risk:       AlertWarning,
			Automated:  true,
			Commands:   []string{fmt.Sprintf("chown %d:%d %s", change.OldUID, change.OldGID, change.Path)},
		})
	case ChangeCreated:
		suggestions = append(suggestions, &RepairSuggestion{
			ID:         generateID(),
			ChangeID:   changeID,
			Path:       change.Path,
			ChangeType: ChangeCreated,
			Suggestion: "检测到新文件。确认是否为预期创建，必要时添加到监控基线。",
			Action:     "review",
			Risk:       AlertWarning,
		})
	}
	return suggestions, nil
}

// ExportAuditLog 导出审计日志
func (m *Manager) ExportAuditLog(req *ExportAuditLogRequest) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filtered := make([]*AuditLogEntry, 0)
	for _, entry := range m.auditLog {
		if req.Since != nil && entry.Timestamp.Before(*req.Since) {
			continue
		}
		if req.Until != nil && entry.Timestamp.After(*req.Until) {
			continue
		}
		if req.Action != "" && entry.Action != req.Action {
			continue
		}
		filtered = append(filtered, entry)
	}

	switch req.Format {
	case "csv":
		return m.exportCSV(filtered)
	case "json", "":
		return json.MarshalIndent(filtered, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format: %s", req.Format)
	}
}

// GetScanResults 获取扫描结果列表
func (m *Manager) GetScanResults(limit int) []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.scanResults) {
		limit = len(m.scanResults)
	}
	start := len(m.scanResults) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*ScanResult, limit)
	copy(result, m.scanResults[start:])
	return result
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*Alert, limit)
	copy(result, m.alerts[start:])
	return result
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *MonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *MonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// --- 内部方法 ---

func (m *Manager) scanPath(ctx context.Context, root string, baseline *Baseline, excludePaths, excludePatterns []string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		for _, ex := range excludePaths {
			if strings.HasPrefix(path, ex) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !info.IsDir() && info.Size() > m.config.MaxFileSize {
			return nil
		}

		var fileHash string
		if !info.IsDir() {
			h, err := m.hashFile(path, baseline.HashAlgorithm)
			if err != nil {
				m.logger.Warn("hash file failed", zap.String("path", path), zap.Error(err))
				return nil
			}
			fileHash = h
		}

		entry := &FileEntry{
			Path:          path,
			Hash:          fileHash,
			HashAlgorithm: baseline.HashAlgorithm,
			Size:          info.Size(),
			ModTime:       info.ModTime(),
			Mode:          uint32(info.Mode()),
			IsDir:         info.IsDir(),
			ScannedAt:     time.Now(),
		}
		baseline.Entries[path] = entry
		return nil
	})
}

func (m *Manager) scanForChanges(ctx context.Context, root string, algo HashAlgorithm, excludePaths, excludePatterns []string, forceRehash bool, changes *[]*FileChange, scanned *int) error {
	// 收集实际存在的文件
	presentFiles := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		for _, ex := range excludePaths {
			if strings.HasPrefix(path, ex) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, info.Name()); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if !info.IsDir() && info.Size() > m.config.MaxFileSize {
			return nil
		}

		*scanned++
		presentFiles[path] = true

		m.mu.RLock()
		var baselineEntry *FileEntry
		for _, bl := range m.baselines {
			if e, ok := bl.Entries[path]; ok {
				baselineEntry = e
				break
			}
		}
		m.mu.RUnlock()

		if baselineEntry == nil {
			*changes = append(*changes, &FileChange{
				ID:         generateID(),
				Path:       path,
				ChangeType: ChangeCreated,
				DetectedAt: time.Now(),
				AlertLevel: AlertWarning,
			})
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// 检查权限
		if baselineEntry.Mode != 0 && uint32(info.Mode()) != baselineEntry.Mode {
			*changes = append(*changes, &FileChange{
				ID:         generateID(),
				Path:       path,
				ChangeType: ChangePermission,
				OldMode:    baselineEntry.Mode,
				NewMode:    uint32(info.Mode()),
				DetectedAt: time.Now(),
				AlertLevel: AlertWarning,
			})
		}

		// 检查哈希（总是检查非空哈希的文件）
		if baselineEntry.Hash != "" {
			currentHash, err := m.hashFile(path, algo)
			if err != nil {
				return nil
			}
			if currentHash != baselineEntry.Hash {
				*changes = append(*changes, &FileChange{
					ID:           generateID(),
					Path:         path,
					ChangeType:   ChangeModified,
					BaselineHash: baselineEntry.Hash,
					CurrentHash:  currentHash,
					DetectedAt:   time.Now(),
					AlertLevel:   AlertCritical,
				})
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 检测基线中存在但文件系统中已删除的文件
	m.mu.RLock()
	for _, bl := range m.baselines {
		for path, entry := range bl.Entries {
			if !strings.HasPrefix(path, root) {
				continue
			}
			if entry.IsDir {
				continue
			}
			if !presentFiles[path] {
				// 检查文件是否确实不存在
				if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
					*changes = append(*changes, &FileChange{
						ID:           generateID(),
						Path:         path,
						ChangeType:   ChangeDeleted,
						BaselineHash: entry.Hash,
						DetectedAt:   time.Now(),
						AlertLevel:   AlertCritical,
					})
				}
			}
		}
	}
	m.mu.RUnlock()

	return nil
}

func (m *Manager) hashFile(path string, algo HashAlgorithm) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash
	switch algo {
	case HashSHA256, "":
		h = sha256.New()
	case HashSHA512:
		h = sha512.New()
	default:
		h = sha256.New()
	}

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (m *Manager) triggerAlerts(change *FileChange) {
	m.mu.RLock()
	var matchedRules []*MonitorRule
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		for _, p := range rule.Paths {
			if strings.HasPrefix(change.Path, p) {
				matchedRules = append(matchedRules, rule)
				break
			}
		}
	}
	m.mu.RUnlock()

	for _, rule := range matchedRules {
		change.RuleID = rule.ID
		if change.AlertLevel == "" {
			change.AlertLevel = rule.AlertLevel
		}
		alertMsg := fmt.Sprintf("[%s] %s: %s", change.AlertLevel, change.ChangeType, change.Path)

		for _, channel := range rule.AlertChannels {
			alert := &Alert{
				ID:       generateID(),
				RuleID:   rule.ID,
				RuleName: rule.Name,
				Change:   change,
				Level:    change.AlertLevel,
				Channel:  channel,
				Message:  alertMsg,
				SentAt:   time.Now(),
			}

			switch channel {
			case AlertChannelWebhook:
				if rule.WebhookURL != "" {
					if err := m.sendWebhook(rule.WebhookURL, alert); err != nil {
						alert.Error = err.Error()
						alert.Delivered = false
					} else {
						alert.Delivered = true
					}
				}
			case AlertChannelNotify:
				alert.Delivered = true
				m.logger.Warn("FIM alert", zap.String("message", alertMsg))
			case AlertChannelEmail:
				alert.Delivered = false
				alert.Error = "email not configured"
			}

			m.mu.Lock()
			m.alerts = append(m.alerts, alert)
			if len(m.alerts) > m.config.AlertBufferSize {
				m.alerts = m.alerts[len(m.alerts)-m.config.AlertBufferSize:]
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) sendWebhook(url string, alert *Alert) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (m *Manager) startWatcher(rule *MonitorRule) {
	m.mu.Lock()
	if _, exists := m.watchers[rule.ID]; exists {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.watchers[rule.ID] = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.watchers, rule.ID)
			m.mu.Unlock()
		}()

		ticker := time.NewTicker(m.config.ScanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopChan:
				return
			case <-ticker.C:
				_, err := m.RunScan(ctx, &ScanRequest{
					RuleIDs: []string{rule.ID},
					Mode:    ScanModeIncremental,
				})
				if err != nil {
					m.logger.Error("watch scan failed",
						zap.String("rule_id", rule.ID), zap.Error(err))
				}
			}
		}
	}()
	m.logger.Info("watcher started", zap.String("rule_id", rule.ID), zap.String("rule_name", rule.Name))
}

func (m *Manager) stopWatcher(ruleID string) {
	m.mu.Lock()
	if cancel, ok := m.watchers[ruleID]; ok {
		cancel()
		delete(m.watchers, ruleID)
	}
	m.mu.Unlock()
}

func (m *Manager) watchLoop(ctx context.Context) {
	m.mu.RLock()
	activeRules := make([]*MonitorRule, 0)
	for _, r := range m.rules {
		if r.Enabled {
			activeRules = append(activeRules, r)
		}
	}
	m.mu.RUnlock()

	for _, rule := range activeRules {
		m.startWatcher(rule)
	}
}

func (m *Manager) addAuditLog(action, resource, details string) {
	entry := &AuditLogEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Action:    action,
		Resource:  resource,
		Details:   details,
		Source:    "fim-manager",
	}
	m.auditLog = append(m.auditLog, entry)
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-10000:]
	}
}

func (m *Manager) exportCSV(entries []*AuditLogEntry) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Write([]string{"id", "timestamp", "action", "resource", "details", "user_id", "source"})
	for _, e := range entries {
		w.Write([]string{
			e.ID,
			e.Timestamp.Format(time.RFC3339),
			e.Action,
			e.Resource,
			e.Details,
			e.UserID,
			e.Source,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}
