package enhanced

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OperationAuditor 操作审计器
type OperationAuditor struct {
	entries    []*OperationAuditEntry
	chains     map[string]*OperationChain
	config     OperationAuditConfig
	sensitive  *SensitiveOperationManager
	mu         sync.RWMutex
	storageDir string
	stopCh     chan struct{}
}

// OperationAuditConfig 操作审计配置
type OperationAuditConfig struct {
	Enabled           bool          `json:"enabled"`
	MaxEntries        int           `json:"max_entries"`
	MaxChains         int           `json:"max_chains"`
	TrackDuration     bool          `json:"track_duration"`
	AutoCorrelate     bool          `json:"auto_correlate"`
	StoreDetails      bool          `json:"store_details"`
	MaxDetailSize     int           `json:"max_detail_size"` // 字节
	SensitiveTracking bool          `json:"sensitive_tracking"`
	StorageInterval   time.Duration `json:"storage_interval"`
}

// DefaultOperationAuditConfig 默认操作审计配置
func DefaultOperationAuditConfig() OperationAuditConfig {
	return OperationAuditConfig{
		Enabled:           true,
		MaxEntries:        200000,
		MaxChains:         10000,
		TrackDuration:     true,
		AutoCorrelate:     true,
		StoreDetails:      true,
		MaxDetailSize:     10240,
		SensitiveTracking: true,
		StorageInterval:   time.Minute * 5,
	}
}

// NewOperationAuditor 创建操作审计器
func NewOperationAuditor(config OperationAuditConfig) *OperationAuditor {
	storageDir := "/var/log/nas-os/audit/operations"
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		// 如果无法创建存储目录，继续运行但不持久化
		storageDir = ""
	}

	oa := &OperationAuditor{
		entries:    make([]*OperationAuditEntry, 0),
		chains:     make(map[string]*OperationChain),
		config:     config,
		storageDir: storageDir,
		stopCh:     make(chan struct{}),
	}

	if config.SensitiveTracking {
		oa.sensitive = NewSensitiveOperationManager()
	}

	return oa
}

// Stop 停止审计器
func (oa *OperationAuditor) Stop() {
	close(oa.stopCh)
	oa.save()
}

// ========== 操作记录 ==========

// RecordOperation 记录操作
func (oa *OperationAuditor) RecordOperation(
	userID, username, ip, userAgent, sessionID string,
	category OperationCategory,
	action OperationAction,
	resourceType, resourceID, resourceName, resourcePath string,
	status string,
	oldValue, newValue interface{},
	details map[string]interface{},
) *OperationAuditEntry {
	oa.mu.Lock()
	defer oa.mu.Unlock()

	if !oa.config.Enabled {
		return nil
	}

	now := time.Now()
	entry := &OperationAuditEntry{
		ID:           uuid.New().String(),
		Timestamp:    now,
		UserID:       userID,
		Username:     username,
		IP:           ip,
		UserAgent:    userAgent,
		SessionID:    sessionID,
		Category:     category,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		ResourcePath: resourcePath,
		Status:       status,
		Details:      details,
	}

	// 存储变更前后值
	if oa.config.StoreDetails {
		if oldValue != nil {
			entry.OldValue = oldValue
		}
		if newValue != nil {
			entry.NewValue = newValue
		}
	}

	// 检查是否为敏感操作
	if oa.sensitive != nil {
		if sensitiveOp := oa.sensitive.CheckSensitive(category, action, resourcePath); sensitiveOp != nil {
			entry.IsSensitive = true
			entry.SensitivityLevel = string(sensitiveOp.SensitivityLevel)
			entry.RiskScore = oa.calculateSensitiveRiskScore(sensitiveOp.SensitivityLevel)
		}
	}

	oa.entries = append(oa.entries, entry)

	// 限制条目数量
	if len(oa.entries) > oa.config.MaxEntries {
		oa.entries = oa.entries[len(oa.entries)-oa.config.MaxEntries:]
	}

	return entry
}

// RecordOperationWithDuration 记录带持续时间的操作
func (oa *OperationAuditor) RecordOperationWithDuration(
	userID, username, ip, userAgent, sessionID string,
	category OperationCategory,
	action OperationAction,
	resourceType, resourceID, resourceName, resourcePath string,
	status string,
	oldValue, newValue interface{},
	details map[string]interface{},
	durationMs int64,
) *OperationAuditEntry {
	entry := oa.RecordOperation(
		userID, username, ip, userAgent, sessionID,
		category, action, resourceType, resourceID, resourceName, resourcePath,
		status, oldValue, newValue, details,
	)

	if entry != nil {
		oa.mu.Lock()
		entry.Duration = durationMs
		oa.mu.Unlock()
	}

	return entry
}

// RecordFailure 记录失败操作
func (oa *OperationAuditor) RecordFailure(
	userID, username, ip, userAgent, sessionID string,
	category OperationCategory,
	action OperationAction,
	resourceType, resourceID, resourceName string,
	errorMessage string,
	details map[string]interface{},
) *OperationAuditEntry {
	oa.mu.Lock()
	defer oa.mu.Unlock()

	if !oa.config.Enabled {
		return nil
	}

	entry := &OperationAuditEntry{
		ID:           uuid.New().String(),
		Timestamp:    time.Now(),
		UserID:       userID,
		Username:     username,
		IP:           ip,
		UserAgent:    userAgent,
		SessionID:    sessionID,
		Category:     category,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Status:       "failure",
		ErrorMessage: errorMessage,
		Details:      details,
	}

	oa.entries = append(oa.entries, entry)
	return entry
}

// ========== 操作链追踪 ==========

// StartOperationChain 开始操作链
func (oa *OperationAuditor) StartOperationChain(userID, username string) string {
	oa.mu.Lock()
	defer oa.mu.Unlock()

	correlationID := uuid.New().String()
	chain := &OperationChain{
		CorrelationID: correlationID,
		UserID:        userID,
		Username:      username,
		StartTime:     time.Now(),
		Operations:    make([]*OperationAuditEntry, 0),
		Status:        "ongoing",
	}

	oa.chains[correlationID] = chain

	// 限制链数量
	if len(oa.chains) > oa.config.MaxChains {
		oa.cleanupOldChains()
	}

	return correlationID
}

// AddToChain 添加操作到链
func (oa *OperationAuditor) AddToChain(correlationID string, entry *OperationAuditEntry) {
	oa.mu.Lock()
	defer oa.mu.Unlock()

	if chain, exists := oa.chains[correlationID]; exists {
		entry.CorrelationID = correlationID
		chain.Operations = append(chain.Operations, entry)
		chain.TotalOps++
	}
}

// EndOperationChain 结束操作链
func (oa *OperationAuditor) EndOperationChain(correlationID string, status string) {
	oa.mu.Lock()
	defer oa.mu.Unlock()

	if chain, exists := oa.chains[correlationID]; exists {
		now := time.Now()
		chain.EndTime = &now
		chain.Status = status
	}
}

// GetOperationChain 获取操作链
func (oa *OperationAuditor) GetOperationChain(correlationID string) *OperationChain {
	oa.mu.RLock()
	defer oa.mu.RUnlock()
	return oa.chains[correlationID]
}

// cleanupOldChains 清理旧的操作链
func (oa *OperationAuditor) cleanupOldChains() {
	// 按开始时间排序并删除最老的
	type chainWithTime struct {
		id        string
		startTime time.Time
	}

	chains := make([]chainWithTime, 0, len(oa.chains))
	for id, chain := range oa.chains {
		chains = append(chains, chainWithTime{id: id, startTime: chain.StartTime})
	}

	sort.Slice(chains, func(i, j int) bool {
		return chains[i].startTime.Before(chains[j].startTime)
	})

	// 删除一半最老的
	removeCount := len(oa.chains) / 2
	for i := 0; i < removeCount && i < len(chains); i++ {
		delete(oa.chains, chains[i].id)
	}
}

// ========== 批量操作记录 ==========

// RecordBatchOperations 批量记录操作
func (oa *OperationAuditor) RecordBatchOperations(
	userID, username, ip, userAgent, sessionID string,
	operations []BatchOperation,
) (string, []*OperationAuditEntry) {
	correlationID := oa.StartOperationChain(userID, username)
	entries := make([]*OperationAuditEntry, 0, len(operations))

	for _, op := range operations {
		entry := oa.RecordOperation(
			userID, username, ip, userAgent, sessionID,
			op.Category, op.Action,
			op.ResourceType, op.ResourceID, op.ResourceName, op.ResourcePath,
			"success", op.OldValue, op.NewValue, op.Details,
		)
		if entry != nil {
			oa.AddToChain(correlationID, entry)
			entries = append(entries, entry)
		}
	}

	return correlationID, entries
}

// BatchOperation 批量操作项
type BatchOperation struct {
	Category     OperationCategory
	Action       OperationAction
	ResourceType string
	ResourceID   string
	ResourceName string
	ResourcePath string
	OldValue     interface{}
	NewValue     interface{}
	Details      map[string]interface{}
}

// ========== 查询功能 ==========

// Query 查询操作审计日志
func (oa *OperationAuditor) Query(opts OperationQueryOptions) ([]*OperationAuditEntry, int) {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	filtered := make([]*OperationAuditEntry, 0)
	for _, entry := range oa.entries {
		if !oa.matchesFilter(entry, opts) {
			continue
		}
		filtered = append(filtered, entry)
	}

	// 按时间倒序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	total := len(filtered)

	// 分页
	start := opts.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := start + opts.Limit
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if end > total {
		end = total
	}

	return filtered[start:end], total
}

// matchesFilter 检查是否匹配筛选条件
func (oa *OperationAuditor) matchesFilter(entry *OperationAuditEntry, opts OperationQueryOptions) bool {
	if opts.StartTime != nil && entry.Timestamp.Before(*opts.StartTime) {
		return false
	}
	if opts.EndTime != nil && entry.Timestamp.After(*opts.EndTime) {
		return false
	}
	if opts.UserID != "" && entry.UserID != opts.UserID {
		return false
	}
	if opts.Username != "" && !strings.Contains(strings.ToLower(entry.Username), strings.ToLower(opts.Username)) {
		return false
	}
	if opts.IP != "" && entry.IP != opts.IP {
		return false
	}
	if opts.Category != "" && entry.Category != opts.Category {
		return false
	}
	if opts.Action != "" && entry.Action != opts.Action {
		return false
	}
	if opts.ResourceType != "" && entry.ResourceType != opts.ResourceType {
		return false
	}
	if opts.ResourceID != "" && entry.ResourceID != opts.ResourceID {
		return false
	}
	if opts.Status != "" && entry.Status != opts.Status {
		return false
	}
	if opts.IsSensitive != nil && entry.IsSensitive != *opts.IsSensitive {
		return false
	}
	if opts.SensitivityLevel != "" && entry.SensitivityLevel != string(opts.SensitivityLevel) {
		return false
	}
	if opts.CorrelationID != "" && entry.CorrelationID != opts.CorrelationID {
		return false
	}
	if opts.MinRiskScore > 0 && entry.RiskScore < opts.MinRiskScore {
		return false
	}
	return true
}

// GetByID 根据ID获取操作审计条目
func (oa *OperationAuditor) GetByID(id string) *OperationAuditEntry {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	for _, entry := range oa.entries {
		if entry.ID == id {
			return entry
		}
	}
	return nil
}

// GetByCorrelationID 根据关联ID获取操作
func (oa *OperationAuditor) GetByCorrelationID(correlationID string) []*OperationAuditEntry {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	entries := make([]*OperationAuditEntry, 0)
	for _, entry := range oa.entries {
		if entry.CorrelationID == correlationID {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries
}

// GetByResource 根据资源获取操作历史
func (oa *OperationAuditor) GetByResource(resourceType, resourceID string, limit int) []*OperationAuditEntry {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	entries := make([]*OperationAuditEntry, 0)
	for _, entry := range oa.entries {
		if entry.ResourceType == resourceType && entry.ResourceID == resourceID {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries
}

// GetUserOperations 获取用户操作历史
func (oa *OperationAuditor) GetUserOperations(userID string, limit int) []*OperationAuditEntry {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	entries := make([]*OperationAuditEntry, 0)
	for _, entry := range oa.entries {
		if entry.UserID == userID {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries
}

// GetSensitiveOperations 获取敏感操作
func (oa *OperationAuditor) GetSensitiveOperations(limit int) []*OperationAuditEntry {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	entries := make([]*OperationAuditEntry, 0)
	for _, entry := range oa.entries {
		if entry.IsSensitive {
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries
}

// ========== 统计功能 ==========

// GetStatistics 获取操作统计
func (oa *OperationAuditor) GetStatistics(start, end time.Time) *OperationStatistics {
	oa.mu.RLock()
	defer oa.mu.RUnlock()

	stats := &OperationStatistics{
		OpsByCategory:     make(map[string]int),
		OpsByAction:       make(map[string]int),
		OpsByHour:         make(map[int]int),
		FailedOpsByReason: make(map[string]int),
		OpsByUser:         make([]UserOperationCount, 0),
		OpsByResource:     make([]ResourceOperationCount, 0),
		TopResources:      make([]ResourceCount, 0),
	}

	userOps := make(map[string]int)
	resourceOps := make(map[string]int)
	resourceCounts := make(map[string]int)
	totalDuration := int64(0)
	durationCount := 0

	for _, entry := range oa.entries {
		if entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			continue
		}

		stats.TotalOperations++

		if entry.Status == "success" {
			stats.SuccessfulOps++
		} else if entry.Status == "failure" {
			stats.FailedOps++
			reason := entry.ErrorMessage
			if reason == "" {
				reason = "unknown"
			}
			stats.FailedOpsByReason[reason]++
		}

		if entry.IsSensitive {
			stats.SensitiveOpCount++
		}

		if entry.Duration > 0 {
			totalDuration += entry.Duration
			durationCount++
		}

		stats.OpsByCategory[string(entry.Category)]++
		stats.OpsByAction[string(entry.Action)]++
		stats.OpsByHour[entry.Timestamp.Hour()]++

		userOps[entry.UserID]++
		resourceOps[entry.ResourceType]++
		resourceCounts[entry.ResourceName]++
	}

	if durationCount > 0 {
		stats.AvgDuration = totalDuration / int64(durationCount)
	}

	// 用户操作统计
	for userID, count := range userOps {
		stats.OpsByUser = append(stats.OpsByUser, UserOperationCount{
			UserID: userID,
			Count:  count,
		})
	}
	sort.Slice(stats.OpsByUser, func(i, j int) bool {
		return stats.OpsByUser[i].Count > stats.OpsByUser[j].Count
	})
	if len(stats.OpsByUser) > 10 {
		stats.OpsByUser = stats.OpsByUser[:10]
	}

	// 资源类型统计
	for resType, count := range resourceOps {
		stats.OpsByResource = append(stats.OpsByResource, ResourceOperationCount{
			ResourceType: resType,
			Count:        count,
		})
	}
	sort.Slice(stats.OpsByResource, func(i, j int) bool {
		return stats.OpsByResource[i].Count > stats.OpsByResource[j].Count
	})

	// 热门资源
	for res, count := range resourceCounts {
		stats.TopResources = append(stats.TopResources, ResourceCount{
			Resource: res,
			Count:    count,
		})
	}
	sort.Slice(stats.TopResources, func(i, j int) bool {
		return stats.TopResources[i].Count > stats.TopResources[j].Count
	})
	if len(stats.TopResources) > 10 {
		stats.TopResources = stats.TopResources[:10]
	}

	return stats
}

// ========== 辅助功能 ==========

// calculateSensitiveRiskScore 计算敏感操作风险分数
func (oa *OperationAuditor) calculateSensitiveRiskScore(level SensitivityLevel) int {
	switch level {
	case SensitivityCritical:
		return 90
	case SensitivityHigh:
		return 70
	case SensitivityMedium:
		return 50
	case SensitivityLow:
		return 30
	default:
		return 40
	}
}

// ========== 持久化 ==========

// save 保存数据
func (oa *OperationAuditor) save() {
	if len(oa.entries) == 0 {
		return
	}

	today := time.Now().Format("2006-01-02")
	filename := filepath.Join(oa.storageDir, "operations-"+today+".log")

	data, err := json.MarshalIndent(oa.entries, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(filename, data, 0640)
}

// Load 加载数据
func (oa *OperationAuditor) Load(date string) error {
	filename := filepath.Join(oa.storageDir, "operations-"+date+".log")
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var entries []*OperationAuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	oa.mu.Lock()
	oa.entries = append(oa.entries, entries...)
	oa.mu.Unlock()

	return nil
}
