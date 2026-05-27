// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Engine 溯源引擎.
type Engine struct {
	mu             sync.RWMutex
	records        map[string]*ProvenanceRecord // id -> record
	fileIndex      map[string][]string           // fileID -> record ids
	userIndex      map[string][]string           // userID -> record ids
	operationIndex map[OperationType][]string    // operation -> record ids
	lineage        map[string]*FileLineage       // fileID -> lineage
	retention      *RetentionPolicy
}

// NewEngine 创建溯源引擎.
func NewEngine(retention *RetentionPolicy) *Engine {
	if retention == nil {
		retention = &RetentionPolicy{
			MaxAge:      365 * 24 * time.Hour, // 默认保留1年
			MaxRecords:  1000000,               // 默认最多100万条
		}
	}
	return &Engine{
		records:        make(map[string]*ProvenanceRecord),
		fileIndex:      make(map[string][]string),
		userIndex:      make(map[string][]string),
		operationIndex: make(map[OperationType][]string),
		lineage:        make(map[string]*FileLineage),
		retention:      retention,
	}
}

// RecordOperation 记录文件操作.
func (e *Engine) RecordOperation(record *ProvenanceRecord) error {
	if record == nil {
		return ErrInvalidInput
	}
	if record.ID == "" || record.FileID == "" {
		return ErrInvalidInput
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	record.Timestamp = time.Now()
	e.records[record.ID] = record

	// 更新索引
	e.fileIndex[record.FileID] = append(e.fileIndex[record.FileID], record.ID)
	if record.UserID != "" {
		e.userIndex[record.UserID] = append(e.userIndex[record.UserID], record.ID)
	}
	e.operationIndex[record.Operation] = append(e.operationIndex[record.Operation], record.ID)

	// 更新血缘关系
	e.updateLineage(record)

	return nil
}

// GetRecord 获取溯源记录.
func (e *Engine) GetRecord(id string) (*ProvenanceRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.records[id]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// GetFileHistory 获取文件变更历史.
func (e *Engine) GetFileHistory(fileID string) ([]*ProvenanceRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids, ok := e.fileIndex[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}

	records := make([]*ProvenanceRecord, 0, len(ids))
	for _, id := range ids {
		if r, exists := e.records[id]; exists {
			records = append(records, r)
		}
	}

	// 按时间排序
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records, nil
}

// GetUserAudit 获取用户审计记录.
func (e *Engine) GetUserAudit(userID string) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := e.userIndex[userID]
	entries := make([]AuditEntry, 0, len(ids))
	for _, id := range ids {
		if r, exists := e.records[id]; exists {
			entries = append(entries, AuditEntry{
				UserID:    r.UserID,
				UserName:  r.UserName,
				Action:    string(r.Operation),
				FileID:    r.FileID,
				FilePath:  r.FilePath,
				Operation: r.Operation,
				Timestamp: r.Timestamp,
				Details:   r.Description,
			})
		}
	}

	// 按时间倒序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	return entries
}

// QueryRecords 查询溯源记录.
func (e *Engine) QueryRecords(filter QueryFilter) []*ProvenanceRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var candidates []string

	// 选择最小候选集
	if filter.FileID != "" {
		candidates = e.fileIndex[filter.FileID]
	} else if filter.UserID != "" {
		candidates = e.userIndex[filter.UserID]
	} else if filter.Operation != "" {
		candidates = e.operationIndex[filter.Operation]
	} else {
		// 全量扫描
		candidates = make([]string, 0, len(e.records))
		for id := range e.records {
			candidates = append(candidates, id)
		}
	}

	var results []*ProvenanceRecord
	for _, id := range candidates {
		r, ok := e.records[id]
		if !ok {
			continue
		}

		// 应用过滤条件
		if filter.FileID != "" && r.FileID != filter.FileID {
			continue
		}
		if filter.UserID != "" && r.UserID != filter.UserID {
			continue
		}
		if filter.Operation != "" && r.Operation != filter.Operation {
			continue
		}
		if filter.StartTime != nil && r.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && r.Timestamp.After(*filter.EndTime) {
			continue
		}
		if filter.FilePath != "" && r.FilePath != filter.FilePath {
			continue
		}

		results = append(results, r)
	}

	// 按时间排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 应用分页
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results
}

// GetLineage 获取文件血缘关系.
func (e *Engine) GetLineage(fileID string) (*FileLineage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lineage, ok := e.lineage[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}
	return lineage, nil
}

// AnalyzeImpact 分析文件影响范围.
func (e *Engine) AnalyzeImpact(fileID string) (*ImpactResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lineage, ok := e.lineage[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}

	result := &ImpactResult{
		SourceFileID:   fileID,
		SourceFilePath: e.getFilePath(fileID),
		AffectedFiles:  make([]AffectedFile, 0),
	}

	// 收集所有后代文件
	for _, desc := range lineage.Descendants {
		result.AffectedFiles = append(result.AffectedFiles, AffectedFile{
			FileID:     desc.FileID,
			FilePath:   desc.FilePath,
			Relation:   string(desc.Operation),
			AffectedAt: desc.Timestamp,
		})
	}

	result.TotalAffected = len(result.AffectedFiles)
	return result, nil
}

// VerifyIntegrity 校验文件完整性.
func (e *Engine) VerifyIntegrity(fileID string, currentHash string) (*IntegrityResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids, ok := e.fileIndex[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}

	// 获取最新的哈希记录
	var expectedHash string
	var latestTime time.Time
	for _, id := range ids {
		r := e.records[id]
		if r.CurrentHash != "" && r.Timestamp.After(latestTime) {
			expectedHash = r.CurrentHash
			latestTime = r.Timestamp
		}
	}

	result := &IntegrityResult{
		FileID:       fileID,
		FilePath:     e.getFilePath(fileID),
		ExpectedHash: expectedHash,
		ActualHash:   currentHash,
		IsValid:      expectedHash == currentHash,
		CheckedAt:    time.Now(),
	}

	return result, nil
}

// GenerateComplianceReport 生成合规报告.
func (e *Engine) GenerateComplianceReport(startTime, endTime time.Time) *ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &ComplianceReport{
		ReportID:         fmt.Sprintf("report-%d", time.Now().UnixNano()),
		GeneratedAt:      time.Now(),
		StartTime:        startTime,
		EndTime:          endTime,
		OperationsByType: make(map[OperationType]int64),
		OperationsByUser: make(map[string]int64),
	}

	fileOpCounts := make(map[string]*FileOperationCount)

	for _, r := range e.records {
		if r.Timestamp.Before(startTime) || r.Timestamp.After(endTime) {
			continue
		}

		report.TotalOperations++
		report.OperationsByType[r.Operation]++
		if r.UserID != "" {
			report.OperationsByUser[r.UserID]++
		}

		// 统计文件操作次数
		foc, ok := fileOpCounts[r.FileID]
		if !ok {
			foc = &FileOperationCount{
				FileID:   r.FileID,
				FilePath: r.FilePath,
			}
			fileOpCounts[r.FileID] = foc
		}
		foc.Count++
	}

	// 取前10个最常修改的文件
	report.TopModifiedFiles = e.getTopModifiedFiles(fileOpCounts, 10)

	return report
}

// CleanupExpired 清理过期溯源数据.
func (e *Engine) CleanupExpired() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-e.retention.MaxAge)
	var cleaned int64

	for id, r := range e.records {
		if r.Timestamp.Before(cutoff) {
			delete(e.records, id)
			cleaned++
		}
	}

	// 重建索引
	if cleaned > 0 {
		e.rebuildIndexes()
	}

	e.retention.CleanedCount += cleaned
	e.retention.LastCleanupAt = time.Now()

	return cleaned
}

// GetRetentionPolicy 获取保留策略.
func (e *Engine) GetRetentionPolicy() *RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.retention
}

// UpdateRetentionPolicy 更新保留策略.
func (e *Engine) UpdateRetentionPolicy(policy *RetentionPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.retention = policy
}

// ========== 内部方法 ==========

// updateLineage 更新血缘关系.
func (e *Engine) updateLineage(record *ProvenanceRecord) {
	lineage, ok := e.lineage[record.FileID]
	if !ok {
		lineage = &FileLineage{
			FileID:      record.FileID,
			FilePath:    record.FilePath,
			Ancestors:   make([]LineageNode, 0),
			Descendants: make([]LineageNode, 0),
		}
		e.lineage[record.FileID] = lineage
	}

	// 更新文件路径
	lineage.FilePath = record.FilePath

	// 如果有父文件，建立血缘关系
	if record.ParentID != "" {
		parentLineage, exists := e.lineage[record.ParentID]
		if exists {
			// 添加祖先
			lineage.Ancestors = append(lineage.Ancestors, LineageNode{
				FileID:    record.ParentID,
				FilePath:  parentLineage.FilePath,
				Operation: record.Operation,
				UserID:    record.UserID,
				Timestamp: record.Timestamp,
			})

			// 在父文件中添加后代
			parentLineage.Descendants = append(parentLineage.Descendants, LineageNode{
				FileID:    record.FileID,
				FilePath:  record.FilePath,
				Operation: record.Operation,
				UserID:    record.UserID,
				Timestamp: record.Timestamp,
			})
		}
	}
}

// getFilePath 获取文件路径.
func (e *Engine) getFilePath(fileID string) string {
	if lineage, ok := e.lineage[fileID]; ok {
		return lineage.FilePath
	}
	// 从记录中查找
	for _, r := range e.records {
		if r.FileID == fileID {
			return r.FilePath
		}
	}
	return ""
}

// getTopModifiedFiles 获取最常修改的文件.
func (e *Engine) getTopModifiedFiles(counts map[string]*FileOperationCount, limit int) []FileOperationCount {
	result := make([]FileOperationCount, 0, len(counts))
	for _, foc := range counts {
		result = append(result, *foc)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

// rebuildIndexes 重建索引.
func (e *Engine) rebuildIndexes() {
	e.fileIndex = make(map[string][]string)
	e.userIndex = make(map[string][]string)
	e.operationIndex = make(map[OperationType][]string)

	for id, r := range e.records {
		e.fileIndex[r.FileID] = append(e.fileIndex[r.FileID], id)
		if r.UserID != "" {
			e.userIndex[r.UserID] = append(e.userIndex[r.UserID], id)
		}
		e.operationIndex[r.Operation] = append(e.operationIndex[r.Operation], id)
	}
}

// CalculateHash 计算数据哈希.
func CalculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
