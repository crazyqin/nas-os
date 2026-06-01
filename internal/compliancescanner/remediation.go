// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RemediationEngine 修复引擎.
type RemediationEngine struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	scanner   *Scanner
	records   map[string]*RemediationRecord
	verified  map[string]bool
}

// NewRemediationEngine 创建修复引擎.
func NewRemediationEngine(logger *zap.Logger, scanner *Scanner) *RemediationEngine {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &RemediationEngine{
		logger:   logger,
		scanner:  scanner,
		records:  make(map[string]*RemediationRecord),
		verified: make(map[string]bool),
	}
}

// SuggestRemediation 生成修复建议.
func (re *RemediationEngine) SuggestRemediation(result *ScanResult) []string {
	suggestions := make([]string, 0)

	if result.Result == ResultPass || result.Result == ResultSkip {
		return suggestions
	}

	// 根据严重级别添加优先级说明
	switch result.Severity {
	case SeverityCritical:
		suggestions = append(suggestions, "[紧急] 立即修复此安全问题")
	case SeverityHigh:
		suggestions = append(suggestions, "[高优] 尽快修复此安全问题")
	case SeverityMedium:
		suggestions = append(suggestions, "[中优] 建议在下次维护窗口修复")
	case SeverityLow:
		suggestions = append(suggestions, "[低优] 可安排在合适时间修复")
	}

	// 添加具体的修复建议
	if result.Remediation != "" {
		suggestions = append(suggestions, result.Remediation)
	}

	// 根据类别添加通用建议
	switch result.Category {
	case CategorySystemConfig:
		suggestions = append(suggestions, "检查系统配置文件并更新安全设置")
	case CategoryFilePermission:
		suggestions = append(suggestions, "使用 chmod/chown 命令修复文件权限")
	case CategoryNetworkSecurity:
		suggestions = append(suggestions, "配置防火墙规则，关闭不必要的端口")
	case CategoryServiceSecurity:
		suggestions = append(suggestions, "检查服务配置，禁用不必要的服务")
	case CategoryCryptoCompliance:
		suggestions = append(suggestions, "更新加密配置，使用安全的算法和协议")
	}

	return suggestions
}

// AutoRemediate 自动修复.
func (re *RemediationEngine) AutoRemediate(ctx context.Context, result *ScanResult) (*RemediationRecord, error) {
	if result.Result == ResultPass || result.Result == ResultSkip {
		return nil, fmt.Errorf("无需修复: 结果为 %s", result.Result)
	}

	record := &RemediationRecord{
		ID:         fmt.Sprintf("rem-%s-%d", result.RuleID, time.Now().UnixNano()),
		RuleID:     result.RuleID,
		ResultID:   result.ID,
		Action:     result.Remediation,
		ExecutedAt: time.Now(),
		ExecutedBy: "auto",
	}

	startTime := time.Now()

	// 查找对应的修复函数
	fixFuncName := re.getFixFuncName(result.RuleID)
	if fixFuncName == "" {
		record.Status = "skipped"
		record.Details = "无自动修复函数"
		re.storeRecord(record)
		return record, nil
	}

	// 执行修复
	err := re.scanner.ExecuteFix(ctx, fixFuncName)
	record.Duration = time.Since(startTime)

	if err != nil {
		record.Status = "failed"
		record.Details = fmt.Sprintf("修复失败: %v", err)
		re.logger.Error("自动修复失败",
			zap.String("rule_id", result.RuleID),
			zap.String("fix_func", fixFuncName),
			zap.Error(err),
		)
	} else {
		record.Status = "success"
		record.Details = "修复成功"
		re.logger.Info("自动修复成功",
			zap.String("rule_id", result.RuleID),
			zap.String("fix_func", fixFuncName),
		)
	}

	re.storeRecord(record)
	return record, nil
}

// getFixFuncName 获取修复函数名称.
func (re *RemediationEngine) getFixFuncName(ruleID string) string {
	// 映射规则ID到修复函数
	fixMap := map[string]string{
		"CIS-6.1.1":    "fixPasswdPermissions",
		"CIS-6.1.2":    "fixShadowPermissions",
		"CIS-3.1.1":    "disableIPForward",
		"MLPS2-8.1.4.1": "fixPasswdPermissions",
	}
	if fn, ok := fixMap[ruleID]; ok {
		return fn
	}
	return ""
}

// storeRecord 存储修复记录.
func (re *RemediationEngine) storeRecord(record *RemediationRecord) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.records[record.ID] = record
}

// GetRecord 获取修复记录.
func (re *RemediationEngine) GetRecord(id string) (*RemediationRecord, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	record, exists := re.records[id]
	if !exists {
		return nil, fmt.Errorf("修复记录不存在: %s", id)
	}
	return record, nil
}

// GetAllRecords 获取所有修复记录.
func (re *RemediationEngine) GetAllRecords() []*RemediationRecord {
	re.mu.RLock()
	defer re.mu.RUnlock()

	records := make([]*RemediationRecord, 0, len(re.records))
	for _, record := range re.records {
		records = append(records, record)
	}
	return records
}

// GetRecordsByRule 获取指定规则的修复记录.
func (re *RemediationEngine) GetRecordsByRule(ruleID string) []*RemediationRecord {
	re.mu.RLock()
	defer re.mu.RUnlock()

	records := make([]*RemediationRecord, 0)
	for _, record := range re.records {
		if record.RuleID == ruleID {
			records = append(records, record)
		}
	}
	return records
}

// VerifyRemediation 验证修复.
func (re *RemediationEngine) VerifyRemediation(ctx context.Context, recordID string) (bool, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	record, exists := re.records[recordID]
	if !exists {
		return false, fmt.Errorf("修复记录不存在: %s", recordID)
	}

	if record.Status != "success" {
		return false, fmt.Errorf("修复未成功执行，无法验证")
	}

	// 重新执行检查来验证修复
	checkFuncName := re.getCheckFuncName(record.RuleID)
	if checkFuncName == "" {
		return false, fmt.Errorf("无对应的检查函数")
	}

	result, err := re.scanner.ExecuteCheck(ctx, checkFuncName)
	if err != nil {
		return false, fmt.Errorf("验证检查执行失败: %v", err)
	}

	isVerified := result.Result == ResultPass
	if isVerified {
		now := time.Now()
		record.Verified = true
		record.VerifiedAt = &now
		re.verified[recordID] = true
		re.logger.Info("修复验证成功", zap.String("record_id", recordID))
	} else {
		re.logger.Warn("修复验证失败",
			zap.String("record_id", recordID),
			zap.String("result", string(result.Result)),
		)
	}

	return isVerified, nil
}

// getCheckFuncName 获取检查函数名称.
func (re *RemediationEngine) getCheckFuncName(ruleID string) string {
	checkMap := map[string]string{
		"CIS-6.1.1":     "checkPasswdPermissions",
		"CIS-6.1.2":     "checkShadowPermissions",
		"CIS-3.1.1":     "checkIPForward",
		"MLPS2-8.1.4.1": "checkImportantFiles",
	}
	if fn, ok := checkMap[ruleID]; ok {
		return fn
	}
	return ""
}

// GetStats 获取修复统计.
func (re *RemediationEngine) GetStats() map[string]int {
	re.mu.RLock()
	defer re.mu.RUnlock()

	stats := map[string]int{
		"total":      len(re.records),
		"success":    0,
		"failed":     0,
		"skipped":    0,
		"verified":   0,
		"unverified": 0,
	}

	for _, record := range re.records {
		switch record.Status {
		case "success":
			stats["success"]++
		case "failed":
			stats["failed"]++
		case "skipped":
			stats["skipped"]++
		}

		if record.Verified {
			stats["verified"]++
		} else if record.Status == "success" {
			stats["unverified"]++
		}
	}

	return stats
}

// GetRecentRecords 获取最近的修复记录.
func (re *RemediationEngine) GetRecentRecords(limit int) []*RemediationRecord {
	re.mu.RLock()
	defer re.mu.RUnlock()

	records := make([]*RemediationRecord, 0, len(re.records))
	for _, record := range re.records {
		records = append(records, record)
	}

	// 按执行时间排序
	sortRemediationRecords(records)

	if limit > 0 && limit < len(records) {
		records = records[:limit]
	}

	return records
}

// sortRemediationRecords 按时间排序修复记录.
func sortRemediationRecords(records []*RemediationRecord) {
	for i := 1; i < len(records); i++ {
		key := records[i]
		j := i - 1
		for j >= 0 && records[j].ExecutedAt.Before(key.ExecutedAt) {
			records[j+1] = records[j]
			j--
		}
		records[j+1] = key
	}
}

// ClearRecords 清除修复记录.
func (re *RemediationEngine) ClearRecords() {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.records = make(map[string]*RemediationRecord)
	re.verified = make(map[string]bool)
	re.logger.Info("清除所有修复记录")
}

// ExportRecords 导出修复记录.
func (re *RemediationEngine) ExportRecords() []*RemediationRecord {
	re.mu.RLock()
	defer re.mu.RUnlock()

	records := make([]*RemediationRecord, 0, len(re.records))
	for _, record := range re.records {
		records = append(records, record)
	}
	return records
}
