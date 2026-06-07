// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Engine 溯源引擎.
type Engine struct {
	mu             sync.RWMutex
	records        map[string]*ProvenanceRecord // id -> record
	fileIndex      map[string][]string          // fileID -> record ids
	userIndex      map[string][]string          // userID -> record ids
	operationIndex map[OperationType][]string   // operation -> record ids
	lineage        map[string]*FileLineage      // fileID -> lineage
	chains         map[string]*AuditChain       // chainID -> chain
	dataLineages   map[string]*DataLineage      // dataID -> lineage
	retention      *RetentionPolicy
}

// NewEngine 创建溯源引擎.
func NewEngine(retention *RetentionPolicy) *Engine {
	if retention == nil {
		retention = &RetentionPolicy{
			MaxAge:     365 * 24 * time.Hour, // 默认保留1年
			MaxRecords: 1000000,              // 默认最多100万条
		}
	}
	return &Engine{
		records:        make(map[string]*ProvenanceRecord),
		fileIndex:      make(map[string][]string),
		userIndex:      make(map[string][]string),
		operationIndex: make(map[OperationType][]string),
		lineage:        make(map[string]*FileLineage),
		chains:         make(map[string]*AuditChain),
		dataLineages:   make(map[string]*DataLineage),
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

// RecordOrigin 记录数据来源.
func (e *Engine) RecordOrigin(dataID, dataType, location string, userID string, metadata map[string]string) (*ProvenanceRecord, error) {
	if dataID == "" || dataType == "" {
		return nil, ErrInvalidInput
	}

	record := &ProvenanceRecord{
		ID:          fmt.Sprintf("origin-%s-%d", dataID, time.Now().UnixNano()),
		FileID:      dataID,
		FilePath:    location,
		Operation:   OpCreate,
		UserID:      userID,
		Source:      SourceGenerate,
		Metadata:    metadata,
		Description: fmt.Sprintf("Data origin recorded: %s", dataType),
	}

	if err := e.RecordOperation(record); err != nil {
		return nil, err
	}

	// 创建数据血缘
	dl := &DataLineage{
		DataID:          dataID,
		DataType:        dataType,
		Origin:          record,
		CurrentLocation: location,
		Transformations: make([]Transformation, 0),
	}

	e.mu.Lock()
	e.dataLineages[dataID] = dl
	e.mu.Unlock()

	return record, nil
}

// TraceLineage 追踪数据血缘关系.
func (e *Engine) TraceLineage(dataID string) (*DataLineage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dl, ok := e.dataLineages[dataID]
	if !ok {
		return nil, ErrFileNotFound
	}
	return dl, nil
}

// VerifyChain 验证审计链完整性.
func (e *Engine) VerifyChain(chainID string) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	chain, ok := e.chains[chainID]
	if !ok {
		return false, ErrRecordNotFound
	}

	if len(chain.Blocks) == 0 {
		return true, nil
	}

	// 验证创世区块
	if chain.Blocks[0].PreviousHash != "" {
		return false, ErrIntegrityCheckFailed
	}

	// 验证链式完整性
	for i := 1; i < len(chain.Blocks); i++ {
		current := chain.Blocks[i]
		previous := chain.Blocks[i-1]

		// 验证前哈希链接
		if current.PreviousHash != previous.Hash {
			return false, ErrIntegrityCheckFailed
		}

		// 验证区块哈希
		expectedHash := e.calculateBlockHash(current)
		if current.Hash != expectedHash {
			return false, ErrIntegrityCheckFailed
		}
	}

	return true, nil
}

// QueryProvenance 高级溯源查询.
func (e *Engine) QueryProvenance(query *ProvenanceQuery) ([]*ProvenanceRecord, error) {
	if query == nil {
		return nil, ErrInvalidInput
	}

	results := e.QueryRecords(query.Filter)

	// 按合规标签过滤
	if len(query.ComplianceFilter) > 0 {
		filtered := make([]*ProvenanceRecord, 0)
		for _, r := range results {
			if e.matchComplianceTag(r, query.ComplianceFilter) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	return results, nil
}

// ExportAuditTrail 导出审计追踪.
func (e *Engine) ExportAuditTrail(config *AuditTrailExport) (*AuditTrailResult, error) {
	if config == nil {
		return nil, ErrInvalidInput
	}

	filter := QueryFilter{
		StartTime: &config.StartTime,
		EndTime:   &config.EndTime,
	}

	records := e.QueryRecords(filter)

	var data []byte
	var err error

	switch config.Format {
	case "json":
		data, err = e.exportJSON(records)
	case "csv":
		data, err = e.exportCSV(records)
	case "xml":
		data, err = e.exportXML(records)
	default:
		return nil, ErrInvalidInput
	}

	if err != nil {
		return nil, err
	}

	result := &AuditTrailResult{
		ExportID:      fmt.Sprintf("export-%d", time.Now().UnixNano()),
		Format:        config.Format,
		GeneratedAt:   time.Now(),
		TotalRecords:  len(records),
		Data:          data,
		ChainVerified: true,
	}

	return result, nil
}

// AddToChain 将记录添加到审计链.
func (e *Engine) AddToChain(chainID string, recordID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.records[recordID]
	if !ok {
		return ErrRecordNotFound
	}

	chain, exists := e.chains[chainID]
	if !exists {
		chain = &AuditChain{
			ChainID:   chainID,
			Blocks:    make([]AuditBlock, 0),
			CreatedAt: time.Now(),
		}
		e.chains[chainID] = chain
	}

	// 创建新区块
	block := AuditBlock{
		Index:     int64(len(chain.Blocks)),
		Timestamp: time.Now(),
		RecordID:  recordID,
		Data:      []byte(fmt.Sprintf("%+v", record)),
	}

	if len(chain.Blocks) > 0 {
		block.PreviousHash = chain.LastBlockHash
	}

	block.Hash = e.calculateBlockHash(block)
	chain.Blocks = append(chain.Blocks, block)
	chain.LastBlockHash = block.Hash

	return nil
}

// AddComplianceTag 为记录添加合规标签.
func (e *Engine) AddComplianceTag(dataID string, tag ComplianceTag) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dl, ok := e.dataLineages[dataID]
	if !ok {
		return ErrFileNotFound
	}

	for _, t := range dl.DataClassification {
		if t == tag {
			return nil // 已存在
		}
	}

	dl.DataClassification = append(dl.DataClassification, tag)
	return nil
}

// GetAuditChain 获取审计链.
func (e *Engine) GetAuditChain(chainID string) (*AuditChain, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	chain, ok := e.chains[chainID]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return chain, nil
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

// calculateBlockHash 计算区块哈希.
func (e *Engine) calculateBlockHash(block AuditBlock) string {
	data := fmt.Sprintf("%d%s%s%s", block.Index, block.Timestamp.Format(time.RFC3339Nano), block.RecordID, block.PreviousHash)
	return CalculateHash([]byte(data))
}

// matchComplianceTag 检查记录是否匹配合规标签.
func (e *Engine) matchComplianceTag(record *ProvenanceRecord, tags []ComplianceTag) bool {
	if record.Metadata == nil {
		return false
	}

	for _, tag := range tags {
		if record.Metadata["compliance"] == string(tag) {
			return true
		}
	}
	return false
}

// exportJSON 导出为JSON格式.
func (e *Engine) exportJSON(records []*ProvenanceRecord) ([]byte, error) {
	type ExportData struct {
		Records    []*ProvenanceRecord `json:"records"`
		Total      int                 `json:"total"`
		ExportedAt time.Time           `json:"exported_at"`
	}

	data := ExportData{
		Records:    records,
		Total:      len(records),
		ExportedAt: time.Now(),
	}

	return json.MarshalIndent(data, "", "  ")
}

// exportCSV 导出为CSV格式.
func (e *Engine) exportCSV(records []*ProvenanceRecord) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// 写入表头
	header := []string{"ID", "FileID", "FilePath", "Operation", "UserID", "UserName", "Timestamp", "Source", "FileSize", "Description"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// 写入数据
	for _, r := range records {
		row := []string{
			r.ID,
			r.FileID,
			r.FilePath,
			string(r.Operation),
			r.UserID,
			r.UserName,
			r.Timestamp.Format(time.RFC3339),
			string(r.Source),
			fmt.Sprintf("%d", r.FileSize),
			r.Description,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), writer.Error()
}

// exportXML 导出为XML格式.
func (e *Engine) exportXML(records []*ProvenanceRecord) ([]byte, error) {
	type XMLRecord struct {
		ID          string `xml:"ID"`
		FileID      string `xml:"FileID"`
		FilePath    string `xml:"FilePath"`
		Operation   string `xml:"Operation"`
		UserID      string `xml:"UserID"`
		UserName    string `xml:"UserName"`
		Timestamp   string `xml:"Timestamp"`
		Source      string `xml:"Source"`
		FileSize    int64  `xml:"FileSize"`
		Description string `xml:"Description"`
	}

	type XMLExport struct {
		XMLName xml.Name    `xml:"AuditTrail"`
		Records []XMLRecord `xml:"Record"`
		Total   int         `xml:"Total,attr"`
	}

	data := XMLExport{
		Total: len(records),
	}

	for _, r := range records {
		data.Records = append(data.Records, XMLRecord{
			ID:          r.ID,
			FileID:      r.FileID,
			FilePath:    r.FilePath,
			Operation:   string(r.Operation),
			UserID:      r.UserID,
			UserName:    r.UserName,
			Timestamp:   r.Timestamp.Format(time.RFC3339),
			Source:      string(r.Source),
			FileSize:    r.FileSize,
			Description: r.Description,
		})
	}

	return xml.MarshalIndent(data, "", "  ")
}

// CalculateHash 计算数据哈希.
func CalculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
