package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// PermissionChecker 权限检查器
type PermissionChecker struct {
	rules  []PermissionRule
	config CheckerConfig
	mu     sync.RWMutex
}

// CheckerConfig 检查器配置
type CheckerConfig struct {
	CheckWorldWritable bool     `json:"check_world_writable"`
	CheckWorldReadable bool     `json:"check_world_readable"`
	CheckSetuid        bool     `json:"check_setuid"`
	CheckSetgid        bool     `json:"check_setgid"`
	CheckOwner         bool     `json:"check_owner"`
	CheckGroup         bool     `json:"check_group"`
	SensitivePaths     []string `json:"sensitive_paths"`
	MaxRecursionDepth  int      `json:"max_recursion_depth"`
	SkipPaths          []string `json:"skip_paths"`
}

// DefaultCheckerConfig 默认检查器配置
func DefaultCheckerConfig() CheckerConfig {
	return CheckerConfig{
		CheckWorldWritable: true,
		CheckWorldReadable: false,
		CheckSetuid:        true,
		CheckSetgid:        true,
		CheckOwner:         true,
		CheckGroup:         true,
		SensitivePaths: []string{
			"/etc/shadow",
			"/etc/passwd",
			"/etc/ssh",
			"/root/.ssh",
			"/var/log",
		},
		MaxRecursionDepth: 20,
		SkipPaths: []string{
			"/proc",
			"/sys",
			"/dev",
		},
	}
}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker(config CheckerConfig) *PermissionChecker {
	return &PermissionChecker{
		rules:  DefaultPermissionRules(),
		config: config,
	}
}

// AddRule 添加权限规则
func (pc *PermissionChecker) AddRule(rule PermissionRule) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.rules = append(pc.rules, rule)
}

// RemoveRule 移除权限规则
func (pc *PermissionChecker) RemoveRule(ruleID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	for i, rule := range pc.rules {
		if rule.ID == ruleID {
			pc.rules = append(pc.rules[:i], pc.rules[i+1:]...)
			break
		}
	}
}

// ListRules 列出权限规则
func (pc *PermissionChecker) ListRules() []PermissionRule {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	rules := make([]PermissionRule, len(pc.rules))
	copy(rules, pc.rules)
	return rules
}

// ========== 权限检查 ==========

// CheckPath 检查单个路径权限
func (pc *PermissionChecker) CheckPath(path string) *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	info, err := os.Lstat(path)
	if err != nil {
		return result
	}

	pc.checkSinglePath(path, info, result)
	return result
}

// CheckPaths 批量检查路径权限
func (pc *PermissionChecker) CheckPaths(paths []string) *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	for _, path := range paths {
		filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			// 检查跳过路径
			for _, skip := range pc.config.SkipPaths {
				if strings.HasPrefix(p, skip) {
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			pc.checkSinglePath(p, info, result)
			return nil
		})
	}

	// 生成建议
	result.Suggestions = pc.generateSuggestions(result)

	return result
}

// checkSinglePath 检查单个路径
func (pc *PermissionChecker) checkSinglePath(path string, info fs.FileInfo, result *PermissionCheckResult) {
	result.TotalChecked++

	mode := info.Mode()
	isDir := info.IsDir()
	typeStr := "file"
	if isDir {
		typeStr = "directory"
	}

	// 检查规则
	pc.mu.RLock()
	rules := pc.rules
	pc.mu.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 匹配路径
		matched, err := filepath.Match(rule.PathPattern, path)
		if err != nil || !matched {
			matched, _ = filepath.Match(rule.PathPattern, info.Name())
		}

		if !matched {
			continue
		}

		// 检查权限
		currentMode := fmt.Sprintf("%04o", mode.Perm())

		if rule.RequiredMode != "" && currentMode != rule.RequiredMode {
			issue := &PermissionIssue{
				Path:            path,
				Type:            typeStr,
				CurrentMode:     currentMode,
				RecommendedMode: rule.RequiredMode,
				Issue:           fmt.Sprintf("权限不符合要求，应为 %s", rule.RequiredMode),
				Severity:        rule.Severity,
				Risk:            pc.getRiskDescription(rule.Severity),
			}
			result.Issues = append(result.Issues, issue)
			result.IssuesFound++

			if rule.Severity == SeverityCritical {
				result.CriticalIssues++
			} else if rule.Severity == SeverityHigh || rule.Severity == SeverityMedium {
				result.WarningIssues++
			}
		}

		// 检查最大权限
		if rule.MaxMode != "" {
			currentPerm := mode.Perm()
			maxPerm := parseFileMode(rule.MaxMode)
			if maxPerm > 0 && currentPerm > maxPerm {
				issue := &PermissionIssue{
					Path:            path,
					Type:            typeStr,
					CurrentMode:     currentMode,
					RecommendedMode: rule.MaxMode,
					Issue:           fmt.Sprintf("权限过于宽松，最大应为 %s", rule.MaxMode),
					Severity:        rule.Severity,
					Risk:            "权限过于宽松可能导致未授权访问",
				}
				result.Issues = append(result.Issues, issue)
				result.IssuesFound++
			}
		}
	}

	// 检查全局可写
	if pc.config.CheckWorldWritable && mode&0002 != 0 {
		issue := &PermissionIssue{
			Path:            path,
			Type:            typeStr,
			CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
			RecommendedMode: "remove world-writable",
			Issue:           "全局可写",
			Severity:        SeverityMedium,
			Risk:            "全局可写文件可能被恶意修改",
		}
		result.Issues = append(result.Issues, issue)
		result.IssuesFound++
		result.WarningIssues++
	}

	// 检查SETUID/SETGID
	if pc.config.CheckSetuid && mode&os.ModeSetuid != 0 {
		issue := &PermissionIssue{
			Path:        path,
			Type:        typeStr,
			CurrentMode: fmt.Sprintf("%04o", mode.Perm()),
			Issue:       "SETUID位设置",
			Severity:    SeverityHigh,
			Risk:        "SETUID程序可能被用于权限提升攻击",
		}
		result.Issues = append(result.Issues, issue)
		result.IssuesFound++
		result.WarningIssues++
	}

	if pc.config.CheckSetgid && mode&os.ModeSetgid != 0 {
		issue := &PermissionIssue{
			Path:        path,
			Type:        typeStr,
			CurrentMode: fmt.Sprintf("%04o", mode.Perm()),
			Issue:       "SETGID位设置",
			Severity:    SeverityMedium,
			Risk:        "SETGID程序可能存在安全风险",
		}
		result.Issues = append(result.Issues, issue)
		result.IssuesFound++
	}
}

// ========== 敏感路径检查 ==========

// CheckSensitivePaths 检查敏感路径权限
func (pc *PermissionChecker) CheckSensitivePaths() *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	for _, path := range pc.config.SensitivePaths {
		filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			pc.checkSinglePath(p, info, result)
			return nil
		})
	}

	result.Suggestions = pc.generateSuggestions(result)
	return result
}

// ========== SSH安全检查 ==========

// CheckSSHSecurity 检查SSH相关安全
func (pc *PermissionChecker) CheckSSHSecurity() *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	// 检查SSH配置目录
	sshPaths := []string{
		"/etc/ssh",
		"/root/.ssh",
	}

	for _, sshPath := range sshPaths {
		info, err := os.Stat(sshPath)
		if err != nil {
			continue
		}

		result.TotalChecked++
		mode := info.Mode()

		// SSH目录应为700
		if mode.Perm() != 0700 {
			issue := &PermissionIssue{
				Path:            sshPath,
				Type:            "directory",
				CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
				RecommendedMode: "0700",
				Issue:           "SSH目录权限不正确",
				Severity:        SeverityHigh,
				Risk:            "SSH目录权限不正确可能导致私钥泄露",
			}
			result.Issues = append(result.Issues, issue)
			result.IssuesFound++
			result.WarningIssues++
		}

		// 检查SSH密钥文件
		filepath.WalkDir(sshPath, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			result.TotalChecked++
			name := d.Name()
			mode := info.Mode()

			// 私钥文件权限应为600
			if strings.Contains(name, "id_") && !strings.HasSuffix(name, ".pub") {
				if mode.Perm() != 0600 {
					issue := &PermissionIssue{
						Path:            p,
						Type:            "file",
						CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
						RecommendedMode: "0600",
						Issue:           "SSH私钥权限不正确",
						Severity:        SeverityCritical,
						Risk:            "私钥权限过于宽松可能导致密钥泄露",
					}
					result.Issues = append(result.Issues, issue)
					result.IssuesFound++
					result.CriticalIssues++
				}
			}

			// 公钥文件权限应为644
			if strings.HasSuffix(name, ".pub") {
				if mode.Perm() != 0644 {
					issue := &PermissionIssue{
						Path:            p,
						Type:            "file",
						CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
						RecommendedMode: "0644",
						Issue:           "SSH公钥权限不正确",
						Severity:        SeverityLow,
						Risk:            "公钥权限设置不当",
					}
					result.Issues = append(result.Issues, issue)
					result.IssuesFound++
				}
			}

			// authorized_keys权限应为600
			if name == "authorized_keys" || name == "authorized_keys2" {
				if mode.Perm() != 0600 {
					issue := &PermissionIssue{
						Path:            p,
						Type:            "file",
						CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
						RecommendedMode: "0600",
						Issue:           "authorized_keys权限不正确",
						Severity:        SeverityHigh,
						Risk:            "authorized_keys权限不当可能导致未授权访问",
					}
					result.Issues = append(result.Issues, issue)
					result.IssuesFound++
					result.WarningIssues++
				}
			}

			return nil
		})
	}

	result.Suggestions = pc.generateSuggestions(result)
	return result
}

// ========== 用户目录检查 ==========

// CheckUserHomeDirs 检查用户主目录权限
func (pc *PermissionChecker) CheckUserHomeDirs() *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	// 读取/etc/passwd获取用户主目录
	passwdData, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return result
	}

	lines := strings.Split(string(passwdData), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 6 {
			continue
		}

		homeDir := fields[5]
		if homeDir == "" || homeDir == "/" {
			continue
		}

		info, err := os.Stat(homeDir)
		if err != nil {
			continue
		}

		result.TotalChecked++
		mode := info.Mode()

		// 检查主目录权限
		if mode&0002 != 0 {
			issue := &PermissionIssue{
				Path:            homeDir,
				Type:            "directory",
				CurrentMode:     fmt.Sprintf("%04o", mode.Perm()),
				RecommendedMode: "remove world-writable",
				Issue:           "用户主目录全局可写",
				Severity:        SeverityMedium,
				Risk:            "全局可写的主目录可能被恶意利用",
				Owner:           fields[0],
			}
			result.Issues = append(result.Issues, issue)
			result.IssuesFound++
			result.WarningIssues++
		}
	}

	result.Suggestions = pc.generateSuggestions(result)
	return result
}

// ========== 系统配置检查 ==========

// CheckSystemConfig 检查系统配置文件权限
func (pc *PermissionChecker) CheckSystemConfig() *PermissionCheckResult {
	result := &PermissionCheckResult{
		ScanTime:     time.Now(),
		TotalChecked: 0,
		IssuesFound:  0,
		Issues:       make([]*PermissionIssue, 0),
		Suggestions:  make([]string, 0),
	}

	// 关键系统文件及其期望权限
	criticalFiles := map[string]string{
		"/etc/passwd":     "0644",
		"/etc/shadow":     "0640",
		"/etc/group":      "0644",
		"/etc/gshadow":    "0640",
		"/etc/sudoers":    "0440",
		"/etc/securetty":  "0600",
		"/etc/login.defs": "0644",
		"/etc/pam.d/":     "0755",
	}

	for path, expectedMode := range criticalFiles {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		result.TotalChecked++
		mode := info.Mode()

		currentMode := fmt.Sprintf("%04o", mode.Perm())
		if currentMode != expectedMode {
			severity := SeverityHigh
			if strings.Contains(path, "shadow") || strings.Contains(path, "gshadow") {
				severity = SeverityCritical
			}

			issue := &PermissionIssue{
				Path:            path,
				Type:            "file",
				CurrentMode:     currentMode,
				RecommendedMode: expectedMode,
				Issue:           fmt.Sprintf("系统关键文件权限不正确，应为 %s", expectedMode),
				Severity:        severity,
				Risk:            "系统配置文件权限不当可能导致系统被入侵",
			}
			result.Issues = append(result.Issues, issue)
			result.IssuesFound++

			if severity == SeverityCritical {
				result.CriticalIssues++
			} else {
				result.WarningIssues++
			}
		}
	}

	result.Suggestions = pc.generateSuggestions(result)
	return result
}

// ========== 辅助方法 ==========

// getRiskDescription 获取风险描述
func (pc *PermissionChecker) getRiskDescription(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "严重安全风险，可能导致系统被完全控制"
	case SeverityHigh:
		return "高风险，可能导致权限提升或数据泄露"
	case SeverityMedium:
		return "中等风险，可能导致未授权访问"
	case SeverityLow:
		return "低风险，建议修复"
	default:
		return "存在潜在安全风险"
	}
}

// generateSuggestions 生成修复建议
func (pc *PermissionChecker) generateSuggestions(result *PermissionCheckResult) []string {
	suggestions := make([]string, 0)

	if result.CriticalIssues > 0 {
		suggestions = append(suggestions, "立即处理严重级别的权限问题")
	}

	if result.WarningIssues > 0 {
		suggestions = append(suggestions, "尽快处理警告级别的权限问题")
	}

	// 检查是否有全局可写问题
	hasWorldWritable := false
	for _, issue := range result.Issues {
		if issue.Issue == "全局可写" {
			hasWorldWritable = true
			break
		}
	}

	if hasWorldWritable {
		suggestions = append(suggestions, "移除不必要的全局可写权限: chmod o-w <path>")
	}

	// 检查是否有SETUID问题
	hasSetuid := false
	for _, issue := range result.Issues {
		if issue.Issue == "SETUID位设置" {
			hasSetuid = true
			break
		}
	}

	if hasSetuid {
		suggestions = append(suggestions, "审查SETUID程序，移除不必要的SETUID位: chmod u-s <path>")
	}

	return suggestions
}

// ========== 统计功能 ==========

// GetStatistics 获取权限检查统计
func (pc *PermissionChecker) GetStatistics(result *PermissionCheckResult) map[string]interface{} {
	issuesByType := make(map[string]int)
	issuesBySeverity := make(map[string]int)

	for _, issue := range result.Issues {
		issuesByType[issue.Type]++
		issuesBySeverity[string(issue.Severity)]++
	}

	return map[string]interface{}{
		"total_checked":      result.TotalChecked,
		"issues_found":       result.IssuesFound,
		"critical_issues":    result.CriticalIssues,
		"warning_issues":     result.WarningIssues,
		"issues_by_type":     issuesByType,
		"issues_by_severity": issuesBySeverity,
	}
}

// ========== 快速检查 ==========

// QuickCheck 快速权限检查
func (pc *PermissionChecker) QuickCheck(paths []string) []*PermissionIssue {
	issues := make([]*PermissionIssue, 0)

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		mode := info.Mode()

		// 检查全局可写
		if mode&0002 != 0 {
			issues = append(issues, &PermissionIssue{
				Path:        path,
				Type:        "file",
				CurrentMode: fmt.Sprintf("%04o", mode.Perm()),
				Issue:       "全局可写",
				Severity:    SeverityMedium,
				Risk:        "全局可写文件可能被恶意修改",
			})
		}

		// 检查SETUID
		if mode&os.ModeSetuid != 0 {
			issues = append(issues, &PermissionIssue{
				Path:        path,
				Type:        "file",
				CurrentMode: fmt.Sprintf("%04o", mode.Perm()),
				Issue:       "SETUID位设置",
				Severity:    SeverityHigh,
				Risk:        "SETUID程序可能被用于权限提升",
			})
		}
	}

	return issues
}

// ========== 批量修复 ==========

// FixPermissions 修复权限
func (pc *PermissionChecker) FixPermissions(issues []*PermissionIssue, dryRun bool) ([]string, error) {
	results := make([]string, 0)

	for _, issue := range issues {
		if issue.RecommendedMode == "" {
			continue
		}

		var cmd string
		if issue.RecommendedMode == "remove world-writable" {
			cmd = fmt.Sprintf("chmod o-w %s", issue.Path)
		} else {
			cmd = fmt.Sprintf("chmod %s %s", issue.RecommendedMode, issue.Path)
		}

		if dryRun {
			results = append(results, "[DRY-RUN] "+cmd)
		} else {
			mode := parseFileMode(issue.RecommendedMode)
			if mode > 0 {
				if err := os.Chmod(issue.Path, mode); err != nil {
					results = append(results, fmt.Sprintf("[FAILED] %s: %v", cmd, err))
				} else {
					results = append(results, "[OK] "+cmd)
				}
			}
		}
	}

	return results, nil
}

// ========== 排序 ==========

// SortIssuesBySeverity 按严重程度排序
func SortIssuesBySeverity(issues []*PermissionIssue) []*PermissionIssue {
	sorted := make([]*PermissionIssue, len(issues))
	copy(sorted, issues)

	severityOrder := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		SeverityInfo:     4,
	}

	sort.Slice(sorted, func(i, j int) bool {
		return severityOrder[sorted[i].Severity] < severityOrder[sorted[j].Severity]
	})

	return sorted
}
