// Package cloudsync 敏感文件同步过滤和安全检测
package cloudsync

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SensitiveFileFilter 敏感文件过滤器
type SensitiveFileFilter struct {
	config      SensitiveFilterConfig
	rules       []SensitiveFileRule
	contentRules []ContentDetectionRule
	auditLogger FilterAuditLogger
	mu          sync.RWMutex
}

// SensitiveFilterConfig 敏感文件过滤配置
type SensitiveFilterConfig struct {
	Enabled               bool               `json:"enabled"`
	DefaultAction         FilterAction       `json:"default_action"`         // block, warn, allow
	LogBlocked            bool               `json:"log_blocked"`            // 记录被阻止的文件
	AllowOverride         bool               `json:"allow_override"`         // 允许用户覆盖规则
	MaxFileSizeToScan     int64              `json:"max_file_size_to_scan"`  // 内容扫描最大文件大小
	ScanTimeout           time.Duration      `json:"scan_timeout"`           // 扫描超时时间
	BaselinePatterns      []string           `json:"baseline_patterns"`      // 基础敏感文件模式
	CustomPatterns        []string           `json:"custom_patterns"`        // 自定义敏感文件模式
	ContentDetection      bool               `json:"content_detection"`      // 启用内容检测
	AuditLevel            AuditLevel         `json:"audit_level"`            // 审计级别
	NotifyOnBlock         bool               `json:"notify_on_block"`        // 阻止时通知
}

// FilterAction 过滤动作
type FilterAction string

const (
	FilterBlock   FilterAction = "block"   // 阻止同步
	FilterWarn    FilterAction = "warn"    // 警告但允许
	FilterAllow   FilterAction = "allow"   // 允许
	FilterReview  FilterAction = "review"  // 需人工审核
)

// AuditLevel 审计级别
type AuditLevel string

const (
	AuditLevelMinimal AuditLevel = "minimal" // 最小审计
	AuditLevelNormal  AuditLevel = "normal"  // 正常审计
	AuditLevelDetailed AuditLevel = "detailed" // 详细审计
)

// SensitiveFileRule 敏感文件规则
type SensitiveFileRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    SensitiveCategory `json:"category"`
	Severity    SeverityLevel `json:"severity"`
	Action      FilterAction `json:"action"`
	Enabled     bool         `json:"enabled"`
	
	// 匹配规则
	Extensions  []string `json:"extensions,omitempty"`  // 文件扩展名
	Patterns    []string `json:"patterns,omitempty"`    // 文件名模式
	ExactNames  []string `json:"exact_names,omitempty"` // 精确文件名
	PathPrefixes []string `json:"path_prefixes,omitempty"` // 路径前缀
	
	// 内容检测
	ContentPatterns []string `json:"content_patterns,omitempty"` // 内容模式
	
	// 元数据
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by,omitempty"`
	Override    bool      `json:"override,omitempty"` // 用户覆盖
}

// ContentDetectionRule 内容检测规则
type ContentDetectionRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Category    SensitiveCategory `json:"category"`
	Severity    SeverityLevel `json:"severity"`
	
	// 正则匹配
	Pattern     string       `json:"pattern"`
	Regex       *regexp.Regexp `json:"-"`
	
	// 上下文检测
	ContextWords []string   `json:"context_words"` // 关键上下文词
	
	Enabled     bool        `json:"enabled"`
}

// SensitiveCategory 敏感文件类别
type SensitiveCategory string

const (
	CategoryCredential   SensitiveCategory = "credential"   // 凭证文件
	CategoryPrivateKey   SensitiveCategory = "private_key"  // 私钥文件
	CategoryEnvironment  SensitiveCategory = "environment"  // 环境配置
	CategoryDatabase     SensitiveCategory = "database"     // 数据库文件
	CategoryBackup       SensitiveCategory = "backup"       // 备份文件
	CategoryLog          SensitiveCategory = "log"          // 日志文件
	CategoryCertificate  SensitiveCategory = "certificate"  // 证书文件
	CategoryConfig       SensitiveCategory = "config"       // 配置文件
	CategoryPersonal     SensitiveCategory = "personal"     // 个人隐私
	CategoryCustom       SensitiveCategory = "custom"       // 自定义
)

// SeverityLevel 严重程度
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "critical" // 严重
	SeverityHigh     SeverityLevel = "high"     // 高
	SeverityMedium   SeverityLevel = "medium"   // 中
	SeverityLow      SeverityLevel = "low"      // 低
)

// FilterResult 过滤结果
type FilterResult struct {
	Path        string            `json:"path"`
	Action      FilterAction      `json:"action"`
	Reason      string            `json:"reason"`
	Rule        *SensitiveFileRule `json:"rule,omitempty"`
	ContentMatches []ContentMatch `json:"content_matches,omitempty"`
	Severity    SeverityLevel     `json:"severity"`
	Category    SensitiveCategory `json:"category"`
	Reviewed    bool              `json:"reviewed"`
	Reviewer    string            `json:"reviewer,omitempty"`
	ReviewTime  *time.Time        `json:"review_time,omitempty"`
}

// ContentMatch 内容匹配结果
type ContentMatch struct {
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	LineNumber  int       `json:"line_number"`
	MatchText   string    `json:"match_text"`
	Context     string    `json:"context"`
}

// FilterAuditLog 过滤审计日志
type FilterAuditLog struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	TaskID       string            `json:"task_id"`
	Path         string            `json:"path"`
	Action       FilterAction      `json:"action"`
	Reason       string            `json:"reason"`
	RuleID       string            `json:"rule_id,omitempty"`
	RuleName     string            `json:"rule_name,omitempty"`
	Severity     SeverityLevel     `json:"severity"`
	Category     SensitiveCategory `json:"category"`
	ContentHits  int               `json:"content_hits"`
	FileSize     int64             `json:"file_size"`
	DecisionBy   string            `json:"decision_by"` // system, user_override, manual_review
	Details      map[string]interface{} `json:"details,omitempty"`
}

// FilterAuditLogger 过滤审计日志接口
type FilterAuditLogger interface {
	LogFilterDecision(log *FilterAuditLog)
	LogBlockedFile(path string, reason string, rule *SensitiveFileRule)
	GetFilterHistory(taskID string, limit int) ([]*FilterAuditLog, error)
}

// DefaultSensitiveFilterConfig 默认敏感文件过滤配置
func DefaultSensitiveFilterConfig() SensitiveFilterConfig {
	return SensitiveFilterConfig{
		Enabled:           true,
		DefaultAction:     FilterBlock,
		LogBlocked:        true,
		AllowOverride:     false,
		MaxFileSizeToScan: 10 * 1024 * 1024, // 10MB
		ScanTimeout:       30 * time.Second,
		BaselinePatterns: []string{
			".env", "*.pem", "*.key", "*.secret",
			"*.password", "*.token", "*.credential",
			"*_rsa", "*_dsa", "*_ecdsa",
			"id_rsa", "id_dsa", "id_ecdsa",
			".htpasswd", ".htaccess",
			"credentials.json", "secrets.json",
			"*.keystore", "*.jks", "*.p12",
			"*.sqlite", "*.db", "*.sql",
		},
		ContentDetection: true,
		AuditLevel:       AuditLevelNormal,
		NotifyOnBlock:    true,
	}
}

// NewSensitiveFileFilter 创建敏感文件过滤器
func NewSensitiveFileFilter(config SensitiveFilterConfig, auditLogger FilterAuditLogger) (*SensitiveFileFilter, error) {
	filter := &SensitiveFileFilter{
		config:      config,
		auditLogger: auditLogger,
	}

	// 初始化默认规则
	if err := filter.initDefaultRules(); err != nil {
		return nil, err
	}

	// 初始化内容检测规则
	if config.ContentDetection {
		if err := filter.initContentRules(); err != nil {
			return nil, err
		}
	}

	// 加载自定义规则
	if len(config.CustomPatterns) > 0 {
		filter.addCustomRules(config.CustomPatterns)
	}

	return filter, nil
}

// initDefaultRules 初始化默认敏感文件规则
func (f *SensitiveFileFilter) initDefaultRules() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rules = []SensitiveFileRule{
		// SSH私钥
		{
			ID:          "ssh_private_key",
			Name:        "SSH私钥文件",
			Description: "SSH私钥文件包含敏感认证信息",
			Category:    CategoryPrivateKey,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"},
			Extensions:  []string{".pem"},
			CreatedAt:   time.Now(),
		},
		// 环境变量文件
		{
			ID:          "env_files",
			Name:        "环境变量文件",
			Description: "环境变量文件通常包含密码和密钥",
			Category:    CategoryEnvironment,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{".env", ".env.local", ".env.production", ".env.development"},
			Patterns:    []string{".env.*"},
			CreatedAt:   time.Now(),
		},
		// 密钥文件
		{
			ID:          "key_files",
			Name:        "密钥文件",
			Description: "各种密钥文件",
			Category:    CategoryPrivateKey,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			Extensions:  []string{".key", ".private", ".secret", ".token"},
			Patterns:    []string{"*_key", "*_private", "*.private.key"},
			CreatedAt:   time.Now(),
		},
		// 凭证文件
		{
			ID:          "credential_files",
			Name:        "凭证文件",
			Description: "包含用户凭证的文件",
			Category:    CategoryCredential,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{"credentials.json", "secrets.json", "passwords.txt"},
			Extensions:  []string{".credential", ".password"},
			CreatedAt:   time.Now(),
		},
		// 证书私钥
		{
			ID:          "certificate_key",
			Name:        "证书私钥",
			Description: "SSL/TLS证书私钥文件",
			Category:    CategoryPrivateKey,
			Severity:    SeverityHigh,
			Action:      FilterBlock,
			Enabled:     true,
			Extensions:  []string{".key", ".pkcs8", ".der"},
			Patterns:    []string{"*.key.pem", "*-key.pem"},
			CreatedAt:   time.Now(),
		},
		// 数据库文件
		{
			ID:          "database_files",
			Name:        "数据库文件",
			Description: "数据库文件可能包含敏感数据",
			Category:    CategoryDatabase,
			Severity:    SeverityHigh,
			Action:      FilterReview,
			Enabled:     true,
			Extensions:  []string{".db", ".sqlite", ".sqlite3", ".sql", ".dump"},
			CreatedAt:   time.Now(),
		},
		// 备份文件
		{
			ID:          "backup_files",
			Name:        "备份文件",
			Description: "备份文件可能包含敏感信息",
			Category:    CategoryBackup,
			Severity:    SeverityMedium,
			Action:      FilterReview,
			Enabled:     true,
			Extensions:  []string{".bak", ".backup", ".old", ".orig"},
			Patterns:    []string{"*_backup", "*.backup.*"},
			CreatedAt:   time.Now(),
		},
		// Apache凭证
		{
			ID:          "apache_credential",
			Name:        "Apache凭证文件",
			Description: "Apache htpasswd等凭证文件",
			Category:    CategoryCredential,
			Severity:    SeverityHigh,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{".htpasswd", ".htaccess"},
			CreatedAt:   time.Now(),
		},
		// Java密钥库
		{
			ID:          "java_keystore",
			Name:        "Java密钥库",
			Description: "Java KeyStore文件包含密钥和证书",
			Category:    CategoryPrivateKey,
			Severity:    SeverityHigh,
			Action:      FilterBlock,
			Enabled:     true,
			Extensions:  []string{".jks", ".keystore", ".p12", ".pfx"},
			CreatedAt:   time.Now(),
		},
		// AWS凭证
		{
			ID:          "aws_credentials",
			Name:        "AWS凭证文件",
			Description: "AWS CLI凭证配置文件",
			Category:    CategoryCredential,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{"credentials", "config"},
			PathPrefixes: []string{".aws/"},
			CreatedAt:   time.Now(),
		},
		// Kubernetes secrets
		{
			ID:          "k8s_secrets",
			Name:        "Kubernetes Secret",
			Description: "Kubernetes Secret配置文件",
			Category:    CategoryCredential,
			Severity:    SeverityCritical,
			Action:      FilterBlock,
			Enabled:     true,
			ExactNames:  []string{"secret.yaml", "secret.yml"},
			PathPrefixes: []string{"kubernetes/", "k8s/", "manifests/"},
			CreatedAt:   time.Now(),
		},
	}

	return nil
}

// initContentRules 初始化内容检测规则
func (f *SensitiveFileFilter) initContentRules() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// AWS Access Key Pattern
	awsAccessKeyRegex := regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	
	// AWS Secret Key Pattern
	awsSecretKeyRegex := regexp.MustCompile(`(?i)(aws_secret_access_key|secret_access_key|aws_secret_key)\s*[=:]\s*[A-Za-z0-9/+=]{40}`)
	
	// Private Key Pattern
	privateKeyRegex := regexp.MustCompile(`-----BEGIN (RSA |DSA |ECDSA |OPENSSH )PRIVATE KEY-----`)
	
	// Generic Password Pattern
	passwordRegex := regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*[^\s]{8,}`)
	
	// API Key Pattern
	apiKeyRegex := regexp.MustCompile(`(?i)(api[_-]?key|apikey|api_secret)\s*[=:]\s*[A-Za-z0-9_-]{20,}`)
	
	// Token Pattern
	tokenRegex := regexp.MustCompile(`(?i)(access[_-]?token|auth[_-]?token|bearer)\s*[=:]\s*[A-Za-z0-9_-]{20,}`)
	
	// Database URL Pattern
	dbURLRegex := regexp.MustCompile(`(?i)(mysql|postgres|mongodb|redis)://[^:]+:[^@]+@[^\s]+`)
	
	// JWT Pattern
	jwtRegex := regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)
	
	// SSH authorized_keys
	sshKeyRegex := regexp.MustCompile(`ssh-(rsa|dsa|ecdsa|ed25519) [A-Za-z0-9+/=]+`)
	
	// Email address
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	f.contentRules = []ContentDetectionRule{
		{
			ID:          "aws_access_key",
			Name:        "AWS Access Key",
			Description: "检测AWS Access Key ID",
			Category:    CategoryCredential,
			Severity:    SeverityCritical,
			Pattern:     `AKIA[0-9A-Z]{16}`,
			Regex:       awsAccessKeyRegex,
			Enabled:     true,
		},
		{
			ID:          "aws_secret_key",
			Name:        "AWS Secret Key",
			Description: "检测AWS Secret Access Key",
			Category:    CategoryCredential,
			Severity:    SeverityCritical,
			Pattern:     `(?i)(aws_secret_access_key|secret_access_key)\s*[=:]\s*[A-Za-z0-9/+=]{40}`,
			Regex:       awsSecretKeyRegex,
			Enabled:     true,
		},
		{
			ID:          "private_key_marker",
			Name:        "Private Key Marker",
			Description: "检测私钥文件开头标记",
			Category:    CategoryPrivateKey,
			Severity:    SeverityCritical,
			Pattern:     `-----BEGIN.*PRIVATE KEY-----`,
			Regex:       privateKeyRegex,
			Enabled:     true,
		},
		{
			ID:          "password_in_file",
			Name:        "Password in File",
			Description: "检测文件中的密码赋值",
			Category:    CategoryCredential,
			Severity:    SeverityHigh,
			Pattern:     `(?i)(password|passwd|pwd)\s*[=:]\s*[^\s]{8,}`,
			Regex:       passwordRegex,
			Enabled:     true,
		},
		{
			ID:          "api_key",
			Name:        "API Key",
			Description: "检测API密钥",
			Category:    CategoryCredential,
			Severity:    SeverityHigh,
			Pattern:     `(?i)(api[_-]?key|apikey)\s*[=:]\s*[A-Za-z0-9_-]{20,}`,
			Regex:       apiKeyRegex,
			Enabled:     true,
		},
		{
			ID:          "auth_token",
			Name:        "Authentication Token",
			Description: "检测认证Token",
			Category:    CategoryCredential,
			Severity:    SeverityHigh,
			Pattern:     `(?i)(access[_-]?token|auth[_-]?token)\s*[=:]\s*[A-Za-z0-9_-]{20,}`,
			Regex:       tokenRegex,
			Enabled:     true,
		},
		{
			ID:          "database_url",
			Name:        "Database URL",
			Description: "检测数据库连接URL",
			Category:    CategoryDatabase,
			Severity:    SeverityHigh,
			Pattern:     `(?i)(mysql|postgres|mongodb|redis)://[^:]+:[^@]+@[^\s]+`,
			Regex:       dbURLRegex,
			Enabled:     true,
		},
		{
			ID:          "jwt_token",
			Name:        "JWT Token",
			Description: "检测JWT格式Token",
			Category:    CategoryCredential,
			Severity:    SeverityMedium,
			Pattern:     `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`,
			Regex:       jwtRegex,
			Enabled:     true,
		},
		{
			ID:          "ssh_public_key",
			Name:        "SSH Public Key",
			Description: "检测SSH公钥",
			Category:    CategoryPrivateKey,
			Severity:    SeverityMedium,
			Pattern:     `ssh-(rsa|dsa|ecdsa|ed25519) [A-Za-z0-9+/=]+`,
			Regex:       sshKeyRegex,
			Enabled:     true,
		},
		{
			ID:          "email_address",
			Name:        "Email Address",
			Description: "检测邮箱地址",
			Category:    CategoryPersonal,
			Severity:    SeverityLow,
			Pattern:     `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			Regex:       emailRegex,
			Enabled:     false, // 默认关闭，隐私级别较低
		},
	}

	return nil
}

// addCustomRules 添加自定义规则
func (f *SensitiveFileFilter) addCustomRules(patterns []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, pattern := range patterns {
		rule := SensitiveFileRule{
			ID:          fmt.Sprintf("custom_%d", i),
			Name:        fmt.Sprintf("自定义规则 %d", i+1),
			Description: "用户自定义敏感文件规则",
			Category:    CategoryCustom,
			Severity:    SeverityHigh,
			Action:      f.config.DefaultAction,
			Enabled:     true,
			Patterns:    []string{pattern},
			CreatedAt:   time.Now(),
			CreatedBy:   "user",
			Override:    true,
		}
		f.rules = append(f.rules, rule)
	}
}

// ==================== 核心过滤方法 ====================

// CheckFile 检查文件是否敏感
func (f *SensitiveFileFilter) CheckFile(ctx context.Context, filePath string, fileInfo os.FileInfo) *FilterResult {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.config.Enabled {
		return &FilterResult{
			Path:    filePath,
			Action:  FilterAllow,
			Reason:  "过滤功能未启用",
			Severity: SeverityLow,
		}
	}

	result := &FilterResult{
		Path:     filePath,
		Action:   FilterAllow,
		Severity: SeverityLow,
	}

	// 1. 检查文件名规则
	for _, rule := range f.rules {
		if !rule.Enabled {
			continue
		}

		if f.matchesRule(filePath, rule) {
			result.Action = rule.Action
			result.Rule = &rule
			result.Category = rule.Category
			result.Severity = rule.Severity
			result.Reason = fmt.Sprintf("匹配规则: %s (%s)", rule.Name, rule.ID)
			break
		}
	}

	// 2. 内容检测（如果启用且文件允许）
	if f.config.ContentDetection && result.Action != FilterBlock && fileInfo != nil {
		if fileInfo.Size() <= f.config.MaxFileSizeToScan {
			contentMatches, err := f.scanFileContent(ctx, filePath)
			if err == nil && len(contentMatches) > 0 {
				result.ContentMatches = contentMatches
				
				// 根据内容匹配严重程度决定动作
				maxSeverity := SeverityLow
				for _, match := range contentMatches {
					for _, contentRule := range f.contentRules {
						if contentRule.ID == match.RuleID && contentRule.Severity > maxSeverity {
							maxSeverity = contentRule.Severity
							result.Category = contentRule.Category
						}
					}
				}
				
				if maxSeverity >= SeverityHigh {
					result.Action = FilterBlock
					result.Severity = maxSeverity
					result.Reason = fmt.Sprintf("检测到敏感内容: %d处匹配", len(contentMatches))
				} else if maxSeverity >= SeverityMedium {
					result.Action = FilterReview
					result.Severity = maxSeverity
					result.Reason = fmt.Sprintf("可能包含敏感内容: %d处匹配", len(contentMatches))
				}
			}
		}
	}

	return result
}

// matchesRule 检查文件是否匹配规则
func (f *SensitiveFileFilter) matchesRule(filePath string, rule SensitiveFileRule) bool {
	fileName := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))

	// 检查精确名称匹配
	for _, exactName := range rule.ExactNames {
		if fileName == exactName {
			return true
		}
	}

	// 检查扩展名匹配
	for _, extPattern := range rule.Extensions {
		if ext == strings.ToLower(extPattern) {
			return true
		}
	}

	// 检查文件名模式匹配
	for _, pattern := range rule.Patterns {
		matched, err := filepath.Match(pattern, fileName)
		if err == nil && matched {
			return true
		}
	}

	// 检查路径前缀匹配
	for _, prefix := range rule.PathPrefixes {
		if strings.Contains(filePath, prefix) {
			return true
		}
	}

	return false
}

// scanFileContent 扫描文件内容
func (f *SensitiveFileFilter) scanFileContent(ctx context.Context, filePath string) ([]ContentMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	matches := []ContentMatch{}
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, f.config.ScanTimeout)
	defer cancel()

	scanDone := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				scanDone <- true
				return
			default:
			}

			lineNumber++
			line := scanner.Text()

			// 检查所有内容规则
			for _, rule := range f.contentRules {
				if !rule.Enabled || rule.Regex == nil {
					continue
				}

				loc := rule.Regex.FindStringIndex(line)
				if loc != nil {
					match := ContentMatch{
						RuleID:     rule.ID,
						RuleName:   rule.Name,
						LineNumber: lineNumber,
						MatchText:  truncateMatch(line[loc[0]:loc[1]], 50),
						Context:    truncateMatch(line, 100),
					}
					matches = append(matches, match)
				}
			}
		}
		scanDone <- true
	}()

	select {
	case <-ctx.Done():
		return matches, ctx.Err()
	case <-scanDone:
		return matches, scanner.Err()
	}
}

// ==================== 批量检查 ====================

// CheckDirectory 检查目录中的所有文件
func (f *SensitiveFileFilter) CheckDirectory(ctx context.Context, dirPath string, recursive bool) ([]*FilterResult, error) {
	results := []*FilterResult{}
	
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查文件
		result := f.CheckFile(ctx, path, info)
		results = append(results, result)

		return nil
	})

	return results, err
}

// FilterSyncFiles 过滤同步文件列表
func (f *SensitiveFileFilter) FilterSyncFiles(ctx context.Context, taskID string, files []FileInfo) ([]FileInfo, []*FilterResult, error) {
	allowedFiles := []FileInfo{}
	blockedResults := []*FilterResult{}

	for _, file := range files {
		// 创建FileInfo
		info := &fileInfoWrapper{
			name: file.Path,
			size: file.Size,
			modTime: file.ModTime,
			isDir: file.IsDir,
		}

		result := f.CheckFile(ctx, file.Path, info)

		// 记录审计日志
		if f.config.AuditLevel != AuditLevelMinimal && f.auditLogger != nil {
			auditLog := &FilterAuditLog{
				ID:        generateAuditID(),
				Timestamp: time.Now(),
				TaskID:    taskID,
				Path:      file.Path,
				Action:    result.Action,
				Reason:    result.Reason,
				Severity:  result.Severity,
				Category:  result.Category,
				FileSize:  file.Size,
				DecisionBy: "system",
			}
			if result.Rule != nil {
				auditLog.RuleID = result.Rule.ID
				auditLog.RuleName = result.Rule.Name
			}
			if len(result.ContentMatches) > 0 {
				auditLog.ContentHits = len(result.ContentMatches)
			}
			f.auditLogger.LogFilterDecision(auditLog)
		}

		// 根据结果决定是否允许同步
		switch result.Action {
		case FilterAllow:
			allowedFiles = append(allowedFiles, file)
		case FilterWarn:
			allowedFiles = append(allowedFiles, file)
			blockedResults = append(blockedResults, result)
		case FilterBlock, FilterReview:
			blockedResults = append(blockedResults, result)
		}
	}

	return allowedFiles, blockedResults, nil
}

// ==================== 审计报告 ====================

// GenerateFilterReport 生成过滤报告
func (f *SensitiveFileFilter) GenerateFilterReport(taskID string, startTime, endTime time.Time) (*FilterReport, error) {
	if f.auditLogger == nil {
		return nil, fmt.Errorf("审计日志未配置")
	}

	logs, err := f.auditLogger.GetFilterHistory(taskID, 1000)
	if err != nil {
		return nil, err
	}

	report := &FilterReport{
		TaskID:     taskID,
		StartTime:  startTime,
		EndTime:    endTime,
		GeneratedAt: time.Now(),
		Statistics: FilterStatistics{},
		ByCategory: make(map[SensitiveCategory]int),
		BySeverity: make(map[SeverityLevel]int),
		BlockedFiles: []BlockedFileInfo{},
	}

	// 统计
	for _, log := range logs {
		report.Statistics.TotalFiles++
		
		switch log.Action {
		case FilterBlock:
			report.Statistics.BlockedFiles++
		case FilterWarn:
			report.Statistics.WarnedFiles++
		case FilterReview:
			report.Statistics.ReviewRequiredFiles++
		case FilterAllow:
			report.Statistics.AllowedFiles++
		}

		report.ByCategory[log.Category]++
		report.BySeverity[log.Severity]++

		// 详细记录被阻止的文件
		if log.Action == FilterBlock {
			info := BlockedFileInfo{
				Path:     log.Path,
				Reason:   log.Reason,
				RuleName: log.RuleName,
				Severity: log.Severity,
				Category: log.Category,
				Time:     log.Timestamp,
			}
			report.BlockedFiles = append(report.BlockedFiles, info)
		}
	}

	return report, nil
}

// FilterReport 过滤报告
type FilterReport struct {
	TaskID        string            `json:"task_id"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Statistics    FilterStatistics  `json:"statistics"`
	ByCategory    map[SensitiveCategory]int `json:"by_category"`
	BySeverity    map[SeverityLevel]int `json:"by_severity"`
	BlockedFiles  []BlockedFileInfo `json:"blocked_files"`
}

// FilterStatistics 过滤统计
type FilterStatistics struct {
	TotalFiles          int `json:"total_files"`
	AllowedFiles        int `json:"allowed_files"`
	BlockedFiles        int `json:"blocked_files"`
	WarnedFiles         int `json:"warned_files"`
	ReviewRequiredFiles int `json:"review_required_files"`
	ContentHits         int `json:"content_hits"`
}

// BlockedFileInfo 被阻止文件信息
type BlockedFileInfo struct {
	Path     string        `json:"path"`
	Reason   string        `json:"reason"`
	RuleName string        `json:"rule_name"`
	Severity SeverityLevel `json:"severity"`
	Category SensitiveCategory `json:"category"`
	Time     time.Time     `json:"time"`
}

// ==================== 规则管理 ====================

// AddRule 添加规则
func (f *SensitiveFileFilter) AddRule(rule SensitiveFileRule) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	f.rules = append(f.rules, rule)
	return nil
}

// RemoveRule 删除规则
func (f *SensitiveFileFilter) RemoveRule(ruleID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, rule := range f.rules {
		if rule.ID == ruleID {
			f.rules = append(f.rules[:i], f.rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("规则不存在: %s", ruleID)
}

// GetRules 获取所有规则
func (f *SensitiveFileFilter) GetRules() []SensitiveFileRule {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.rules
}

// ==================== 辅助函数 ====================

type fileInfoWrapper struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (f *fileInfoWrapper) Name() string       { return f.name }
func (f *fileInfoWrapper) Size() int64        { return f.size }
func (f *fileInfoWrapper) Mode() os.FileMode  { return 0 }
func (f *fileInfoWrapper) ModTime() time.Time { return f.modTime }
func (f *fileInfoWrapper) IsDir() bool        { return f.isDir }
func (f *fileInfoWrapper) Sys() interface{}   { return nil }

func truncateMatch(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// 对敏感内容进行遮蔽处理
	return s[:maxLen/2] + "..." + s[len(s)-maxLen/4:]
}

func generateAuditID() string {
	return fmt.Sprintf("audit_%d", time.Now().UnixNano())
}

// GetConfig 获取配置
func (f *SensitiveFileFilter) GetConfig() SensitiveFilterConfig {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config
}

// UpdateConfig 更新配置
func (f *SensitiveFileFilter) UpdateConfig(config SensitiveFilterConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = config
	return nil
}

// Enable 启用过滤
func (f *SensitiveFileFilter) Enable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config.Enabled = true
}

// Disable 禁用过滤
func (f *SensitiveFileFilter) Disable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config.Enabled = false
}