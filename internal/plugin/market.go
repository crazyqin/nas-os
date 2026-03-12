package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Market 插件市场客户端
type Market struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cache      map[string]*cachedPlugin
	cacheMu    sync.RWMutex
}

// MarketConfig 市场配置
type MarketConfig struct {
	BaseURL string // 市场服务器地址
	APIKey  string // API 密钥
}

// NewMarket 创建市场客户端
func NewMarket(cfg MarketConfig) *Market {
	return &Market{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: make(map[string]*cachedPlugin),
	}
}

// cachedPlugin 缓存的插件信息
type cachedPlugin struct {
	info      *PluginMarketInfo
	expiresAt time.Time
}

// PluginMarketInfo 市场插件信息
type PluginMarketInfo struct {
	// 基本信息
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	AuthorID    string   `json:"authorId"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	Tags        []string `json:"tags"`

	// 统计信息
	Downloads   int     `json:"downloads"`
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"ratingCount"`
	Reviews     int     `json:"reviews"`

	// 市场信息
	Price      string `json:"price"` // "free" 或价格
	Homepage   string `json:"homepage"`
	Repository string `json:"repository"`
	License    string `json:"license"`

	// UI
	Icon        string   `json:"icon"`
	Screenshots []string `json:"screenshots"`

	// 版本信息
	Changelog   string    `json:"changelog"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// 下载信息
	DownloadURL string `json:"downloadUrl"`
	Checksum    string `json:"checksum"`
	Size        int64  `json:"size"`

	// 兼容性
	NASVersion string   `json:"nasVersion"`
	GoVersion  string   `json:"goVersion"`
	Arch       []string `json:"arch"` // 支持的架构

	// 是否已安装（客户端填充）
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
}

// Review 插件评论
type Review struct {
	ID        string    `json:"id"`
	PluginID  string    `json:"pluginId"`
	UserID    string    `json:"userId"`
	UserName  string    `json:"userName"`
	Rating    int       `json:"rating"`
	Review    string    `json:"review"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Helpful   int       `json:"helpful"` // 有帮助数
}

// List 获取插件列表
func (m *Market) List(category, sort string, page, pageSize int) ([]PluginMarketInfo, int, error) {
	if m.baseURL == "" {
		// 返回模拟数据
		return m.getMockPlugins(category), 0, nil
	}

	u, err := url.Parse(m.baseURL + "/plugins")
	if err != nil {
		return nil, 0, err
	}

	q := u.Query()
	if category != "" {
		q.Set("category", category)
	}
	q.Set("sort", sort)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	u.RawQuery = q.Encode()

	var resp struct {
		Code    int                `json:"code"`
		Data    []PluginMarketInfo `json:"data"`
		Total   int                `json:"total"`
		Message string             `json:"message"`
	}

	if err := m.request("GET", u.String(), nil, &resp); err != nil {
		return nil, 0, err
	}

	return resp.Data, resp.Total, nil
}

// Search 搜索插件
func (m *Market) Search(query string, page, pageSize int) ([]PluginMarketInfo, int, error) {
	if m.baseURL == "" {
		return m.searchMockPlugins(query), 0, nil
	}

	u, err := url.Parse(m.baseURL + "/plugins/search")
	if err != nil {
		return nil, 0, err
	}

	q := u.Query()
	q.Set("q", query)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	u.RawQuery = q.Encode()

	var resp struct {
		Code    int                `json:"code"`
		Data    []PluginMarketInfo `json:"data"`
		Total   int                `json:"total"`
		Message string             `json:"message"`
	}

	if err := m.request("GET", u.String(), nil, &resp); err != nil {
		return nil, 0, err
	}

	return resp.Data, resp.Total, nil
}

// GetDetail 获取插件详情
func (m *Market) GetDetail(pluginID string) (*PluginMarketInfo, error) {
	// 检查缓存
	m.cacheMu.RLock()
	if cached, ok := m.cache[pluginID]; ok && cached.expiresAt.After(time.Now()) {
		m.cacheMu.RUnlock()
		return cached.info, nil
	}
	m.cacheMu.RUnlock()

	if m.baseURL == "" {
		return m.getMockPluginDetail(pluginID)
	}

	var resp struct {
		Code    int              `json:"code"`
		Data    PluginMarketInfo `json:"data"`
		Message string           `json:"message"`
	}

	if err := m.request("GET", m.baseURL+"/plugins/"+pluginID, nil, &resp); err != nil {
		return nil, err
	}

	// 缓存结果
	m.cacheMu.Lock()
	m.cache[pluginID] = &cachedPlugin{
		info:      &resp.Data,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	m.cacheMu.Unlock()

	return &resp.Data, nil
}

// Rate 提交评分
func (m *Market) Rate(pluginID, userID string, rating int, review string) error {
	if m.baseURL == "" {
		return nil // 模拟模式下直接返回成功
	}

	data := map[string]interface{}{
		"rating": rating,
		"review": review,
		"userId": userID,
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	return m.request("POST", m.baseURL+"/plugins/"+pluginID+"/rate", data, &resp)
}

// GetReviews 获取评论列表
func (m *Market) GetReviews(pluginID string, page, pageSize int) ([]Review, int, error) {
	if m.baseURL == "" {
		return []Review{}, 0, nil
	}

	u, err := url.Parse(m.baseURL + "/plugins/" + pluginID + "/reviews")
	if err != nil {
		return nil, 0, err
	}

	q := u.Query()
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	u.RawQuery = q.Encode()

	var resp struct {
		Code    int      `json:"code"`
		Data    []Review `json:"data"`
		Total   int      `json:"total"`
		Message string   `json:"message"`
	}

	if err := m.request("GET", u.String(), nil, &resp); err != nil {
		return nil, 0, err
	}

	return resp.Data, resp.Total, nil
}

// Download 下载插件
func (m *Market) Download(pluginID, version string) (string, error) {
	if m.baseURL == "" {
		return "", fmt.Errorf("未配置插件市场")
	}

	downloadURL := m.baseURL + "/plugins/" + pluginID + "/download"
	if version != "" {
		downloadURL += "?version=" + version
	}

	return downloadURL, nil
}

// request 发送 HTTP 请求
func (m *Market) request(method, url string, data interface{}, result interface{}) error {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}

	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

// ========== 模拟数据 ==========

func (m *Market) getMockPlugins(category string) []PluginMarketInfo {
	plugins := []PluginMarketInfo{
		{
			ID:          "com.nas-os.filemanager-enhance",
			Name:        "文件管理器增强",
			Version:     "1.0.0",
			Author:      "NAS-OS Team",
			Description: "增强文件管理器功能，支持批量操作、快捷键、文件预览等",
			Category:    CategoryFileManager,
			Downloads:   1523,
			Rating:      4.8,
			RatingCount: 45,
			Price:       "free",
			Icon:        "folder-open",
			Tags:        []string{"文件管理", "批量操作", "预览"},
		},
		{
			ID:          "com.nas-os.dark-theme",
			Name:        "暗黑主题",
			Version:     "1.2.0",
			Author:      "NAS-OS Team",
			Description: "暗黑主题皮肤，保护眼睛，适合夜间使用",
			Category:    CategoryTheme,
			Downloads:   3421,
			Rating:      4.9,
			RatingCount: 128,
			Price:       "free",
			Icon:        "moon",
			Tags:        []string{"主题", "暗黑", "护眼"},
		},
		{
			ID:          "com.nas-os.photo-organizer",
			Name:        "照片整理器",
			Version:     "0.9.0",
			Author:      "Community",
			Description: "自动整理照片，按日期/地点分类，支持 AI 识别",
			Category:    CategoryMedia,
			Downloads:   892,
			Rating:      4.5,
			RatingCount: 23,
			Price:       "free",
			Icon:        "image",
			Tags:        []string{"照片", "AI", "整理"},
		},
		{
			ID:          "com.nas-os.cloud-sync",
			Name:        "云同步助手",
			Version:     "2.1.0",
			Author:      "NAS-OS Team",
			Description: "支持 Dropbox、Google Drive、OneDrive 等云盘同步",
			Category:    CategoryBackup,
			Downloads:   2156,
			Rating:      4.7,
			RatingCount: 67,
			Price:       "free",
			Icon:        "cloud",
			Tags:        []string{"云同步", "备份", "Dropbox"},
		},
		{
			ID:          "com.nas-os.network-monitor",
			Name:        "网络监控",
			Version:     "1.0.0",
			Author:      "Community",
			Description: "实时监控网络流量、带宽使用、连接状态",
			Category:    CategoryNetwork,
			Downloads:   654,
			Rating:      4.3,
			RatingCount: 18,
			Price:       "free",
			Icon:        "activity",
			Tags:        []string{"网络", "监控", "流量"},
		},
	}

	if category != "" {
		var filtered []PluginMarketInfo
		for _, p := range plugins {
			if string(p.Category) == category {
				filtered = append(filtered, p)
			}
		}
		return filtered
	}

	return plugins
}

func (m *Market) searchMockPlugins(query string) []PluginMarketInfo {
	plugins := m.getMockPlugins("")
	var results []PluginMarketInfo

	for _, p := range plugins {
		if contains(p.Name, query) || contains(p.Description, query) {
			results = append(results, p)
		}
	}

	return results
}

func (m *Market) getMockPluginDetail(pluginID string) (*PluginMarketInfo, error) {
	plugins := m.getMockPlugins("")
	for i := range plugins {
		if plugins[i].ID == pluginID {
			plugins[i].PublishedAt = time.Now().Add(-30 * 24 * time.Hour)
			plugins[i].UpdatedAt = time.Now().Add(-2 * 24 * time.Hour)
			plugins[i].Arch = []string{"amd64", "arm64"}
			plugins[i].NASVersion = ">=0.2.0"
			plugins[i].Size = 2048576
			return &plugins[i], nil
		}
	}
	return nil, fmt.Errorf("插件不存在")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && containsIgnoreCase(s, substr)))
}

func containsIgnoreCase(s, substr string) bool {
	// 简单实现
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
