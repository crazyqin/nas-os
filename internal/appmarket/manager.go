// Package appmarket 应用市场模块
package appmarket

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 应用市场管理器
type Manager struct {
	mu         sync.RWMutex
	apps       map[string]*AppInfo
	installed  map[string]*InstalledApp
	reviews    map[string][]*ReviewRecord
	ratings    map[string][]*Rating
	configPath string
}

// NewManager 创建应用市场管理器
func NewManager(configPath string) *Manager {
	m := &Manager{
		apps:       make(map[string]*AppInfo),
		installed:  make(map[string]*InstalledApp),
		reviews:    make(map[string][]*ReviewRecord),
		ratings:    make(map[string][]*Rating),
		configPath: configPath,
	}
	_ = m.loadConfig()
	return m
}

// ========== 应用发布 ==========

// PublishApp 发布新应用
func (m *Manager) PublishApp(req *PublishRequest, developerID string) (*AppInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("应用名称不能为空")
	}
	if req.Version == "" {
		return nil, fmt.Errorf("应用版本不能为空")
	}
	if req.Category == "" {
		return nil, fmt.Errorf("应用分类不能为空")
	}

	now := time.Now()
	app := &AppInfo{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		Icon:         req.Icon,
		Version:      req.Version,
		Author:       developerID,
		Category:     req.Category,
		Tags:         req.Tags,
		License:      req.License,
		Homepage:     req.Homepage,
		Repository:   req.Repository,
		Size:         req.Size,
		MinCPU:       req.MinCPU,
		MinMemory:    req.MinMemory,
		MinDisk:      req.MinDisk,
		Dependencies: req.Dependencies,
		Status:       StatusPendingReview,
		Downloads:    0,
		Rating:       0,
		RatingCount:  0,
		DeveloperID:  developerID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.apps[app.ID] = app
	if err := m.saveConfig(); err != nil {
		delete(m.apps, app.ID)
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return app, nil
}

// UpdateApp 更新应用信息（开发者重新提交）
func (m *Manager) UpdateApp(id string, req *PublishRequest, developerID string) (*AppInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", id)
	}
	if app.DeveloperID != developerID {
		return nil, fmt.Errorf("无权更新此应用")
	}

	if req.Name != "" {
		app.Name = req.Name
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	if req.Icon != "" {
		app.Icon = req.Icon
	}
	if req.Version != "" {
		app.Version = req.Version
	}
	if req.Category != "" {
		app.Category = req.Category
	}
	if req.Tags != nil {
		app.Tags = req.Tags
	}
	if req.License != "" {
		app.License = req.License
	}
	if req.Homepage != "" {
		app.Homepage = req.Homepage
	}
	if req.Repository != "" {
		app.Repository = req.Repository
	}
	if req.Size > 0 {
		app.Size = req.Size
	}
	if req.MinCPU > 0 {
		app.MinCPU = req.MinCPU
	}
	if req.MinMemory > 0 {
		app.MinMemory = req.MinMemory
	}
	if req.MinDisk > 0 {
		app.MinDisk = req.MinDisk
	}
	if req.Dependencies != nil {
		app.Dependencies = req.Dependencies
	}

	app.Status = StatusPendingReview
	app.ReviewNote = ""
	app.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return app, nil
}

// ========== 应用审核 ==========

// ReviewApp 审核应用
func (m *Manager) ReviewApp(appID string, req *ReviewRequest, reviewer string) (*ReviewRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[appID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", appID)
	}
	if app.Status != StatusPendingReview {
		return nil, fmt.Errorf("应用当前状态 %s 不可审核", app.Status)
	}

	now := time.Now()
	record := &ReviewRecord{
		ID:        uuid.New().String(),
		AppID:     appID,
		Reviewer:  reviewer,
		Action:    req.Action,
		Note:      req.Note,
		CreatedAt: now,
	}

	switch req.Action {
	case ReviewApprove:
		app.Status = StatusApproved
		app.ReviewedBy = reviewer
		app.ReviewedAt = &now
	case ReviewReject:
		app.Status = StatusRejected
		app.ReviewNote = req.Note
		app.ReviewedBy = reviewer
		app.ReviewedAt = &now
	case ReviewRevision:
		app.Status = StatusRevision
		app.ReviewNote = req.Note
	default:
		return nil, fmt.Errorf("无效的审核动作: %s", req.Action)
	}

	app.UpdatedAt = now
	m.reviews[appID] = append(m.reviews[appID], record)

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return record, nil
}

// GetReviewHistory 获取审核历史
func (m *Manager) GetReviewHistory(appID string) []*ReviewRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.reviews[appID]
}

// ========== 应用安装/卸载/更新 ==========

// InstallApp 安装应用
func (m *Manager) InstallApp(req *InstallRequest, userID string) (*InstalledApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[req.AppID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", req.AppID)
	}
	if app.Status != StatusApproved && app.Status != StatusPublished {
		return nil, fmt.Errorf("应用 %s 状态 %s 不可安装", req.AppID, app.Status)
	}
	if _, installed := m.installed[req.AppID]; installed {
		return nil, fmt.Errorf("应用 %s 已安装", req.AppID)
	}

	// 检查依赖
	for _, dep := range app.Dependencies {
		if _, ok := m.installed[dep]; !ok {
			return nil, fmt.Errorf("依赖应用 %s 未安装", dep)
		}
	}

	version := req.Version
	if version == "" {
		version = app.Version
	}

	now := time.Now()
	installed := &InstalledApp{
		AppID:       req.AppID,
		Version:     version,
		Status:      "running",
		InstalledAt: now,
		UpdatedAt:   now,
		ConfigPath:  filepath.Join("apps", req.AppID, "config.json"),
	}

	m.installed[req.AppID] = installed
	app.Downloads++
	app.Status = StatusPublished

	if err := m.saveConfig(); err != nil {
		delete(m.installed, req.AppID)
		app.Downloads--
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return installed, nil
}

// UninstallApp 卸载应用
func (m *Manager) UninstallApp(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.installed[appID]
	if !ok {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	// 检查是否被其他应用依赖
	for _, app := range m.apps {
		for _, dep := range app.Dependencies {
			if dep == appID {
				if _, isInstalled := m.installed[app.ID]; isInstalled {
					return fmt.Errorf("应用 %s 被 %s 依赖，无法卸载", appID, app.ID)
				}
			}
		}
	}

	delete(m.installed, appID)
	return m.saveConfig()
}

// UpdateApp 更新已安装应用
func (m *Manager) UpdateInstalledApp(req *UpdateRequest) (*InstalledApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[req.AppID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", req.AppID)
	}

	installed, ok := m.installed[req.AppID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 未安装", req.AppID)
	}

	targetVersion := req.TargetVersion
	if targetVersion == "" {
		targetVersion = app.Version
	}

	if targetVersion == installed.Version {
		return nil, fmt.Errorf("应用 %s 已是最新版本", req.AppID)
	}

	installed.Version = targetVersion
	installed.Status = "running"
	installed.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return installed, nil
}

// ========== 浏览和搜索 ==========

// SearchApps 搜索应用
func (m *Manager) SearchApps(req *SearchRequest) *SearchResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 过滤应用
	var filtered []*AppInfo
	query := strings.ToLower(req.Query)

	for _, app := range m.apps {
		// 只显示已发布/已通过的应用
		if app.Status != StatusPublished && app.Status != StatusApproved {
			continue
		}

		// 分类过滤
		if req.Category != "" && app.Category != req.Category {
			continue
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range req.Tags {
				for _, appTag := range app.Tags {
					if strings.EqualFold(tag, appTag) {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 全文搜索
		if query != "" {
			if !strings.Contains(strings.ToLower(app.Name), query) &&
				!strings.Contains(strings.ToLower(app.Description), query) &&
				!strings.Contains(strings.ToLower(app.Author), query) {
				// 检查标签
				tagMatch := false
				for _, tag := range app.Tags {
					if strings.Contains(strings.ToLower(tag), query) {
						tagMatch = true
						break
					}
				}
				if !tagMatch {
					continue
				}
			}
		}

		filtered = append(filtered, app)
	}

	// 排序
	switch req.Sort {
	case SortRating:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Rating > filtered[j].Rating
		})
	case SortDownloads:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Downloads > filtered[j].Downloads
		})
	case SortName:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Name < filtered[j].Name
		})
	default: // SortLatest
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		})
	}

	total := len(filtered)
	totalPages := (total + pageSize - 1) / pageSize

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &SearchResponse{
		Apps:       filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// ListApps 列出所有应用（管理员）
func (m *Manager) ListApps(status AppStatus) []*AppInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var apps []*AppInfo
	for _, app := range m.apps {
		if status != "" && app.Status != status {
			continue
		}
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].UpdatedAt.After(apps[j].UpdatedAt)
	})

	return apps
}

// GetApp 获取单个应用
func (m *Manager) GetApp(id string) (*AppInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.apps[id]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", id)
	}
	return app, nil
}

// GetInstalledApps 获取已安装应用列表
func (m *Manager) GetInstalledApps() []*InstalledApp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var apps []*InstalledApp
	for _, app := range m.installed {
		apps = append(apps, app)
	}
	return apps
}

// GetInstalledApp 获取单个已安装应用
func (m *Manager) GetInstalledApp(appID string) (*InstalledApp, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, ok := m.installed[appID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 未安装", appID)
	}
	return app, nil
}

// ========== 评分评论 ==========

// RateApp 评分应用
func (m *Manager) RateApp(appID string, req *RatingRequest, userID string) (*Rating, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[appID]
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", appID)
	}

	if req.Score < 1 || req.Score > 5 {
		return nil, fmt.Errorf("评分必须在 1-5 之间")
	}

	// 检查用户是否已评分
	for _, r := range m.ratings[appID] {
		if r.UserID == userID {
			// 更新评分
			r.Score = req.Score
			r.Comment = req.Comment
			r.UpdatedAt = time.Now()
			m.updateRating(app)
			if err := m.saveConfig(); err != nil {
				return nil, fmt.Errorf("保存配置失败: %w", err)
			}
			return r, nil
		}
	}

	// 新评分
	rating := &Rating{
		ID:        uuid.New().String(),
		AppID:     appID,
		UserID:    userID,
		Score:     req.Score,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.ratings[appID] = append(m.ratings[appID], rating)
	m.updateRating(app)

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return rating, nil
}

// GetAppRatings 获取应用评分列表
func (m *Manager) GetAppRatings(appID string) []*Rating {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.ratings[appID]
}

// updateRating 更新应用平均评分
func (m *Manager) updateRating(app *AppInfo) {
	ratings := m.ratings[app.ID]
	if len(ratings) == 0 {
		app.Rating = 0
		app.RatingCount = 0
		return
	}

	total := 0
	for _, r := range ratings {
		total += r.Score
	}
	app.Rating = float64(total) / float64(len(ratings))
	app.RatingCount = len(ratings)
}

// ========== 分类浏览 ==========

// ListCategories 列出所有分类
func (m *Manager) ListCategories() []AppCategory {
	return []AppCategory{
		CategoryProductivity,
		CategoryMedia,
		CategoryNetwork,
		CategoryStorage,
		CategorySecurity,
		CategoryDevOps,
		CategoryDatabase,
		CategoryAI,
		CategoryGaming,
		CategoryUtility,
		CategoryOther,
	}
}

// ListAppsByCategory 按分类列出应用
func (m *Manager) ListAppsByCategory(category AppCategory) []*AppInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var apps []*AppInfo
	for _, app := range m.apps {
		if app.Category == category && (app.Status == StatusPublished || app.Status == StatusApproved) {
			apps = append(apps, app)
		}
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Downloads > apps[j].Downloads
	})

	return apps
}

// ========== 持久化 ==========

type appMarketConfig struct {
	Apps      map[string]*AppInfo        `json:"apps"`
	Installed map[string]*InstalledApp   `json:"installed"`
	Reviews   map[string][]*ReviewRecord `json:"reviews"`
	Ratings   map[string][]*Rating       `json:"ratings"`
}

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config appMarketConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Apps != nil {
		m.apps = config.Apps
	}
	if config.Installed != nil {
		m.installed = config.Installed
	}
	if config.Reviews != nil {
		m.reviews = config.Reviews
	}
	if config.Ratings != nil {
		m.ratings = config.Ratings
	}

	return nil
}

func (m *Manager) saveConfig() error {
	config := appMarketConfig{
		Apps:      m.apps,
		Installed: m.installed,
		Reviews:   m.reviews,
		Ratings:   m.ratings,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}
