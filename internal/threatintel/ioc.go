// Package threatintel - ioc.go 实现 IOC（威胁指标）管理功能，包括 IP/域名/文件哈希/URL
// 威胁指标的匹配与阻断。
package threatintel

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// IOC 验证器
// ============================================================

// IOCValidator IOC 验证器
type IOCValidator struct {
	mu sync.RWMutex
}

// NewIOCValidator 创建 IOC 验证器
func NewIOCValidator() *IOCValidator {
	return &IOCValidator{}
}

// ValidateIP 验证 IP 地址格式
func (v *IOCValidator) ValidateIP(value string) bool {
	return net.ParseIP(value) != nil
}

// ValidateCIDR 验证 CIDR 格式
func (v *IOCValidator) ValidateCIDR(value string) bool {
	_, _, err := net.ParseCIDR(value)
	return err == nil
}

// ValidateDomain 验证域名格式
func (v *IOCValidator) ValidateDomain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(value)
}

// ValidateURL 验证 URL 格式
func (v *IOCValidator) ValidateURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// ValidateFileHash 验证文件哈希格式（MD5/SHA1/SHA256）
func (v *IOCValidator) ValidateFileHash(value string) bool {
	value = strings.ToLower(value)
	md5Regex := regexp.MustCompile(`^[a-f0-9]{32}$`)
	sha1Regex := regexp.MustCompile(`^[a-f0-9]{40}$`)
	sha256Regex := regexp.MustCompile(`^[a-f0-9]{64}$`)
	return md5Regex.MatchString(value) || sha1Regex.MatchString(value) || sha256Regex.MatchString(value)
}

// ValidateEmail 验证邮箱格式
func (v *IOCValidator) ValidateEmail(value string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(value)
}

// ValidateIOC 验证 IOC 是否合法
func (v *IOCValidator) ValidateIOC(ioc *IOC) error {
	switch ioc.Type {
	case IOCTypeIP:
		if !v.ValidateIP(ioc.Value) {
			return NewThreatIntelError("INVALID_IP", fmt.Sprintf("无效的 IP 地址: %s", ioc.Value), nil)
		}
	case IOCTypeDomain:
		if !v.ValidateDomain(ioc.Value) {
			return NewThreatIntelError("INVALID_DOMAIN", fmt.Sprintf("无效的域名: %s", ioc.Value), nil)
		}
	case IOCTypeURL:
		if !v.ValidateURL(ioc.Value) {
			return NewThreatIntelError("INVALID_URL", fmt.Sprintf("无效的 URL: %s", ioc.Value), nil)
		}
	case IOCTypeFileHash:
		if !v.ValidateFileHash(ioc.Value) {
			return NewThreatIntelError("INVALID_HASH", fmt.Sprintf("无效的文件哈希: %s", ioc.Value), nil)
		}
	case IOCTypeEmail:
		if !v.ValidateEmail(ioc.Value) {
			return NewThreatIntelError("INVALID_EMAIL", fmt.Sprintf("无效的邮箱: %s", ioc.Value), nil)
		}
	case IOCTypeCIDR:
		if !v.ValidateCIDR(ioc.Value) {
			return NewThreatIntelError("INVALID_CIDR", fmt.Sprintf("无效的 CIDR: %s", ioc.Value), nil)
		}
	default:
		return NewThreatIntelError("INVALID_IOC_TYPE", fmt.Sprintf("不支持的 IOC 类型: %s", ioc.Type), nil)
	}

	return nil
}

// ============================================================
// IOC 匹配器
// ============================================================

// IOCMatcher IOC 匹配器
type IOCMatcher struct {
	engine    *Engine
	validator *IOCValidator
	mu        sync.RWMutex
}

// NewIOCMatcher 创建 IOC 匹配器
func NewIOCMatcher(engine *Engine) *IOCMatcher {
	return &IOCMatcher{
		engine:    engine,
		validator: NewIOCValidator(),
	}
}

// MatchIP 匹配 IP 地址
func (m *IOCMatcher) MatchIP(ip string) []*IOC {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*IOC
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return matches
	}

	for _, ioc := range m.engine.iocs {
		switch ioc.Type {
		case IOCTypeIP:
			if ioc.Value == ip {
				matches = append(matches, ioc)
			}
		case IOCTypeCIDR:
			_, cidr, err := net.ParseCIDR(ioc.Value)
			if err == nil && cidr.Contains(parsedIP) {
				matches = append(matches, ioc)
			}
		}
	}

	return matches
}

// MatchDomain 匹配域名
func (m *IOCMatcher) MatchDomain(domain string) []*IOC {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*IOC
	domain = strings.ToLower(domain)

	for _, ioc := range m.engine.iocs {
		if ioc.Type == IOCTypeDomain {
			iocDomain := strings.ToLower(ioc.Value)
			if domain == iocDomain || strings.HasSuffix(domain, "."+iocDomain) {
				matches = append(matches, ioc)
			}
		}
	}

	return matches
}

// MatchFileHash 匹配文件哈希
func (m *IOCMatcher) MatchFileHash(hash string) []*IOC {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*IOC
	hash = strings.ToLower(hash)

	for _, ioc := range m.engine.iocs {
		if ioc.Type == IOCTypeFileHash && strings.ToLower(ioc.Value) == hash {
			matches = append(matches, ioc)
		}
	}

	return matches
}

// MatchURL 匹配 URL
func (m *IOCMatcher) MatchURL(url string) []*IOC {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*IOC
	url = strings.ToLower(url)

	for _, ioc := range m.engine.iocs {
		if ioc.Type == IOCTypeURL {
			iocURL := strings.ToLower(ioc.Value)
			if strings.Contains(url, iocURL) {
				matches = append(matches, ioc)
			}
		}
	}

	return matches
}

// MatchAny 自动检测 IOC 类型并匹配
func (m *IOCMatcher) MatchAny(value string) []*IOC {
	if m.validator.ValidateIP(value) {
		return m.MatchIP(value)
	}
	if m.validator.ValidateDomain(value) {
		return m.MatchDomain(value)
	}
	if m.validator.ValidateFileHash(value) {
		return m.MatchFileHash(value)
	}
	if m.validator.ValidateURL(value) {
		return m.MatchURL(value)
	}
	return nil
}

// ============================================================
// IOC 阻断管理
// ============================================================

// BlockManager IOC 阻断管理器
type BlockManager struct {
	engine         *Engine
	blockedIPs     map[string]time.Time
	blockedDomains map[string]time.Time
	mu             sync.RWMutex
}

// NewBlockManager 创建阻断管理器
func NewBlockManager(engine *Engine) *BlockManager {
	return &BlockManager{
		engine:         engine,
		blockedIPs:     make(map[string]time.Time),
		blockedDomains: make(map[string]time.Time),
	}
}

// BlockIP 阻断 IP
func (bm *BlockManager) BlockIP(ip string, duration time.Duration) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if net.ParseIP(ip) == nil {
		return NewThreatIntelError("INVALID_IP", "无效的 IP 地址", nil)
	}

	expiresAt := time.Now().Add(duration)
	bm.blockedIPs[ip] = expiresAt

	// 同步到引擎 IOC
	ioc := bm.engine.LookupIOC(IOCTypeIP, ip)
	if ioc != nil {
		bm.engine.BlockIOC(ioc.ID)
	}

	return nil
}

// UnblockIP 取消阻断 IP
func (bm *BlockManager) UnblockIP(ip string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	delete(bm.blockedIPs, ip)

	ioc := bm.engine.LookupIOC(IOCTypeIP, ip)
	if ioc != nil {
		bm.engine.UnblockIOC(ioc.ID)
	}
}

// IsIPBlocked 检查 IP 是否被阻断
func (bm *BlockManager) IsIPBlocked(ip string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	expiresAt, exists := bm.blockedIPs[ip]
	if !exists {
		return false
	}

	if time.Now().After(expiresAt) {
		delete(bm.blockedIPs, ip)
		return false
	}

	return true
}

// BlockDomain 阻断域名
func (bm *BlockManager) BlockDomain(domain string, duration time.Duration) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	domain = strings.ToLower(domain)
	expiresAt := time.Now().Add(duration)
	bm.blockedDomains[domain] = expiresAt

	ioc := bm.engine.LookupIOC(IOCTypeDomain, domain)
	if ioc != nil {
		bm.engine.BlockIOC(ioc.ID)
	}

	return nil
}

// UnblockDomain 取消阻断域名
func (bm *BlockManager) UnblockDomain(domain string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	domain = strings.ToLower(domain)
	delete(bm.blockedDomains, domain)

	ioc := bm.engine.LookupIOC(IOCTypeDomain, domain)
	if ioc != nil {
		bm.engine.UnblockIOC(ioc.ID)
	}
}

// IsDomainBlocked 检查域名是否被阻断
func (bm *BlockManager) IsDomainBlocked(domain string) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	domain = strings.ToLower(domain)
	expiresAt, exists := bm.blockedDomains[domain]
	if !exists {
		return false
	}

	if time.Now().After(expiresAt) {
		delete(bm.blockedDomains, domain)
		return false
	}

	return true
}

// GetBlockedIPs 获取所有被阻断的 IP
func (bm *BlockManager) GetBlockedIPs() map[string]time.Time {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make(map[string]time.Time)
	now := time.Now()
	for ip, expiresAt := range bm.blockedIPs {
		if now.Before(expiresAt) {
			result[ip] = expiresAt
		}
	}
	return result
}

// GetBlockedDomains 获取所有被阻断的域名
func (bm *BlockManager) GetBlockedDomains() map[string]time.Time {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	result := make(map[string]time.Time)
	now := time.Now()
	for domain, expiresAt := range bm.blockedDomains {
		if now.Before(expiresAt) {
			result[domain] = expiresAt
		}
	}
	return result
}

// CleanupExpired 清理过期的阻断记录
func (bm *BlockManager) CleanupExpired() int {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	cleaned := 0
	now := time.Now()

	for ip, expiresAt := range bm.blockedIPs {
		if now.After(expiresAt) {
			delete(bm.blockedIPs, ip)
			cleaned++
		}
	}

	for domain, expiresAt := range bm.blockedDomains {
		if now.After(expiresAt) {
			delete(bm.blockedDomains, domain)
			cleaned++
		}
	}

	return cleaned
}

// AutoBlockHighThreat 自动阻断高威胁 IOC
func (bm *BlockManager) AutoBlockHighThreat(threshold int) int {
	blocked := 0
	iocs := bm.engine.ListIOCs()

	for _, ioc := range iocs {
		if ioc.ThreatScore >= threshold && !ioc.Blocked {
			switch ioc.Type {
			case IOCTypeIP:
				bm.BlockIP(ioc.Value, 24*time.Hour)
				blocked++
			case IOCTypeDomain:
				bm.BlockDomain(ioc.Value, 24*time.Hour)
				blocked++
			}
		}
	}

	return blocked
}
