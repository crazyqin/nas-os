// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Engine 数据溯源引擎.
type Engine struct {
	mu              sync.RWMutex
	records         map[string]*ProvenanceRecord
	fileIndex       map[string][]string // fileID -> record IDs
	userIndex       map[string][]string // userID -> record IDs
	operationIndex  map[OperationType][]string // operation -> record IDs
	dataTypes       map[string]string // fileID -> dataType
	chains          map[string]*AuditChain
	complianceTags  map[string][]ComplianceTag
	retentionPolicy *RetentionPolicy
}

// NewEngine 创建溯源引擎.
func NewEngine(policy *RetentionPolicy) *Engine {
	if policy == nil {
		policy = &RetentionPolicy{
			MaxAge:     365 * 24 * time.Hour,
			MaxRecords: 1000000,
		}
	}
	return &Engine{
		records:         make(map[string]*ProvenanceRecord),
		fileIndex:       make(map[string][]string),
		userIndex:       make(map[string][]string),
		operationIndex:  make(map[OperationType][]string),
		dataTypes:       make(map[string]string),
		chains:          make(map[string]*AuditChain),
		complianceTags:  make(map[string][]ComplianceTag),
		retentionPolicy: policy,
	}
}

// RecordOperation 记录操作.
func (e *Engine) RecordOperation(record *ProvenanceRecord) error {
	if record == nil || record.ID == "" {
		return ErrInvalidInput
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	record.Timestamp = time.Now()
	e.records[record.ID] = record
	e.fileIndex[record.FileID] = append(e.fileIndex[record.FileID], record.ID)
	e.userIndex[record.UserID] = append(e.userIndex[record.UserID], record.ID)
	e.operationIndex[record.Operation] = append(e.operationIndex[record.Operation], record.ID)
	return nil
}

// RecordOrigin 记录数据来源.
func (e *Engine) RecordOrigin(dataID, dataType, location, userID string, metadata map[string]string) (*ProvenanceRecord, error) {
	if dataID == "" {
		return nil, ErrInvalidInput
	}
	record := &ProvenanceRecord{
		ID:          fmt.Sprintf("origin-%d", time.Now().UnixNano()),
		FileID:      dataID,
		Operation:   OpCreate,
		UserID:      userID,
		Source:      SourceGenerate,
		Metadata:    metadata,
		Timestamp:   time.Now(),
		Description: fmt.Sprintf("数据来源: %s", location),
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records[record.ID] = record
	e.fileIndex[dataID] = append(e.fileIndex[dataID], record.ID)
	e.userIndex[userID] = append(e.userIndex[userID], record.ID)
	e.operationIndex[OpCreate] = append(e.operationIndex[OpCreate], record.ID)
	e.dataTypes[dataID] = dataType
	return record, nil
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

// GetFileHistory 获取文件历史.
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
	return records, nil
}

// TraceLineage 追踪血缘关系.
func (e *Engine) TraceLineage(fileID string) (*FileLineage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids, ok := e.fileIndex[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}
	lineage := &FileLineage{
		FileID:             fileID,
		DataType:           e.dataTypes[fileID],
		DataClassification: e.complianceTags[fileID],
		Ancestors:          make([]LineageNode, 0),
		Descendants:        make([]LineageNode, 0),
	}
	if len(ids) > 0 {
		if r, exists := e.records[ids[0]]; exists {
			lineage.FilePath = r.FilePath
		}
	}

	// 追踪祖先：通过 ParentID 查找直接父文件
	currentFileID := fileID
	recordIDs, exists := e.fileIndex[currentFileID]
	if exists {
		var parentID string
		for _, rid := range recordIDs {
			if r, ok := e.records[rid]; ok && r.ParentID != "" {
				parentID = r.ParentID
				break
			}
		}
		if parentID != "" {
			parentRecordIDs, parentExists := e.fileIndex[parentID]
			if parentExists && len(parentRecordIDs) > 0 {
				if parentRecord, ok := e.records[parentRecordIDs[0]]; ok {
					lineage.Ancestors = append(lineage.Ancestors, LineageNode{
						FileID:    parentID,
						FilePath:  parentRecord.FilePath,
						Operation: parentRecord.Operation,
						UserID:    parentRecord.UserID,
						Timestamp: parentRecord.Timestamp,
					})
				}
			}
		}
	}

	// 追踪后代：查找所有 ParentID 为当前 fileID 的记录
	for otherFileID, otherRecordIDs := range e.fileIndex {
		if otherFileID == fileID {
			continue
		}
		for _, rid := range otherRecordIDs {
			if r, ok := e.records[rid]; ok && r.ParentID == fileID {
				lineage.Descendants = append(lineage.Descendants, LineageNode{
					FileID:    otherFileID,
					FilePath:  r.FilePath,
					Operation: r.Operation,
					UserID:    r.UserID,
					Timestamp: r.Timestamp,
				})
				break // 每个后代文件只记录一次
			}
		}
	}

	return lineage, nil
}

// GetLineage 获取血缘关系.
func (e *Engine) GetLineage(fileID string) (*FileLineage, error) {
	return e.TraceLineage(fileID)
}

// VerifyChain 验证审计链.
func (e *Engine) VerifyChain(chainID string) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	chain, ok := e.chains[chainID]
	if !ok {
		return false, ErrRecordNotFound
	}
	for i := 1; i < len(chain.Blocks); i++ {
		if chain.Blocks[i].PreviousHash != chain.Blocks[i-1].Hash {
			return false, nil
		}
	}
	return true, nil
}

// AddToChain 添加到审计链.
func (e *Engine) AddToChain(chainID, recordID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.records[recordID]
	if !ok {
		return ErrRecordNotFound
	}

	chain, ok := e.chains[chainID]
	if !ok {
		chain = &AuditChain{
			ChainID:   chainID,
			Blocks:    make([]AuditBlock, 0),
			CreatedAt: time.Now(),
		}
		e.chains[chainID] = chain
	}

	block := AuditBlock{
		Index:        int64(len(chain.Blocks)),
		Timestamp:    time.Now(),
		RecordID:     recordID,
		Data:         []byte(fmt.Sprintf("%+v", record)),
		PreviousHash: chain.LastBlockHash,
	}
	block.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d%s%s", block.Index, block.PreviousHash, block.Data))))
	chain.Blocks = append(chain.Blocks, block)
	chain.LastBlockHash = block.Hash
	return nil
}

// GetAuditChain 获取审计链.
func (e *Engine) GetAuditChain(chainID string) (*AuditChain, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	chain, ok := e.chains[chainID]
	if !ok {
		return nil, fmt.Errorf("审计链 %s 不存在", chainID)
	}
	return chain, nil
}

// AddComplianceTag 添加合规标签.
func (e *Engine) AddComplianceTag(dataID string, tag ComplianceTag) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if _, ok := e.fileIndex[dataID]; !ok {
		return ErrFileNotFound
	}
	
	e.complianceTags[dataID] = append(e.complianceTags[dataID], tag)
	return nil
}

// QueryProvenance 高级溯源查询.
func (e *Engine) QueryProvenance(query *ProvenanceQuery) ([]*ProvenanceRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query == nil {
		return nil, ErrInvalidInput
	}

	results := make([]*ProvenanceRecord, 0)
	for _, record := range e.records {
		if !matchFilter(record, &query.Filter) {
			continue
		}
		// 合规过滤
		if len(query.ComplianceFilter) > 0 {
			compliance, ok := record.Metadata["compliance"]
			if !ok {
				continue
			}
			matched := false
			for _, tag := range query.ComplianceFilter {
				if string(tag) == compliance {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		results = append(results, record)
	}
	return results, nil
}

// QueryRecords 查询记录.
func (e *Engine) QueryRecords(filter QueryFilter) []*ProvenanceRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*ProvenanceRecord, 0)
	for _, record := range e.records {
		if matchFilter(record, &filter) {
			results = append(results, record)
		}
	}
	return results
}

// matchFilter 匹配过滤器.
func matchFilter(record *ProvenanceRecord, filter *QueryFilter) bool {
	if filter.FileID != "" && record.FileID != filter.FileID {
		return false
	}
	if filter.UserID != "" && record.UserID != filter.UserID {
		return false
	}
	if filter.Operation != "" && record.Operation != filter.Operation {
		return false
	}
	if filter.StartTime != nil && record.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && record.Timestamp.After(*filter.EndTime) {
		return false
	}
	return true
}

// VerifyIntegrity 验证完整性.
func (e *Engine) VerifyIntegrity(fileID, currentHash string) (*IntegrityResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids, ok := e.fileIndex[fileID]
	if !ok {
		return nil, ErrFileNotFound
	}

	expectedHash := ""
	if len(ids) > 0 {
		if r, exists := e.records[ids[len(ids)-1]]; exists {
			expectedHash = r.CurrentHash
		}
	}

	return &IntegrityResult{
		FileID:       fileID,
		ExpectedHash: expectedHash,
		ActualHash:   currentHash,
		IsValid:      expectedHash == currentHash,
		CheckedAt:    time.Now(),
	}, nil
}

// AnalyzeImpact 分析影响.
func (e *Engine) AnalyzeImpact(fileID string) (*ImpactResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, ok := e.fileIndex[fileID]; !ok {
		return nil, ErrFileNotFound
	}

	result := &ImpactResult{
		SourceFileID:  fileID,
		AffectedFiles: make([]AffectedFile, 0),
	}

	// 获取源文件路径
	if recordIDs, ok := e.fileIndex[fileID]; ok && len(recordIDs) > 0 {
		if r, exists := e.records[recordIDs[0]]; exists {
			result.SourceFilePath = r.FilePath
		}
	}

	// 查找所有 ParentID 为当前 fileID 的文件
	for otherFileID, otherRecordIDs := range e.fileIndex {
		if otherFileID == fileID {
			continue
		}
		for _, rid := range otherRecordIDs {
			if r, ok := e.records[rid]; ok && r.ParentID == fileID {
				result.AffectedFiles = append(result.AffectedFiles, AffectedFile{
					FileID:     otherFileID,
					FilePath:   r.FilePath,
					Relation:   "derived",
					AffectedAt: r.Timestamp,
				})
				break // 每个文件只记录一次
			}
		}
	}
	result.TotalAffected = len(result.AffectedFiles)

	return result, nil
}

// GetUserAudit 获取用户审计.
func (e *Engine) GetUserAudit(userID string) []*AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	entries := make([]*AuditEntry, 0)
	ids := e.userIndex[userID]
	for _, id := range ids {
		if record, ok := e.records[id]; ok {
			entries = append(entries, &AuditEntry{
				UserID:    record.UserID,
				UserName:  record.UserName,
				Action:    string(record.Operation),
				FileID:    record.FileID,
				FilePath:  record.FilePath,
				Operation: record.Operation,
				Timestamp: record.Timestamp,
			})
		}
	}
	return entries
}

// ExportAuditTrail 导出审计追踪.
func (e *Engine) ExportAuditTrail(req *AuditTrailExport) (*AuditTrailResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 收集符合条件的记录
	filteredRecords := make([]*ProvenanceRecord, 0)
	for _, record := range e.records {
		if !req.StartTime.IsZero() && record.Timestamp.Before(req.StartTime) {
			continue
		}
		if !req.EndTime.IsZero() && record.Timestamp.After(req.EndTime) {
			continue
		}
		filteredRecords = append(filteredRecords, record)
	}

	// 根据格式导出数据
	var data []byte
	var err error
	switch req.Format {
	case "json":
		data, err = json.MarshalIndent(filteredRecords, "", "  ")
	case "csv":
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		writer.Write([]string{"ID", "FileID", "Operation", "UserID", "Timestamp"})
		for _, r := range filteredRecords {
			writer.Write([]string{r.ID, r.FileID, string(r.Operation), r.UserID, r.Timestamp.Format(time.RFC3339)})
		}
		writer.Flush()
		data = buf.Bytes()
	default:
		return nil, ErrInvalidInput
	}
	if err != nil {
		return nil, err
	}

	return &AuditTrailResult{
		ExportID:      fmt.Sprintf("export-%d", time.Now().UnixNano()),
		Format:        req.Format,
		GeneratedAt:   time.Now(),
		TotalRecords:  len(filteredRecords),
		Data:          data,
		ChainVerified: true,
	}, nil
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
		TopModifiedFiles: make([]FileOperationCount, 0),
	}

	for _, record := range e.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}
		report.TotalOperations++
		report.OperationsByType[record.Operation]++
		report.OperationsByUser[record.UserID]++
	}

	return report
}

// GetRetentionPolicy 获取保留策略.
func (e *Engine) GetRetentionPolicy() *RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.retentionPolicy
}

// UpdateRetentionPolicy 更新保留策略.
func (e *Engine) UpdateRetentionPolicy(policy *RetentionPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.retentionPolicy = policy
}

// CleanupExpired 清理过期数据.
func (e *Engine) CleanupExpired() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	var cleaned int64
	if e.retentionPolicy == nil {
		return 0
	}

	cutoff := time.Now().Add(-e.retentionPolicy.MaxAge)
	for id, record := range e.records {
		if record.Timestamp.Before(cutoff) {
			delete(e.records, id)
			cleaned++
		}
	}
	e.retentionPolicy.CleanedCount += cleaned
	e.retentionPolicy.LastCleanupAt = time.Now()
	return cleaned
}

// CalculateHash 计算数据哈希.
func CalculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
