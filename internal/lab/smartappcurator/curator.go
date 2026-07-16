package smartappcurator

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Curator 应用推荐引擎。
type Curator struct {
	mu       sync.RWMutex
	apps     map[string]*AppInfo
	profiles map[string]*UserProfile
}

// New 创建推荐引擎。
func New() *Curator {
	c := &Curator{
		apps:     make(map[string]*AppInfo),
		profiles: make(map[string]*UserProfile),
	}
	c.initDefaultApps()
	return c
}

// RegisterApp 注册应用。
func (c *Curator) RegisterApp(app AppInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apps[app.ID] = &app
}

// UpdateProfile 更新用户画像。
func (c *Curator) UpdateProfile(profile UserProfile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	profile.LastUpdated = time.Now()
	c.profiles[profile.UserID] = &profile
}

// GetProfile 获取用户画像。
func (c *Curator) GetProfile(userID string) (*UserProfile, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	profile, ok := c.profiles[userID]
	if !ok {
		return nil, ErrNoProfile
	}
	return profile, nil
}

// Recommend 生成推荐。
func (c *Curator) Recommend(req RecommendRequest) (*RecommendationSet, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if req.Limit <= 0 {
		req.Limit = 10
	}

	profile, hasProfile := c.profiles[req.UserID]

	// 构建排除集合
	excluded := make(map[string]bool)
	for _, id := range req.Exclude {
		excluded[id] = true
	}
	if hasProfile {
		for _, id := range profile.InstalledApps {
			excluded[id] = true
		}
	}

	// 计算推荐分数
	var recs []Recommendation
	for _, app := range c.apps {
		if excluded[app.ID] {
			continue
		}

		score := c.calculateScore(app, profile, hasProfile)
		reason := c.generateReason(app, profile, hasProfile)

		recs = append(recs, Recommendation{
			App:    *app,
			Score:  score,
			Reason: reason,
		})
	}

	// 按分数排序
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})

	// 限制数量
	if len(recs) > req.Limit {
		recs = recs[:req.Limit]
	}

	// 获取热门和新应用
	trending := c.getTrendingApps(excluded, 5)
	newApps := c.getNewApps(excluded, 5)

	return &RecommendationSet{
		UserID:          req.UserID,
		GeneratedAt:     time.Now(),
		Recommendations: recs,
		TrendingApps:    trending,
		NewApps:         newApps,
	}, nil
}

// GetApp 获取应用信息。
func (c *Curator) GetApp(appID string) (*AppInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	app, ok := c.apps[appID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", appID)
	}
	return app, nil
}

// ListApps 列出所有应用。
func (c *Curator) ListApps(category AppCategory) []AppInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var apps []AppInfo
	for _, app := range c.apps {
		if category == "" || app.Category == category {
			apps = append(apps, *app)
		}
	}
	return apps
}

// calculateScore 计算推荐分数。
func (c *Curator) calculateScore(app *AppInfo, profile *UserProfile, hasProfile bool) float64 {
	score := 0.0

	// 基础分：评分和下载量
	score += app.Rating * 10
	if app.Downloads > 10000 {
		score += 10
	} else if app.Downloads > 1000 {
		score += 5
	}

	if !hasProfile {
		return score
	}

	// 偏好匹配
	for _, cat := range profile.Preferences.Categories {
		if app.Category == cat {
			score += 20
			break
		}
	}

	// 标签匹配
	for _, tag := range app.Tags {
		for _, prefTag := range profile.Preferences.Tags {
			if tag == prefTag {
				score += 5
			}
		}
	}

	// 使用模式匹配
	for _, usage := range profile.UsageStats {
		if installedApp, ok := c.apps[usage.AppID]; ok {
			if installedApp.Category == app.Category {
				score += 15
				break
			}
		}
	}

	return score
}

// generateReason 生成推荐理由。
func (c *Curator) generateReason(app *AppInfo, profile *UserProfile, hasProfile bool) string {
	if !hasProfile {
		return fmt.Sprintf("热门应用，评分 %.1f，下载量 %d", app.Rating, app.Downloads)
	}

	for _, cat := range profile.Preferences.Categories {
		if app.Category == cat {
			return fmt.Sprintf("匹配您偏好的 %s 类别", cat)
		}
	}

	if app.Rating >= 4.5 {
		return fmt.Sprintf("高评分应用 (%.1f)，用户口碑好", app.Rating)
	}

	return "根据您的使用习惯推荐"
}

// getTrendingApps 获取热门应用。
func (c *Curator) getTrendingApps(excluded map[string]bool, limit int) []AppInfo {
	var apps []AppInfo
	for _, app := range c.apps {
		if excluded[app.ID] {
			continue
		}
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Downloads > apps[j].Downloads
	})
	if len(apps) > limit {
		apps = apps[:limit]
	}
	return apps
}

// getNewApps 获取新应用。
func (c *Curator) getNewApps(excluded map[string]bool, limit int) []AppInfo {
	var apps []AppInfo
	for _, app := range c.apps {
		if excluded[app.ID] {
			continue
		}
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Version > apps[j].Version
	})
	if len(apps) > limit {
		apps = apps[:limit]
	}
	return apps
}

// initDefaultApps 初始化默认应用列表。
func (c *Curator) initDefaultApps() {
	defaultApps := []AppInfo{
		{ID: "photos", Name: "智能相册", Category: CategoryMedia, Description: "AI 驱动的照片管理和分类", Rating: 4.8, Downloads: 50000, Tags: []string{"ai", "photo", "media"}},
		{ID: "drive", Name: "文件同步", Category: CategoryProductivity, Description: "多端文件同步和协作", Rating: 4.7, Downloads: 45000, Tags: []string{"sync", "cloud", "files"}},
		{ID: "backup-pro", Name: "专业备份", Category: CategoryBackup, Description: "企业级备份解决方案", Rating: 4.6, Downloads: 30000, Tags: []string{"backup", "enterprise", "recovery"}},
		{ID: "vpn-gateway", Name: "VPN 网关", Category: CategoryNetwork, Description: "安全远程访问", Rating: 4.5, Downloads: 25000, Tags: []string{"vpn", "security", "remote"}},
		{ID: "ai-assistant", Name: "AI 助手", Category: CategoryAI, Description: "智能文件管理和问答", Rating: 4.9, Downloads: 40000, Tags: []string{"ai", "assistant", "smart"}},
		{ID: "media-server", Name: "媒体服务器", Category: CategoryMedia, Description: "家庭影院媒体服务", Rating: 4.7, Downloads: 35000, Tags: []string{"media", "streaming", "video"}},
		{ID: "docker-manager", Name: "容器管理", Category: CategoryDev, Description: "Docker 容器可视化管理", Rating: 4.6, Downloads: 28000, Tags: []string{"docker", "container", "dev"}},
		{ID: "smart-home", Name: "智能家居", Category: CategoryHome, Description: "智能家居设备统一管理", Rating: 4.4, Downloads: 20000, Tags: []string{"iot", "home", "automation"}},
		{ID: "office-suite", Name: "办公套件", Category: CategoryOffice, Description: "在线文档、表格、演示", Rating: 4.5, Downloads: 22000, Tags: []string{"office", "docs", "collaboration"}},
		{ID: "security-scanner", Name: "安全扫描", Category: CategorySecurity, Description: "系统安全漏洞扫描", Rating: 4.3, Downloads: 18000, Tags: []string{"security", "scan", "vulnerability"}},
	}

	for i := range defaultApps {
		c.apps[defaultApps[i].ID] = &defaultApps[i]
	}
}
