package privacyproxy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// 审计日志管理器
// 功能：记录每次脱敏操作，支持查询、导出和合规报告生成
// =========================================================================

// Auditor 审计日志管理器（线程安全的环形缓冲）.
type Auditor struct {
	mu       sync.RWMutex
	entries  []AuditEntry // 环形缓冲
	capacity int          // 最大容量
	head     int          // 下一个写入位置
	count    int          // 当前条数
}

// NewAuditor 创建审计日志管理器.
func NewAuditor(capacity int) *Auditor {
	if capacity <= 0 {
		capacity = 10000
	}
	return &Auditor{
		entries:  make([]AuditEntry, capacity),
		capacity: capacity,
	}
}

// Log 记录一条审计日志.
func (a *Auditor) Log(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateID()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	a.entries[a.head] = entry
	a.head = (a.head + 1) % a.capacity
	if a.count < a.capacity {
		a.count++
	}
}

// Query 查询审计日志.
func (a *Auditor) Query(q AuditQuery) ([]AuditEntry, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 收所有有效条目（按时间倒序）
	all := make([]AuditEntry, 0, a.count)
	for i := 0; i < a.count; i++ {
		idx := (a.head - 1 - i + a.capacity) % a.capacity
		entry := a.entries[idx]
		if a.matchQuery(&entry, &q) {
			all = append(all, entry)
		}
	}

	total := len(all)
	// 分页
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []AuditEntry{}, total
	}
	limit := q.Limit
	if limit <= 0 || limit > total-offset {
		limit = total - offset
	}
	return all[offset : offset+limit], total
}

// matchQuery 检查条目是否匹配查询条件.
func (a *Auditor) matchQuery(entry *AuditEntry, q *AuditQuery) bool {
	if q == nil {
		return true
	}
	if q.StartTime != nil && entry.Timestamp.Before(*q.StartTime) {
		return false
	}
	if q.EndTime != nil && entry.Timestamp.After(*q.EndTime) {
		return false
	}
	if q.RuleID != "" && entry.RuleID != q.RuleID {
		return false
	}
	if q.TargetAPI != "" && !strings.Contains(entry.TargetAPI, q.TargetAPI) {
		return false
	}
	if q.Success != nil && entry.Success != *q.Success {
		return false
	}
	return true
}

// GetByID 按 ID 获取审计条目.
func (a *Auditor) GetByID(id string) (*AuditEntry, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for i := 0; i < a.count; i++ {
		idx := (a.head - 1 - i + a.capacity) % a.capacity
		if a.entries[idx].ID == id {
			cp := a.entries[idx]
			return &cp, true
		}
	}
	return nil, false
}

// ExportJSON 导出为 JSON 格式.
func (a *Auditor) ExportJSON(w io.Writer, q AuditQuery) error {
	entries, total := a.Query(q)
	result := map[string]interface{}{
		"total":   total,
		"entries": entries,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// ExportCSV 导出为 CSV 格式.
func (a *Auditor) ExportCSV(w io.Writer, q AuditQuery) error {
	entries, _ := a.Query(q)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	// 写表头
	if err := cw.Write([]string{
		"ID", "时间", "规则ID", "规则名称", "原始文本", "脱敏后", "目标API", "命中次数", "请求方法", "请求路径", "成功", "错误",
	}); err != nil {
		return fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	for _, e := range entries {
		row := []string{
			e.ID,
			e.Timestamp.Format(time.RFC3339),
			e.RuleID,
			e.RuleName,
			e.Original,
			e.Masked,
			e.TargetAPI,
			fmt.Sprintf("%d", e.MatchCount),
			e.RequestMethod,
			e.RequestPath,
			fmt.Sprintf("%t", e.Success),
			e.Error,
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}
	return nil
}

// GenerateReport 生成合规报告.
func (a *Auditor) GenerateReport(start, end time.Time) *ComplianceReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt:    time.Now(),
		PeriodStart:    start,
		PeriodEnd:      end,
		TopRules:       []RuleUsageStat{},
		TopTargetAPIs:  []APIUsageStat{},
		ByAction:       make(map[MaskAction]int),
		MaskedExamples: []AuditEntry{},
	}

	ruleStats := make(map[string]*RuleUsageStat)
	apiStats := make(map[string]*APIUsageStat)
	exampleCount := 0

	for i := 0; i < a.count; i++ {
		idx := (a.head - 1 - i + a.capacity) % a.capacity
		entry := a.entries[idx]

		// 时间范围过滤
		if entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			continue
		}

		report.TotalRequests++

		if entry.Success && entry.MatchCount > 0 {
			report.TotalMasked += entry.MatchCount
		}

		// 规则统计
		stat, ok := ruleStats[entry.RuleID]
		if !ok {
			stat = &RuleUsageStat{RuleID: entry.RuleID, RuleName: entry.RuleName}
			ruleStats[entry.RuleID] = stat
		}
		stat.Count++

		// API 统计
		apiStat, ok := apiStats[entry.TargetAPI]
		if !ok {
			apiStat = &APIUsageStat{TargetAPI: entry.TargetAPI}
			apiStats[entry.TargetAPI] = apiStat
		}
		apiStat.Count++

		// 收集示例（最多 20 条）
		if exampleCount < 20 {
			report.MaskedExamples = append(report.MaskedExamples, entry)
			exampleCount++
		}
	}

	report.UniqueRulesUsed = len(ruleStats)

	// 转换为排序列表
	for _, s := range ruleStats {
		report.TopRules = append(report.TopRules, *s)
	}
	sort.Slice(report.TopRules, func(i, j int) bool {
		return report.TopRules[i].Count > report.TopRules[j].Count
	})
	if len(report.TopRules) > 10 {
		report.TopRules = report.TopRules[:10]
	}

	for _, s := range apiStats {
		report.TopTargetAPIs = append(report.TopTargetAPIs, *s)
	}
	sort.Slice(report.TopTargetAPIs, func(i, j int) bool {
		return report.TopTargetAPIs[i].Count > report.TopTargetAPIs[j].Count
	})
	if len(report.TopTargetAPIs) > 10 {
		report.TopTargetAPIs = report.TopTargetAPIs[:10]
	}

	return report
}

// Stats 返回基本统计信息.
func (a *Auditor) Stats() (total, capacity int) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.count, a.capacity
}

// Clear 清空审计日志.
func (a *Auditor) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.head = 0
	a.count = 0
}
