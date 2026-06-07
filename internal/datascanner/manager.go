// Package datascanner 提供隐私数据扫描核心业务逻辑
package datascanner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== PII 正则规则 ==========

// piiPattern PII 类型与正则表达式映射.
type piiPattern struct {
	piiType   PIIType
	pattern   *regexp.Regexp
	riskLevel RiskLevel
	riskScore float64
}

// 默认 PII 检测规则.
var defaultPatterns = []piiPattern{
	{PIIIDCard, regexp.MustCompile(`\b[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`), RiskHigh, 95},
	{PIIPhone, regexp.MustCompile(`\b1[3-9]\d{9}\b`), RiskMedium, 60},
	{PIIBankCard, regexp.MustCompile(`\b(?:6[0-9]{15,18}|4[0-9]{15}|5[1-5][0-9]{14}|3[47][0-9]{13})\b`), RiskHigh, 90},
	{PIIEmail, regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`), RiskMedium, 50},
	{PIIAddress, regexp.MustCompile(`[\x{4e00}-\x{9fa5}]{2,}(省|市|区|县|镇|乡|村|街|路|巷|号|弄|室|栋|单元|楼)`), RiskLow, 30},
	{PIIName, regexp.MustCompile(`(?:姓名|名字|联系人|负责人)[：:\s]*[\x{4e00}-\x{9fa5}]{2,4}`), RiskLow, 25},
	{PIICreditCode, regexp.MustCompile(`\b[0-9A-HJ-NPQRTUWXY]{2}\d{6}[0-9A-HJ-NPQRTUWXY]{10}\b`), RiskHigh, 85},
	{PIIPassport, regexp.MustCompile(`\b[EeGg]\d{8}\b`), RiskHigh, 80},
	{PIIMilitaryID, regexp.MustCompile(`[\x{4e00}-\x{9fa5}]字第\d{8}号`), RiskHigh, 85},
	{PIILicensePlate, regexp.MustCompile(`[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤川青藏琼宁][A-HJ-NP-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]`), RiskLow, 20},
}

// 合规标准与 PII 类型的映射.
var complianceMapping = map[PIIType][]ComplianceStandard{
	PIIIDCard:       {CompliancePIPL, ComplianceCSL, ComplianceGDPR},
	PIIPhone:        {CompliancePIPL, ComplianceCSL, ComplianceGDPR},
	PIIBankCard:     {CompliancePIPL, ComplianceCSL, ComplianceGDPR},
	PIIEmail:        {CompliancePIPL, ComplianceGDPR},
	PIIAddress:      {CompliancePIPL, ComplianceGDPR},
	PIIName:         {CompliancePIPL, ComplianceGDPR},
	PIICreditCode:   {CompliancePIPL, ComplianceCSL},
	PIIPassport:     {CompliancePIPL, ComplianceGDPR},
	PIIMilitaryID:   {CompliancePIPL, ComplianceCSL},
	PIILicensePlate: {CompliancePIPL},
}

// 脱敏策略映射.
var desensitizeStrategies = map[PIIType]DesensitizeStrategy{
	PIIIDCard:       {PIIIDCard, "掩码", "110101********1234", []ComplianceStandard{CompliancePIPL, ComplianceCSL}},
	PIIPhone:        {PIIPhone, "掩码", "138****5678", []ComplianceStandard{CompliancePIPL, ComplianceCSL}},
	PIIBankCard:     {PIIBankCard, "掩码", "6222 **** **** 1234", []ComplianceStandard{CompliancePIPL, ComplianceCSL}},
	PIIEmail:        {PIIEmail, "掩码", "t***@example.com", []ComplianceStandard{CompliancePIPL}},
	PIIAddress:      {PIIAddress, "截断", "北京市**区", []ComplianceStandard{CompliancePIPL}},
	PIIName:         {PIIName, "掩码", "张*明", []ComplianceStandard{CompliancePIPL}},
	PIICreditCode:   {PIICreditCode, "掩码", "91110000****1234", []ComplianceStandard{CompliancePIPL, ComplianceCSL}},
	PIIPassport:     {PIIPassport, "掩码", "E********", []ComplianceStandard{CompliancePIPL}},
	PIIMilitaryID:   {PIIMilitaryID, "掩码", "文字第********号", []ComplianceStandard{CompliancePIPL, ComplianceCSL}},
	PIILicensePlate: {PIILicensePlate, "掩码", "京A***89", []ComplianceStandard{CompliancePIPL}},
}

// Manager 隐私数据扫描管理器.
type Manager struct {
	tasks      map[string]*ScanTask
	results    map[string][]*ScanResult // taskID -> results
	reports    map[string]*ScanReport
	whitelists map[string]*WhitelistRule
	patterns   []piiPattern
	mu         sync.RWMutex
}

// NewManager 创建扫描管理器.
func NewManager() *Manager {
	return &Manager{
		tasks:      make(map[string]*ScanTask),
		results:    make(map[string][]*ScanResult),
		reports:    make(map[string]*ScanReport),
		whitelists: make(map[string]*WhitelistRule),
		patterns:   defaultPatterns,
	}
}

// ========== 扫描任务管理 ==========

// CreateTask 创建扫描任务.
func (m *Manager) CreateTask(req CreateTaskRequest) *ScanTask {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	task := &ScanTask{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Path:        req.Path,
		Recursive:   req.Recursive,
		FileTypes:   req.FileTypes,
		PIITypes:    req.PIITypes,
		Status:      TaskStatusPending,
		Progress:    0,
		WhitelistID: req.WhitelistID,
		CreatedAt:   now,
	}

	// 默认扫描所有文件类型
	if len(task.FileTypes) == 0 {
		task.FileTypes = []FileType{FileTypeText, FileTypeDocument, FileTypePDF, FileTypeImage}
	}
	// 默认检测所有 PII 类型
	if len(task.PIITypes) == 0 {
		task.PIITypes = []PIIType{
			PIIIDCard, PIIPhone, PIIBankCard, PIIEmail, PIIAddress, PIIName,
			PIICreditCode, PIIPassport, PIIMilitaryID, PIILicensePlate,
		}
	}

	m.tasks[task.ID] = task
	m.results[task.ID] = []*ScanResult{}
	return task
}

// GetTask 获取扫描任务.
func (m *Manager) GetTask(id string) (*ScanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有扫描任务.
func (m *Manager) ListTasks() []*ScanTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ScanTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks
}

// StartTask 启动扫描任务（模拟扫描流程）.
func (m *Manager) StartTask(id string) (*ScanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if task.Status == TaskStatusRunning {
		return nil, ErrTaskRunning
	}

	now := time.Now()
	task.Status = TaskStatusRunning
	task.StartedAt = &now
	task.Progress = 0
	task.ScannedFiles = 0
	task.FoundItems = 0
	return task, nil
}

// PauseTask 暂停扫描任务.
func (m *Manager) PauseTask(id string) (*ScanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if task.Status != TaskStatusRunning {
		return nil, ErrTaskNotRunning
	}

	task.Status = TaskStatusPaused
	return task, nil
}

// CancelTask 取消扫描任务.
func (m *Manager) CancelTask(id string) (*ScanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}

	now := time.Now()
	task.Status = TaskStatusCanceled
	task.CompletedAt = &now
	return task, nil
}

// DeleteTask 删除扫描任务及其结果.
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return ErrTaskNotFound
	}
	delete(m.tasks, id)
	delete(m.results, id)
	return nil
}

// ========== 扫描结果管理 ==========

// GetResults 获取指定任务的扫描结果.
func (m *Manager) GetResults(taskID string, riskLevel string, piiType string, limit, offset int) ([]*ScanResult, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results, ok := m.results[taskID]
	if !ok {
		return nil, 0, ErrTaskNotFound
	}

	// 过滤
	var filtered []*ScanResult
	for _, r := range results {
		if riskLevel != "" && string(r.RiskLevel) != riskLevel {
			continue
		}
		if piiType != "" && string(r.PIIType) != piiType {
			continue
		}
		filtered = append(filtered, r)
	}

	total := len(filtered)

	// 分页
	if offset >= total {
		return []*ScanResult{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total, nil
}

// GetResult 获取单条扫描结果.
func (m *Manager) GetResult(resultID string) (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, results := range m.results {
		for _, r := range results {
			if r.ID == resultID {
				return r, nil
			}
		}
	}
	return nil, ErrResultNotFound
}

// SubmitResult 提交扫描结果（供扫描器调用）.
func (m *Manager) SubmitResult(taskID string, result ScanResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[taskID]; !ok {
		return ErrTaskNotFound
	}

	result.ID = uuid.New().String()
	result.TaskID = taskID
	result.CreatedAt = time.Now()
	m.results[taskID] = append(m.results[taskID], &result)

	// 更新任务计数
	m.tasks[taskID].FoundItems++
	return nil
}

// CompleteTask 标记任务完成.
func (m *Manager) CompleteTask(taskID string, totalFiles int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	now := time.Now()
	task.Status = TaskStatusDone
	task.CompletedAt = &now
	task.TotalFiles = totalFiles
	task.ScannedFiles = totalFiles
	task.Progress = 1.0
	return nil
}

// UpdateTaskProgress 更新任务进度.
func (m *Manager) UpdateTaskProgress(taskID string, scanned, total int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	task.ScannedFiles = scanned
	task.TotalFiles = total
	if total > 0 {
		task.Progress = float64(scanned) / float64(total)
	}
	return nil
}

// ========== PII 检测 ==========

// ScanContent 扫描文本内容，返回检测到的 PII.
func (m *Manager) ScanContent(content string, filePath string, piiTypes []PIIType) []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 构建需要检测的 PII 类型集合，为空则扫描全部
	typeSet := make(map[PIIType]bool)
	for _, t := range piiTypes {
		typeSet[t] = true
	}
	scanAll := len(typeSet) == 0

	lines := strings.Split(content, "\n")
	var results []*ScanResult

	for lineNum, line := range lines {
		for _, p := range m.patterns {
			if !scanAll && !typeSet[p.piiType] {
				continue
			}

			matches := p.pattern.FindAllStringIndex(line, -1)
			for _, loc := range matches {
				matchedText := line[loc[0]:loc[1]]
				// 脱敏处理
				masked := maskText(matchedText, p.piiType)
				// 上下文
				ctxStart := max(0, loc[0]-20)
				ctxEnd := min(len(line), loc[1]+20)
				context := line[ctxStart:ctxEnd]

				results = append(results, &ScanResult{
					FilePath:    filePath,
					LineNumber:  lineNum + 1,
					ColumnStart: loc[0] + 1,
					ColumnEnd:   loc[1],
					PIIType:     p.piiType,
					MatchedText: masked,
					Context:     context,
					RiskLevel:   p.riskLevel,
					RiskScore:   p.riskScore,
					Compliance:  complianceMapping[p.piiType],
					Suggestion:  desensitizeStrategies[p.piiType].Strategy,
				})
			}
		}
	}

	return results
}

// GetDesensitizeStrategies 获取所有脱敏策略建议.
func (m *Manager) GetDesensitizeStrategies() []DesensitizeStrategy {
	strategies := make([]DesensitizeStrategy, 0, len(desensitizeStrategies))
	for _, s := range desensitizeStrategies {
		strategies = append(strategies, s)
	}
	return strategies
}

// ========== 白名单管理 ==========

// CreateWhitelist 创建白名单规则.
func (m *Manager) CreateWhitelist(req CreateWhitelistRequest) *WhitelistRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &WhitelistRule{
		ID:           uuid.New().String(),
		Name:         req.Name,
		ExcludeDirs:  req.ExcludeDirs,
		ExcludeExts:  req.ExcludeExts,
		ExcludeFiles: req.ExcludeFiles,
		MarkedFiles:  req.MarkedFiles,
		CreatedAt:    time.Now(),
	}

	m.whitelists[rule.ID] = rule
	return rule
}

// GetWhitelist 获取白名单规则.
func (m *Manager) GetWhitelist(id string) (*WhitelistRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.whitelists[id]
	if !ok {
		return nil, ErrWhitelistNotFound
	}
	return rule, nil
}

// ListWhitelists 列出所有白名单规则.
func (m *Manager) ListWhitelists() []*WhitelistRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*WhitelistRule, 0, len(m.whitelists))
	for _, r := range m.whitelists {
		rules = append(rules, r)
	}
	return rules
}

// UpdateWhitelist 更新白名单规则.
func (m *Manager) UpdateWhitelist(id string, req UpdateWhitelistRequest) (*WhitelistRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.whitelists[id]
	if !ok {
		return nil, ErrWhitelistNotFound
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.ExcludeDirs != nil {
		rule.ExcludeDirs = req.ExcludeDirs
	}
	if req.ExcludeExts != nil {
		rule.ExcludeExts = req.ExcludeExts
	}
	if req.ExcludeFiles != nil {
		rule.ExcludeFiles = req.ExcludeFiles
	}
	if req.MarkedFiles != nil {
		rule.MarkedFiles = req.MarkedFiles
	}

	return rule, nil
}

// DeleteWhitelist 删除白名单规则.
func (m *Manager) DeleteWhitelist(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.whitelists[id]; !ok {
		return ErrWhitelistNotFound
	}
	delete(m.whitelists, id)
	return nil
}

// IsFileExcluded 检查文件是否被白名单排除.
func (m *Manager) IsFileExcluded(whitelistID string, filePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.whitelists[whitelistID]
	if !ok {
		return false
	}

	// 检查排除目录
	for _, dir := range rule.ExcludeDirs {
		if strings.HasPrefix(filePath, dir) {
			return true
		}
	}

	// 检查排除文件
	for _, f := range rule.ExcludeFiles {
		if filePath == f {
			return true
		}
	}

	// 检查已标记文件
	for _, f := range rule.MarkedFiles {
		if filePath == f {
			return true
		}
	}

	// 检查排除扩展名
	for _, ext := range rule.ExcludeExts {
		if strings.HasSuffix(strings.ToLower(filePath), strings.ToLower(ext)) {
			return true
		}
	}

	return false
}

// ========== 报告生成 ==========

// GenerateReport 生成扫描报告.
func (m *Manager) GenerateReport(taskID string, format ReportFormat) (*ScanReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	results := m.results[taskID]

	// 统计风险分布
	riskDist := RiskDistribution{}
	piiDist := make(map[PIIType]int)
	fileRisk := make(map[string]*FileRiskStat)

	for _, r := range results {
		switch r.RiskLevel {
		case RiskHigh:
			riskDist.High++
		case RiskMedium:
			riskDist.Medium++
		case RiskLow:
			riskDist.Low++
		}
		piiDist[r.PIIType]++

		// 按文件聚合
		if fr, ok := fileRisk[r.FilePath]; ok {
			fr.Findings++
			fr.RiskScore += r.RiskScore
		} else {
			fileRisk[r.FilePath] = &FileRiskStat{
				FilePath:  r.FilePath,
				Findings:  1,
				RiskScore: r.RiskScore,
			}
		}
	}

	// 计算文件风险等级
	topRiskFiles := make([]FileRiskStat, 0, len(fileRisk))
	for _, fr := range fileRisk {
		if fr.Findings > 0 {
			fr.RiskScore = fr.RiskScore / float64(fr.Findings)
		}
		fr.RiskLevel = scoreToLevel(fr.RiskScore)
		topRiskFiles = append(topRiskFiles, *fr)
	}

	// 排序 Top 风险文件
	sort.Slice(topRiskFiles, func(i, j int) bool {
		return topRiskFiles[i].RiskScore > topRiskFiles[j].RiskScore
	})
	if len(topRiskFiles) > 10 {
		topRiskFiles = topRiskFiles[:10]
	}

	report := &ScanReport{
		ID:     uuid.New().String(),
		TaskID: taskID,
		Format: format,
		Summary: ReportSummary{
			TotalFiles:    task.TotalFiles,
			ScannedFiles:  task.ScannedFiles,
			TotalFindings: len(results),
			RiskDist:      riskDist,
			PIIDist:       piiDist,
		},
		TopRiskFiles: topRiskFiles,
		GeneratedAt:  time.Now(),
	}

	m.reports[report.ID] = report
	return report, nil
}

// GetReport 获取扫描报告.
func (m *Manager) GetReport(id string) (*ScanReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report %q not found", id)
	}
	return report, nil
}

// ListReports 列出指定任务的所有报告.
func (m *Manager) ListReports(taskID string) []*ScanReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var reports []*ScanReport
	for _, r := range m.reports {
		if r.TaskID == taskID {
			reports = append(reports, r)
		}
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})
	return reports
}

// ========== 统计 ==========

// GetStats 获取指定任务的扫描统计.
func (m *Manager) GetStats(taskID string) (*ReportSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	results := m.results[taskID]
	riskDist := RiskDistribution{}
	piiDist := make(map[PIIType]int)

	for _, r := range results {
		switch r.RiskLevel {
		case RiskHigh:
			riskDist.High++
		case RiskMedium:
			riskDist.Medium++
		case RiskLow:
			riskDist.Low++
		}
		piiDist[r.PIIType]++
	}

	return &ReportSummary{
		TotalFiles:    task.TotalFiles,
		ScannedFiles:  task.ScannedFiles,
		TotalFindings: len(results),
		RiskDist:      riskDist,
		PIIDist:       piiDist,
	}, nil
}

// ========== 辅助函数 ==========

// maskText 对匹配文本进行脱敏处理.
func maskText(text string, piiType PIIType) string {
	runes := []rune(text)
	length := len(runes)

	switch piiType {
	case PIIIDCard, PIIPassport:
		// 保留前3后4
		if length > 7 {
			return string(runes[:3]) + strings.Repeat("*", length-7) + string(runes[length-4:])
		}
	case PIIPhone:
		// 保留前3后4
		if length == 11 {
			return string(runes[:3]) + "****" + string(runes[7:])
		}
	case PIIBankCard:
		// 保留后4
		if length > 4 {
			return strings.Repeat("*", length-4) + string(runes[length-4:])
		}
	case PIIEmail:
		// 保留首字符和域名
		parts := strings.Split(text, "@")
		if len(parts) == 2 {
			return string([]rune(parts[0])[0]) + "***@" + parts[1]
		}
	case PIIName:
		// 保留首尾
		if length >= 2 {
			return string(runes[0]) + strings.Repeat("*", length-2) + string(runes[length-1])
		}
	}

	// 默认：保留首尾，中间替换
	if length > 2 {
		return string(runes[0]) + strings.Repeat("*", length-2) + string(runes[length-1])
	}
	return strings.Repeat("*", length)
}

// scoreToLevel 根据风险评分判定风险等级.
func scoreToLevel(score float64) RiskLevel {
	if score >= 70 {
		return RiskHigh
	}
	if score >= 40 {
		return RiskMedium
	}
	return RiskLow
}
