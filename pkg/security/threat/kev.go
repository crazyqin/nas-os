// Package threat 实现群晖DSM 7.3风格的威胁优先级系统
// 包含 KEV/EPSS/LEV 计算和漏洞优先级排序
package threat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== KEV (Known Exploited Vulnerabilities) 数据库 ==========

// KEVCatalog CISA KEV 目录结构
type KEVCatalog struct {
	Title           string     `json:"title"`
	CatalogVersion  string     `json:"catalogVersion"`
	DateReleased    string     `json:"dateReleased"`
	Count           int        `json:"count"`
	Vulnerabilities []KEVEntry `json:"vulnerabilities"`
}

// KEVEntry KEV 漏洞条目
type KEVEntry struct {
	CVEID                      string `json:"cveID"`
	VendorProject              string `json:"vendorProject"`
	Product                    string `json:"product"`
	VulnerabilityName          string `json:"vulnerabilityName"`
	DateAdded                  string `json:"dateAdded"`
	ShortDescription           string `json:"shortDescription"`
	RequiredAction             string `json:"requiredAction"`
	DueDate                    string `json:"dueDate"`
	KnownRansomwareCampaignUse string `json:"knownRansomwareCampaignUse"`
	Notes                      string `json:"notes"`
}

// KEVDatabase KEV 数据库管理器
type KEVDatabase struct {
	catalog      *KEVCatalog
	cachePath    string
	sourceURL    string
	httpClient   *http.Client
	lastSync     time.Time
	syncInterval time.Duration
	mu           sync.RWMutex
}

// KEVConfig KEV 配置
type KEVConfig struct {
	SourceURL    string        `json:"source_url"`
	CachePath    string        `json:"cache_path"`
	SyncInterval time.Duration `json:"sync_interval"`
	OfflineMode  bool          `json:"offline_mode"`
}

// DefaultKEVConfig 默认 KEV 配置
func DefaultKEVConfig() KEVConfig {
	return KEVConfig{
		SourceURL:    "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json",
		CachePath:    "/var/lib/nas-os/threat/kev_catalog.json",
		SyncInterval: 24 * time.Hour,
		OfflineMode:  false,
	}
}

// NewKEVDatabase 创建 KEV 数据库
func NewKEVDatabase(config KEVConfig) *KEVDatabase {
	// 确保缓存目录存在
	cacheDir := filepath.Dir(config.CachePath)
	_ = os.MkdirAll(cacheDir, 0750)

	kdb := &KEVDatabase{
		catalog:      &KEVCatalog{},
		cachePath:    config.CachePath,
		sourceURL:    config.SourceURL,
		syncInterval: config.SyncInterval,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 尝试加载本地缓存
	_ = kdb.loadCache()

	return kdb
}

// loadCache 加载本地缓存
func (kdb *KEVDatabase) loadCache() error {
	data, err := os.ReadFile(kdb.cachePath)
	if err != nil {
		return err
	}

	var catalog KEVCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return err
	}

	kdb.mu.Lock()
	kdb.catalog = &catalog
	kdb.mu.Unlock()

	return nil
}

// saveCache 保存缓存
func (kdb *KEVDatabase) saveCache() error {
	kdb.mu.RLock()
	data, err := json.MarshalIndent(kdb.catalog, "", "  ")
	kdb.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(kdb.cachePath, data, 0600)
}

// Sync 同步 KEV 目录
func (kdb *KEVDatabase) Sync(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kdb.sourceURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "NAS-OS-ThreatSystem/1.0")

	resp, err := kdb.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var catalog KEVCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w", err)
	}

	kdb.mu.Lock()
	kdb.catalog = &catalog
	kdb.lastSync = time.Now()
	kdb.mu.Unlock()

	// 保存到本地缓存
	return kdb.saveCache()
}

// IsInKEV 检查 CVE 是否在 KEV 目录中
func (kdb *KEVDatabase) IsInKEV(cveID string) bool {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	for _, entry := range kdb.catalog.Vulnerabilities {
		if entry.CVEID == cveID {
			return true
		}
	}
	return false
}

// GetKEVEntry 获取 KEV 条目
func (kdb *KEVDatabase) GetKEVEntry(cveID string) *KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	for _, entry := range kdb.catalog.Vulnerabilities {
		if entry.CVEID == cveID {
			return &entry
		}
	}
	return nil
}

// GetAllEntries 获取所有 KEV 条目
func (kdb *KEVDatabase) GetAllEntries() []KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	entries := make([]KEVEntry, len(kdb.catalog.Vulnerabilities))
	copy(entries, kdb.catalog.Vulnerabilities)
	return entries
}

// GetRansomwareRelated 获取与勒索软件相关的漏洞
func (kdb *KEVDatabase) GetRansomwareRelated() []KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var entries []KEVEntry
	for _, entry := range kdb.catalog.Vulnerabilities {
		if entry.KnownRansomwareCampaignUse == "Known" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetOverdue 获取已过修复期限的漏洞
func (kdb *KEVDatabase) GetOverdue() []KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var entries []KEVEntry
	now := time.Now()

	for _, entry := range kdb.catalog.Vulnerabilities {
		if entry.DueDate != "" {
			dueDate, err := time.Parse("2006-01-02", entry.DueDate)
			if err == nil && dueDate.Before(now) {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// Search 搜索 KEV 目录
func (kdb *KEVDatabase) Search(query string) []KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var entries []KEVEntry
	queryLower := strings.ToLower(query)

	for _, entry := range kdb.catalog.Vulnerabilities {
		if strings.Contains(strings.ToLower(entry.CVEID), queryLower) ||
			strings.Contains(strings.ToLower(entry.VendorProject), queryLower) ||
			strings.Contains(strings.ToLower(entry.Product), queryLower) ||
			strings.Contains(strings.ToLower(entry.VulnerabilityName), queryLower) {
			entries = append(entries, entry)
		}
	}
	return entries
}

// FilterByVendor 按厂商过滤
func (kdb *KEVDatabase) FilterByVendor(vendor string) []KEVEntry {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var entries []KEVEntry
	vendorLower := strings.ToLower(vendor)

	for _, entry := range kdb.catalog.Vulnerabilities {
		if strings.Contains(strings.ToLower(entry.VendorProject), vendorLower) {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetCatalogInfo 获取目录信息
func (kdb *KEVDatabase) GetCatalogInfo() map[string]interface{} {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	return map[string]interface{}{
		"version":       kdb.catalog.CatalogVersion,
		"date_released": kdb.catalog.DateReleased,
		"total_count":   kdb.catalog.Count,
		"last_sync":     kdb.lastSync,
	}
}
