// Package compliancescan 提供数据合规扫描核心业务逻辑
package compliancescan

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// compiledRule 编译后的规则.
type compiledRule struct {
	rule *ScanRule
	re   *regexp.Regexp
}

// Manager 合规扫描管理器.
type Manager struct {
	mu           sync.RWMutex
	storagePath  string
	rules        map[string]*ScanRule
	tasks        map[string]*ScanTask
	results      map[string]*ScanResult
	violations   map[string]*Violation
	schedules    map[string]*ScanSchedule
}

// NewManager 创建合规扫描管理器.
func NewManager(storagePath string) *Manager {
	m := &Manager{
		storagePath: storagePath,
		rules:       make(map[string]*ScanRule),
		tasks:       make(map[string]*ScanTask),
		results:     make(map[string]*ScanResult),
		violations:  make(map[string]*Violation),
		schedules:   make(map[string]*ScanSchedule),
	}

	// 加载内置规则
	for _, rule := range m.getBuiltinRulesData() {
		m.rules[rule.ID] = &rule
	}

	return m
}

// AddRule 添加扫描规则.
func (m *Manager) AddRule(ctx context.Context, rule ScanRule) (*ScanRule, error) {
	if rule.Name == "" || rule.Pattern == "" {
		return nil, ErrInvalidRule
	}

	// 验证正则表达式
	if _, err := regexp.Compile(rule.Pattern); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRule, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()

	m.rules[rule.ID] = &rule
	return &rule, nil
}

// UpdateRule 更新扫描规则.
func (m *Manager) UpdateRule(ctx context.Context, ruleID string, rule ScanRule) error {
	// 验证正则表达式
	if rule.Pattern != "" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidRule, err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}

	if rule.Name != "" {
		existing.Name = rule.Name
	}
	if rule.Category != "" {
		existing.Category = rule.Category
	}
	if rule.Pattern != "" {
		existing.Pattern = rule.Pattern
	}
	if rule.Severity != "" {
		existing.Severity = rule.Severity
	}
	if rule.Action != "" {
		existing.Action = rule.Action
	}
	if rule.Description != "" {
		existing.Description = rule.Description
	}
	existing.Enabled = rule.Enabled

	return nil
}

// CreateScanTask 创建扫描任务.
func (m *Manager) CreateScanTask(ctx context.Context, task ScanTask) (*ScanTask, error) {
	if task.TargetPath == "" {
		return nil, fmt.Errorf("目标路径不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.Status = StatusPending
	task.Progress = 0
	task.CreatedAt = time.Now()

	m.tasks[task.ID] = &task
	return &task, nil
}

// RunScan 执行扫描任务.
func (m *Manager) RunScan(ctx context.Context, taskID string) (*ScanResult, error) {
	m.mu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return nil, ErrTaskNotFound
	}
	if task.Status != StatusPending {
		m.mu.Unlock()
		return nil, ErrTaskNotPending
	}

	// 更新任务状态
	now := time.Now()
	task.Status = StatusScanning
	task.StartTime = &now
	task.Progress = 0
	m.mu.Unlock()

	// 收集要使用的规则
	m.mu.RLock()
	var scanRules []*ScanRule
	if len(task.RuleIDs) > 0 {
		for _, ruleID := range task.RuleIDs {
			if rule, ok := m.rules[ruleID]; ok && rule.Enabled {
				scanRules = append(scanRules, rule)
			}
		}
	} else {
		for _, rule := range m.rules {
			if rule.Enabled {
				scanRules = append(scanRules, rule)
			}
		}
	}
	m.mu.RUnlock()

	// 扫描文件
	startTime := time.Now()
	totalFiles, scannedFiles, violations := m.scanPath(ctx, task.TargetPath, taskID, scanRules, task)
	duration := time.Since(startTime)

	// 计算风险分数
	riskScore := m.calculateRiskScore(violations, scannedFiles)

	// 创建结果
	result := &ScanResult{
		ID:             uuid.New().String(),
		TaskID:         taskID,
		TotalFiles:     totalFiles,
		ScannedFiles:   scannedFiles,
		ViolationCount: len(violations),
		RiskScore:      riskScore,
		Duration:       duration.String(),
		CreatedAt:      time.Now(),
	}

	// 存储结果和违规
	m.mu.Lock()
	m.results[result.ID] = result
	for i := range violations {
		violations[i].ResultID = result.ID
		m.violations[violations[i].ID] = &violations[i]
	}

	// 更新任务状态
	endTime := time.Now()
	task.Status = StatusCompleted
	task.EndTime = &endTime
	task.Progress = 100
	m.mu.Unlock()

	return result, nil
}

// scanPath 扫描指定路径.
func (m *Manager) scanPath(ctx context.Context, targetPath, taskID string, rules []*ScanRule, task *ScanTask) (int, int, []Violation) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return 0, 0, nil
	}

	// 收集文件
	var files []string
	if info.IsDir() {
		filepath.Walk(targetPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			// 只扫描文本文件
			if isTextFile(path) {
				files = append(files, path)
			}
			return nil
		})
	} else {
		files = append(files, targetPath)
	}

	totalFiles := len(files)
	scannedFiles := 0
	var violations []Violation

	// 编译正则
	var compiledRules []compiledRule
	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		compiledRules = append(compiledRules, compiledRule{rule: rule, re: re})
	}

	// 扫描每个文件
	for i, filePath := range files {
		select {
		case <-ctx.Done():
			return totalFiles, scannedFiles, violations
		default:
		}

		fileViolations := m.scanFile(filePath, compiledRules)
		violations = append(violations, fileViolations...)
		scannedFiles++

		// 更新进度
		m.mu.Lock()
		task.Progress = float64(i+1) / float64(totalFiles) * 100
		m.mu.Unlock()
	}

	return totalFiles, scannedFiles, violations
}

// scanFile 扫描单个文件.
func (m *Manager) scanFile(filePath string, compiledRules []compiledRule) []Violation {
	var violations []Violation

	file, err := os.Open(filePath)
	if err != nil {
		return violations
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, cr := range compiledRules {
			locs := cr.re.FindAllStringIndex(line, -1)
			for _, loc := range locs {
				matchValue := line[loc[0]:loc[1]]
				masked := maskContent(matchValue)

				v := Violation{
					ID:           uuid.New().String(),
				RuleID:       cr.rule.ID,
				RuleName:     cr.rule.Name,
				FilePath:     filePath,
				LineNumber:   lineNum,
				MatchContent: masked,
				Severity:     cr.rule.Severity,
				Action:       cr.rule.Action,
				Resolved:     false,
				CreatedAt:    time.Now(),
			}
			violations = append(violations, v)
			}
		}
	}

	return violations
}

// GetViolations 获取违规列表.
func (m *Manager) GetViolations(ctx context.Context, resultID, severity string) []Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var violations []Violation
	for _, v := range m.violations {
		if resultID != "" && v.ResultID != resultID {
			continue
		}
		if severity != "" && string(v.Severity) != severity {
			continue
		}
		violations = append(violations, *v)
	}

	sort.Slice(violations, func(i, j int) bool {
		return violations[i].CreatedAt.After(violations[j].CreatedAt)
	})

	return violations
}

// ResolveViolation 解决违规.
func (m *Manager) ResolveViolation(ctx context.Context, violationID, resolvedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, exists := m.violations[violationID]
	if !exists {
		return ErrViolationNotFound
	}
	if v.Resolved {
		return ErrViolationAlreadyResolved
	}

	now := time.Now()
	v.Resolved = true
	v.ResolvedBy = resolvedBy
	v.ResolvedAt = &now

	return nil
}

// QuarantineFile 隔离违规文件.
func (m *Manager) QuarantineFile(ctx context.Context, violationID string) error {
	m.mu.RLock()
	v, exists := m.violations[violationID]
	if !exists {
		m.mu.RUnlock()
		return ErrViolationNotFound
	}
	filePath := v.FilePath
	m.mu.RUnlock()

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ErrFileNotQuarantinable
	}

	// 创建隔离目录
	quarantineDir := filepath.Join(m.storagePath, "quarantine")
	if err := os.MkdirAll(quarantineDir, 0750); err != nil {
		return fmt.Errorf("创建隔离目录失败: %w", err)
	}

	// 移动文件到隔离区
	destPath := filepath.Join(quarantineDir, filepath.Base(filePath)+"."+time.Now().Format("20060102150405"))
	if err := os.Rename(filePath, destPath); err != nil {
		return fmt.Errorf("隔离文件失败: %w", err)
	}

	return nil
}

// GenerateReport 生成合规报告.
func (m *Manager) GenerateReport(ctx context.Context) (*ComplianceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ComplianceReport{
		GeneratedAt:          time.Now(),
		TotalScans:           len(m.results),
		ViolationsBySeverity: make(map[string]int),
		ViolationsByCategory: make(map[string]int),
	}

	// 统计违规
	ruleViolationCount := make(map[string]int)
	for _, v := range m.violations {
		report.ViolationsBySeverity[string(v.Severity)]++
		if rule, ok := m.rules[v.RuleID]; ok {
			report.ViolationsByCategory[string(rule.Category)]++
		}
		ruleViolationCount[v.RuleID]++
	}

	// 构建 TopViolatedRules
	var ruleSummaries []RuleSummary
	for ruleID, count := range ruleViolationCount {
		if rule, ok := m.rules[ruleID]; ok {
			ruleSummaries = append(ruleSummaries, RuleSummary{
				RuleID:         ruleID,
				RuleName:       rule.Name,
				ViolationCount: count,
				Severity:       rule.Severity,
			})
		}
	}
	sort.Slice(ruleSummaries, func(i, j int) bool {
		return ruleSummaries[i].ViolationCount > ruleSummaries[j].ViolationCount
	})
	if len(ruleSummaries) > 10 {
		ruleSummaries = ruleSummaries[:10]
	}
	report.TopViolatedRules = ruleSummaries

	// 计算风险分数
	totalViolations := 0
	for _, count := range report.ViolationsBySeverity {
		totalViolations += count
	}
	if totalViolations > 0 {
		weighted := float64(report.ViolationsBySeverity["critical"])*4 +
			float64(report.ViolationsBySeverity["high"])*3 +
			float64(report.ViolationsBySeverity["medium"])*2 +
			float64(report.ViolationsBySeverity["low"])*1
		report.RiskScore = math.Min(100, weighted/float64(totalViolations)*25)
	}

	// 生成建议
	report.Recommendations = m.generateRecommendations(report)

	return report, nil
}

// ClassifyData 数据分类.
func (m *Manager) ClassifyData(ctx context.Context, filePath string) (*DataClassification, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	classification := &DataClassification{
		FilePath:    filePath,
		LastScanned: time.Now(),
	}

	// 如果是目录，扫描目录下所有文件
	if info.IsDir() {
		var categories []string
		violationCount := 0
		filepath.Walk(filePath, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			if isTextFile(path) {
				cats, vc := m.classifyFile(path)
				categories = append(categories, cats...)
				violationCount += vc
			}
			return nil
		})
		classification.Categories = uniqueStrings(categories)
		classification.ViolationCount = violationCount
	} else {
		categories, vc := m.classifyFile(filePath)
		classification.Categories = categories
		classification.ViolationCount = vc
	}

	// 确定敏感级别
	classification.SensitivityLevel = m.determineSensitivity(classification.Categories, classification.ViolationCount)

	return classification, nil
}

// classifyFile 分类单个文件.
func (m *Manager) classifyFile(filePath string) ([]string, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0
	}
	defer file.Close()

	categories := make(map[string]bool)
	violationCount := 0

	// 编译规则
	var compiledRules []compiledRule
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		compiledRules = append(compiledRules, compiledRule{rule: rule, re: re})
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, cr := range compiledRules {
			if cr.re.MatchString(line) {
				categories[string(cr.rule.Category)] = true
				violationCount++
			}
		}
	}

	result := make([]string, 0, len(categories))
	for cat := range categories {
		result = append(result, cat)
	}
	return result, violationCount
}

// determineSensitivity 确定敏感级别.
func (m *Manager) determineSensitivity(categories []string, violationCount int) SensitivityLevel {
	if violationCount == 0 {
		return SensitivityPublic
	}

	categorySet := make(map[string]bool)
	for _, c := range categories {
		categorySet[c] = true
	}

	if categorySet["health"] || categorySet["financial"] {
		return SensitivityRestricted
	}
	if categorySet["pii"] && violationCount > 10 {
		return SensitivityConfidential
	}
	if violationCount > 0 {
		return SensitivityInternal
	}
	return SensitivityPublic
}

// GetBuiltinRules 获取内置规则.
func (m *Manager) GetBuiltinRules(ctx context.Context) []ScanRule {
	return m.getBuiltinRulesData()
}

// ScheduleScan 定时扫描.
func (m *Manager) ScheduleScan(ctx context.Context, taskID, cronExpr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[taskID]; !exists {
		return ErrTaskNotFound
	}

	schedule := &ScanSchedule{
		ID:       uuid.New().String(),
		TaskID:   taskID,
		CronExpr: cronExpr,
		Enabled:  true,
	}

	m.schedules[schedule.ID] = schedule
	return nil
}

// getBuiltinRulesData 获取内置规则数据.
func (m *Manager) getBuiltinRulesData() []ScanRule {
	return []ScanRule{
		{
			ID:          "builtin-id-card",
			Name:        "中国身份证号检测",
			Category:    CategoryPII,
			Pattern:     `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
			Severity:    SeverityHigh,
			Enabled:     true,
			Action:      ActionLog,
			Description: "检测18位中国居民身份证号码",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-phone",
			Name:        "手机号码检测",
			Category:    CategoryPII,
			Pattern:     `\b1[3-9]\d{9}\b`,
			Severity:    SeverityMedium,
			Enabled:     true,
			Action:      ActionLog,
			Description: "检测中国大陆手机号码",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-email",
			Name:        "电子邮箱检测",
			Category:    CategoryPII,
			Pattern:     `\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`,
			Severity:    SeverityLow,
			Enabled:     true,
			Action:      ActionLog,
			Description: "检测电子邮箱地址",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-bank-card",
			Name:        "银行卡号检测",
			Category:    CategoryFinancial,
			Pattern:     `\b(?:6[0-9]{15,18}|4[0-9]{12,15}|5[1-5][0-9]{14}|3[47][0-9]{13})\b`,
			Severity:    SeverityCritical,
			Enabled:     true,
			Action:      ActionNotify,
			Description: "检测银行卡号（含 Visa/MasterCard/银联）",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-passport",
			Name:        "护照号码检测",
			Category:    CategoryPII,
			Pattern:     `\b[A-Z][0-9]{8}\b`,
			Severity:    SeverityHigh,
			Enabled:     true,
			Action:      ActionLog,
			Description: "检测中国护照号码",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-ip-address",
			Name:        "IP地址检测",
			Category:    CategoryPII,
			Pattern:     `\b(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			Severity:    SeverityLow,
			Enabled:     true,
			Action:      ActionLog,
			Description: "检测IPv4地址",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-medical-record",
			Name:        "医疗记录号检测",
			Category:    CategoryHealth,
			Pattern:     `\b(?:病历号|住院号|门诊号)[：:]\s*[A-Z0-9]{6,20}\b`,
			Severity:    SeverityCritical,
			Enabled:     true,
			Action:      ActionEncrypt,
			Description: "检测医疗记录编号",
			CreatedAt:   time.Now(),
		},
		{
			ID:          "builtin-ssn",
			Name:        "社会安全号检测",
			Category:    CategoryPII,
			Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
			Severity:    SeverityCritical,
			Enabled:     true,
			Action:      ActionQuarantine,
			Description: "检测美国社会安全号码（SSN）",
			CreatedAt:   time.Now(),
		},
	}
}

// calculateRiskScore 计算风险分数.
func (m *Manager) calculateRiskScore(violations []Violation, scannedFiles int) float64 {
	if scannedFiles == 0 || len(violations) == 0 {
		return 0
	}

	severityWeight := map[Severity]float64{
		SeverityLow:      1,
		SeverityMedium:   3,
		SeverityHigh:     7,
		SeverityCritical: 10,
	}

	totalWeight := 0.0
	for _, v := range violations {
		totalWeight += severityWeight[v.Severity]
	}

	// 归一化到 0-100
	maxPossible := float64(len(violations)) * 10
	score := (totalWeight / maxPossible) * 100

	// 考虑违规密度
	density := float64(len(violations)) / float64(scannedFiles)
	score = score * (1 + density*0.5)

	return math.Min(100, math.Round(score*100)/100)
}

// generateRecommendations 生成建议.
func (m *Manager) generateRecommendations(report *ComplianceReport) []string {
	var recommendations []string

	if report.ViolationsBySeverity["critical"] > 0 {
		recommendations = append(recommendations, "存在严重违规，建议立即处理并加强数据保护措施")
	}
	if report.ViolationsBySeverity["high"] > 0 {
		recommendations = append(recommendations, "存在高风险违规，建议定期审查敏感数据访问权限")
	}
	if report.ViolationsByCategory["pii"] > 0 {
		recommendations = append(recommendations, "检测到个人身份信息泄露风险，建议实施数据脱敏措施")
	}
	if report.ViolationsByCategory["financial"] > 0 {
		recommendations = append(recommendations, "检测到金融数据风险，建议加密存储并限制访问权限")
	}
	if report.ViolationsByCategory["health"] > 0 {
		recommendations = append(recommendations, "检测到健康数据风险，建议按照《个人信息保护法》加强保护")
	}
	if len(report.TopViolatedRules) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("最常触发的规则是「%s」，建议重点关注该类数据", report.TopViolatedRules[0].RuleName))
	}

	recommendations = append(recommendations, "建议定期执行合规扫描，确保数据安全合规")
	recommendations = append(recommendations, "根据《个人信息保护法》第51条，应采取加密、去标识化等安全技术措施")

	return recommendations
}

// 辅助函数

// maskContent 对敏感内容进行脱敏.
func maskContent(content string) string {
	runes := []rune(content)
	length := len(runes)
	if length <= 4 {
		return strings.Repeat("*", length)
	}
	// 保留前2后2，中间用*替代
	prefix := string(runes[:2])
	suffix := string(runes[length-2:])
	masked := strings.Repeat("*", length-4)
	return prefix + masked + suffix
}

// isTextFile 判断是否为文本文件.
func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".txt": true, ".csv": true, ".json": true, ".xml": true,
		".log": true, ".md": true, ".html": true, ".htm": true,
		".yml": true, ".yaml": true, ".conf": true, ".cfg": true,
		".ini": true, ".toml": true, ".go": true, ".py": true,
		".js": true, ".ts": true, ".java": true, ".c": true,
		".cpp": true, ".h": true, ".sql": true, ".sh": true,
		".env": true, ".properties": true,
	}
	return textExts[ext]
}

// uniqueStrings 去重字符串切片.
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
