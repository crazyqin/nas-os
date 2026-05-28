// Package nasgateway 提供 WAF (Web Application Firewall) 防火墙功能
package nasgateway

import (
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// WAF Web应用防火墙.
type WAF struct {
	mu              sync.RWMutex
	config          *WAFConfig
	rules           []*WAFRule
	ipBlacklist     map[string]bool
	ipWhitelist     map[string]bool
	blockedPatterns []*regexp.Regexp
	stats           *WAFStats
	enabled         bool
}

// WAFStats WAF统计.
type WAFStats struct {
	TotalRequests int64 `json:"total_requests"`
	Blocked       int64 `json:"blocked"`
	Allowed       int64 `json:"allowed"`
	SQLInjection  int64 `json:"sql_injection"`
	XSS           int64 `json:"xss"`
	CSRF          int64 `json:"csrf"`
	IPBlocked     int64 `json:"ip_blocked"`
	PathTraversal int64 `json:"path_traversal"`
}

// NewWAF 创建WAF.
func NewWAF() *WAF {
	waf := &WAF{
		config: &WAFConfig{
			Enabled:       true,
			Mode:          "block",
			DefaultAction: WAFActionAllow,
			MaxBodySize:   10 * 1024 * 1024, // 10MB
			EnableLogging: true,
		},
		rules:           make([]*WAFRule, 0),
		ipBlacklist:     make(map[string]bool),
		ipWhitelist:     make(map[string]bool),
		blockedPatterns: make([]*regexp.Regexp, 0),
		stats:           &WAFStats{},
		enabled:         true,
	}

	// 初始化默认规则
	waf.initDefaultRules()

	return waf
}

// Enable 启用WAF.
func (w *WAF) Enable() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = true
	w.config.Enabled = true
}

// Disable 禁用WAF.
func (w *WAF) Disable() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.enabled = false
	w.config.Enabled = false
}

// IsEnabled 返回是否启用.
func (w *WAF) IsEnabled() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled
}

// SetMode 设置模式.
func (w *WAF) SetMode(mode string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config.Mode = mode
}

// Check 检查请求.
func (w *WAF) Check(clientIP, path, method string, headers http.Header) *WAFResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.stats.TotalRequests++

	// 检查IP白名单
	if w.ipWhitelist[clientIP] {
		return &WAFResult{
			Blocked:  false,
			ClientIP: clientIP,
			Path:     path,
		}
	}

	// 检查IP黑名单
	if w.ipBlacklist[clientIP] {
		w.stats.Blocked++
		w.stats.IPBlocked++
		return &WAFResult{
			Blocked:  true,
			Reason:   "IP在黑名单中",
			ClientIP: clientIP,
			Path:     path,
			Severity: "high",
		}
	}

	// 检查路径
	if w.checkPath(path) {
		w.stats.Blocked++
		w.stats.PathTraversal++
		return &WAFResult{
			Blocked:  true,
			Reason:   "路径遍历攻击",
			ClientIP: clientIP,
			Path:     path,
			Severity: "high",
		}
	}

	// 检查自定义规则
	for _, rule := range w.rules {
		if !rule.Enabled {
			continue
		}

		// 检查路径匹配
		if len(rule.Paths) > 0 {
			pathMatch := false
			for _, p := range rule.Paths {
				if strings.HasPrefix(path, p) {
					pathMatch = true
					break
				}
			}
			if !pathMatch {
				continue
			}
		}

		// 检查方法匹配
		if len(rule.Methods) > 0 {
			methodMatch := false
			for _, m := range rule.Methods {
				if m == method || m == "*" {
					methodMatch = true
					break
				}
			}
			if !methodMatch {
				continue
			}
		}

		// 检查规则模式
		if w.matchRule(rule, path, headers) {
			w.stats.Blocked++

			// 更新统计
			switch rule.Type {
			case WAFRuleSQLInjection:
				w.stats.SQLInjection++
			case WAFRuleXSS:
				w.stats.XSS++
			case WAFRuleCSRF:
				w.stats.CSRF++
			case WAFRulePathTraversal:
				w.stats.PathTraversal++
			}

			if rule.Action == WAFActionBlock {
				return &WAFResult{
					Blocked:  true,
					Rule:     rule,
					Reason:   rule.Description,
					ClientIP: clientIP,
					Path:     path,
					Severity: w.getSeverity(rule.Type),
				}
			}
		}
	}

	w.stats.Allowed++
	return &WAFResult{
		Blocked:  false,
		ClientIP: clientIP,
		Path:     path,
	}
}

// AddRule 添加规则.
func (w *WAF) AddRule(rule *WAFRule) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.rules = append(w.rules, rule)

	// 编译正则
	if rule.Pattern != "" {
		if regex, err := regexp.Compile(rule.Pattern); err == nil {
			w.blockedPatterns = append(w.blockedPatterns, regex)
		}
	}
}

// RemoveRule 删除规则.
func (w *WAF) RemoveRule(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, rule := range w.rules {
		if rule.ID == id {
			w.rules = append(w.rules[:i], w.rules[i+1:]...)
			break
		}
	}
}

// ListRules 列出规则.
func (w *WAF) ListRules() []*WAFRule {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.rules
}

// AddToBlacklist 添加到黑名单.
func (w *WAF) AddToBlacklist(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ipBlacklist[ip] = true
}

// RemoveFromBlacklist 从黑名单移除.
func (w *WAF) RemoveFromBlacklist(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.ipBlacklist, ip)
}

// GetBlacklist 获取黑名单.
func (w *WAF) GetBlacklist() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	list := make([]string, 0, len(w.ipBlacklist))
	for ip := range w.ipBlacklist {
		list = append(list, ip)
	}
	return list
}

// AddToWhitelist 添加到白名单.
func (w *WAF) AddToWhitelist(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ipWhitelist[ip] = true
}

// RemoveFromWhitelist 从白名单移除.
func (w *WAF) RemoveFromWhitelist(ip string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.ipWhitelist, ip)
}

// GetWhitelist 获取白名单.
func (w *WAF) GetWhitelist() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	list := make([]string, 0, len(w.ipWhitelist))
	for ip := range w.ipWhitelist {
		list = append(list, ip)
	}
	return list
}

// AddIPRange 添加IP范围到黑名单.
func (w *WAF) AddIPRange(cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// 存储CIDR，后续检查时解析
	w.ipBlacklist[cidr] = true
	_ = ipNet
	return nil
}

// GetStats 获取统计.
func (w *WAF) GetStats() *WAFStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// ResetStats 重置统计.
func (w *WAF) ResetStats() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stats = &WAFStats{}
}

// SetConfig 设置配置.
func (w *WAF) SetConfig(config *WAFConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config = config
	w.enabled = config.Enabled

	// 重新加载规则
	w.rules = config.Rules
	w.ipBlacklist = make(map[string]bool)
	for _, ip := range config.IPBlacklist {
		w.ipBlacklist[ip] = true
	}
	w.ipWhitelist = make(map[string]bool)
	for _, ip := range config.IPWhitelist {
		w.ipWhitelist[ip] = true
	}
}

// initDefaultRules 初始化默认规则.
func (w *WAF) initDefaultRules() {
	// SQL注入规则
	sqlPatterns := []struct {
		id      string
		pattern string
	}{
		{"sql_union_select", `(?i)(union\s+select)`},
		{"sql_select_from", `(?i)(select\s+.*\s+from)`},
		{"sql_insert_into", `(?i)(insert\s+into)`},
		{"sql_delete_from", `(?i)(delete\s+from)`},
		{"sql_drop_table", `(?i)(drop\s+table)`},
		{"sql_update_set", `(?i)(update\s+.*\s+set)`},
		{"sql_exec_func", `(?i)(exec\s*\()`},
		{"sql_or_true", `(?i)(or\s+1\s*=\s*1)`},
		{"sql_comment", `(?i)(;\s*--)`},
		{"sql_or_quote", `(?i)('\s*or\s+')`},
	}

	for _, sp := range sqlPatterns {
		rule := &WAFRule{
			ID:       sp.id,
			Name:     "SQL注入防护",
			Type:     WAFRuleSQLInjection,
			Action:   WAFActionBlock,
			Pattern:  sp.pattern,
			Enabled:  true,
			Priority: 100,
		}
		w.AddRule(rule)
	}

	// XSS规则
	xssPatterns := []struct {
		id      string
		pattern string
	}{
		{"xss_script_tag", `(?i)(<script[^>]*>)`},
		{"xss_javascript", `(?i)(javascript:)`},
		{"xss_on_event", `(?i)(on\w+\s*=)`},
		{"xss_iframe", `(?i)(<iframe[^>]*>)`},
		{"xss_eval", `(?i)(eval\s*\()`},
		{"xss_doc_cookie", `(?i)(document\.cookie)`},
	}

	for _, sp := range xssPatterns {
		rule := &WAFRule{
			ID:       sp.id,
			Name:     "XSS防护",
			Type:     WAFRuleXSS,
			Action:   WAFActionBlock,
			Pattern:  sp.pattern,
			Enabled:  true,
			Priority: 90,
		}
		w.AddRule(rule)
	}

	// 路径遍历规则
	pathPatterns := []struct {
		id      string
		pattern string
	}{
		{"path_dotdot_slash", `\.\./`},
		{"path_dotdot_back", `\.\.\\`},
		{"path_etc_passwd", `/etc/passwd`},
		{"path_etc_shadow", `/etc/shadow`},
		{"path_proc_self", `/proc/self`},
	}

	for _, sp := range pathPatterns {
		rule := &WAFRule{
			ID:       sp.id,
			Name:     "路径遍历防护",
			Type:     WAFRulePathTraversal,
			Action:   WAFActionBlock,
			Pattern:  sp.pattern,
			Enabled:  true,
			Priority: 80,
		}
		w.AddRule(rule)
	}
}

// matchRule 匹配规则.
func (w *WAF) matchRule(rule *WAFRule, path string, headers http.Header) bool {
	// 检查路径
	if matched, _ := regexp.MatchString(rule.Pattern, path); matched {
		return true
	}

	// 检查查询参数（从路径中提取）
	if idx := strings.Index(path, "?"); idx != -1 {
		query := path[idx+1:]
		if matched, _ := regexp.MatchString(rule.Pattern, query); matched {
			return true
		}
	}

	// 检查请求头
	if headers != nil {
		for _, values := range headers {
			for _, value := range values {
				if matched, _ := regexp.MatchString(rule.Pattern, value); matched {
					return true
				}
			}
		}
	}

	return false
}

// checkPath 检查路径.
func (w *WAF) checkPath(path string) bool {
	// 检查路径遍历
	dangerousPatterns := []string{
		"../",
		"..\\",
		"/etc/passwd",
		"/etc/shadow",
		"/proc/self",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// getSeverity 获取严重程度.
func (w *WAF) getSeverity(ruleType WAFRuleType) string {
	switch ruleType {
	case WAFRuleSQLInjection:
		return "critical"
	case WAFRuleCommandInjection:
		return "critical"
	case WAFRuleXSS:
		return "high"
	case WAFRuleCSRF:
		return "medium"
	case WAFRulePathTraversal:
		return "high"
	case WAFRuleFileUpload:
		return "medium"
	case WAFRuleIPBlacklist:
		return "high"
	default:
		return "low"
	}
}

// ========== WAF 中间件 ==========

// WAFMiddleware WAF中间件.
type WAFMiddleware struct {
	waf *WAF
}

// NewWAFMiddleware 创建WAF中间件.
func NewWAFMiddleware(waf *WAF) *WAFMiddleware {
	return &WAFMiddleware{waf: waf}
}

// Middleware 获取中间件函数.
func (m *WAFMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.waf.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := getClientIP(r)
		result := m.waf.Check(clientIP, r.URL.Path, r.Method, r.Header)

		if result.Blocked {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP 获取客户端IP.
func getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 检查X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从RemoteAddr获取
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ========== 防护检查器 ==========

// SQLInjectionChecker SQL注入检查器.
type SQLInjectionChecker struct {
	patterns []*regexp.Regexp
}

// NewSQLInjectionChecker 创建SQL注入检查器.
func NewSQLInjectionChecker() *SQLInjectionChecker {
	checker := &SQLInjectionChecker{
		patterns: make([]*regexp.Regexp, 0),
	}

	patterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(select\s+.*\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(delete\s+from)`,
		`(?i)(drop\s+table)`,
		`(?i)(update\s+.*\s+set)`,
		`(?i)(exec\s*\()`,
		`(?i)(or\s+1\s*=\s*1)`,
		`(?i)(;\s*--)`,
	}

	for _, p := range patterns {
		if regex, err := regexp.Compile(p); err == nil {
			checker.patterns = append(checker.patterns, regex)
		}
	}

	return checker
}

// Check 检查SQL注入.
func (c *SQLInjectionChecker) Check(input string) bool {
	for _, pattern := range c.patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// XSSChecker XSS检查器.
type XSSChecker struct {
	patterns []*regexp.Regexp
}

// NewXSSChecker 创建XSS检查器.
func NewXSSChecker() *XSSChecker {
	checker := &XSSChecker{
		patterns: make([]*regexp.Regexp, 0),
	}

	patterns := []string{
		`(?i)(<script[^>]*>)`,
		`(?i)(javascript:)`,
		`(?i)(on\w+\s*=)`,
		`(?i)(<iframe[^>]*>)`,
		`(?i)(eval\s*\()`,
	}

	for _, p := range patterns {
		if regex, err := regexp.Compile(p); err == nil {
			checker.patterns = append(checker.patterns, regex)
		}
	}

	return checker
}

// Check 检查XSS.
func (c *XSSChecker) Check(input string) bool {
	for _, pattern := range c.patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// CSRFProtector CSRF防护器.
type CSRFProtector struct {
	tokens    map[string]time.Time
	mu        sync.RWMutex
	tokenTTL  time.Duration
}

// NewCSRFProtector 创建CSRF防护器.
func NewCSRFProtector() *CSRFProtector {
	return &CSRFProtector{
		tokens:   make(map[string]time.Time),
		tokenTTL: time.Hour,
	}
}

// GenerateToken 生成CSRF令牌.
func (p *CSRFProtector) GenerateToken(sessionID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	token := generateRandomToken(32)
	p.tokens[token] = time.Now().Add(p.tokenTTL)
	return token
}

// ValidateToken 验证CSRF令牌.
func (p *CSRFProtector) ValidateToken(token string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	expiresAt, exists := p.tokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expiresAt) {
		delete(p.tokens, token)
		return false
	}

	return true
}

// Cleanup 清理过期令牌.
func (p *CSRFProtector) Cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for token, expiresAt := range p.tokens {
		if now.After(expiresAt) {
			delete(p.tokens, token)
		}
	}
}

// generateRandomToken 生成随机令牌.
func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
