package ipprotection

import (
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== 异常行为检测器 ====================

// Detector 异常行为检测器
type Detector struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *IPProtectionConfig
	loginAttempts map[string][]*LoginAttempt   // IP -> 登录记录
	accessRecords map[string][]*AccessRecord   // IP -> 访问记录
	portAccess    map[string]map[int]time.Time // IP -> port -> 首次访问时间
}

// NewDetector 创建异常行为检测器
func NewDetector(logger *zap.Logger, config *IPProtectionConfig) *Detector {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultIPProtectionConfig()
	}

	return &Detector{
		logger:        logger,
		config:        config,
		loginAttempts: make(map[string][]*LoginAttempt),
		accessRecords: make(map[string][]*AccessRecord),
		portAccess:    make(map[string]map[int]time.Time),
	}
}

// RecordLoginAttempt 记录登录尝试
func (d *Detector) RecordLoginAttempt(attempt *LoginAttempt) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ip := attempt.IP
	d.loginAttempts[ip] = append(d.loginAttempts[ip], attempt)

	// 清理过期记录
	d.cleanLoginAttempts(ip)
}

// RecordAccess 记录访问
func (d *Detector) RecordAccess(record *AccessRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ip := record.IP
	d.accessRecords[ip] = append(d.accessRecords[ip], record)

	// 记录端口访问
	if record.Port > 0 {
		if d.portAccess[ip] == nil {
			d.portAccess[ip] = make(map[int]time.Time)
		}
		if _, exists := d.portAccess[ip][record.Port]; !exists {
			d.portAccess[ip][record.Port] = record.Timestamp
		}
	}

	// 清理过期记录
	d.cleanAccessRecords(ip)
}

// DetectLoginFailure 检测登录失败是否触发阈值
// 返回：是否触发、当前失败次数、阈值
func (d *Detector) DetectLoginFailure(ip string) (bool, int, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	attempts := d.loginAttempts[ip]
	window := d.config.LoginFailureWindow
	threshold := d.config.LoginFailureThreshold
	now := time.Now()

	count := 0
	for i := len(attempts) - 1; i >= 0; i-- {
		if now.Sub(attempts[i].Timestamp) > window {
			break
		}
		if !attempts[i].Success {
			count++
		}
	}

	return count >= threshold, count, threshold
}

// DetectPortScan 检测端口扫描行为
func (d *Detector) DetectPortScan(ip string) *DetectionResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ports := d.portAccess[ip]
	if ports == nil {
		return &DetectionResult{
			Detected: false,
			Type:     DetectionPortScan,
			IP:       ip,
		}
	}

	window := d.config.PortScanWindow
	threshold := d.config.PortScanThreshold
	now := time.Now()

	count := 0
	for _, firstSeen := range ports {
		if now.Sub(firstSeen) <= window {
			count++
		}
	}

	detected := count >= threshold
	result := &DetectionResult{
		Detected:  detected,
		Type:      DetectionPortScan,
		IP:        ip,
		Timestamp: now,
	}

	if detected {
		result.ThreatLevel = ThreatLevelHigh
		result.Details = "检测到端口扫描行为"
		result.Confidence = minFloat64(float64(count)/float64(threshold)*0.5, 1.0)
	}

	return result
}

// DetectBruteForce 检测暴力破解行为
func (d *Detector) DetectBruteForce(ip string) *DetectionResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	attempts := d.loginAttempts[ip]
	if attempts == nil {
		return &DetectionResult{
			Detected: false,
			Type:     DetectionBruteForce,
			IP:       ip,
		}
	}

	window := d.config.BruteForceWindow
	threshold := d.config.BruteForceThreshold
	now := time.Now()

	failCount := 0
	var usernames []string
	for i := len(attempts) - 1; i >= 0; i-- {
		if now.Sub(attempts[i].Timestamp) > window {
			break
		}
		if !attempts[i].Success {
			failCount++
			if attempts[i].Username != "" {
				usernames = append(usernames, attempts[i].Username)
			}
		}
	}

	detected := failCount >= threshold
	result := &DetectionResult{
		Detected:  detected,
		Type:      DetectionBruteForce,
		IP:        ip,
		Timestamp: now,
	}

	if detected {
		result.ThreatLevel = ThreatLevelCritical
		// 检查是否尝试多个用户名
		uniqueUsers := uniqueStrings(usernames)
		if len(uniqueUsers) > 3 {
			result.Details = "检测到字典暴力破解攻击"
			result.Confidence = 0.95
		} else {
			result.Details = "检测到暴力破解攻击"
			result.Confidence = minFloat64(float64(failCount)/float64(threshold)*0.6, 1.0)
		}
	}

	return result
}

// DetectSuspiciousUserAgent 检测可疑 User-Agent
func (d *Detector) DetectSuspiciousUserAgent(ip, userAgent string) *DetectionResult {
	result := &DetectionResult{
		Detected:  false,
		Type:      DetectionSuspiciousUA,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if userAgent == "" {
		result.Detected = true
		result.ThreatLevel = ThreatLevelLow
		result.Details = "空 User-Agent"
		result.Confidence = 0.3
		return result
	}

	suspiciousPatterns := []string{
		"sqlmap", "nikto", "nmap", "masscan", "zgrab",
		"dirbuster", "gobuster", "wfuzz", "hydra",
		"metasploit", "burp", "scanner", "exploit",
	}

	ua := strings.ToLower(userAgent)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(ua, pattern) {
			result.Detected = true
			result.ThreatLevel = ThreatLevelHigh
			result.Details = "检测到攻击工具 User-Agent: " + pattern
			result.Confidence = 0.9
			return result
		}
	}

	return result
}

// DetectBotPattern 检测爬虫/机器人模式
func (d *Detector) DetectBotPattern(ip, userAgent string) *DetectionResult {
	result := &DetectionResult{
		Detected:  false,
		Type:      DetectionBotPattern,
		IP:        ip,
		Timestamp: time.Now(),
	}

	botPatterns := []string{
		"bot", "crawler", "spider", "scraper",
		"python-requests", "curl/", "wget/",
		"go-http-client", "java/", "perl",
	}

	ua := strings.ToLower(userAgent)
	for _, pattern := range botPatterns {
		if strings.Contains(ua, pattern) {
			result.Detected = true
			result.ThreatLevel = ThreatLevelLow
			result.Details = "检测到机器人: " + pattern
			result.Confidence = 0.6
			return result
		}
	}

	return result
}

// GetRecentLoginFailures 获取 IP 最近的登录失败次数
func (d *Detector) GetRecentLoginFailures(ip string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	attempts := d.loginAttempts[ip]
	window := d.config.LoginFailureWindow
	now := time.Now()

	count := 0
	for i := len(attempts) - 1; i >= 0; i-- {
		if now.Sub(attempts[i].Timestamp) > window {
			break
		}
		if !attempts[i].Success {
			count++
		}
	}
	return count
}

// GetRecentPortCount 获取 IP 最近扫描的端口数
func (d *Detector) GetRecentPortCount(ip string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ports := d.portAccess[ip]
	if ports == nil {
		return 0
	}

	window := d.config.PortScanWindow
	now := time.Now()

	count := 0
	for _, firstSeen := range ports {
		if now.Sub(firstSeen) <= window {
			count++
		}
	}
	return count
}

// cleanLoginAttempts 清理过期的登录记录
func (d *Detector) cleanLoginAttempts(ip string) {
	attempts := d.loginAttempts[ip]
	if len(attempts) == 0 {
		return
	}

	// 保留最近窗口时间 2 倍的记录，使用较大的窗口
	window := d.config.LoginFailureWindow
	if d.config.BruteForceWindow > window {
		window = d.config.BruteForceWindow
	}
	if window <= 0 {
		return // 窗口为 0 时不清理
	}
	cutoff := time.Now().Add(-window * 2)
	valid := make([]*LoginAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a.Timestamp.After(cutoff) {
			valid = append(valid, a)
		}
	}

	if len(valid) == 0 {
		delete(d.loginAttempts, ip)
	} else {
		d.loginAttempts[ip] = valid
	}
}

// cleanAccessRecords 清理过期的访问记录
func (d *Detector) cleanAccessRecords(ip string) {
	records := d.accessRecords[ip]
	if len(records) == 0 {
		return
	}

	window := d.config.PortScanWindow
	if window <= 0 {
		return
	}
	cutoff := time.Now().Add(-window * 2)
	valid := make([]*AccessRecord, 0, len(records))
	for _, r := range records {
		if r.Timestamp.After(cutoff) {
			valid = append(valid, r)
		}
	}

	if len(valid) == 0 {
		delete(d.accessRecords, ip)
		// 同时清理端口记录
		delete(d.portAccess, ip)
	} else {
		d.accessRecords[ip] = valid
		// 清理过期端口
		if ports, exists := d.portAccess[ip]; exists {
			for port, firstSeen := range ports {
				if firstSeen.Before(cutoff) {
					delete(ports, port)
				}
			}
			if len(ports) == 0 {
				delete(d.portAccess, ip)
			}
		}
	}
}

// uniqueStrings 去重
func uniqueStrings(s []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// minFloat64 返回两个 float64 的较小值
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
