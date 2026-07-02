// Package emailsecurity 提供邮件安全扫描功能
package emailsecurity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Scanner 邮件安全扫描器.
type Scanner struct {
	mu              sync.RWMutex
	policies        map[string]*SecurityPolicy
	quarantineStore map[string]*QuarantineItem
	urlBlacklist    map[string]bool
	domainBlacklist map[string]bool
	executableExts  map[string]bool
	macroDocTypes   map[string]bool
	keywordPatterns []*regexp.Regexp
}

// NewScanner 创建新的扫描器实例.
func NewScanner() *Scanner {
	s := &Scanner{
		policies:        make(map[string]*SecurityPolicy),
		quarantineStore: make(map[string]*QuarantineItem),
		urlBlacklist:    make(map[string]bool),
		domainBlacklist: make(map[string]bool),
		executableExts:  make(map[string]bool),
		macroDocTypes:   make(map[string]bool),
		keywordPatterns: make([]*regexp.Regexp, 0),
	}
	s.initDefaultConfig()
	return s
}

// initDefaultConfig 初始化默认配置.
func (s *Scanner) initDefaultConfig() {
	// 可执行文件扩展名
	execExts := []string{
		".exe", ".bat", ".cmd", ".com", ".msi", ".scr", ".pif",
		".vbs", ".vbe", ".js", ".jse", ".wsf", ".wsh", ".ps1",
		".reg", ".dll", ".sys", ".drv",
	}
	for _, ext := range execExts {
		s.executableExts[ext] = true
	}

	// 含宏的文档类型
	macroTypes := []string{
		".doc", ".docm", ".xls", ".xlsm", ".ppt", ".pptm",
		".dot", ".dotm", ".xlt", ".xltm", ".pot", ".potm",
	}
	for _, t := range macroTypes {
		s.macroDocTypes[t] = true
	}
}

// ScanEmail 扫描邮件.
func (s *Scanner) ScanEmail(req ScanEmailRequest) (*ScanResult, error) {
	startTime := time.Now()

	result := &ScanResult{
		Threats:       make([]ThreatItem, 0),
		Score:         0,
		ScannerEngine: "nas-os-email-security",
		ScannedAt:     startTime,
	}

	// 获取适用的安全策略
	policies := s.getApplicablePolicies()

	// 执行各项扫描
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		// 附件安全扫描
		if policy.AttachmentScan.Enabled {
			threats := s.scanAttachments(req.Attachments, policy.AttachmentScan)
			result.Threats = append(result.Threats, threats...)
		}

		// 钓鱼链接检测
		if policy.PhishingDetection.Enabled {
			threats := s.scanForPhishing(req.Body, req.Subject, policy.PhishingDetection)
			result.Threats = append(result.Threats, threats...)
		}

		// 内容合规检查
		if policy.ContentCompliance.Enabled {
			threats := s.scanContentCompliance(req.Body, req.Subject, policy.ContentCompliance)
			result.Threats = append(result.Threats, threats...)
		}
	}

	// 计算威胁评分
	result.Score = s.calculateThreatScore(result.Threats)
	result.ScanDuration = time.Since(startTime).Milliseconds()

	log.Printf("[邮件安全] 扫描完成: messageID=%s, 威胁数=%d, 评分=%d, 耗时=%dms",
		req.MessageID, len(result.Threats), result.Score, result.ScanDuration)

	return result, nil
}

// scanAttachments 扫描附件.
func (s *Scanner) scanAttachments(attachments []AttachmentInfo, config AttachmentScanConfig) []ThreatItem {
	threats := make([]ThreatItem, 0)

	for _, att := range attachments {
		// 检查附件大小
		if config.MaxSizeMB > 0 && att.Size > int64(config.MaxSizeMB)*1024*1024 {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeAttachment,
				Name:        "附件过大",
				Description: fmt.Sprintf("附件 %s 超过大小限制 (%dMB)", att.Filename, config.MaxSizeMB),
				Severity:    ThreatLevelMedium,
				Location:    att.Filename,
				Action:      AuditActionBlock,
			})
		}

		// 检查可执行文件
		if config.BlockExecutables && s.isExecutable(att.Filename) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeAttachment,
				Name:        "可执行文件",
				Description: fmt.Sprintf("检测到可执行文件: %s", att.Filename),
				Severity:    ThreatLevelHigh,
				Location:    att.Filename,
				Action:      AuditActionBlock,
			})
		}

		// 检查宏文档
		if config.BlockMacroDocs && s.isMacroDocument(att.Filename) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeAttachment,
				Name:        "宏文档",
				Description: fmt.Sprintf("检测到可能包含宏的文档: %s", att.Filename),
				Severity:    ThreatLevelMedium,
				Location:    att.Filename,
				Action:      AuditActionQuarantine,
			})
		}

		// 检查压缩文件类型
		if len(config.BlockArchiveTypes) > 0 && s.isBlockedArchive(att.Filename, config.BlockArchiveTypes) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeAttachment,
				Name:        "受限压缩文件",
				Description: fmt.Sprintf("检测到受限压缩文件类型: %s", att.Filename),
				Severity:    ThreatLevelLow,
				Location:    att.Filename,
				Action:      AuditActionBlock,
			})
		}
	}

	return threats
}

// scanForPhishing 扫描钓鱼链接.
func (s *Scanner) scanForPhishing(body, subject string, config PhishingDetectionConfig) []ThreatItem {
	threats := make([]ThreatItem, 0)

	// 提取所有URL
	urls := s.extractURLs(body)

	// 检查URL数量
	if config.MaxURLsPerEmail > 0 && len(urls) > config.MaxURLsPerEmail {
		threats = append(threats, ThreatItem{
			Type:        ThreatTypePhishing,
			Name:        "过多链接",
			Description: fmt.Sprintf("邮件包含过多链接: %d (限制: %d)", len(urls), config.MaxURLsPerEmail),
			Severity:    ThreatLevelMedium,
			Location:    "body",
			Action:      AuditActionAlert,
		})
	}

	// 检查每个URL
	for _, url := range urls {
		domain := s.extractDomain(url)
		if domain == "" {
			continue
		}

		// 检查黑名单域名
		if config.BlockSuspiciousURLs && s.isDomainBlacklisted(domain, config.BlacklistDomains) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypePhishing,
				Name:        "黑名单域名",
				Description: fmt.Sprintf("检测到黑名单域名: %s", domain),
				Severity:    ThreatLevelHigh,
				Location:    url,
				Action:      AuditActionBlock,
			})
			continue
		}

		// 检查URL信誉
		if config.CheckURLReputation && !s.isDomainWhitelisted(domain, config.WhitelistDomains) {
			if s.isSuspiciousURL(url) {
				threats = append(threats, ThreatItem{
					Type:        ThreatTypePhishing,
					Name:        "可疑链接",
					Description: fmt.Sprintf("检测到可疑链接: %s", url),
					Severity:    ThreatLevelMedium,
					Location:    url,
					Action:      AuditActionQuarantine,
				})
			}
		}
	}

	// 检查主题行钓鱼特征
	if s.hasPhishingSubject(subject) {
		threats = append(threats, ThreatItem{
			Type:        ThreatTypePhishing,
			Name:        "钓鱼主题",
			Description: "邮件主题包含钓鱼特征",
			Severity:    ThreatLevelMedium,
			Location:    "subject",
			Action:      AuditActionAlert,
		})
	}

	return threats
}

// scanContentCompliance 扫描内容合规.
func (s *Scanner) scanContentCompliance(body, subject string, config ContentComplianceConfig) []ThreatItem {
	threats := make([]ThreatItem, 0)
	content := strings.ToLower(subject + " " + body)

	// 关键词过滤
	for _, keyword := range config.KeywordFilters {
		if strings.Contains(content, strings.ToLower(keyword)) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeContent,
				Name:        "关键词违规",
				Description: fmt.Sprintf("检测到敏感关键词: %s", keyword),
				Severity:    ThreatLevelMedium,
				Location:    "content",
				Action:      AuditActionQuarantine,
			})
		}
	}

	// 正则表达式匹配
	for _, pattern := range config.RegexPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("[邮件安全] 正则表达式编译失败: %s, error: %v", pattern, err)
			continue
		}
		if re.MatchString(content) {
			threats = append(threats, ThreatItem{
				Type:        ThreatTypeContent,
				Name:        "模式匹配违规",
				Description: fmt.Sprintf("内容匹配敏感模式: %s", pattern),
				Severity:    ThreatLevelMedium,
				Location:    "content",
				Action:      AuditActionQuarantine,
			})
		}
	}

	// 检测机密信息
	if config.BlockConfidential {
		confidentialPatterns := []struct {
			name    string
			pattern string
		}{
			{"信用卡号", `\b(?:\d{4}[- ]?){3}\d{4}\b`},
			{"身份证号", `\b\d{17}[\dXx]\b`},
			{"手机号码", `\b1[3-9]\d{9}\b`},
			{"邮箱地址", `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`},
		}

		for _, cp := range confidentialPatterns {
			re := regexp.MustCompile(cp.pattern)
			if re.MatchString(content) {
				threats = append(threats, ThreatItem{
					Type:        ThreatTypeContent,
					Name:        "机密信息泄露",
					Description: fmt.Sprintf("检测到可能的%s", cp.name),
					Severity:    ThreatLevelHigh,
					Location:    "content",
					Action:      AuditActionBlock,
				})
			}
		}
	}

	return threats
}

// calculateThreatScore 计算威胁评分.
func (s *Scanner) calculateThreatScore(threats []ThreatItem) int {
	if len(threats) == 0 {
		return 0
	}

	score := 0
	severityScores := map[string]int{
		ThreatLevelLow:      10,
		ThreatLevelMedium:   30,
		ThreatLevelHigh:     60,
		ThreatLevelCritical: 100,
	}

	for _, threat := range threats {
		if s, ok := severityScores[threat.Severity]; ok {
			score += s
		}
	}

	// 限制最大分数为100
	if score > 100 {
		score = 100
	}

	return score
}

// getApplicablePolicies 获取适用的安全策略.
func (s *Scanner) getApplicablePolicies() []*SecurityPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]*SecurityPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}

	// 按优先级排序
	for i := 0; i < len(policies)-1; i++ {
		for j := i + 1; j < len(policies); j++ {
			if policies[i].Priority > policies[j].Priority {
				policies[i], policies[j] = policies[j], policies[i]
			}
		}
	}

	return policies
}

// isExecutable 检查是否为可执行文件.
func (s *Scanner) isExecutable(filename string) bool {
	lower := strings.ToLower(filename)
	for ext := range s.executableExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// isMacroDocument 检查是否为宏文档.
func (s *Scanner) isMacroDocument(filename string) bool {
	lower := strings.ToLower(filename)
	for ext := range s.macroDocTypes {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// isBlockedArchive 检查是否为受限压缩文件.
func (s *Scanner) isBlockedArchive(filename string, blockedTypes []string) bool {
	lower := strings.ToLower(filename)
	for _, t := range blockedTypes {
		if strings.HasSuffix(lower, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

// extractURLs 提取文本中的URL.
func (s *Scanner) extractURLs(text string) []string {
	urlPattern := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	return urlPattern.FindAllString(text, -1)
}

// extractDomain 从URL中提取域名.
func (s *Scanner) extractDomain(url string) string {
	// 移除协议前缀
	domain := url
	if idx := strings.Index(domain, "://"); idx != -1 {
		domain = domain[idx+3:]
	}
	// 移除路径
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	// 移除端口
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}
	return strings.ToLower(domain)
}

// isDomainBlacklisted 检查域名是否在黑名单中.
func (s *Scanner) isDomainBlacklisted(domain string, blacklist []string) bool {
	for _, bd := range blacklist {
		if strings.EqualFold(domain, bd) || strings.HasSuffix(domain, "."+bd) {
			return true
		}
	}
	// 检查内置黑名单
	if s.domainBlacklist[domain] {
		return true
	}
	return false
}

// isDomainWhitelisted 检查域名是否在白名单中.
func (s *Scanner) isDomainWhitelisted(domain string, whitelist []string) bool {
	for _, wd := range whitelist {
		if strings.EqualFold(domain, wd) || strings.HasSuffix(domain, "."+wd) {
			return true
		}
	}
	return false
}

// isSuspiciousURL 检查URL是否可疑.
func (s *Scanner) isSuspiciousURL(url string) bool {
	suspiciousPatterns := []string{
		"login", "signin", "verify", "update", "secure",
		"account", "banking", "paypal", "amazon", "apple",
		"microsoft", "google", "facebook", "instagram",
	}

	lower := strings.ToLower(url)
	score := 0
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lower, pattern) {
			score++
		}
	}

	// 检查IP地址作为域名
	ipPattern := regexp.MustCompile(`https?://\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	if ipPattern.MatchString(url) {
		score += 3
	}

	// 检查过长的URL
	if len(url) > 200 {
		score++
	}

	// 检查过多的子域名
	domain := s.extractDomain(url)
	if strings.Count(domain, ".") > 3 {
		score++
	}

	return score >= 2
}

// hasPhishingSubject 检查主题是否包含钓鱼特征.
func (s *Scanner) hasPhishingSubject(subject string) bool {
	phishingKeywords := []string{
		"urgent", "immediately", "action required", "verify your",
		"account suspended", "unauthorized", "suspicious activity",
		"click here", "confirm your", "update your",
		"紧急", "立即", "需要操作", "验证您的",
		"账户暂停", "未授权", "可疑活动",
	}

	lower := strings.ToLower(subject)
	for _, keyword := range phishingKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// AddPolicy 添加安全策略.
func (s *Scanner) AddPolicy(policy *SecurityPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[policy.ID] = policy
}

// RemovePolicy 移除安全策略.
func (s *Scanner) RemovePolicy(policyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.policies, policyID)
}

// GetPolicy 获取安全策略.
func (s *Scanner) GetPolicy(policyID string) (*SecurityPolicy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[policyID]
	return p, ok
}

// ListPolicies 列出所有安全策略.
func (s *Scanner) ListPolicies() []*SecurityPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policies := make([]*SecurityPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	return policies
}

// AddURLToBlacklist 添加URL到黑名单.
func (s *Scanner) AddURLToBlacklist(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urlBlacklist[strings.ToLower(url)] = true
}

// AddDomainToBlacklist 添加域名到黑名单.
func (s *Scanner) AddDomainToBlacklist(domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.domainBlacklist[strings.ToLower(domain)] = true
}

// GenerateContentHash 生成内容哈希.
func GenerateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
