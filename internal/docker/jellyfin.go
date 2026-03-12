package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JellyfinConfig Jellyfin 配置
type JellyfinConfig struct {
	ServerURL       string `json:"serverUrl"`
	APIKey          string `json:"apiKey"`
	ConfigPath      string `json:"configPath"`
	EnableChinese   bool   `json:"enableChinese"`
	DoubanEnabled   bool   `json:"doubanEnabled"`
	TMDBEnabled     bool   `json:"tmdbEnabled"`
	ChinesePriority int    `json:"chinesePriority"` // 中文元数据优先级 1-5
}

// JellyfinManager Jellyfin 管理器
type JellyfinManager struct {
	config     *JellyfinConfig
	httpClient *http.Client
}

// NewJellyfinManager 创建 Jellyfin 管理器
func NewJellyfinManager(configPath string) *JellyfinManager {
	config := &JellyfinConfig{
		ServerURL:     "http://localhost:8096",
		ConfigPath:    configPath,
		EnableChinese: true,
		TMDBEnabled:   true,
		DoubanEnabled: false,
	}

	// 加载配置
	if err := config.load(); err != nil {
		fmt.Printf("加载 Jellyfin 配置失败：%v\n", err)
	}

	return &JellyfinManager{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// load 加载配置
func (c *JellyfinConfig) load() error {
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, c)
}

// save 保存配置
func (c *JellyfinConfig) save() error {
	dir := filepath.Dir(c.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.ConfigPath, data, 0644)
}

// SetAPIKey 设置 API 密钥
func (jm *JellyfinManager) SetAPIKey(apiKey string) error {
	jm.config.APIKey = apiKey
	return jm.config.save()
}

// GetSystemInfo 获取系统信息
func (jm *JellyfinManager) GetSystemInfo() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/System/Info/Public", jm.config.ServerURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := jm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	return info, nil
}

// GetLibraries 获取媒体库列表
func (jm *JellyfinManager) GetLibraries() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/Library/VirtualFolders", jm.config.ServerURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if jm.config.APIKey != "" {
		req.Header.Set("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Client="NAS-OS", Device="NAS", DeviceId="NAS-OS", Version="1.0", Token="%s"`, jm.config.APIKey))
	}

	resp, err := jm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var libraries []map[string]interface{}
	if err := json.Unmarshal(body, &libraries); err != nil {
		return nil, err
	}

	return libraries, nil
}

// CreateLibrary 创建媒体库
func (jm *JellyfinManager) CreateLibrary(name, libraryType string, paths []string) error {
	url := fmt.Sprintf("%s/Library/VirtualFolders", jm.config.ServerURL)

	// 构建请求体
	payload := map[string]interface{}{
		"Name":            name,
		"CollectionType":  libraryType, // movies, tvshows, music, photos
		"Paths":           paths,
		"RefreshInterval": 0,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if jm.config.APIKey != "" {
		req.Header.Set("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Client="NAS-OS", Device="NAS", DeviceId="NAS-OS", Version="1.0", Token="%s"`, jm.config.APIKey))
	}

	resp, err := jm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建失败：%s", string(body))
	}

	return nil
}

// ConfigureMetadata 配置元数据提供商
func (jm *JellyfinManager) ConfigureMetadata(libraryID string, enableChinese bool) error {
	// Jellyfin 原生支持 TMDB，中文元数据通过插件实现
	// 这里提供配置接口，实际配置需要通过 Jellyfin Web UI 完成

	if enableChinese {
		// 启用中文元数据
		// 1. 安装中文元数据插件（需要用户手动在 Jellyfin 中安装）
		// 2. 配置 TMDB 使用中文
		// 3. 可选：配置豆瓣插件（如果可用）

		fmt.Println("中文元数据配置指南:")
		fmt.Println("1. 在 Jellyfin 控制台 -> 插件 -> 目录中安装 'TheMovieDb' 插件")
		fmt.Println("2. 在 Jellyfin 控制台 -> 插件 -> 目录中安装 'Douban' 插件（如有）")
		fmt.Println("3. 在媒体库设置中，将元数据语言设置为 'zh-CN' 或 'zh-Hans'")
		fmt.Println("4. 在 Jellyfin 控制台 -> 插件 -> 已安装 中配置 TMDB API Key")
	}

	return nil
}

// ScanLibrary 扫描媒体库
func (jm *JellyfinManager) ScanLibrary(libraryID string) error {
	url := fmt.Sprintf("%s/Library/Media/Updated", jm.config.ServerURL)

	payload := map[string]interface{}{
		"Updates": []map[string]interface{}{
			{
				"ItemId":     libraryID,
				"UpdateType": "Created",
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if jm.config.APIKey != "" {
		req.Header.Set("X-Emby-Authorization", fmt.Sprintf(`MediaBrowser Client="NAS-OS", Device="NAS", DeviceId="NAS-OS", Version="1.0", Token="%s"`, jm.config.APIKey))
	}

	resp, err := jm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// GetRecommendedConfig 获取推荐配置
func (jm *JellyfinManager) GetRecommendedConfig() map[string]interface{} {
	return map[string]interface{}{
		"enableChinese":     true,
		"tmdbEnabled":       true,
		"doubanEnabled":     false, // 豆瓣插件需要手动安装
		"chinesePriority":   3,
		"metadataLanguages": []string{"zh-CN", "zh-Hans", "en-US"},
		"recommendedPlugins": []map[string]interface{}{
			{
				"name":        "TheMovieDb",
				"description": "TMDB 元数据提供商，支持中文",
				"required":    true,
			},
			{
				"name":        "TheOpenMovieDatabase",
				"description": "OMDb 元数据提供商",
				"required":    false,
			},
			{
				"name":        "Douban",
				"description": "豆瓣元数据插件（需要手动安装）",
				"required":    false,
			},
		},
		"setupSteps": []string{
			"1. 首次启动 Jellyfin 后完成初始化向导",
			"2. 在控制台 -> 插件中安装中文元数据插件",
			"3. 在控制台 -> 插件 -> TheMovieDb 中配置 API Key",
			"4. 创建媒体库时选择对应的媒体类型（电影/电视剧/音乐）",
			"5. 在媒体库设置中将元数据语言设置为 'zh-CN'",
			"6. 添加媒体文件夹路径",
			"7. 等待媒体库扫描完成",
		},
	}
}

// EnableChineseMetadata 启用中文元数据
func (jm *JellyfinManager) EnableChineseMetadata() error {
	jm.config.EnableChinese = true
	jm.config.TMDBEnabled = true
	return jm.config.save()
}

// GetConfig 获取当前配置
func (jm *JellyfinManager) GetConfig() *JellyfinConfig {
	return jm.config
}

// TestConnection 测试连接
func (jm *JellyfinManager) TestConnection() error {
	url := fmt.Sprintf("%s/System/Info/Public", jm.config.ServerURL)

	resp, err := jm.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("连接失败：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("连接失败：HTTP %d", resp.StatusCode)
	}

	return nil
}
