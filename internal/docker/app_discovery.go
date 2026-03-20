package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GitHubRepo GitHub 仓库信息
type GitHubRepo struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	Stars       int       `json:"stargazers_count"`
	Forks       int       `json:"forks_count"`
	Language    string    `json:"language"`
	Topics      []string  `json:"topics"`
	HTMLURL     string    `json:"html_url"`
	CloneURL    string    `json:"clone_url"`
	PushedAt    time.Time `json:"pushed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// HubImage Docker Hub 镜像信息
type HubImage struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	Stars       int    `json:"star_count"`
	Official    bool   `json:"is_official"`
}

// DiscoveredApp 发现的应用
type DiscoveredApp struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Source      string    `json:"source"` // github, dockerhub, custom
	Stars       int       `json:"stars"`
	Category    string    `json:"category"`
	Image       string    `json:"image"`
	GitHubURL   string    `json:"githubUrl,omitempty"`
	DockerHub   string    `json:"dockerHub,omitempty"`
	HasCompose  bool      `json:"hasCompose"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AppDiscovery 应用发现器
type AppDiscovery struct {
	store       *AppStore
	cacheFile   string
	cacheExpiry time.Duration
	lastUpdate  time.Time
	discovered  map[string]*DiscoveredApp
	mu          sync.RWMutex
	httpClient  *http.Client
}

// NewAppDiscovery 创建应用发现器
func NewAppDiscovery(store *AppStore, dataDir string) (*AppDiscovery, error) {
	cacheFile := filepath.Join(dataDir, "discovered-apps.json")

	ad := &AppDiscovery{
		store:       store,
		cacheFile:   cacheFile,
		cacheExpiry: 24 * time.Hour,
		discovered:  make(map[string]*DiscoveredApp),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 加载缓存
	if err := ad.loadCache(); err != nil {
		fmt.Printf("加载发现应用缓存失败: %v\n", err)
	}

	return ad, nil
}

// loadCache 加载缓存
func (ad *AppDiscovery) loadCache() error {
	data, err := os.ReadFile(ad.cacheFile)
	if err != nil {
		return err
	}

	var cache struct {
		LastUpdate time.Time                 `json:"lastUpdate"`
		Apps       map[string]*DiscoveredApp `json:"apps"`
	}

	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	ad.lastUpdate = cache.LastUpdate
	ad.discovered = cache.Apps
	return nil
}

// saveCache 保存缓存
func (ad *AppDiscovery) saveCache() error {
	cache := struct {
		LastUpdate time.Time                 `json:"lastUpdate"`
		Apps       map[string]*DiscoveredApp `json:"apps"`
	}{
		LastUpdate: ad.lastUpdate,
		Apps:       ad.discovered,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ad.cacheFile, data, 0640)
}

// DiscoverFromGitHub 从 GitHub 发现应用
func (ad *AppDiscovery) DiscoverFromGitHub() ([]*DiscoveredApp, error) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	var allApps []*DiscoveredApp

	// 搜索热门 Docker 相关仓库
	queries := []string{
		"docker-compose in:readme,filename stars:>1000",
		"docker-compose.yml in:path stars:>500",
		"awesome-docker in:name,description stars:>500",
		"self-hosted docker-compose in:readme stars:>200",
	}

	for _, query := range queries {
		repos, err := ad.searchGitHub(query)
		if err != nil {
			fmt.Printf("GitHub 搜索失败 (%s): %v\n", query, err)
			continue
		}

		for _, repo := range repos {
			app := ad.parseGitHubRepo(repo)
			if app != nil {
				ad.discovered[app.ID] = app
				allApps = append(allApps, app)
			}
		}
	}

	ad.lastUpdate = time.Now()
	if err := ad.saveCache(); err != nil {
		fmt.Printf("保存发现应用缓存失败: %v\n", err)
	}

	return allApps, nil
}

// searchGitHub 搜索 GitHub
func (ad *AppDiscovery) searchGitHub(query string) ([]*GitHubRepo, error) {
	url := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=30", query)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 添加 GitHub Token 如果有
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := ad.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}

	var result struct {
		Items []*GitHubRepo `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// parseGitHubRepo 解析 GitHub 仓库
func (ad *AppDiscovery) parseGitHubRepo(repo *GitHubRepo) *DiscoveredApp {
	if repo == nil {
		return nil
	}

	// 生成唯一 ID
	id := fmt.Sprintf("gh-%d", repo.ID)

	// 推断分类
	category := ad.inferCategory(repo.Topics, repo.Description, repo.Name)

	// 检查是否有 docker-compose
	hasCompose := ad.checkDockerCompose(repo.FullName)

	return &DiscoveredApp{
		ID:          id,
		Name:        repo.Name,
		DisplayName: strings.ReplaceAll(repo.Name, "-", " "),
		Description: repo.Description,
		Source:      "github",
		Stars:       repo.Stars,
		Category:    category,
		GitHubURL:   repo.HTMLURL,
		HasCompose:  hasCompose,
		UpdatedAt:   repo.UpdatedAt,
	}
}

// inferCategory 推断分类
func (ad *AppDiscovery) inferCategory(topics []string, description, name string) string {
	// 关键词映射
	categoryKeywords := map[string][]string{
		"Media":        {"media", "video", "music", "photo", "streaming", "plex", "jellyfin", "emby"},
		"Productivity": {"cloud", "storage", "sync", "file", "nextcloud", "owncloud"},
		"Smart Home":   {"home", "automation", "iot", "smart", "homeassistant", "home-assistant"},
		"Network":      {"network", "proxy", "dns", "vpn", "firewall", "nginx", "traefik"},
		"Download":     {"download", "torrent", "bt", "transmission", "qbittorrent"},
		"Development":  {"git", "code", "dev", "ci", "cd", "gitea", "gogs"},
		"Security":     {"security", "password", "auth", "vault", "bitwarden"},
		"Database":     {"database", "db", "mysql", "postgres", "mongo", "redis"},
	}

	// 检查 topics
	for _, topic := range topics {
		topicLower := strings.ToLower(topic)
		for cat, keywords := range categoryKeywords {
			for _, kw := range keywords {
				if strings.Contains(topicLower, kw) {
					return cat
				}
			}
		}
	}

	// 检查描述和名称
	combined := strings.ToLower(description + " " + name)
	for cat, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				return cat
			}
		}
	}

	return "Other"
}

// checkDockerCompose 检查是否有 docker-compose
func (ad *AppDiscovery) checkDockerCompose(fullName string) bool {
	// 检查仓库内容 API
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents", fullName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := ad.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return false
	}

	var contents []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return false
	}

	// 查找 docker-compose 文件
	for _, c := range contents {
		lowerName := strings.ToLower(c.Name)
		if strings.Contains(lowerName, "docker-compose") || strings.Contains(lowerName, "compose.yml") {
			return true
		}
	}

	return false
}

// DiscoverFromDockerHub 从 Docker Hub 发现应用
func (ad *AppDiscovery) DiscoverFromDockerHub() ([]*DiscoveredApp, error) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	var allApps []*DiscoveredApp

	// 搜索热门官方镜像
	url := "https://hub.docker.com/api/content/v1/products/search/?page_size=50&type=image"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Search-Version", "v3")

	resp, err := ad.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Summaries []struct {
			Name string `json:"name"`
			Logo struct {
				URL string `json:"url"`
			} `json:"logo"`
			Score int `json:"score"`
		} `json:"summaries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	for _, summary := range result.Summaries {
		id := fmt.Sprintf("dh-%s", summary.Name)
		app := &DiscoveredApp{
			ID:          id,
			Name:        summary.Name,
			DisplayName: summary.Name,
			Description: fmt.Sprintf("Docker 官方镜像: %s", summary.Name),
			Source:      "dockerhub",
			Stars:       summary.Score,
			Category:    "Official",
			Image:       summary.Name,
			DockerHub:   fmt.Sprintf("https://hub.docker.com/_/%s", summary.Name),
			UpdatedAt:   time.Now(),
		}
		ad.discovered[id] = app
		allApps = append(allApps, app)
	}

	ad.lastUpdate = time.Now()
	if err := ad.saveCache(); err != nil {
		fmt.Printf("保存发现应用缓存失败: %v\n", err)
	}

	return allApps, nil
}

// GetDiscoveredApps 获取发现的应用
func (ad *AppDiscovery) GetDiscoveredApps(source string, category string, limit int) []*DiscoveredApp {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	var result []*DiscoveredApp

	for _, app := range ad.discovered {
		if source != "" && app.Source != source {
			continue
		}
		if category != "" && app.Category != category {
			continue
		}
		result = append(result, app)
	}

	// 按星数排序
	sortDiscoveredByStars(result)

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// sortDiscoveredByStars 按星数排序
func sortDiscoveredByStars(apps []*DiscoveredApp) {
	for i := 0; i < len(apps); i++ {
		for j := i + 1; j < len(apps); j++ {
			if apps[i].Stars < apps[j].Stars {
				apps[i], apps[j] = apps[j], apps[i]
			}
		}
	}
}

// RefreshDiscovery 刷新发现
func (ad *AppDiscovery) RefreshDiscovery() error {
	_, err1 := ad.DiscoverFromGitHub()
	_, err2 := ad.DiscoverFromDockerHub()

	if err1 != nil && err2 != nil {
		return fmt.Errorf("GitHub: %v, DockerHub: %v", err1, err2)
	}

	return nil
}

// IsCacheValid 检查缓存是否有效
func (ad *AppDiscovery) IsCacheValid() bool {
	return time.Since(ad.lastUpdate) < ad.cacheExpiry
}

// GetLastUpdateTime 获取最后更新时间
func (ad *AppDiscovery) GetLastUpdateTime() time.Time {
	ad.mu.RLock()
	defer ad.mu.RUnlock()
	return ad.lastUpdate
}

// FetchComposeFile 获取 docker-compose 文件
func (ad *AppDiscovery) FetchComposeFile(repoFullName string) (string, error) {
	// 尝试获取 docker-compose.yml 或 compose.yml
	filenames := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}

	for _, filename := range filenames {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", repoFullName, filename)

		resp, err := ad.httpClient.Get(url)
		if err != nil {
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != 200 {
			continue
		}

		var content []byte
		_, err = resp.Body.Read(content)
		if err != nil {
			continue
		}

		return string(content), nil
	}

	return "", fmt.Errorf("未找到 docker-compose 文件")
}
