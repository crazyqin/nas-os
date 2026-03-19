package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// FilesystemScanner 文件系统安全扫描器
type FilesystemScanner struct {
	config           Config
	tasks            map[string]*ScanTask
	findings         map[string][]*FileFinding
	sensitiveRules   []SensitiveDataRule
	permissionRules  []PermissionRule
	mu               sync.RWMutex
	stopChan         chan struct{}
	progressCallback func(taskID string, progress int)
}

// NewFilesystemScanner 创建文件系统扫描器
func NewFilesystemScanner(config Config) *FilesystemScanner {
	return &FilesystemScanner{
		config:          config,
		tasks:           make(map[string]*ScanTask),
		findings:        make(map[string][]*FileFinding),
		sensitiveRules:  config.SensitiveDataRules,
		permissionRules: config.PermissionRules,
		stopChan:        make(chan struct{}),
	}
}

// SetProgressCallback 设置进度回调
func (s *FilesystemScanner) SetProgressCallback(callback func(taskID string, progress int)) {
	s.progressCallback = callback
}

// ========== 扫描任务管理 ==========

// CreateScanTask 创建扫描任务
func (s *FilesystemScanner) CreateScanTask(name string, scanType ScanType, targetPaths, excludePaths []string, options *ScanOptions) *ScanTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	if options == nil {
		opts := s.config.DefaultOptions
		options = &opts
	}

	task := &ScanTask{
		ID:                 generateScanID(),
		Name:               name,
		Type:               scanType,
		Status:             ScanStatusPending,
		TargetPaths:        targetPaths,
		ExcludePaths:       excludePaths,
		Options:            *options,
		CreatedAt:          time.Now(),
		Progress:           0,
		FilesScanned:       0,
		FilesTotal:         0,
		FindingsCount:      0,
		FindingsBySeverity: make(map[string]int),
	}

	s.tasks[task.ID] = task
	s.findings[task.ID] = make([]*FileFinding, 0)

	return task
}

// StartScan 启动扫描
func (s *FilesystemScanner) StartScan(taskID string) error {
	s.mu.Lock()
	task, exists := s.tasks[taskID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == ScanStatusRunning {
		s.mu.Unlock()
		return fmt.Errorf("scan already running")
	}

	task.Status = ScanStatusRunning
	now := time.Now()
	task.StartedAt = &now
	s.mu.Unlock()

	// 异步执行扫描
	go s.executeScan(task)

	return nil
}

// CancelScan 取消扫描
func (s *FilesystemScanner) CancelScan(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == ScanStatusRunning {
		task.Status = ScanStatusCancelled
		now := time.Now()
		task.CompletedAt = &now
	}

	return nil
}

// GetScanTask 获取扫描任务
func (s *FilesystemScanner) GetScanTask(taskID string) *ScanTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[taskID]
}

// ListScanTasks 列出扫描任务
func (s *FilesystemScanner) ListScanTasks(limit int) []*ScanTask {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ScanTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	// 按创建时间倒序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	return tasks
}

// ========== 扫描执行 ==========

// executeScan 执行扫描
func (s *FilesystemScanner) executeScan(task *ScanTask) {
	defer func() {
		s.mu.Lock()
		if task.Status == ScanStatusRunning {
			task.Status = ScanStatusCompleted
			now := time.Now()
			task.CompletedAt = &now
		}
		s.mu.Unlock()
	}()

	// 统计文件总数
	totalFiles := 0
	for _, path := range task.TargetPaths {
		count, _ := s.countFiles(path, task.Options)
		totalFiles += count
	}

	s.mu.Lock()
	task.FilesTotal = totalFiles
	s.mu.Unlock()

	// 扫描每个目标路径
	for _, targetPath := range task.TargetPaths {
		if task.Status == ScanStatusCancelled {
			return
		}

		s.scanPath(task, targetPath)
	}

	// 生成报告
	if task.Options.GenerateReport {
		s.generateReport(task)
	}
}

// scanPath 扫描路径
func (s *FilesystemScanner) scanPath(task *ScanTask, rootPath string) {
	excludeMap := make(map[string]bool)
	for _, p := range task.ExcludePaths {
		excludeMap[p] = true
	}

	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续扫描
		}

		// 检查是否取消
		if task.Status == ScanStatusCancelled {
			return filepath.SkipAll
		}

		// 检查排除路径
		for exclude := range excludeMap {
			if strings.HasPrefix(path, exclude) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 跳过目录
		if info.IsDir() {
			// 检查目录权限
			if task.Options.CheckPermissions {
				s.checkPathPermissions(task, path, info)
			}
			return nil
		}

		// 检查文件大小限制
		if info.Size() > task.Options.MaxFileSize {
			return nil
		}

		// 检查文件扩展名
		if len(task.Options.FileExtensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, allowed := range task.Options.FileExtensions {
				if strings.ToLower(allowed) == ext {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// 检查排除的扩展名
		if len(task.Options.ExcludeExtensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			for _, excluded := range task.Options.ExcludeExtensions {
				if strings.ToLower(excluded) == ext {
					return nil
				}
			}
		}

		// 扫描文件
		s.scanFile(task, path, info)

		// 更新进度
		s.mu.Lock()
		task.FilesScanned++
		task.Progress = int(float64(task.FilesScanned) / float64(task.FilesTotal) * 100)
		if task.Progress > 100 {
			task.Progress = 100
		}
		s.mu.Unlock()

		if s.progressCallback != nil {
			s.progressCallback(task.ID, task.Progress)
		}

		return nil
	})
}

// scanFile 扫描单个文件
func (s *FilesystemScanner) scanFile(task *ScanTask, path string, info os.FileInfo) {
	// 计算文件哈希
	var fileHash string
	if len(task.Options.HashAlgorithms) > 0 {
		fileHash, _ = s.calculateFileHash(path, task.Options.HashAlgorithms[0])
	}

	// 检查权限问题
	if task.Options.CheckPermissions {
		s.checkFilePermissions(task, path, info)
	}

	// 检查敏感数据
	if task.Options.CheckSensitiveData {
		s.checkSensitiveData(task, path, info, fileHash)
	}

	// 检查文件完整性
	if task.Options.CheckIntegrity {
		s.checkFileIntegrity(task, path, info, fileHash)
	}
}

// ========== 权限检查 ==========

// checkPathPermissions 检查路径权限
func (s *FilesystemScanner) checkPathPermissions(task *ScanTask, path string, info os.FileInfo) {
	mode := info.Mode()

	for _, rule := range s.permissionRules {
		if !rule.Enabled {
			continue
		}

		// 检查路径匹配
		matched, err := filepath.Match(rule.PathPattern, path)
		if err != nil || !matched {
			// 尝试匹配文件名
			matched, err = filepath.Match(rule.PathPattern, info.Name())
			if err != nil || !matched {
				continue
			}
		}

		// 检查权限
		currentMode := fmt.Sprintf("%04o", mode.Perm())
		var issue *PermissionIssue

		if rule.RequiredMode != "" && currentMode != rule.RequiredMode {
			issue = &PermissionIssue{
				Path:            path,
				Type:            "directory",
				CurrentMode:     currentMode,
				RecommendedMode: rule.RequiredMode,
				Issue:           fmt.Sprintf("目录权限应为 %s，当前为 %s", rule.RequiredMode, currentMode),
				Severity:        rule.Severity,
				Risk:            "权限设置不当可能导致安全风险",
			}
		}

		if rule.MaxMode != "" {
			currentPerm := mode.Perm()
			maxPerm := parseFileMode(rule.MaxMode)
			if maxPerm > 0 && currentPerm > maxPerm {
				issue = &PermissionIssue{
					Path:            path,
					Type:            "directory",
					CurrentMode:     currentMode,
					RecommendedMode: rule.MaxMode,
					Issue:           fmt.Sprintf("目录权限过于宽松，应不超过 %s", rule.MaxMode),
					Severity:        rule.Severity,
					Risk:            "权限过于宽松可能导致未授权访问",
				}
			}
		}

		if issue != nil {
			finding := &FileFinding{
				ID:          generateFindingID(),
				Timestamp:   time.Now(),
				Type:        FindingTypePermission,
				Severity:    issue.Severity,
				FilePath:    path,
				FileName:    info.Name(),
				Description: issue.Issue,
				Remediation: fmt.Sprintf("运行: chmod %s %s", issue.RecommendedMode, path),
				RiskScore:   severityToRiskScore(issue.Severity),
				Details: map[string]interface{}{
					"current_mode":     issue.CurrentMode,
					"recommended_mode": issue.RecommendedMode,
				},
			}
			s.addFinding(task, finding)
		}
	}
}

// checkFilePermissions 检查文件权限
func (s *FilesystemScanner) checkFilePermissions(task *ScanTask, path string, info os.FileInfo) {
	mode := info.Mode()

	for _, rule := range s.permissionRules {
		if !rule.Enabled {
			continue
		}

		// 检查路径匹配
		matched, err := filepath.Match(rule.PathPattern, path)
		if err != nil || !matched {
			// 尝试匹配文件名
			matched, err = filepath.Match(rule.PathPattern, info.Name())
			if err != nil || !matched {
				continue
			}
		}

		currentMode := fmt.Sprintf("%04o", mode.Perm())

		// 检查必需权限
		if rule.RequiredMode != "" && currentMode != rule.RequiredMode {
			finding := &FileFinding{
				ID:          generateFindingID(),
				Timestamp:   time.Now(),
				Type:        FindingTypePermission,
				Severity:    rule.Severity,
				FilePath:    path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				FileModTime: info.ModTime(),
				Description: fmt.Sprintf("文件权限应为 %s，当前为 %s", rule.RequiredMode, currentMode),
				Remediation: fmt.Sprintf("运行: chmod %s %s", rule.RequiredMode, path),
				RiskScore:   severityToRiskScore(rule.Severity),
				Details: map[string]interface{}{
					"current_mode":     currentMode,
					"recommended_mode": rule.RequiredMode,
					"rule_name":        rule.Name,
				},
			}
			s.addFinding(task, finding)
		}

		// 检查最大权限
		if rule.MaxMode != "" {
			currentPerm := mode.Perm()
			maxPerm := parseFileMode(rule.MaxMode)
			if maxPerm > 0 && currentPerm > maxPerm {
				finding := &FileFinding{
					ID:          generateFindingID(),
					Timestamp:   time.Now(),
					Type:        FindingTypePermission,
					Severity:    rule.Severity,
					FilePath:    path,
					FileName:    info.Name(),
					FileSize:    info.Size(),
					FileModTime: info.ModTime(),
					Description: fmt.Sprintf("文件权限过于宽松，当前为 %s，应不超过 %s", currentMode, rule.MaxMode),
					Remediation: fmt.Sprintf("运行: chmod %s %s", rule.MaxMode, path),
					RiskScore:   severityToRiskScore(rule.Severity),
					Details: map[string]interface{}{
						"current_mode": currentMode,
						"max_mode":     rule.MaxMode,
						"rule_name":    rule.Name,
					},
				}
				s.addFinding(task, finding)
			}
		}
	}

	// 检查全局可写
	if mode&0002 != 0 {
		finding := &FileFinding{
			ID:          generateFindingID(),
			Timestamp:   time.Now(),
			Type:        FindingTypePermission,
			Severity:    SeverityMedium,
			FilePath:    path,
			FileName:    info.Name(),
			FileSize:    info.Size(),
			FileModTime: info.ModTime(),
			Description: "文件全局可写，存在安全风险",
			Remediation: fmt.Sprintf("运行: chmod o-w %s", path),
			RiskScore:   severityToRiskScore(SeverityMedium),
		}
		s.addFinding(task, finding)
	}
}

// ========== 敏感数据检测 ==========

// checkSensitiveData 检查敏感数据
func (s *FilesystemScanner) checkSensitiveData(task *ScanTask, path string, info os.FileInfo, fileHash string) {
	// 跳过二进制文件
	if isBinaryFile(path) {
		return
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	// 限制检查的文件大小
	if len(content) > 10*1024*1024 { // 10MB
		return
	}

	contentStr := string(content)

	for _, rule := range s.sensitiveRules {
		if !rule.Enabled {
			continue
		}

		// 检查文件类型
		if len(rule.FileTypes) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			matched := false
			for _, ft := range rule.FileTypes {
				if strings.ToLower(ft) == ext {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 编译正则表达式
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		// 查找匹配
		matches := re.FindAllStringIndex(contentStr, -1)
		for _, match := range matches {
			start, end := match[0], match[1]

			// 获取上下文
			contextStart := start - 50
			if contextStart < 0 {
				contextStart = 0
			}
			contextEnd := end + 50
			if contextEnd > len(contentStr) {
				contextEnd = len(contentStr)
			}

			// 计算行号
			lineNum := strings.Count(contentStr[:start], "\n") + 1

			// 脱敏匹配内容
			matchStr := contentStr[start:end]
			maskedMatch := maskSensitiveData(matchStr)

			finding := &FileFinding{
				ID:          generateFindingID(),
				Timestamp:   time.Now(),
				Type:        FindingTypeSensitiveData,
				Severity:    rule.Severity,
				FilePath:    path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				FileHash:    fileHash,
				FileModTime: info.ModTime(),
				Description: fmt.Sprintf("检测到敏感数据: %s", rule.Name),
				Remediation: "移除或加密敏感数据，或添加到.gitignore等忽略列表",
				RiskScore:   severityToRiskScore(rule.Severity),
				Details: map[string]interface{}{
					"sensitive_type": rule.Type,
					"rule_name":      rule.Name,
					"line_number":    lineNum,
					"masked_match":   maskedMatch,
					"context":        contentStr[contextStart:contextEnd],
				},
			}
			s.addFinding(task, finding)
		}
	}
}

// ========== 完整性检查 ==========

// checkFileIntegrity 检查文件完整性
func (s *FilesystemScanner) checkFileIntegrity(task *ScanTask, path string, info os.FileInfo, fileHash string) {
	// 检查可疑文件名
	suspiciousNames := []string{
		".htaccess", ".htpasswd", ".env", ".gitignore",
		"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
		".npmrc", ".pypirc", ".pgpass",
	}

	for _, name := range suspiciousNames {
		if info.Name() == name || strings.HasSuffix(info.Name(), name) {
			finding := &FileFinding{
				ID:          generateFindingID(),
				Timestamp:   time.Now(),
				Type:        FindingTypeSuspicious,
				Severity:    SeverityMedium,
				FilePath:    path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				FileHash:    fileHash,
				FileModTime: info.ModTime(),
				Description: fmt.Sprintf("发现潜在敏感文件: %s", info.Name()),
				Remediation: "检查文件内容，确保不包含敏感信息",
				RiskScore:   severityToRiskScore(SeverityMedium),
			}
			s.addFinding(task, finding)
			break
		}
	}

	// 检查可执行文件
	if info.Mode()&0111 != 0 {
		// 检查是否在预期路径
		expectedPaths := []string{"/bin", "/usr/bin", "/usr/local/bin", "/sbin", "/usr/sbin"}
		inExpectedPath := false
		for _, ep := range expectedPaths {
			if strings.HasPrefix(path, ep) {
				inExpectedPath = true
				break
			}
		}

		if !inExpectedPath {
			finding := &FileFinding{
				ID:          generateFindingID(),
				Timestamp:   time.Now(),
				Type:        FindingTypeSuspicious,
				Severity:    SeverityLow,
				FilePath:    path,
				FileName:    info.Name(),
				FileSize:    info.Size(),
				FileHash:    fileHash,
				FileModTime: info.ModTime(),
				Description: "在非标准位置发现可执行文件",
				Remediation: "验证此文件是否为合法的可执行文件",
				RiskScore:   severityToRiskScore(SeverityLow),
			}
			s.addFinding(task, finding)
		}
	}
}

// ========== 辅助方法 ==========

// addFinding 添加发现
func (s *FilesystemScanner) addFinding(task *ScanTask, finding *FileFinding) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.findings[task.ID] = append(s.findings[task.ID], finding)
	task.FindingsCount++
	task.FindingsBySeverity[string(finding.Severity)]++
}

// countFiles 统计文件数量
func (s *FilesystemScanner) countFiles(rootPath string, options ScanOptions) (int, error) {
	count := 0

	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if info.Size() <= options.MaxFileSize {
			count++
		}
		return nil
	})

	return count, nil
}

// calculateFileHash 计算文件哈希（使用 SHA256）
func (s *FilesystemScanner) calculateFileHash(path string, algorithm string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	// 统一使用 SHA256，忽略算法参数以确保安全性
	hash := sha256.New()
	_, _ = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateReport 生成报告
func (s *FilesystemScanner) generateReport(task *ScanTask) *FileScanReport {
	s.mu.RLock()
	findings := s.findings[task.ID]
	s.mu.RUnlock()

	report := &FileScanReport{
		ReportID:        fmt.Sprintf("RPT-%s", task.ID),
		TaskID:          task.ID,
		GeneratedAt:     time.Now(),
		Findings:        findings,
		Recommendations: make([]string, 0),
	}

	// 计算摘要
	report.Summary = FileScanSummary{
		TotalFiles:     task.FilesTotal,
		ScannedFiles:   task.FilesScanned,
		TotalFindings:  len(findings),
		FindingsByType: make(map[string]int),
	}

	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			report.Summary.CriticalCount++
		case SeverityHigh:
			report.Summary.HighCount++
		case SeverityMedium:
			report.Summary.MediumCount++
		case SeverityLow:
			report.Summary.LowCount++
		case SeverityInfo:
			report.Summary.InfoCount++
		}
		report.Summary.FindingsByType[string(f.Type)]++
	}

	// 计算扫描时长
	if task.StartedAt != nil && task.CompletedAt != nil {
		report.ScanDuration = int64(task.CompletedAt.Sub(*task.StartedAt).Seconds())
	}

	// 计算风险分数和等级
	report.RiskScore = s.calculateRiskScore(&report.Summary)
	report.RiskLevel = scoreToRiskLevel(report.RiskScore)

	// 生成建议
	report.Recommendations = s.generateRecommendations(&report.Summary, findings)

	return report
}

// calculateRiskScore 计算风险分数
func (s *FilesystemScanner) calculateRiskScore(summary *FileScanSummary) int {
	score := 100

	// 根据发现的问题扣分
	score -= summary.CriticalCount * 25
	score -= summary.HighCount * 15
	score -= summary.MediumCount * 5
	score -= summary.LowCount * 2

	if score < 0 {
		score = 0
	}

	return score
}

// generateRecommendations 生成建议
func (s *FilesystemScanner) generateRecommendations(summary *FileScanSummary, findings []*FileFinding) []string {
	recommendations := make([]string, 0)

	if summary.CriticalCount > 0 {
		recommendations = append(recommendations, "立即处理严重级别的安全问题")
	}
	if summary.HighCount > 0 {
		recommendations = append(recommendations, "尽快处理高危级别的安全问题")
	}
	if summary.FindingsByType[string(FindingTypePermission)] > 0 {
		recommendations = append(recommendations, "审查并修复文件权限问题")
	}
	if summary.FindingsByType[string(FindingTypeSensitiveData)] > 0 {
		recommendations = append(recommendations, "移除或加密文件中的敏感数据")
	}
	if summary.FindingsByType[string(FindingTypeSuspicious)] > 0 {
		recommendations = append(recommendations, "检查可疑文件，确认其合法性")
	}

	return recommendations
}

// GetFindings 获取扫描发现
func (s *FilesystemScanner) GetFindings(taskID string) []*FileFinding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findings[taskID]
}

// GetReport 获取扫描报告
func (s *FilesystemScanner) GetReport(taskID string) *FileScanReport {
	task := s.GetScanTask(taskID)
	if task == nil {
		return nil
	}
	return s.generateReport(task)
}

// Stop 停止扫描器
func (s *FilesystemScanner) Stop() {
	close(s.stopChan)
}

// ========== 辅助函数 ==========

func generateScanID() string {
	return fmt.Sprintf("SCAN-%d", time.Now().UnixNano())
}

func generateFindingID() string {
	return fmt.Sprintf("FIND-%d", time.Now().UnixNano())
}

func parseFileMode(mode string) os.FileMode {
	var m uint32
	_, _ = fmt.Sscanf(mode, "%o", &m)
	return os.FileMode(m)
}

func severityToRiskScore(severity Severity) int {
	switch severity {
	case SeverityCritical:
		return 90
	case SeverityHigh:
		return 70
	case SeverityMedium:
		return 50
	case SeverityLow:
		return 30
	default:
		return 20
	}
}

func scoreToRiskLevel(score int) string {
	if score >= 80 {
		return "low"
	} else if score >= 60 {
		return "medium"
	} else if score >= 40 {
		return "high"
	}
	return "critical"
}

func isBinaryFile(path string) bool {
	// 简单的二进制文件检测
	binaryExts := []string{".exe", ".dll", ".so", ".dylib", ".bin", ".zip", ".tar", ".gz", ".rar", ".7z", ".jpg", ".jpeg", ".png", ".gif", ".mp3", ".mp4", ".avi", ".mov", ".pdf"}
	ext := strings.ToLower(filepath.Ext(path))
	for _, be := range binaryExts {
		if ext == be {
			return true
		}
	}
	return false
}

func maskSensitiveData(data string) string {
	if len(data) <= 4 {
		return "****"
	}
	return data[:2] + strings.Repeat("*", len(data)-4) + data[len(data)-2:]
}
