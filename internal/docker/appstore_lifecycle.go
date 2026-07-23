package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// App store versioning, updates, backup and health (split from appstore.go).

// TemplateVersionManager 模板版本管理器.
type TemplateVersionManager struct {
	mu       sync.RWMutex
	store    *AppStore
	dataDir  string
	versions map[string][]*TemplateVersion // templateID -> versions
}

// NewTemplateVersionManager 创建模板版本管理器.
func NewTemplateVersionManager(store *AppStore, dataDir string) (*TemplateVersionManager, error) {
	tvm := &TemplateVersionManager{
		store:    store,
		dataDir:  dataDir,
		versions: make(map[string][]*TemplateVersion),
	}

	// 加载版本数据
	if err := tvm.load(); err != nil {
		fmt.Printf("加载模板版本数据失败: %v\n", err)
	}

	return tvm, nil
}

// load 加载版本数据.
func (tvm *TemplateVersionManager) load() error {
	dataFile := filepath.Join(tvm.dataDir, "template-versions.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	return json.Unmarshal(data, &tvm.versions)
}

// save 保存版本数据.
func (tvm *TemplateVersionManager) save() error {
	dataFile := filepath.Join(tvm.dataDir, "template-versions.json")
	data, err := json.MarshalIndent(tvm.versions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0640)
}

// AddVersion 添加模板版本.
func (tvm *TemplateVersionManager) AddVersion(templateID string, version *TemplateVersion) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	version.TemplateID = templateID
	version.ID = fmt.Sprintf("%s-%s", templateID, version.Version)

	// 检查版本是否已存在
	for _, v := range tvm.versions[templateID] {
		if v.Version == version.Version {
			return fmt.Errorf("版本已存在: %s", version.Version)
		}
	}

	tvm.versions[templateID] = append(tvm.versions[templateID], version)
	return tvm.save()
}

// GetVersions 获取模板的所有版本.
func (tvm *TemplateVersionManager) GetVersions(templateID string) []*TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	versions := tvm.versions[templateID]
	if versions == nil {
		return []*TemplateVersion{}
	}

	// 返回副本
	result := make([]*TemplateVersion, len(versions))
	copy(result, versions)
	return result
}

// GetLatestVersion 获取最新版本.
func (tvm *TemplateVersionManager) GetLatestVersion(templateID string) *TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	versions := tvm.versions[templateID]
	if len(versions) == 0 {
		return nil
	}

	// 返回最新的非弃用版本
	for i := len(versions) - 1; i >= 0; i-- {
		if !versions[i].Deprecated {
			return versions[i]
		}
	}

	return versions[len(versions)-1]
}

// GetVersion 获取指定版本.
func (tvm *TemplateVersionManager) GetVersion(templateID, version string) *TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	for _, v := range tvm.versions[templateID] {
		if v.Version == version {
			return v
		}
	}

	return nil
}

// DeprecateVersion 标记版本为弃用.
func (tvm *TemplateVersionManager) DeprecateVersion(templateID, version string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	for _, v := range tvm.versions[templateID] {
		if v.Version == version {
			v.Deprecated = true
			return tvm.save()
		}
	}

	return fmt.Errorf("版本不存在: %s", version)
}

// RemoveVersion 移除版本.
func (tvm *TemplateVersionManager) RemoveVersion(templateID, version string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	versions := tvm.versions[templateID]
	for i, v := range versions {
		if v.Version == version {
			tvm.versions[templateID] = append(versions[:i], versions[i+1:]...)
			return tvm.save()
		}
	}

	return fmt.Errorf("版本不存在: %s", version)
}

// =============================================================================
// 应用更新检测增强
// =============================================================================

// UpdateInfo 更新信息.
type UpdateInfo struct {
	AppID          string    `json:"appId"`
	AppName        string    `json:"appName"`
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion"`
	CurrentDigest  string    `json:"currentDigest"`
	LatestDigest   string    `json:"latestDigest"`
	HasUpdate      bool      `json:"hasUpdate"`
	ReleaseNotes   string    `json:"releaseNotes"`
	PublishedAt    time.Time `json:"publishedAt"`
	ImageSize      int64     `json:"imageSize"`
	CheckTime      time.Time `json:"checkTime"`
	AutoUpdate     bool      `json:"autoUpdate"`
	IgnoreUntil    time.Time `json:"ignoreUntil"`
}

// UpdateChecker 更新检测器.
type UpdateChecker struct {
	mu          sync.RWMutex
	store       *AppStore
	httpClient  *http.Client
	registry    string // Docker Registry 地址
	credentials map[string]RegistryCredential
}

// RegistryCredential Registry 凭证.
type RegistryCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewUpdateChecker 创建更新检测器.
func NewUpdateChecker(store *AppStore) *UpdateChecker {
	return &UpdateChecker{
		store: store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		registry:    "https://registry.hub.docker.com",
		credentials: make(map[string]RegistryCredential),
	}
}

// SetRegistry 设置 Registry 地址.
func (uc *UpdateChecker) SetRegistry(registry string) {
	uc.registry = registry
}

// SetCredential 设置 Registry 凭证.
func (uc *UpdateChecker) SetCredential(registry string, cred RegistryCredential) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.credentials[registry] = cred
}

// CheckAppUpdate 检查单个应用的更新.
func (uc *UpdateChecker) CheckAppUpdate(appID string) (*UpdateInfo, error) {
	app := uc.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	template := uc.store.GetTemplate(app.TemplateID)
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", app.TemplateID)
	}

	return uc.CheckImageUpdate(template.Image, app.Version)
}

// CheckImageUpdate 检查镜像更新.
func (uc *UpdateChecker) CheckImageUpdate(image, currentTag string) (*UpdateInfo, error) {
	// 解析镜像名称
	imageName, tag := parseImageName(image)
	if tag != "" && tag != "latest" {
		currentTag = tag
	}

	// 获取镜像信息
	imageInfo, err := uc.fetchImageInfo(imageName)
	if err != nil {
		return nil, fmt.Errorf("获取镜像信息失败: %w", err)
	}

	// 查找当前版本
	var currentDigest string
	for _, t := range imageInfo.Tags {
		if t.Name == currentTag {
			currentDigest = t.Digest
			break
		}
	}

	// 查找最新版本
	latestTag := findLatestStableTag(imageInfo.Tags)
	if latestTag == nil && len(imageInfo.Tags) > 0 {
		latestTag = &imageInfo.Tags[0]
	}

	if latestTag == nil {
		return nil, fmt.Errorf("未找到可用版本")
	}

	// 构建更新信息
	info := &UpdateInfo{
		CurrentVersion: currentTag,
		LatestVersion:  latestTag.Name,
		CurrentDigest:  currentDigest,
		LatestDigest:   latestTag.Digest,
		HasUpdate:      currentDigest != "" && currentDigest != latestTag.Digest,
		ReleaseNotes:   latestTag.ReleaseNotes,
		PublishedAt:    latestTag.LastUpdated,
		ImageSize:      latestTag.FullSize,
		CheckTime:      time.Now(),
	}

	return info, nil
}

// ImageInfo 镜像信息.
type ImageInfo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []TagInfo `json:"tags"`
}

// TagInfo 标签信息.
type TagInfo struct {
	Name         string    `json:"name"`
	Digest       string    `json:"digest"`
	FullSize     int64     `json:"full_size"`
	LastUpdated  time.Time `json:"last_updated"`
	ReleaseNotes string    `json:"release_notes"`
}

// fetchImageInfo 获取镜像信息.
func (uc *UpdateChecker) fetchImageInfo(imageName string) (*ImageInfo, error) {
	namespace, name := parseImageNamespace(imageName)

	// 构建 Docker Hub API URL
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=100", namespace, name)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证（如果有）
	if cred, ok := uc.credentials["docker.io"]; ok {
		req.SetBasicAuth(cred.Username, cred.Password)
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry 返回 %d", resp.StatusCode)
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

	// 转换结果
	info := &ImageInfo{
		Name: imageName,
		Tags: make([]TagInfo, 0, len(result.Results)),
	}

	for _, r := range result.Results {
		var digest string
		if len(r.Images) > 0 {
			digest = r.Images[0].Digest
		}

		lastUpdated, _ := time.Parse(time.RFC3339, r.LastUpdated)

		info.Tags = append(info.Tags, TagInfo{
			Name:        r.Name,
			Digest:      digest,
			FullSize:    r.FullSize,
			LastUpdated: lastUpdated,
		})
	}

	return info, nil
}

// parseImageName 解析镜像名称.
func parseImageName(image string) (string, string) {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) == 1 {
		return parts[0], "latest"
	}
	return parts[0], parts[1]
}

// parseImageNamespace 解析镜像命名空间.
func parseImageNamespace(imageName string) (string, string) {
	if strings.Contains(imageName, "/") {
		parts := strings.SplitN(imageName, "/", 2)
		return parts[0], parts[1]
	}
	return "library", imageName
}

// findLatestStableTag 查找最新稳定版本标签.
func findLatestStableTag(tags []TagInfo) *TagInfo {
	// 优先选择非 latest、非预发布版本
	for _, t := range tags {
		if t.Name == "latest" {
			continue
		}
		// 跳过预发布版本（包含 -alpha, -beta, -rc 等）
		if strings.Contains(t.Name, "-alpha") ||
			strings.Contains(t.Name, "-beta") ||
			strings.Contains(t.Name, "-rc") ||
			strings.Contains(t.Name, "-preview") {
			continue
		}
		return &t
	}

	// 如果没有稳定版本，返回第一个非 latest 版本
	for _, t := range tags {
		if t.Name != "latest" {
			return &t
		}
	}

	return nil
}

// =============================================================================
// 应用备份与恢复
// =============================================================================

// BackupInfo 备份信息.
type BackupInfo struct {
	ID         string            `json:"id"`
	AppID      string            `json:"appId"`
	AppName    string            `json:"appName"`
	Version    string            `json:"version"`
	CreateTime time.Time         `json:"createTime"`
	Size       int64             `json:"size"`
	Notes      string            `json:"notes"`
	Includes   []string          `json:"includes"` // 包含的内容: config, data, env
	Checksum   string            `json:"checksum"`
	Labels     map[string]string `json:"labels"`
}

// BackupManager 备份管理器.
type BackupManager struct {
	mu        sync.RWMutex
	store     *AppStore
	backupDir string
}

// NewBackupManager 创建备份管理器.
func NewBackupManager(store *AppStore, dataDir string) (*BackupManager, error) {
	backupDir := filepath.Join(dataDir, "app-backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, err
	}

	return &BackupManager{
		store:     store,
		backupDir: backupDir,
	}, nil
}

// BackupApp 备份应用.
func (bm *BackupManager) BackupApp(appID string, opts BackupOptions) (*BackupInfo, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	app := bm.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	// 创建备份目录
	backupID := fmt.Sprintf("%s-%d", appID, time.Now().Unix())
	backupPath := filepath.Join(bm.backupDir, backupID)
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return nil, err
	}

	includes := []string{}
	var totalSize int64

	// 备份配置
	if opts.IncludeConfig {
		configPath := filepath.Join(backupPath, "config.json")
		configData := map[string]interface{}{
			"appId":       app.ID,
			"name":        app.Name,
			"displayName": app.DisplayName,
			"templateId":  app.TemplateID,
			"version":     app.Version,
			"ports":       app.Ports,
			"volumes":     app.Volumes,
			"environment": app.Environment,
			"installTime": app.InstallTime,
		}

		data, err := json.MarshalIndent(configData, "", "  ")
		if err != nil {
			_ = os.RemoveAll(backupPath)
			return nil, err
		}

		if err := os.WriteFile(configPath, data, 0640); err != nil {
			_ = os.RemoveAll(backupPath)
			return nil, err
		}

		includes = append(includes, "config")
		totalSize += int64(len(data))
	}

	// 备份 Docker Compose 文件
	if app.ComposePath != "" && opts.IncludeCompose {
		composeData, err := os.ReadFile(app.ComposePath)
		if err == nil {
			composeBackup := filepath.Join(backupPath, "docker-compose.yml")
			if err := os.WriteFile(composeBackup, composeData, 0640); err == nil {
				includes = append(includes, "compose")
				totalSize += int64(len(composeData))
			}
		}
	}

	// 备份数据卷（可选）
	if opts.IncludeData && len(app.Volumes) > 0 {
		dataDir := filepath.Join(backupPath, "volumes")
		if err := os.MkdirAll(dataDir, 0750); err == nil {
			ctx := context.Background()
			for containerPath, hostPath := range app.Volumes {
				if _, err := os.Stat(hostPath); err == nil {
					// 使用 tar 打包数据
					archiveName := strings.ReplaceAll(strings.TrimPrefix(containerPath, "/"), "/", "_") + ".tar.gz"
					archivePath := filepath.Join(dataDir, archiveName)

					cmd := exec.CommandContext(ctx, "tar", "-czf", archivePath, "-C", hostPath, ".")
					if err := cmd.Run(); err == nil {
						if fi, err := os.Stat(archivePath); err == nil {
							totalSize += fi.Size()
						}
					}
				}
			}
			includes = append(includes, "data")
		}
	}

	// 计算校验和
	checksum, err := bm.calculateChecksum(backupPath)
	if err != nil {
		_ = os.RemoveAll(backupPath)
		return nil, err
	}

	// 保存备份信息
	info := &BackupInfo{
		ID:         backupID,
		AppID:      appID,
		AppName:    app.DisplayName,
		Version:    app.Version,
		CreateTime: time.Now(),
		Size:       totalSize,
		Notes:      opts.Notes,
		Includes:   includes,
		Checksum:   checksum,
		Labels:     opts.Labels,
	}

	infoPath := filepath.Join(backupPath, "backup.json")
	infoData, _ := json.MarshalIndent(info, "", "  ")
	_ = os.WriteFile(infoPath, infoData, 0640)

	return info, nil
}

// BackupOptions 备份选项.
type BackupOptions struct {
	IncludeConfig  bool              `json:"includeConfig"`
	IncludeCompose bool              `json:"includeCompose"`
	IncludeData    bool              `json:"includeData"`
	Notes          string            `json:"notes"`
	Labels         map[string]string `json:"labels"`
}

// RestoreApp 恢复应用.
func (bm *BackupManager) RestoreApp(backupID string, opts RestoreOptions) (*InstalledApp, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	backupPath := filepath.Join(bm.backupDir, backupID)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("备份不存在: %s", backupID)
	}

	// 读取备份信息
	infoPath := filepath.Join(backupPath, "backup.json")
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info BackupInfo
	if err := json.Unmarshal(infoData, &info); err != nil {
		return nil, err
	}

	// 验证校验和
	checksum, err := bm.calculateChecksum(backupPath)
	if err != nil {
		return nil, err
	}

	if checksum != info.Checksum {
		return nil, fmt.Errorf("备份校验失败，文件可能已损坏")
	}

	// 读取配置
	configPath := filepath.Join(backupPath, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		AppID       string            `json:"appId"`
		Name        string            `json:"name"`
		DisplayName string            `json:"displayName"`
		TemplateID  string            `json:"templateId"`
		Version     string            `json:"version"`
		Ports       map[int]int       `json:"ports"`
		Volumes     map[string]string `json:"volumes"`
		Environment map[string]string `json:"environment"`
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
	}

	// 检查应用是否已存在
	if !opts.ForceRestore {
		if existing := bm.store.GetInstalled(config.AppID); existing != nil {
			return nil, fmt.Errorf("应用已存在，使用 forceRestore 强制恢复")
		}
	}

	// 恢复数据卷
	if opts.RestoreData && containsStr(info.Includes, "data") {
		dataDir := filepath.Join(backupPath, "volumes")
		if _, err := os.Stat(dataDir); err == nil {
			ctx := context.Background()
			for containerPath, hostPath := range config.Volumes {
				archiveName := strings.ReplaceAll(strings.TrimPrefix(containerPath, "/"), "/", "_") + ".tar.gz"
				archivePath := filepath.Join(dataDir, archiveName)

				if _, err := os.Stat(archivePath); err == nil {
					// 创建目标目录
					_ = os.MkdirAll(hostPath, 0750)

					// 解压数据
					cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", hostPath)
					_ = cmd.Run()
				}
			}
		}
	}

	// 恢复 Docker Compose 文件
	composeBackup := filepath.Join(backupPath, "docker-compose.yml")
	var composePath string

	if composeData, err := os.ReadFile(composeBackup); err == nil {
		appDir := filepath.Join(bm.store.installDir, config.Name)
		_ = os.MkdirAll(appDir, 0750)
		composePath = filepath.Join(appDir, "docker-compose.yml")
		_ = os.WriteFile(composePath, composeData, 0640)

		// 启动容器
		if opts.StartAfterRestore {
			ctx := context.Background()
			cmd := exec.CommandContext(ctx, "docker-compose", "-f", composePath, "up", "-d")
			_ = cmd.Run()
		}
	}

	// 创建安装记录
	app := &InstalledApp{
		ID:          config.AppID,
		Name:        config.Name,
		DisplayName: config.DisplayName,
		TemplateID:  config.TemplateID,
		Version:     config.Version,
		Status:      "restored",
		InstallTime: time.Now(),
		Ports:       config.Ports,
		Volumes:     config.Volumes,
		Environment: config.Environment,
		ComposePath: composePath,
	}

	// 保存到安装列表
	bm.store.mu.Lock()
	bm.store.installed[config.AppID] = app
	_ = bm.store.saveInstalled()
	bm.store.mu.Unlock()

	return app, nil
}

// RestoreOptions 恢复选项.
type RestoreOptions struct {
	ForceRestore      bool `json:"forceRestore"`
	RestoreData       bool `json:"restoreData"`
	StartAfterRestore bool `json:"startAfterRestore"`
}

// ListBackups 列出所有备份.
func (bm *BackupManager) ListBackups(appID string) ([]*BackupInfo, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var backups []*BackupInfo

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		infoPath := filepath.Join(bm.backupDir, entry.Name(), "backup.json")
		data, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}

		var info BackupInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if appID == "" || info.AppID == appID {
			backups = append(backups, &info)
		}
	}

	// 按时间排序（最新的在前）
	sortBackupsByTime(backups)

	return backups, nil
}

// GetBackup 获取备份信息.
func (bm *BackupManager) GetBackup(backupID string) (*BackupInfo, error) {
	infoPath := filepath.Join(bm.backupDir, backupID, "backup.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info BackupInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// DeleteBackup 删除备份.
func (bm *BackupManager) DeleteBackup(backupID string) error {
	backupPath := filepath.Join(bm.backupDir, backupID)
	return os.RemoveAll(backupPath)
}

// calculateChecksum 计算备份目录校验和.
func (bm *BackupManager) calculateChecksum(backupPath string) (string, error) {
	// 简单实现：遍历文件计算 MD5
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("find %s -type f -exec md5sum {} \\; | sort | md5sum | cut -d' ' -f1", backupPath))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// sortBackupsByTime 按时间排序备份.
func sortBackupsByTime(backups []*BackupInfo) {
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].CreateTime.Before(backups[j].CreateTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}
}

// containsStr 检查字符串是否在切片中.
func containsStr(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// =============================================================================
// 应用健康检查
// =============================================================================

// HealthStatus 健康状态.
type HealthStatus struct {
	AppID        string        `json:"appId"`
	AppName      string        `json:"appName"`
	Status       string        `json:"status"` // healthy, unhealthy, starting, stopped
	LastCheck    time.Time     `json:"lastCheck"`
	Checks       []HealthCheck `json:"checks"`
	Uptime       time.Duration `json:"uptime"`
	RestartCount int           `json:"restartCount"`
	LastRestart  time.Time     `json:"lastRestart"`
	Message      string        `json:"message"`
}

// HealthCheck 健康检查项.
type HealthCheck struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // http, tcp, exec, container
	Status   string        `json:"status"`
	Message  string        `json:"message"`
	Latency  time.Duration `json:"latency"`
	Endpoint string        `json:"endpoint,omitempty"`
}

// HealthChecker 健康检查器.
type HealthChecker struct {
	store   *AppStore
	manager *Manager
	config  HealthCheckConfig
}

// HealthCheckConfig 健康检查配置.
type HealthCheckConfig struct {
	CheckInterval      time.Duration `json:"checkInterval"`
	Timeout            time.Duration `json:"timeout"`
	HealthyThreshold   int           `json:"healthyThreshold"`
	UnhealthyThreshold int           `json:"unhealthyThreshold"`
}

// DefaultHealthCheckConfig 默认健康检查配置.
var DefaultHealthCheckConfig = HealthCheckConfig{
	CheckInterval:      30 * time.Second,
	Timeout:            10 * time.Second,
	HealthyThreshold:   2,
	UnhealthyThreshold: 3,
}

// NewHealthChecker 创建健康检查器.
func NewHealthChecker(store *AppStore, mgr *Manager, config HealthCheckConfig) *HealthChecker {
	if config.CheckInterval == 0 {
		config = DefaultHealthCheckConfig
	}

	return &HealthChecker{
		store:   store,
		manager: mgr,
		config:  config,
	}
}

// CheckHealth 检查应用健康状态.
func (hc *HealthChecker) CheckHealth(appID string) (*HealthStatus, error) {
	app := hc.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	status := &HealthStatus{
		AppID:     appID,
		AppName:   app.DisplayName,
		Status:    "unknown",
		LastCheck: time.Now(),
		Checks:    []HealthCheck{},
	}

	// 获取容器信息
	container, err := hc.manager.GetContainer(app.Name)
	if err != nil {
		status.Status = "stopped"
		status.Message = "容器不存在或已停止"
		return status, nil
	}

	// 更新运行时间
	if !container.Created.IsZero() {
		status.Uptime = time.Since(container.Created)
	}

	// 1. 容器状态检查
	containerCheck := hc.checkContainerStatus(container)
	status.Checks = append(status.Checks, containerCheck)

	// 2. 端口检查
	if len(app.Ports) > 0 {
		portChecks := hc.checkPorts(app.Ports)
		status.Checks = append(status.Checks, portChecks...)
	}

	// 3. HTTP 健康检查（如果有 Web 端口）
	if webCheck := hc.checkHTTPEndpoint(app); webCheck != nil {
		status.Checks = append(status.Checks, *webCheck)
	}

	// 4. 资源检查
	resourceCheck := hc.checkResources(container)
	status.Checks = append(status.Checks, resourceCheck)

	// 汇总状态
	status.Status = hc.aggregateStatus(status.Checks)
	status.Message = hc.getStatusMessage(status.Checks)

	return status, nil
}

// checkContainerStatus 检查容器状态.
func (hc *HealthChecker) checkContainerStatus(container *Container) HealthCheck {
	check := HealthCheck{
		Name: "容器状态",
		Type: "container",
	}

	if container.State == "running" {
		check.Status = "healthy"
		check.Message = fmt.Sprintf("容器运行中，状态: %s", container.Status)
	} else {
		check.Status = "unhealthy"
		check.Message = fmt.Sprintf("容器未运行，状态: %s", container.State)
	}

	return check
}

// checkPorts 检查端口.
func (hc *HealthChecker) checkPorts(ports map[int]int) []HealthCheck {
	checks := make([]HealthCheck, 0, len(ports))

	for _, hostPort := range ports {
		check := HealthCheck{
			Name:     fmt.Sprintf("端口 %d", hostPort),
			Type:     "tcp",
			Endpoint: fmt.Sprintf("127.0.0.1:%d", hostPort),
		}

		start := time.Now()

		// 尝试连接端口
		dialer := &net.Dialer{Timeout: hc.config.Timeout}
		ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
		conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", hostPort))
		cancel()
		check.Latency = time.Since(start)

		if err != nil {
			check.Status = "unhealthy"
			check.Message = fmt.Sprintf("端口 %d 无法连接", hostPort)
		} else {
			check.Status = "healthy"
			check.Message = fmt.Sprintf("端口 %d 正常响应 (%.0fms)", hostPort, float64(check.Latency.Milliseconds()))
			_ = conn.Close()
		}

		checks = append(checks, check)
	}

	return checks
}

// checkHTTPEndpoint 检查 HTTP 端点.
func (hc *HealthChecker) checkHTTPEndpoint(app *InstalledApp) *HealthCheck {
	// 查找可能的 Web 端口
	var webPort int
	for containerPort, hostPort := range app.Ports {
		if containerPort == 80 || containerPort == 443 || containerPort == 8080 ||
			containerPort == 3000 || containerPort == 8000 || containerPort == 5000 {
			webPort = hostPort
			break
		}
	}

	if webPort == 0 {
		return nil
	}

	check := HealthCheck{
		Name:     "HTTP 健康检查",
		Type:     "http",
		Endpoint: fmt.Sprintf("http://127.0.0.1:%d/", webPort),
	}

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.Endpoint, nil)
	if err != nil {
		check.Status = "unhealthy"
		check.Message = fmt.Sprintf("创建请求失败: %v", err)
		return &check
	}

	client := &http.Client{
		Timeout: hc.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = "unhealthy"
		check.Message = fmt.Sprintf("HTTP 请求失败: %v", err)
	} else {
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			check.Status = "healthy"
			check.Message = fmt.Sprintf("HTTP %d (%.0fms)", resp.StatusCode, float64(check.Latency.Milliseconds()))
		} else {
			check.Status = "degraded"
			check.Message = fmt.Sprintf("HTTP %d 响应异常", resp.StatusCode)
		}
	}

	return &check
}

// checkResources 检查资源使用.
func (hc *HealthChecker) checkResources(container *Container) HealthCheck {
	check := HealthCheck{
		Name: "资源使用",
		Type: "resource",
	}

	// CPU 使用率检查
	if container.CPUUsage > 80 {
		check.Status = "degraded"
		check.Message = fmt.Sprintf("CPU 使用率过高: %.1f%%", container.CPUUsage)
		return check
	}

	// 内存使用检查
	if container.MemLimit > 0 {
		memUsagePercent := float64(container.MemUsage) / float64(container.MemLimit) * 100
		if memUsagePercent > 90 {
			check.Status = "degraded"
			check.Message = fmt.Sprintf("内存使用率过高: %.1f%%", memUsagePercent)
			return check
		}
	}

	check.Status = "healthy"
	check.Message = fmt.Sprintf("CPU: %.1f%%, 内存: %s/%s",
		container.CPUUsage,
		formatBytes(container.MemUsage),
		formatBytes(container.MemLimit))

	return check
}

// aggregateStatus 汇总状态.
func (hc *HealthChecker) aggregateStatus(checks []HealthCheck) string {
	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case "unhealthy":
			hasUnhealthy = true
		case "degraded":
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return "unhealthy"
	}
	if hasDegraded {
		return "degraded"
	}
	return "healthy"
}

// getStatusMessage 获取状态消息.
func (hc *HealthChecker) getStatusMessage(checks []HealthCheck) string {
	var unhealthy []string
	for _, check := range checks {
		if check.Status == "unhealthy" {
			unhealthy = append(unhealthy, check.Name)
		}
	}

	if len(unhealthy) == 0 {
		return "所有检查项正常"
	}

	return fmt.Sprintf("异常检查项: %s", strings.Join(unhealthy, ", "))
}

// formatBytes 格式化字节.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// StartPeriodicCheck 启动定期健康检查.
func (hc *HealthChecker) StartPeriodicCheck() chan *HealthStatus {
	results := make(chan *HealthStatus, 100)

	go func() {
		ticker := time.NewTicker(hc.config.CheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			installed := hc.store.ListInstalled()
			for _, app := range installed {
				if status, err := hc.CheckHealth(app.ID); err == nil {
					select {
					case results <- status:
					default:
						// 通道满了，跳过
					}
				}
			}
		}
	}()

	return results
}
