package docker

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ManualInstallRequest 手动安装请求.
type ManualInstallRequest struct {
	// 安装方式: "compose" 或 "image"
	Type string `json:"type"`

	// Compose 方式
	ComposeContent string `json:"composeContent,omitempty"` // docker-compose.yml 内容
	ComposeURL     string `json:"composeUrl,omitempty"`     // docker-compose.yml URL

	// Image 方式
	Image       string             `json:"image,omitempty"`       // Docker 镜像名
	Tag         string             `json:"tag,omitempty"`         // 镜像标签，默认 latest
	Name        string             `json:"name,omitempty"`        // 容器名称
	Ports       []PortMappingReq   `json:"ports,omitempty"`       // 端口映射
	Volumes     []VolumeMappingReq `json:"volumes,omitempty"`     // 卷映射
	Environment map[string]string  `json:"environment,omitempty"` // 环境变量
	Network     string             `json:"network,omitempty"`     // 网络模式
	Restart     string             `json:"restart,omitempty"`     // 重启策略

	// 元数据
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// PortMappingReq 端口映射请求.
type PortMappingReq struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"` // tcp 或 udp，默认 tcp
}

// VolumeMappingReq 卷映射请求.
type VolumeMappingReq struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly,omitempty"`
}

// ManualInstallResult 手动安装结果.
type ManualInstallResult struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName"`
	Status       string            `json:"status"`
	Type         string            `json:"type"` // compose 或 image
	Ports        map[int]int       `json:"ports"`
	Volumes      map[string]string `json:"volumes"`
	Environment  map[string]string `json:"environment"`
	InstallTime  time.Time         `json:"installTime"`
	ComposePath  string            `json:"composePath,omitempty"`
	ContainerID  string            `json:"containerId,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"` // 自动安装的依赖
}

// DependencyDetector 依赖检测器.
type DependencyDetector struct {
	manager *Manager
}

// NewDependencyDetector 创建依赖检测器.
func NewDependencyDetector(mgr *Manager) *DependencyDetector {
	return &DependencyDetector{manager: mgr}
}

// DetectFromCompose 从 docker-compose 检测依赖.
func (dd *DependencyDetector) DetectFromCompose(composeContent string) ([]string, error) {
	var dependencies []string

	// 解析服务依赖
	// 检查 depends_on 字段
	lines := strings.Split(composeContent, "\n")
	inDependsOn := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "depends_on:") {
			inDependsOn = true
			continue
		}
		if inDependsOn {
			if strings.HasPrefix(trimmed, "-") {
				// 列表格式
				dep := strings.TrimPrefix(trimmed, "-")
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			} else if !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") {
				// 结束 depends_on 块
				inDependsOn = false
			}
		}
	}

	return dependencies, nil
}

// DetectFromImage 从镜像检测依赖.
func (dd *DependencyDetector) DetectFromImage(image string) ([]string, error) {
	// 常见镜像的依赖映射
	knownDependencies := map[string][]string{
		"immich":             {"postgres", "redis"},
		"nextcloud":          {"postgres", "redis"},
		"gitea":              {"postgres", "mysql"},
		"gitlab":             {"postgres", "redis"},
		"grafana":            {"postgres", "mysql"},
		"prometheus":         {"alertmanager"},
		"wordpress":          {"mysql", "postgres"},
		"drone/drone":        {"postgres", "mysql"},
		"drone/drone-runner": {"docker"},
	}

	// 检查镜像名
	imageLower := strings.ToLower(image)
	for base, deps := range knownDependencies {
		if strings.Contains(imageLower, base) {
			return deps, nil
		}
	}

	return nil, nil
}

// ManualInstaller 手动安装器.
type ManualInstaller struct {
	store              *AppStore
	manager            *Manager
	dependencyDetector *DependencyDetector
	installDir         string
}

// NewManualInstaller 创建手动安装器.
func NewManualInstaller(store *AppStore, mgr *Manager, dataDir string) *ManualInstaller {
	installDir := filepath.Join(dataDir, "manual-apps")
	_ = os.MkdirAll(installDir, 0750)

	return &ManualInstaller{
		store:              store,
		manager:            mgr,
		dependencyDetector: NewDependencyDetector(mgr),
		installDir:         installDir,
	}
}

// Install 手动安装应用.
func (mi *ManualInstaller) Install(req *ManualInstallRequest) (*ManualInstallResult, error) {
	switch req.Type {
	case "compose":
		return mi.installFromCompose(req)
	case "image":
		return mi.installFromImage(req)
	default:
		return nil, fmt.Errorf("不支持的安装类型: %s", req.Type)
	}
}

// installFromCompose 从 docker-compose 安装.
func (mi *ManualInstaller) installFromCompose(req *ManualInstallRequest) (*ManualInstallResult, error) {
	if req.ComposeContent == "" && req.ComposeURL == "" {
		return nil, fmt.Errorf("需要提供 compose 内容或 URL")
	}

	// 获取 compose 内容
	composeContent := req.ComposeContent
	if composeContent == "" && req.ComposeURL != "" {
		content, err := mi.fetchComposeFromURL(req.ComposeURL)
		if err != nil {
			return nil, fmt.Errorf("获取 compose 文件失败: %w", err)
		}
		composeContent = content
	}

	// 检测依赖
	dependencies, err := mi.dependencyDetector.DetectFromCompose(composeContent)
	if err != nil {
		log.Printf("依赖检测失败: %v", err)
	}

	// 生成应用名称
	appName := req.Name
	if appName == "" {
		appName = mi.extractAppNameFromCompose(composeContent)
	}
	if appName == "" {
		appName = fmt.Sprintf("manual-%d", time.Now().Unix())
	}

	// 创建应用目录
	appDir := filepath.Join(mi.installDir, appName)
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return nil, fmt.Errorf("创建应用目录失败: %w", err)
	}

	// 保存 compose 文件
	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0640); err != nil {
		return nil, fmt.Errorf("保存 compose 文件失败: %w", err)
	}

	// 保存元数据
	displayName := req.DisplayName
	if displayName == "" {
		displayName = appName
	}
	meta := &ManualAppMeta{
		Name:        appName,
		DisplayName: displayName,
		Description: req.Description,
		Category:    req.Category,
		Icon:        req.Icon,
		InstallTime: time.Now(),
		Type:        "compose",
	}
	if err := mi.saveMeta(appDir, meta); err != nil {
		log.Printf("保存元数据失败: %v", err)
	}

	// 启动应用
	cmd := exec.Command("docker-compose", "-f", composePath, "up", "-d")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("启动应用失败: %w, %s", err, string(output))
	}

	// 获取容器信息
	containers, _ := mi.manager.ListContainers(false)
	var containerID string
	var ports = make(map[int]int)
	var volumes = make(map[string]string)

	for _, c := range containers {
		if c.Name == appName || strings.HasPrefix(c.Name, appName+"-") || strings.Contains(c.Name, appName) {
			containerID = c.ID
			for _, p := range c.Ports {
				ports[parseInt(p.ContainerPort)] = parseInt(p.HostPort)
			}
			for _, v := range c.Volumes {
				volumes[v.Destination] = v.Source
			}
			break
		}
	}

	// 注册到已安装应用
	installedApp := &InstalledApp{
		ID:          "manual-" + appName,
		Name:        appName,
		DisplayName: displayName,
		TemplateID:  "manual",
		Version:     "custom",
		Status:      "running",
		InstallTime: time.Now(),
		Ports:       ports,
		Volumes:     volumes,
		Environment: make(map[string]string),
		ContainerID: containerID,
		ComposePath: composePath,
	}

	// 添加到 store 的 installed 列表
	mi.store.mu.Lock()
	mi.store.installed[installedApp.ID] = installedApp
	mi.store.mu.Unlock()

	return &ManualInstallResult{
		ID:           installedApp.ID,
		Name:         appName,
		DisplayName:  displayName,
		Status:       "running",
		Type:         "compose",
		Ports:        ports,
		Volumes:      volumes,
		Environment:  make(map[string]string),
		InstallTime:  time.Now(),
		ComposePath:  composePath,
		ContainerID:  containerID,
		Dependencies: dependencies,
	}, nil
}

// installFromImage 从 Docker 镜像安装.
func (mi *ManualInstaller) installFromImage(req *ManualInstallRequest) (*ManualInstallResult, error) {
	if req.Image == "" {
		return nil, fmt.Errorf("镜像名称不能为空")
	}

	// 处理镜像标签
	image := req.Image
	tag := req.Tag
	if tag == "" {
		tag = "latest"
	}
	if !strings.Contains(image, ":") {
		image = image + ":" + tag
	}

	// 容器名称
	name := req.Name
	if name == "" {
		// 从镜像名提取
		parts := strings.Split(image, "/")
		name = strings.Split(parts[len(parts)-1], ":")[0]
		name = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	}

	// 检测依赖
	dependencies, err := mi.dependencyDetector.DetectFromImage(req.Image)
	if err != nil {
		log.Printf("依赖检测失败: %v", err)
	}

	// 创建应用目录
	appDir := filepath.Join(mi.installDir, name)
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return nil, fmt.Errorf("创建应用目录失败: %w", err)
	}

	// 拉取镜像
	log.Printf("拉取镜像: %s", image)
	if err := mi.manager.PullImage(image); err != nil {
		return nil, fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 构建容器选项
	opts := make(map[string]interface{})

	// 端口映射
	var portMappings []string
	ports := make(map[int]int)
	for _, p := range req.Ports {
		protocol := p.Protocol
		if protocol == "" {
			protocol = "tcp"
		}
		mapping := fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, protocol)
		portMappings = append(portMappings, mapping)
		ports[p.ContainerPort] = p.HostPort
	}
	opts["ports"] = portMappings

	// 卷映射
	var volumeMappings []string
	volumes := make(map[string]string)
	for _, v := range req.Volumes {
		mapping := fmt.Sprintf("%s:%s", v.HostPath, v.ContainerPath)
		if v.ReadOnly {
			mapping += ":ro"
		}
		volumeMappings = append(volumeMappings, mapping)
		volumes[v.ContainerPath] = v.HostPath
	}
	opts["volumes"] = volumeMappings

	// 环境变量
	opts["env"] = req.Environment

	// 网络模式
	if req.Network != "" {
		opts["network"] = req.Network
	}

	// 重启策略
	restart := req.Restart
	if restart == "" {
		restart = "unless-stopped"
	}
	opts["restart"] = restart

	// 创建容器
	container, err := mi.manager.CreateContainer(name, image, opts)
	if err != nil {
		return nil, fmt.Errorf("创建容器失败: %w", err)
	}

	// 保存元数据
	displayName := req.DisplayName
	if displayName == "" {
		displayName = name
	}
	meta := &ManualAppMeta{
		Name:        name,
		DisplayName: displayName,
		Description: req.Description,
		Category:    req.Category,
		Icon:        req.Icon,
		Image:       image,
		InstallTime: time.Now(),
		Type:        "image",
	}
	if err := mi.saveMeta(appDir, meta); err != nil {
		log.Printf("保存元数据失败: %v", err)
	}

	// 注册到已安装应用
	installedApp := &InstalledApp{
		ID:          "manual-" + name,
		Name:        name,
		DisplayName: displayName,
		TemplateID:  "manual",
		Version:     tag,
		Status:      "running",
		InstallTime: time.Now(),
		Ports:       ports,
		Volumes:     volumes,
		Environment: req.Environment,
		ContainerID: container.ID,
	}

	mi.store.mu.Lock()
	mi.store.installed[installedApp.ID] = installedApp
	mi.store.mu.Unlock()

	return &ManualInstallResult{
		ID:           installedApp.ID,
		Name:         name,
		DisplayName:  displayName,
		Status:       "running",
		Type:         "image",
		Ports:        ports,
		Volumes:      volumes,
		Environment:  req.Environment,
		InstallTime:  time.Now(),
		ContainerID:  container.ID,
		Dependencies: dependencies,
	}, nil
}

// ManualAppMeta 手动安装应用元数据.
type ManualAppMeta struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Image       string            `json:"image,omitempty"`
	InstallTime time.Time         `json:"installTime"`
	Type        string            `json:"type"` // compose 或 image
	Environment map[string]string `json:"environment,omitempty"`
}

// saveMeta 保存元数据.
func (mi *ManualInstaller) saveMeta(appDir string, meta *ManualAppMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, "meta.json"), data, 0640)
}

// fetchComposeFromURL 从 URL 获取 compose 内容.
func (mi *ManualInstaller) fetchComposeFromURL(url string) (string, error) {
	// 支持的 URL 格式:
	// - 直接的 docker-compose.yml URL
	// - GitHub 仓库 URL (自动查找 docker-compose.yml)

	if strings.Contains(url, "github.com") {
		return mi.fetchFromGitHub(url)
	}

	// 直接下载
	resp, err := fetchURL(url)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// fetchFromGitHub 从 GitHub 获取 compose.
func (mi *ManualInstaller) fetchFromGitHub(githubURL string) (string, error) {
	// 解析 GitHub URL
	// 格式: https://github.com/owner/repo 或 https://github.com/owner/repo/tree/branch/path
	var owner, repo, branch, path string

	// 移除协议前缀
	url := strings.TrimPrefix(githubURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	parts := strings.Split(url, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("无效的 GitHub URL")
	}

	owner = parts[1]
	repo = parts[2]

	// 检查是否有分支和路径
	if len(parts) > 4 && parts[3] == "tree" {
		branch = parts[4]
		if len(parts) > 5 {
			path = strings.Join(parts[5:], "/")
		}
	}

	if branch == "" {
		branch = "main"
	}

	// 尝试不同的 compose 文件名
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	if path != "" {
		// 如果指定了路径，优先尝试该路径
		composeFiles = append([]string{path + "/docker-compose.yml", path + "/docker-compose.yaml"}, composeFiles...)
	}

	for _, file := range composeFiles {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, file)
		content, err := fetchURL(rawURL)
		if err == nil {
			return content, nil
		}
	}

	return "", fmt.Errorf("未找到 docker-compose 文件")
}

// extractAppNameFromCompose 从 compose 内容提取应用名.
func (mi *ManualInstaller) extractAppNameFromCompose(content string) string {
	lines := strings.Split(content, "\n")

	// 首先查找 container_name（优先级更高）
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "container_name:") {
			// 找到 container_name
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[1])
				name = strings.Trim(name, "\"'")
				return name
			}
		}
	}

	// 如果没有 container_name，尝试从服务名获取
	for i, line := range lines {
		if i > 0 && strings.HasSuffix(lines[i-1], ":") && !strings.HasPrefix(strings.TrimSpace(lines[i-1]), "#") {
			// 服务名
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "image:") {
				// 上一行可能是服务名
				prevLine := strings.TrimSpace(lines[i-1])
				if strings.HasSuffix(prevLine, ":") && !strings.Contains(prevLine, " ") && !strings.Contains(prevLine, "services") && !strings.Contains(prevLine, "version") {
					return strings.TrimSuffix(prevLine, ":")
				}
			}
		}
	}
	return ""
}

// LatestAppsResponse 最新应用列表响应.
type LatestAppsResponse struct {
	Trending   []*AppTemplate `json:"trending"`   // 热门应用
	New        []*AppTemplate `json:"new"`        // 新上架
	Updated    []*AppTemplate `json:"updated"`    // 最近更新
	Categories map[string]int `json:"categories"` // 分类统计
}

// GetLatestApps 获取最新应用列表.
func (mi *ManualInstaller) GetLatestApps() (*LatestAppsResponse, error) {
	// 从 store 获取模板
	templates := mi.store.ListTemplates()

	// 统计分类
	categories := make(map[string]int)
	for _, t := range templates {
		categories[t.Category]++
	}

	// 按评分/下载量排序（模拟）
	trending := append([]*AppTemplate{}, templates...)
	newApps := make([]*AppTemplate, 0)
	updated := make([]*AppTemplate, 0)

	// 截取前 10
	if len(trending) > 10 {
		trending = trending[:10]
	}

	return &LatestAppsResponse{
		Trending:   trending,
		New:        newApps,
		Updated:    updated,
		Categories: categories,
	}, nil
}

// Helper functions

func fetchURL(url string) (string, error) {
	cmd := exec.Command("curl", "-sL", url)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	return string(output), nil
}

func parseInt(s string) int {
	var i int
	_, _ = fmt.Sscanf(s, "%d", &i)
	return i
}
