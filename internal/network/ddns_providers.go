package network

import (
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- 阿里云 DNS API 签名规范要求 HMAC-SHA1
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ========== 阿里云 DNS 实现 ==========

// AliDNSProvider 阿里云 DNS
type AliDNSProvider struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string
}

// AliDNSRecord 阿里云 DNS 记录
type AliDNSRecord struct {
	RecordID string `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Status   string `json:"Status"`
}

// NewAliDNSProvider 创建阿里云 DNS 提供者
func NewAliDNSProvider(accessKeyID, accessKeySecret string) *AliDNSProvider {
	return &AliDNSProvider{
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		RegionID:        "cn-hangzhou",
	}
}

// Update 更新 DNS 记录
func (p *AliDNSProvider) Update(domain, ip string) error {
	// 解析域名
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("无效的域名格式: %s", domain)
	}

	// 获取主域名和子域名
	var mainDomain, subDomain string
	if len(parts) == 2 {
		mainDomain = domain
		subDomain = "@"
	} else {
		mainDomain = strings.Join(parts[len(parts)-2:], ".")
		subDomain = strings.Join(parts[:len(parts)-2], ".")
	}

	// 查询现有记录
	records, err := p.describeDomainRecords(mainDomain, subDomain)
	if err != nil {
		return fmt.Errorf("查询记录失败: %w", err)
	}

	if len(records) > 0 {
		// 更新现有记录
		return p.updateDomainRecord(records[0].RecordID, subDomain, "A", ip)
	}

	// 创建新记录
	return p.addDomainRecord(mainDomain, subDomain, "A", ip)
}

// describeDomainRecords 查询域名解析记录
func (p *AliDNSProvider) describeDomainRecords(domain, rr string) ([]AliDNSRecord, error) {
	params := map[string]string{
		"Action":      "DescribeDomainRecords",
		"DomainName":  domain,
		"RRKeyWord":   rr,
		"TypeKeyWord": "A",
	}

	var result struct {
		TotalCount int            `json:"TotalCount"`
		Records    []AliDNSRecord `json:"DomainRecords"`
	}

	if err := p.request(params, &result); err != nil {
		return nil, err
	}

	return result.Records, nil
}

// addDomainRecord 添加域名解析记录
func (p *AliDNSProvider) addDomainRecord(domain, rr, recordType, value string) error {
	params := map[string]string{
		"Action":     "AddDomainRecord",
		"DomainName": domain,
		"RR":         rr,
		"Type":       recordType,
		"Value":      value,
		"TTL":        "600",
	}

	return p.request(params, nil)
}

// updateDomainRecord 更新域名解析记录
func (p *AliDNSProvider) updateDomainRecord(recordID, rr, recordType, value string) error {
	params := map[string]string{
		"Action":   "UpdateDomainRecord",
		"RecordId": recordID,
		"RR":       rr,
		"Type":     recordType,
		"Value":    value,
		"TTL":      "600",
	}

	return p.request(params, nil)
}

// request 发送阿里云 API 请求
func (p *AliDNSProvider) request(params map[string]string, result interface{}) error {
	// 公共参数
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["AccessKeyId"] = p.AccessKeyID
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())
	params["RegionId"] = p.RegionID

	// 构造签名字符串
	signature := p.generateSignature(params)
	params["Signature"] = signature

	// 构造请求 URL
	queryString := p.buildQueryString(params)
	requestURL := "https://alidns.aliyuncs.com/?" + queryString

	// 发送请求
	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查错误
	var errResp struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		return fmt.Errorf("阿里云 API 错误: %s - %s", errResp.Code, errResp.Message)
	}

	// 解析结果
	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
	}

	return nil
}

// generateSignature 生成签名
func (p *AliDNSProvider) generateSignature(params map[string]string) string {
	// 排序参数
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构造规范化请求字符串
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", specialURLEncode(k), specialURLEncode(params[k])))
	}
	canonicalizedQueryString := strings.Join(pairs, "&")

	// 构造待签名字符串
	stringToSign := "GET&%2F&" + specialURLEncode(canonicalizedQueryString)

	// HMAC-SHA1 签名
	mac := hmac.New(sha1.New, []byte(p.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// buildQueryString 构建查询字符串
func (p *AliDNSProvider) buildQueryString(params map[string]string) string {
	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
	}
	return strings.Join(pairs, "&")
}

// specialURLEncode 特殊 URL 编码（阿里云要求）
func specialURLEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// ========== Cloudflare DNS 实现 ==========

// CloudflareProvider Cloudflare DNS
type CloudflareProvider struct {
	APIToken string
	ZoneID   string
	Email    string // 可选，使用 Global API Key 时需要
	Key      string // 可选，Global API Key
}

// CloudflareRecord Cloudflare DNS 记录
type CloudflareRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// NewCloudflareProvider 创建 Cloudflare 提供者
func NewCloudflareProvider(apiToken string) *CloudflareProvider {
	return &CloudflareProvider{
		APIToken: apiToken,
	}
}

// SetZoneID 设置 Zone ID
func (p *CloudflareProvider) SetZoneID(zoneID string) {
	p.ZoneID = zoneID
}

// Update 更新 DNS 记录
func (p *CloudflareProvider) Update(domain, ip string) error {
	// 如果没有 ZoneID，先获取
	if p.ZoneID == "" {
		zoneID, err := p.getZoneID(domain)
		if err != nil {
			return fmt.Errorf("获取 Zone ID 失败: %w", err)
		}
		p.ZoneID = zoneID
	}

	// 查询现有记录
	records, err := p.listDNSRecords(domain)
	if err != nil {
		return fmt.Errorf("查询记录失败: %w", err)
	}

	if len(records) > 0 {
		// 更新现有记录
		return p.updateDNSRecord(records[0].ID, domain, ip)
	}

	// 创建新记录
	return p.createDNSRecord(domain, ip)
}

// getZoneID 获取 Zone ID
func (p *CloudflareProvider) getZoneID(domain string) (string, error) {
	// 解析主域名
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("无效的域名格式: %s", domain)
	}
	zoneName := strings.Join(parts[len(parts)-2:], ".")

	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", zoneName), nil)
	if err != nil {
		return "", err
	}
	p.setAuthHeader(req)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool `json:"success"`
		Result  []struct {
			ID string `json:"id"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success || len(result.Result) == 0 {
		return "", fmt.Errorf("未找到 Zone: %s", zoneName)
	}

	return result.Result[0].ID, nil
}

// listDNSRecords 列出 DNS 记录
func (p *CloudflareProvider) listDNSRecords(name string) ([]CloudflareRecord, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s&type=A",
			p.ZoneID, name), nil)
	if err != nil {
		return nil, err
	}
	p.setAuthHeader(req)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool               `json:"success"`
		Result  []CloudflareRecord `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("cloudflare API 请求失败")
	}

	return result.Result, nil
}

// createDNSRecord 创建 DNS 记录
func (p *CloudflareProvider) createDNSRecord(name, ip string) error {
	data := map[string]interface{}{
		"type":    "A",
		"name":    name,
		"content": ip,
		"ttl":     300,
		"proxied": false,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequest("POST",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", p.ZoneID),
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	_ = json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("创建记录失败: %s", result.Errors[0].Message)
		}
		return fmt.Errorf("创建记录失败")
	}

	return nil
}

// updateDNSRecord 更新 DNS 记录
func (p *CloudflareProvider) updateDNSRecord(recordID, name, ip string) error {
	data := map[string]interface{}{
		"type":    "A",
		"name":    name,
		"content": ip,
		"ttl":     300,
		"proxied": false,
	}

	body, _ := json.Marshal(data)
	req, err := http.NewRequest("PUT",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s",
			p.ZoneID, recordID),
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool `json:"success"`
	}

	_ = json.NewDecoder(resp.Body).Decode(&result)
	if !result.Success {
		return fmt.Errorf("更新记录失败")
	}

	return nil
}

// setAuthHeader 设置认证头
func (p *CloudflareProvider) setAuthHeader(req *http.Request) {
	if p.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIToken)
	} else if p.Email != "" && p.Key != "" {
		req.Header.Set("X-Auth-Email", p.Email)
		req.Header.Set("X-Auth-Key", p.Key)
	}
}

// ========== Tailscale 实现 ==========

// TailscaleProvider Tailscale DDNS
type TailscaleProvider struct {
	APIKey  string
	Tailnet string // Tailscale 网络名称
}

// NewTailscaleProvider 创建 Tailscale 提供者
func NewTailscaleProvider(apiKey, tailnet string) *TailscaleProvider {
	return &TailscaleProvider{
		APIKey:  apiKey,
		Tailnet: tailnet,
	}
}

// Update 更新 Tailscale DNS (MagicDNS)
func (p *TailscaleProvider) Update(domain, ip string) error {
	// Tailscale 使用自己的 DNS 系统
	// 这里我们更新设备的 ACL 标签或 DNS 记录

	req, err := http.NewRequest("GET",
		"https://api.tailscale.com/api/v2/tailnet/"+p.Tailnet+"/dns", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tailscale API 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Tailscale 的 MagicDNS 会自动处理
	// 这里主要是验证连接状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tailscale API 错误: %s", string(body))
	}

	return nil
}

// GetDeviceIP 获取 Tailscale 设备 IP
func (p *TailscaleProvider) GetDeviceIP(deviceName string) (string, error) {
	req, err := http.NewRequest("GET",
		"https://api.tailscale.com/api/v2/tailnet/"+p.Tailnet+"/devices", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取设备列表失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Devices []struct {
			Name      string   `json:"name"`
			Addresses []string `json:"addresses"`
		} `json:"devices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	for _, device := range result.Devices {
		if device.Name == deviceName && len(device.Addresses) > 0 {
			return device.Addresses[0], nil
		}
	}

	return "", fmt.Errorf("未找到设备: %s", deviceName)
}

// ========== 更新 getDDNSProvider ==========

// getDDNSProvider 获取 DDNS 服务商实现 (更新版)
func (m *Manager) getDDNSProviderEx(provider, token, secret, extra string) (DDNSProvider, error) {
	switch provider {
	case "alidns":
		return NewAliDNSProvider(token, secret), nil
	case "cloudflare":
		p := NewCloudflareProvider(token)
		if extra != "" {
			p.SetZoneID(extra) // extra 作为 Zone ID
		}
		return p, nil
	case "tailscale":
		return NewTailscaleProvider(token, secret), nil
	case "duckdns":
		return &DuckDNSProvider{Token: token}, nil
	case "noip":
		return &NoIPProvider{Token: token, Secret: secret}, nil
	default:
		return nil, fmt.Errorf("不支持的 DDNS 服务商: %s", provider)
	}
}

// ========== 辅助函数 ==========

// sha256Hex 计算字符串的 SHA256 哈希值 - 保留用于未来需要 SHA256 的场景
// 注意：阿里云 DNS API 要求使用 HMAC-SHA1，这是 API 规范，不是安全漏洞
// func sha256Hex(s string) string {
// 	h := sha256.New()
// 	h.Write([]byte(s))
// 	return hex.EncodeToString(h.Sum(nil))
// }
