// Package appstore 应用推荐引擎
// 基于用户使用习惯和系统配置推荐应用
package appstore

import (
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 推荐引擎 ==========

// Recommender 应用推荐引擎
type Recommender struct {
	mu           sync.RWMutex
	usageHistory map[string]*UsageRecord // 应用使用记录
	userPrefs    *UserPreferences        // 用户偏好
	sysInfo      *SystemInfo             // 系统信息
	catalog      *Catalog                // 应用目录引用
}

// UsageRecord 使用记录
type UsageRecord struct {
	AppID        string    `json:"appId"`
	LaunchCount  int       `json:"launchCount"`
	TotalRuntime int64     `json:"totalRuntime"` // 总运行时长（秒）
	LastUsed     time.Time `json:"lastUsed"`
	FirstUsed    time.Time `json:"firstUsed"`
	Uninstalled  bool      `json:"uninstalled"`
	Rating       int       `json:"rating"` // 用户评分 1-5
}

// UserPreferences 用户偏好
type UserPreferences struct {
	FavoriteCategories []string          `json:"favoriteCategories"`
	FavoriteTags       []string          `json:"favoriteTags"`
	BlockedApps        map[string]bool   `json:"blockedApps"`
	InstalledCategories map[string]int   `json:"installedCategories"` // 分类 -> 安装数量
	CustomWeights      map[string]float64 `json:"customWeights,omitempty"`
}

// SystemInfo 系统信息（用于推荐上下文）
type SystemInfo struct {
	TotalMemoryMB  int64   `json:"totalMemoryMB"`
	UsedMemoryMB   int64   `json:"usedMemoryMB"`
	TotalDiskGB    int64   `json:"totalDiskGB"`
	UsedDiskGB     int64   `json:"usedDiskGB"`
	CPUCores       int     `json:"cpuCores"`
	HasGPU         bool    `json:"hasGPU"`
	HasDocker      bool    `json:"hasDocker"`
	HasLXC         bool    `json:"hasLXC"`
	NetworkType    string  `json:"networkType"` // "home", "office", "datacenter"
}

// Recommendation 推荐结果
type Recommendation struct {
	AppID       string   `json:"appId"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Score       float64  `json:"score"`
	Reasons     []string `json:"reasons"`
	IsNew       bool     `json:"isNew"`
	IsPopular   bool     `json:"isPopular"`
}

// NewRecommender 创建推荐引擎
func NewRecommender(catalog *Catalog, sysInfo *SystemInfo) *Recommender {
	return &Recommender{
		usageHistory: make(map[string]*UsageRecord),
		userPrefs: &UserPreferences{
			BlockedApps:         make(map[string]bool),
			InstalledCategories: make(map[string]int),
		},
		sysInfo:   sysInfo,
		catalog:   catalog,
	}
}

// RecordUsage 记录应用使用
func (r *Recommender) RecordUsage(appID string, runtimeSeconds int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.usageHistory[appID]
	if !exists {
		record = &UsageRecord{
			AppID:     appID,
			FirstUsed: time.Now(),
		}
		r.usageHistory[appID] = record
	}

	record.LaunchCount++
	record.TotalRuntime += runtimeSeconds
	record.LastUsed = time.Now()
}

// SetRating 设置应用评分
func (r *Recommender) SetRating(appID string, rating int) {
	if rating < 1 || rating > 5 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.usageHistory[appID]
	if !exists {
		record = &UsageRecord{AppID: appID, FirstUsed: time.Now()}
		r.usageHistory[appID] = record
	}
	record.Rating = rating
}

// RecordUninstall 记录卸载
func (r *Recommender) RecordUninstall(appID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record, exists := r.usageHistory[appID]; exists {
		record.Uninstalled = true
	}
}

// UpdateInstalledCategories 更新已安装分类统计
func (r *Recommender) UpdateInstalledCategories(installed []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	categories := make(map[string]int)
	for _, appID := range installed {
		if entry, ok := r.catalog.GetApp(appID); ok {
			categories[entry.Category]++
		}
	}
	r.userPrefs.InstalledCategories = categories
}

// GetRecommendations 获取推荐列表
func (r *Recommender) GetRecommendations(installed map[string]bool, limit int) []*Recommendation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	allApps := r.catalog.ListApps(nil)
	var scored []*Recommendation

	for _, app := range allApps {
		// 跳过已安装和已屏蔽的应用
		if installed[app.ID] || r.userPrefs.BlockedApps[app.ID] {
			continue
		}

		rec := &Recommendation{
			AppID:       app.ID,
			DisplayName: app.DisplayName,
			Description: app.Description,
			Category:    app.Category,
			Icon:        app.Icon,
		}

		// 计算推荐分数
		rec.Score = r.calculateScore(app, installed)
		rec.Reasons = r.generateReasons(app)
		rec.IsNew = time.Since(app.UpdatedAt) < 30*24*time.Hour
		rec.IsPopular = app.Downloads > 10000

		scored = append(scored, rec)
	}

	// 按分数排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored
}

// calculateScore 计算推荐分数
func (r *Recommender) calculateScore(app *CatalogEntry, installed map[string]bool) float64 {
	var score float64

	// 1. 基础分数：评分和下载量
	ratingScore := app.Rating / 5.0 * 30 // 最高30分
	downloadScore := math.Log10(float64(app.Downloads)+1) / 5.0 * 20 // 最高20分
	score += ratingScore + downloadScore

	// 2. 分类偏好匹配
	if r.userPrefs != nil {
		// 偏好分类加分
		for _, cat := range r.userPrefs.FavoriteCategories {
			if app.Category == cat {
				score += 15
				break
			}
		}

		// 已安装较少的分类加分（鼓励多样性）
		if installedCount, ok := r.userPrefs.InstalledCategories[app.Category]; ok {
			if installedCount <= 1 {
				score += 10 // 该分类安装少，加分鼓励
			}
		}

		// 标签匹配加分
		for _, tag := range app.Tags {
			for _, prefTag := range r.userPrefs.FavoriteTags {
				if tag == prefTag {
					score += 5
					break
				}
			}
		}
	}

	// 3. 系统兼容性加分
	if r.sysInfo != nil {
		// 检查资源需求
		for _, container := range app.Containers {
			if container.ResourceLimits != nil {
				rl := container.ResourceLimits
				// 内存检查
				availableMB := r.sysInfo.TotalMemoryMB - r.sysInfo.UsedMemoryMB
				if rl.MemoryMB > 0 && rl.MemoryMB <= availableMB {
					score += 5
				}
				// 磁盘检查
				availableGB := r.sysInfo.TotalDiskGB - r.sysInfo.UsedDiskGB
				if rl.DiskGB > 0 && rl.DiskGB <= availableGB {
					score += 3
				}
				// GPU 检查
				if rl.CPUCores > 0 && r.sysInfo.HasGPU {
					score += 2
				}
			}
		}

		// 网络类型匹配
		if r.sysInfo.NetworkType == "home" {
			for _, tag := range app.Tags {
				if tag == "smart-home" || tag == "media" || tag == "iot" {
					score += 8
					break
				}
			}
		}
	}

	// 4. 依赖完整性加分（如果所有依赖都已安装）
	allDepsInstalled := true
	for _, dep := range app.Dependencies {
		if !installed[dep] {
			allDepsInstalled = false
			break
		}
	}
	if allDepsInstalled && len(app.Dependencies) > 0 {
		score += 10
	}

	// 5. 使用历史关联加分（类似应用的使用习惯）
	if record, exists := r.usageHistory[app.ID]; exists {
		if !record.Uninstalled && record.Rating >= 4 {
			score += 5
		}
	}

	return score
}

// generateReasons 生成推荐理由
func (r *Recommender) generateReasons(app *CatalogEntry) []string {
	var reasons []string

	if app.Rating >= 4.5 {
		reasons = append(reasons, "高评分应用")
	}
	if app.Downloads > 10000 {
		reasons = append(reasons, "热门应用")
	}
	if time.Since(app.UpdatedAt) < 30*24*time.Hour {
		reasons = append(reasons, "新上架应用")
	}

	if r.userPrefs != nil {
		for _, cat := range r.userPrefs.FavoriteCategories {
			if app.Category == cat {
				reasons = append(reasons, "匹配您偏好的分类")
				break
			}
		}
	}

	if r.sysInfo != nil {
		if r.sysInfo.NetworkType == "home" {
			for _, tag := range app.Tags {
				if tag == "smart-home" || tag == "media" {
					reasons = append(reasons, "适合家庭NAS使用")
					break
				}
			}
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "综合推荐")
	}

	return reasons
}

// GetSimilarApps 获取相似应用
func (r *Recommender) GetSimilarApps(appID string, limit int) []*Recommendation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		limit = 5
	}

	target, ok := r.catalog.GetApp(appID)
	if !ok {
		return nil
	}

	allApps := r.catalog.ListApps(nil)
	var similar []*Recommendation

	for _, app := range allApps {
		if app.ID == appID {
			continue
		}

		sim := r.calculateSimilarity(target, app)
		if sim > 0.2 { // 相似度阈值
			similar = append(similar, &Recommendation{
				AppID:       app.ID,
				DisplayName: app.DisplayName,
				Description: app.Description,
				Category:    app.Category,
				Icon:        app.Icon,
				Score:       sim * 100,
			})
		}
	}

	sort.Slice(similar, func(i, j int) bool {
		return similar[i].Score > similar[j].Score
	})

	if len(similar) > limit {
		similar = similar[:limit]
	}

	return similar
}

// calculateSimilarity 计算两个应用的相似度
func (r *Recommender) calculateSimilarity(a, b *CatalogEntry) float64 {
	var score float64

	// 分类匹配
	if a.Category == b.Category {
		score += 0.4
	}

	// 标签重叠
	commonTags := 0
	for _, ta := range a.Tags {
		for _, tb := range b.Tags {
			if ta == tb {
				commonTags++
				break
			}
		}
	}
	if len(a.Tags) > 0 || len(b.Tags) > 0 {
		maxTags := len(a.Tags)
		if len(b.Tags) > maxTags {
			maxTags = len(b.Tags)
		}
		score += float64(commonTags) / float64(maxTags) * 0.4
	}

	// 同作者
	if a.Author == b.Author && a.Author != "" {
		score += 0.2
	}

	return score
}
