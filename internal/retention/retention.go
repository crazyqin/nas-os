// Package retention 提供数据保留策略引擎功能
// 支持按文件类型、路径、大小、年龄、标签定义保留策略
// 支持法律保留（Legal Hold）、策略模拟、合规报告等
package retention

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExecutionMode 策略执行模式.
type ExecutionMode string

const (
	// ModeDelete 自动删除.
	ModeDelete ExecutionMode = "delete"
	// ModeArchive 归档到冷存储.
	ModeArchive ExecutionMode = "archive"
	// ModeNotify 通知管理员.
	ModeNotify ExecutionMode = "notify"
	// ModeRecycle 移动到回收站.
	ModeRecycle ExecutionMode = "recycle"
)

// RetentionPeriod 保留期限.
type RetentionPeriod string

const (
	Period7Days     RetentionPeriod = "7d"
	Period30Days    RetentionPeriod = "30d"
	Period90Days    RetentionPeriod = "90d"
	Period1Year     RetentionPeriod = "1y"
	PeriodPermanent RetentionPeriod = "permanent"
)

// ConditionOperator 条件匹配逻辑.
type ConditionOperator string

const (
	OpAnd ConditionOperator = "and"
	OpOr  ConditionOperator = "or"
)

// PolicyCondition 策略条件.
type PolicyCondition struct {
	Field    string   `json:"field"`    // fileType, path, size, age, tags
	Operator string   `json:"operator"` // eq, ne, gt, lt, gte, lte, contains, prefix, matches
	Values   []string `json:"values"`   // 匹配值
}

// RetentionPolicy 数据保留策略.
type RetentionPolicy struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Enabled        bool              `json:"enabled"`
	Priority       int               `json:"priority"`       // 越大优先级越高
	Period         RetentionPeriod   `json:"period"`         // 保留期限
	Mode           ExecutionMode     `json:"mode"`           // 执行模式
	Conditions     []PolicyCondition `json:"conditions"`     // 匹配条件
	ConditionLogic ConditionOperator `json:"conditionLogic"` // 条件逻辑: and/or
	Tags           []string          `json:"tags"`           // 策略标签
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	CreatedBy      string            `json:"createdBy"`
}

// LegalHold 法律保留.
type LegalHold struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	FilePaths   []string   `json:"filePaths"`  // 被保留的文件路径列表（支持通配符）
	CaseNumber  string     `json:"caseNumber"` // 案件编号
	IssuedBy    string     `json:"issuedBy"`   // 发起人
	ExpiresAt   *time.Time `json:"expiresAt"`  // 过期时间，nil表示手动解除
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// AuditEntry 审计日志条目.
type AuditEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // create_policy, update_policy, delete_policy, apply_policy, create_hold, release_hold, delete_file, archive_file, recycle_file
	PolicyID  string    `json:"policyId,omitempty"`
	Target    string    `json:"target"` // 受影响的文件或策略ID
	Details   string    `json:"details"`
	Operator  string    `json:"operator"` // 操作人
	Success   bool      `json:"success"`
}

// FileRecord 文件记录（用于策略匹配）.
type FileRecord struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"` // 字节
	ModTime  time.Time `json:"modTime"`
	FileType string    `json:"fileType"` // 扩展名，如 .pdf, .jpg
	Tags     []string  `json:"tags"`
	IsDir    bool      `json:"isDir"`
}

// SimulationResult 策略模拟结果.
type SimulationResult struct {
	PolicyID       string        `json:"policyId"`
	MatchedFiles   []FileRecord  `json:"matchedFiles"`
	MatchedCount   int           `json:"matchedCount"`
	TotalSize      int64         `json:"totalSize"`
	ProtectedFiles []FileRecord  `json:"protectedFiles"` // 被法律保留保护的文件
	Action         ExecutionMode `json:"action"`
	GeneratedAt    time.Time     `json:"generatedAt"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	GeneratedAt      time.Time    `json:"generatedAt"`
	TotalPolicies    int          `json:"totalPolicies"`
	ActivePolicies   int          `json:"activePolicies"`
	TotalFiles       int          `json:"totalFiles"`
	CoveredFiles     int          `json:"coveredFiles"`
	CoverageRate     float64      `json:"coverageRate"`
	ExpiringFiles    []FileRecord `json:"expiringFiles"`
	ViolatingFiles   []FileRecord `json:"violatingFiles"`
	ActiveLegalHolds int          `json:"activeLegalHolds"`
}

// RetentionEngine 保留策略引擎.
type RetentionEngine struct {
	policies   map[string]*RetentionPolicy
	legalHolds map[string]*LegalHold
	auditLog   []AuditEntry
	files      map[string]*FileRecord // path -> record
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	nextID     int64
}

// NewRetentionEngine 创建保留策略引擎.
func NewRetentionEngine() *RetentionEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &RetentionEngine{
		policies:   make(map[string]*RetentionPolicy),
		legalHolds: make(map[string]*LegalHold),
		auditLog:   make([]AuditEntry, 0),
		files:      make(map[string]*FileRecord),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// generateID 生成唯一ID.
func (e *RetentionEngine) generateID(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), e.nextID)
}

// periodToDuration 将保留期限转换为时间间隔.
func periodToDuration(p RetentionPeriod) (time.Duration, bool) {
	switch p {
	case Period7Days:
		return 7 * 24 * time.Hour, true
	case Period30Days:
		return 30 * 24 * time.Hour, true
	case Period90Days:
		return 90 * 24 * time.Hour, true
	case Period1Year:
		return 365 * 24 * time.Hour, true
	case PeriodPermanent:
		return 0, false // 永不过期
	default:
		return 0, false
	}
}

// CreatePolicy 创建保留策略.
func (e *RetentionEngine) CreatePolicy(p *RetentionPolicy) (*RetentionPolicy, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if p.Name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if p.Period == "" {
		return nil, fmt.Errorf("retention period is required")
	}
	if p.Mode == "" {
		return nil, fmt.Errorf("execution mode is required")
	}

	p.ID = e.generateID("pol")
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	e.policies[p.ID] = p

	e.addAuditEntry("create_policy", p.ID, p.ID, fmt.Sprintf("created policy: %s", p.Name), "", true)
	log.Printf("[Retention] policy created: %s (%s)", p.ID, p.Name)
	return p, nil
}

// UpdatePolicy 更新保留策略.
func (e *RetentionEngine) UpdatePolicy(id string, update *RetentionPolicy) (*RetentionPolicy, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}

	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	if update.Priority != 0 {
		existing.Priority = update.Priority
	}
	if update.Period != "" {
		existing.Period = update.Period
	}
	if update.Mode != "" {
		existing.Mode = update.Mode
	}
	if update.Conditions != nil {
		existing.Conditions = update.Conditions
	}
	if update.ConditionLogic != "" {
		existing.ConditionLogic = update.ConditionLogic
	}
	existing.Enabled = update.Enabled
	existing.UpdatedAt = time.Now()

	e.addAuditEntry("update_policy", id, id, fmt.Sprintf("updated policy: %s", existing.Name), "", true)
	return existing, nil
}

// DeletePolicy 删除保留策略.
func (e *RetentionEngine) DeletePolicy(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.policies[id]
	if !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	name := p.Name
	delete(e.policies, id)
	e.addAuditEntry("delete_policy", id, id, fmt.Sprintf("deleted policy: %s", name), "", true)
	log.Printf("[Retention] policy deleted: %s (%s)", id, name)
	return nil
}

// ListPolicies 列出所有策略.
func (e *RetentionEngine) ListPolicies() []*RetentionPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*RetentionPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		cp := *p
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result
}

// GetPolicy 获取单个策略.
func (e *RetentionEngine) GetPolicy(id string) (*RetentionPolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, ok := e.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	cp := *p
	return &cp, nil
}

// CreateLegalHold 创建法律保留.
func (e *RetentionEngine) CreateLegalHold(h *LegalHold) (*LegalHold, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if h.Name == "" {
		return nil, fmt.Errorf("legal hold name is required")
	}
	if len(h.FilePaths) == 0 {
		return nil, fmt.Errorf("at least one file path is required")
	}

	h.ID = e.generateID("hold")
	h.Active = true
	h.CreatedAt = time.Now()
	e.legalHolds[h.ID] = h

	e.addAuditEntry("create_hold", "", h.ID, fmt.Sprintf("legal hold created: %s, case: %s", h.Name, h.CaseNumber), h.IssuedBy, true)
	log.Printf("[Retention] legal hold created: %s (%s), files: %d", h.ID, h.Name, len(h.FilePaths))
	return h, nil
}

// ReleaseLegalHold 解除法律保留.
func (e *RetentionEngine) ReleaseLegalHold(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hold, ok := e.legalHolds[id]
	if !ok {
		return fmt.Errorf("legal hold %s not found", id)
	}
	if !hold.Active {
		return fmt.Errorf("legal hold %s already released", id)
	}

	hold.Active = false
	e.addAuditEntry("release_hold", "", id, fmt.Sprintf("legal hold released: %s", hold.Name), "", true)
	log.Printf("[Retention] legal hold released: %s (%s)", id, hold.Name)
	return nil
}

// ListLegalHolds 列出所有法律保留.
func (e *RetentionEngine) ListLegalHolds() []*LegalHold {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*LegalHold, 0, len(e.legalHolds))
	for _, h := range e.legalHolds {
		cp := *h
		result = append(result, &cp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// IsFileProtected 检查文件是否被法律保留保护.
func (e *RetentionEngine) IsFileProtected(filePath string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, hold := range e.legalHolds {
		if !hold.Active {
			continue
		}
		// 检查过期
		if hold.ExpiresAt != nil && hold.ExpiresAt.Before(time.Now()) {
			continue
		}
		for _, pattern := range hold.FilePaths {
			if matchPath(pattern, filePath) {
				return true
			}
		}
	}
	return false
}

// RegisterFile 注册文件到引擎.
func (e *RetentionEngine) RegisterFile(f *FileRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.files[f.Path] = f
}

// RegisterFiles 批量注册文件.
func (e *RetentionEngine) RegisterFiles(files []*FileRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range files {
		e.files[f.Path] = f
	}
}

// matchPath 路径匹配（支持 * 和 ** 通配符）.
func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}
	// 精确前缀匹配目录
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	// 通配符匹配
	matched, err := filepath.Match(pattern, filepath.Base(path))
	if err == nil && matched {
		return true
	}
	// 使用filepath.Match对全路径做通配
	matched, err = filepath.Match(pattern, path)
	if err == nil && matched {
		return true
	}
	// 前缀匹配（pattern是目录）
	return strings.HasPrefix(path, pattern+"/")
}

// matchCondition 检查单个条件是否匹配.
func matchCondition(cond PolicyCondition, f *FileRecord) bool {
	switch cond.Field {
	case "fileType":
		for _, v := range cond.Values {
			ext := strings.ToLower(f.FileType)
			val := strings.ToLower(v)
			if !strings.HasPrefix(val, ".") {
				val = "." + val
			}
			if matchOperator(cond.Operator, ext, val) {
				return true
			}
		}
		return false
	case "path":
		for _, v := range cond.Values {
			if matchOperator(cond.Operator, f.Path, v) {
				return true
			}
		}
		return false
	case "size":
		for _, v := range cond.Values {
			var threshold int64
			fmt.Sscanf(v, "%d", &threshold)
			if matchSizeOperator(cond.Operator, f.Size, threshold) {
				return true
			}
		}
		return false
	case "age":
		age := time.Since(f.ModTime)
		for _, v := range cond.Values {
			var days int
			fmt.Sscanf(v, "%d", &days)
			threshold := time.Duration(days) * 24 * time.Hour
			if matchAgeOperator(cond.Operator, age, threshold) {
				return true
			}
		}
		return false
	case "tags":
		for _, v := range cond.Values {
			for _, tag := range f.Tags {
				if matchOperator(cond.Operator, tag, v) {
					return true
				}
			}
		}
		return false
	}
	return false
}

func matchOperator(op, value, pattern string) bool {
	switch op {
	case "eq":
		return value == pattern
	case "ne":
		return value != pattern
	case "contains":
		return strings.Contains(value, pattern)
	case "prefix":
		return strings.HasPrefix(value, pattern)
	case "matches":
		matched, _ := filepath.Match(pattern, value)
		return matched
	default:
		return value == pattern
	}
}

func matchSizeOperator(op string, size, threshold int64) bool {
	switch op {
	case "gt":
		return size > threshold
	case "lt":
		return size < threshold
	case "gte":
		return size >= threshold
	case "lte":
		return size <= threshold
	case "eq":
		return size == threshold
	default:
		return size >= threshold
	}
}

func matchAgeOperator(op string, age, threshold time.Duration) bool {
	switch op {
	case "gt":
		return age > threshold
	case "lt":
		return age < threshold
	case "gte":
		return age >= threshold
	case "lte":
		return age <= threshold
	default:
		return age >= threshold
	}
}

// matchFileToPolicy 检查文件是否匹配策略的所有/任一条件.
func matchFileToPolicy(f *FileRecord, p *RetentionPolicy) bool {
	if len(p.Conditions) == 0 {
		return true
	}
	logic := p.ConditionLogic
	if logic == "" {
		logic = OpAnd
	}
	if logic == OpAnd {
		for _, cond := range p.Conditions {
			if !matchCondition(cond, f) {
				return false
			}
		}
		return true
	}
	// OpOr
	for _, cond := range p.Conditions {
		if matchCondition(cond, f) {
			return true
		}
	}
	return false
}

// isExpired 检查文件是否已过保留期.
func isExpired(f *FileRecord, p *RetentionPolicy) bool {
	if p.Period == PeriodPermanent {
		return false
	}
	dur, ok := periodToDuration(p.Period)
	if !ok {
		return false
	}
	return time.Since(f.ModTime) > dur
}

// getEffectivePolicies 获取有效策略列表（按优先级排序）.
func (e *RetentionEngine) getEffectivePolicies() []*RetentionPolicy {
	policies := make([]*RetentionPolicy, 0)
	for _, p := range e.policies {
		if p.Enabled {
			cp := *p
			policies = append(policies, &cp)
		}
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})
	return policies
}

// ApplyPolicy 应用策略到匹配的文件.
func (e *RetentionEngine) ApplyPolicy(policyID string) (*SimulationResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	result := &SimulationResult{
		PolicyID:    policyID,
		Action:      p.Mode,
		GeneratedAt: time.Now(),
	}

	for _, f := range e.files {
		if !matchFileToPolicy(f, p) {
			continue
		}
		if !isExpired(f, p) {
			continue
		}
		if e.isFileProtectedLocked(f.Path) {
			result.ProtectedFiles = append(result.ProtectedFiles, *f)
			continue
		}
		result.MatchedFiles = append(result.MatchedFiles, *f)
		result.TotalSize += f.Size
	}
	result.MatchedCount = len(result.MatchedFiles)

	// 记录审计日志
	e.addAuditEntry("apply_policy", policyID, policyID,
		fmt.Sprintf("applied policy %s, matched: %d, protected: %d", p.Name, result.MatchedCount, len(result.ProtectedFiles)),
		"", true)

	log.Printf("[Retention] policy %s applied: matched=%d, protected=%d", policyID, result.MatchedCount, len(result.ProtectedFiles))
	return result, nil
}

// Simulate 模拟策略执行效果（不实际执行）.
func (e *RetentionEngine) Simulate(p *RetentionPolicy) *SimulationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &SimulationResult{
		PolicyID:    p.ID,
		Action:      p.Mode,
		GeneratedAt: time.Now(),
	}

	for _, f := range e.files {
		if !matchFileToPolicy(f, p) {
			continue
		}
		if !isExpired(f, p) {
			continue
		}
		if e.isFileProtectedLocked(f.Path) {
			result.ProtectedFiles = append(result.ProtectedFiles, *f)
			continue
		}
		result.MatchedFiles = append(result.MatchedFiles, *f)
		result.TotalSize += f.Size
	}
	result.MatchedCount = len(result.MatchedFiles)
	return result
}

// isFileProtectedLocked 内部方法，需持有锁.
func (e *RetentionEngine) isFileProtectedLocked(filePath string) bool {
	for _, hold := range e.legalHolds {
		if !hold.Active {
			continue
		}
		if hold.ExpiresAt != nil && hold.ExpiresAt.Before(time.Now()) {
			continue
		}
		for _, pattern := range hold.FilePaths {
			if matchPath(pattern, filePath) {
				return true
			}
		}
	}
	return false
}

// GetExpiringFiles 获取即将过期的文件（默认7天内）.
func (e *RetentionEngine) GetExpiringFiles(withinDays int) []FileRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if withinDays <= 0 {
		withinDays = 7
	}
	threshold := time.Duration(withinDays) * 24 * time.Hour
	now := time.Now()

	// 获取有效策略
	policies := make([]*RetentionPolicy, 0)
	for _, p := range e.policies {
		if p.Enabled {
			cp := *p
			policies = append(policies, &cp)
		}
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	seen := make(map[string]bool)
	var result []FileRecord

	for _, f := range e.files {
		if seen[f.Path] {
			continue
		}
		for _, p := range policies {
			if !matchFileToPolicy(f, p) {
				continue
			}
			if p.Period == PeriodPermanent {
				continue
			}
			dur, ok := periodToDuration(p.Period)
			if !ok {
				continue
			}
			expiry := f.ModTime.Add(dur)
			remaining := expiry.Sub(now)
			if remaining > 0 && remaining <= threshold {
				result = append(result, *f)
				seen[f.Path] = true
				break
			}
		}
	}
	return result
}

// GetComplianceReport 生成合规报告.
func (e *RetentionEngine) GetComplianceReport() *ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt: time.Now(),
		TotalFiles:  len(e.files),
	}

	for _, p := range e.policies {
		report.TotalPolicies++
		if p.Enabled {
			report.ActivePolicies++
		}
	}

	for _, h := range e.legalHolds {
		if h.Active {
			report.ActiveLegalHolds++
		}
	}

	// 计算覆盖率：被至少一个策略覆盖的文件比例
	covered := make(map[string]bool)
	for _, f := range e.files {
		for _, p := range e.policies {
			if p.Enabled && matchFileToPolicy(f, p) {
				covered[f.Path] = true
				break
			}
		}
	}
	report.CoveredFiles = len(covered)
	if report.TotalFiles > 0 {
		report.CoverageRate = float64(report.CoveredFiles) / float64(report.TotalFiles) * 100
	}

	// 即将过期文件（7天内）
	report.ExpiringFiles = e.getExpiringFilesLocked(7)

	// 违规文件：已过保留期但未被任何策略处理的文件
	for _, f := range e.files {
		for _, p := range e.policies {
			if !p.Enabled || p.Period == PeriodPermanent {
				continue
			}
			if !matchFileToPolicy(f, p) {
				continue
			}
			dur, ok := periodToDuration(p.Period)
			if !ok {
				continue
			}
			if time.Since(f.ModTime) > dur {
				report.ViolatingFiles = append(report.ViolatingFiles, *f)
				break
			}
		}
	}
	return report
}

// getExpiringFilesLocked 获取即将过期文件（内部，需持有读锁）.
func (e *RetentionEngine) getExpiringFilesLocked(withinDays int) []FileRecord {
	threshold := time.Duration(withinDays) * 24 * time.Hour
	now := time.Now()
	policies := make([]*RetentionPolicy, 0)
	for _, p := range e.policies {
		if p.Enabled {
			cp := *p
			policies = append(policies, &cp)
		}
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority > policies[j].Priority
	})

	seen := make(map[string]bool)
	var result []FileRecord
	for _, f := range e.files {
		if seen[f.Path] {
			continue
		}
		for _, p := range policies {
			if !matchFileToPolicy(f, p) {
				continue
			}
			if p.Period == PeriodPermanent {
				continue
			}
			dur, ok := periodToDuration(p.Period)
			if !ok {
				continue
			}
			expiry := f.ModTime.Add(dur)
			remaining := expiry.Sub(now)
			if remaining > 0 && remaining <= threshold {
				result = append(result, *f)
				seen[f.Path] = true
				break
			}
		}
	}
	return result
}

// GetAuditLog 获取审计日志.
func (e *RetentionEngine) GetAuditLog(limit int) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.auditLog) {
		limit = len(e.auditLog)
	}
	// 返回最新的日志
	start := len(e.auditLog) - limit
	result := make([]AuditEntry, limit)
	copy(result, e.auditLog[start:])
	// 反转，最新在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// addAuditEntry 添加审计日志条目（需已持有写锁）.
func (e *RetentionEngine) addAuditEntry(action, policyID, target, details, operator string, success bool) {
	entry := AuditEntry{
		ID:        e.generateID("audit"),
		Timestamp: time.Now(),
		Action:    action,
		PolicyID:  policyID,
		Target:    target,
		Details:   details,
		Operator:  operator,
		Success:   success,
	}
	e.auditLog = append(e.auditLog, entry)
}
