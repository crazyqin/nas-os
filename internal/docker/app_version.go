package docker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AppVersion 应用版本
type AppVersion struct {
	ID           string    `json:"id"`
	TemplateID   string    `json:"templateId"`
	Version      string    `json:"version"`
	ImageTag     string    `json:"imageTag"`
	ReleaseNotes string    `json:"releaseNotes"`
	PublishedAt  time.Time `json:"publishedAt"`
	Digest       string    `json:"digest"`
	Size         int64     `json:"size"`
	IsLatest     bool      `json:"isLatest"`
}

// UpdateNotification 更新通知
type UpdateNotification struct {
	ID           string    `json:"id"`
	AppID        string    `json:"appId"`
	AppName      string    `json:"appName"`
	CurrentVer   string    `json:"currentVer"`
	LatestVer    string    `json:"latestVer"`
	ReleaseNotes string    `json:"releaseNotes"`
	CreatedAt    time.Time `json:"createdAt"`
	Read         bool      `json:"read"`
	Dismissed    bool      `json:"dismissed"`
}

// VersionManager 版本管理器
type VersionManager struct {
	store         *AppStore
	dataDir       string
	versionsFile  string
	notifyFile    string
	versions      map[string][]*AppVersion       // templateID -> versions
	notifications map[string]*UpdateNotification // notificationID -> notification
	httpClient    *http.Client
	mu            sync.RWMutex
}

// NewVersionManager 创建版本管理器
func NewVersionManager(store *AppStore, dataDir string) (*VersionManager, error) {
	versionsFile := filepath.Join(dataDir, "app-versions.json")
	notifyFile := filepath.Join(dataDir, "update-notifications.json")

	vm := &VersionManager{
		store:         store,
		dataDir:       dataDir,
		versionsFile:  versionsFile,
		notifyFile:    notifyFile,
		versions:      make(map[string][]*AppVersion),
		notifications: make(map[string]*UpdateNotification),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// 加载数据
	if err := vm.loadVersions(); err != nil {
		fmt.Printf("加载版本数据失败: %v\n", err)
	}
	if err := vm.loadNotifications(); err != nil {
		fmt.Printf("加载通知数据失败: %v\n", err)
	}

	return vm, nil
}

// loadVersions 加载版本数据
func (vm *VersionManager) loadVersions() error {
	data, err := os.ReadFile(vm.versionsFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &vm.versions)
}

// saveVersions 保存版本数据
func (vm *VersionManager) saveVersions() error {
	data, err := json.MarshalIndent(vm.versions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(vm.versionsFile, data, 0644)
}

// loadNotifications 加载通知数据
func (vm *VersionManager) loadNotifications() error {
	data, err := os.ReadFile(vm.notifyFile)
	if err != nil {
		return err
	}

	var notifications []*UpdateNotification
	if err := json.Unmarshal(data, &notifications); err != nil {
		return err
	}

	for _, n := range notifications {
		vm.notifications[n.ID] = n
	}
	return nil
}

// saveNotifications 保存通知数据
func (vm *VersionManager) saveNotifications() error {
	var notifications []*UpdateNotification
	for _, n := range vm.notifications {
		notifications = append(notifications, n)
	}

	data, err := json.MarshalIndent(notifications, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(vm.notifyFile, data, 0644)
}

// CheckForUpdates 检查更新
func (vm *VersionManager) CheckForUpdates() ([]*UpdateNotification, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	var updates []*UpdateNotification
	installed := vm.store.ListInstalled()

	for _, app := range installed {
		// 获取模板
		template := vm.store.GetTemplate(app.TemplateID)
		if template == nil {
			continue
		}

		// 检查 Docker Hub 是否有新版本
		latestTag, err := vm.getLatestTag(template.Image)
		if err != nil {
			fmt.Printf("检查 %s 更新失败: %v\n", app.Name, err)
			continue
		}

		currentTag := app.Version
		if currentTag == "" || currentTag == "latest" {
			currentTag = "latest"
		}

		// 如果有新版本
		if latestTag != "" && latestTag != currentTag && currentTag != "latest" {
			// 检查是否已通知过
			notifyID := fmt.Sprintf("%s-%s-%s", app.ID, currentTag, latestTag)
			if _, exists := vm.notifications[notifyID]; exists {
				continue
			}

			notification := &UpdateNotification{
				ID:         notifyID,
				AppID:      app.ID,
				AppName:    app.DisplayName,
				CurrentVer: currentTag,
				LatestVer:  latestTag,
				CreatedAt:  time.Now(),
				Read:       false,
			}

			vm.notifications[notifyID] = notification
			updates = append(updates, notification)
		}
	}

	if len(updates) > 0 {
		if err := vm.saveNotifications(); err != nil {
			log.Printf("保存通知失败: %v", err)
		}
	}

	return updates, nil
}

// getLatestTag 获取最新标签
func (vm *VersionManager) getLatestTag(image string) (string, error) {
	// 解析镜像名称
	parts := strings.Split(image, ":")
	imageName := parts[0]

	// 处理官方镜像和用户镜像
	var namespace, name string
	if strings.Contains(imageName, "/") {
		nsParts := strings.SplitN(imageName, "/", 2)
		namespace = nsParts[0]
		name = nsParts[1]
	} else {
		namespace = "library"
		name = imageName
	}

	// 查询 Docker Hub API
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=10", namespace, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := vm.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("docker Hub API 返回 %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// 返回最新的稳定版本标签（跳过 latest）
	for _, tag := range result.Results {
		if tag.Name != "latest" && !strings.Contains(tag.Name, "-") {
			return tag.Name, nil
		}
	}

	// 如果没有稳定版本，返回第一个非 latest 标签
	for _, tag := range result.Results {
		if tag.Name != "latest" {
			return tag.Name, nil
		}
	}

	return "latest", nil
}

// GetAvailableVersions 获取可用版本
func (vm *VersionManager) GetAvailableVersions(templateID string) ([]*AppVersion, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// 检查缓存
	if versions, ok := vm.versions[templateID]; ok {
		// 检查缓存是否过期（1小时）
		if len(versions) > 0 && time.Since(versions[0].PublishedAt) < time.Hour {
			return versions, nil
		}
	}

	// 从 Docker Hub 获取
	template := vm.store.GetTemplate(templateID)
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	versions, err := vm.fetchVersionsFromDockerHub(template.Image)
	if err != nil {
		return nil, err
	}

	vm.versions[templateID] = versions
	if err := vm.saveVersions(); err != nil {
		log.Printf("保存版本失败: %v", err)
	}

	return versions, nil
}

// fetchVersionsFromDockerHub 从 Docker Hub 获取版本
func (vm *VersionManager) fetchVersionsFromDockerHub(image string) ([]*AppVersion, error) {
	parts := strings.Split(image, ":")
	imageName := parts[0]

	var namespace, name string
	if strings.Contains(imageName, "/") {
		nsParts := strings.SplitN(imageName, "/", 2)
		namespace = nsParts[0]
		name = nsParts[1]
	} else {
		namespace = "library"
		name = imageName
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=50", namespace, name)

	resp, err := vm.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("docker Hub API 返回 %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name        string `json:"name"`
			FullSize    int64  `json:"full_size"`
			LastUpdated string `json:"last_updated"`
			Images      []struct {
				Digest string `json:"digest"`
			} `json:"images"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var versions []*AppVersion
	for i, tag := range result.Results {
		var digest string
		if len(tag.Images) > 0 {
			digest = tag.Images[0].Digest
		}

		publishedAt, _ := time.Parse(time.RFC3339, tag.LastUpdated)

		versions = append(versions, &AppVersion{
			ID:          fmt.Sprintf("%s-%s", imageName, tag.Name),
			TemplateID:  imageName,
			Version:     tag.Name,
			ImageTag:    tag.Name,
			PublishedAt: publishedAt,
			Digest:      digest,
			Size:        tag.FullSize,
			IsLatest:    i == 0 || tag.Name == "latest",
		})
	}

	return versions, nil
}

// GetNotifications 获取通知列表
func (vm *VersionManager) GetNotifications(unreadOnly bool) []*UpdateNotification {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	var result []*UpdateNotification
	for _, n := range vm.notifications {
		if !n.Dismissed {
			if !unreadOnly || !n.Read {
				result = append(result, n)
			}
		}
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CreatedAt.Before(result[j].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// MarkNotificationRead 标记通知已读
func (vm *VersionManager) MarkNotificationRead(id string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	n, ok := vm.notifications[id]
	if !ok {
		return fmt.Errorf("通知不存在: %s", id)
	}

	n.Read = true
	return vm.saveNotifications()
}

// DismissNotification 忽略通知
func (vm *VersionManager) DismissNotification(id string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	n, ok := vm.notifications[id]
	if !ok {
		return fmt.Errorf("通知不存在: %s", id)
	}

	n.Dismissed = true
	return vm.saveNotifications()
}

// MarkAllNotificationsRead 标记所有通知已读
func (vm *VersionManager) MarkAllNotificationsRead() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, n := range vm.notifications {
		n.Read = true
	}
	return vm.saveNotifications()
}

// ClearNotifications 清除所有通知
func (vm *VersionManager) ClearNotifications() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.notifications = make(map[string]*UpdateNotification)
	return vm.saveNotifications()
}

// GetUnreadCount 获取未读通知数量
func (vm *VersionManager) GetUnreadCount() int {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	count := 0
	for _, n := range vm.notifications {
		if !n.Read && !n.Dismissed {
			count++
		}
	}
	return count
}

// UpdateAppVersion 更新应用版本
func (vm *VersionManager) UpdateAppVersion(appID, newVersion string) error {
	app := vm.store.GetInstalled(appID)
	if app == nil {
		return fmt.Errorf("应用未安装: %s", appID)
	}

	template := vm.store.GetTemplate(app.TemplateID)
	if template == nil {
		return fmt.Errorf("模板不存在: %s", app.TemplateID)
	}

	// 构建新镜像名
	image := template.Image
	if strings.Contains(image, ":") {
		parts := strings.Split(image, ":")
		image = parts[0] + ":" + newVersion
	} else {
		image = image + ":" + newVersion
	}

	// 拉取新镜像
	if err := vm.store.manager.PullImage(image); err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 更新版本
	app.Version = newVersion

	// 重新创建容器
	if app.ComposePath != "" {
		// 更新 compose 文件中的镜像版本
		composeData, err := os.ReadFile(app.ComposePath)
		if err != nil {
			return err
		}

		composeStr := string(composeData)
		// 简单替换镜像版本
		if strings.Contains(composeStr, template.Image) {
			composeStr = strings.ReplaceAll(composeStr, template.Image, image)
		}

		if err := os.WriteFile(app.ComposePath, []byte(composeStr), 0644); err != nil {
			return err
		}

		// 重启容器
		return vm.store.RestartApp(appID)
	}

	return nil
}

// StartUpdateChecker 启动定期更新检查（后台协程）
func (vm *VersionManager) StartUpdateChecker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			updates, err := vm.CheckForUpdates()
			if err != nil {
				fmt.Printf("检查更新失败: %v\n", err)
			} else if len(updates) > 0 {
				fmt.Printf("发现 %d 个应用更新\n", len(updates))
			}
		}
	}()
}
