package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DDNSProvider DDNS 服务商接口
type DDNSProvider interface {
	Update(domain, ip string) error
}

// ListDDNS 列出所有 DDNS 配置
func (m *Manager) ListDDNS() []*DDNSConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var configs []*DDNSConfig
	for _, cfg := range m.ddnsConfigs {
		configs = append(configs, cfg)
	}
	return configs
}

// GetDDNS 获取单个 DDNS 配置
func (m *Manager) GetDDNS(domain string) (*DDNSConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.ddnsConfigs[domain]
	if !ok {
		return nil, fmt.Errorf("DDNS 配置不存在: %s", domain)
	}
	return cfg, nil
}

// AddDDNS 添加 DDNS 配置
func (m *Manager) AddDDNS(config DDNSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	if config.Provider == "" {
		return fmt.Errorf("服务商不能为空")
	}

	// 设置默认值
	if config.Interval == 0 {
		config.Interval = 300 // 5分钟
	}
	config.Enabled = true
	config.Status = "pending"
	config.LastUpdate = ""

	m.ddnsConfigs[config.Domain] = &config
	m.saveConfig()

	return nil
}

// UpdateDDNS 更新 DDNS 配置
func (m *Manager) UpdateDDNS(domain string, config DDNSConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ddnsConfigs[domain]; !ok {
		return fmt.Errorf("DDNS 配置不存在: %s", domain)
	}

	// 如果域名改变了，需要删除旧的
	if config.Domain != domain {
		delete(m.ddnsConfigs, domain)
	}

	m.ddnsConfigs[config.Domain] = &config
	m.saveConfig()

	return nil
}

// DeleteDDNS 删除 DDNS 配置
func (m *Manager) DeleteDDNS(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.ddnsConfigs[domain]; !ok {
		return fmt.Errorf("DDNS 配置不存在: %s", domain)
	}

	delete(m.ddnsConfigs, domain)
	m.saveConfig()

	return nil
}

// EnableDDNS 启用/禁用 DDNS
func (m *Manager) EnableDDNS(domain string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.ddnsConfigs[domain]
	if !ok {
		return fmt.Errorf("DDNS 配置不存在: %s", domain)
	}

	cfg.Enabled = enabled
	m.saveConfig()

	return nil
}

// RefreshDDNS 手动刷新 DDNS
func (m *Manager) RefreshDDNS(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.ddnsConfigs[domain]
	if !ok {
		return fmt.Errorf("DDNS 配置不存在: %s", domain)
	}

	// 获取当前公网 IP
	ip, err := m.getPublicIP(cfg.Interface)
	if err != nil {
		cfg.Status = "error"
		return fmt.Errorf("获取公网 IP 失败: %w", err)
	}

	// 如果 IP 没有变化，跳过更新
	if cfg.LastIP == ip {
		cfg.Status = "active"
		return nil
	}

	// 调用对应的 DDNS 服务商 API
	provider, err := m.getDDNSProvider(cfg.Provider, cfg.Token, cfg.Secret)
	if err != nil {
		cfg.Status = "error"
		return err
	}

	if err := provider.Update(domain, ip); err != nil {
		cfg.Status = "error"
		return fmt.Errorf("DDNS 更新失败: %w", err)
	}

	cfg.LastIP = ip
	cfg.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
	cfg.Status = "active"
	m.saveConfig()

	return nil
}

// getPublicIP 获取公网 IP
func (m *Manager) getPublicIP(iface string) (string, error) {
	// 使用多个 IP 检测服务，提高可用性
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := strings.TrimSpace(string(data))
		if ip != "" {
			return ip, nil
		}
	}

	return "", fmt.Errorf("无法获取公网 IP")
}

// getDDNSProvider 获取 DDNS 服务商实现
func (m *Manager) getDDNSProvider(provider, token, secret string) (DDNSProvider, error) {
	return m.getDDNSProviderEx(provider, token, secret, "")
}

// ========== DuckDNS 实现 ==========

// DuckDNSProvider DuckDNS
type DuckDNSProvider struct {
	Token string
}

func (p *DuckDNSProvider) Update(domain, ip string) error {
	// DuckDNS API 非常简单
	url := fmt.Sprintf("https://www.duckdns.org/update?domains=%s&token=%s&ip=%s",
		domain, p.Token, ip)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(data) != "OK" {
		return fmt.Errorf("DuckDNS 更新失败: %s", string(data))
	}

	return nil
}

// ========== No-IP 实现 ==========

// NoIPProvider No-IP
type NoIPProvider struct {
	Token  string // 用户名
	Secret string // 密码
}

func (p *NoIPProvider) Update(domain, ip string) error {
	// No-IP 使用 Basic Auth
	url := fmt.Sprintf("https://dynupdate.no-ip.com/nic/update?hostname=%s&myip=%s",
		domain, ip)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(p.Token, p.Secret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 检查响应
	response := string(data)
	if strings.HasPrefix(response, "good") || strings.HasPrefix(response, "nochg") {
		return nil
	}

	return fmt.Errorf("No-IP 更新失败: %s", response)
}

// StartDDNSWorker 启动 DDNS 后台更新任务
func (m *Manager) StartDDNSWorker() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			m.mu.RLock()
			configs := make([]*DDNSConfig, 0, len(m.ddnsConfigs))
			for _, cfg := range m.ddnsConfigs {
				if cfg.Enabled {
					configs = append(configs, cfg)
				}
			}
			m.mu.RUnlock()

			for _, cfg := range configs {
				// 检查是否需要更新
				if time.Since(m.parseTime(cfg.LastUpdate)).Seconds() < float64(cfg.Interval) {
					continue
				}

				m.RefreshDDNS(cfg.Domain)
			}
		}
	}()
}

// parseTime 解析时间字符串
func (m *Manager) parseTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02 15:04:05", timeStr)
	return t
}

// URL encode helper
func urlEncode(s string) string {
	return url.QueryEscape(s)
}

// JSON helper
func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}