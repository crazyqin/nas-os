package docker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CustomTemplate 自定义模板
type CustomTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Source      string            `json:"source"` // github, url, upload
	SourceURL   string            `json:"sourceUrl"`
	Compose     string            `json:"compose"`
	Ports       []PortConfig      `json:"ports"`
	Volumes     []VolumeConfig    `json:"volumes"`
	Environment map[string]string `json:"environment"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Verified    bool              `json:"verified"` // 是否验证过
	Downloads   int               `json:"downloads"`
}

// ComposeService Docker Compose 服务定义
type ComposeService struct {
	Image       string            `yaml:"image"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Environment map[string]string `yaml:"environment"`
	Restart     string            `yaml:"restart"`
	ContainerName string          `yaml:"container_name"`
}

// ComposeFile Docker Compose 文件
type ComposeFile struct {
	Version  string                   `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
}

// CustomTemplateManager 自定义模板管理器
type CustomTemplateManager struct {
	store      *AppStore
	templateDir string
	templates  map[string]*CustomTemplate
}

// NewCustomTemplateManager 创建自定义模板管理器
func NewCustomTemplateManager(store *AppStore, dataDir string) (*CustomTemplateManager, error) {
	templateDir := filepath.Join(dataDir, "custom-templates")
	
	ctm := &CustomTemplateManager{
		store:       store,
		templateDir: templateDir,
		templates:   make(map[string]*CustomTemplate),
	}
	
	// 创建目录
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return nil, err
	}
	
	// 加载已保存的模板
	if err := ctm.loadTemplates(); err != nil {
		fmt.Printf("加载自定义模板失败: %v\n", err)
	}
	
	return ctm, nil
}

// loadTemplates 加载模板
func (ctm *CustomTemplateManager) loadTemplates() error {
	files, err := os.ReadDir(ctm.templateDir)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		path := filepath.Join(ctm.templateDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		
		var template CustomTemplate
		if err := json.Unmarshal(data, &template); err != nil {
			continue
		}
		
		ctm.templates[template.ID] = &template
	}
	
	return nil
}

// saveTemplate 保存模板
func (ctm *CustomTemplateManager) saveTemplate(template *CustomTemplate) error {
	path := filepath.Join(ctm.templateDir, template.ID+".json")
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ParseCompose 解析 Docker Compose 文件
func (ctm *CustomTemplateManager) ParseCompose(content string) (*ComposeFile, error) {
	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(content), &compose); err != nil {
		return nil, fmt.Errorf("解析 Docker Compose 失败: %w", err)
	}
	
	if len(compose.Services) == 0 {
		return nil, fmt.Errorf("Docker Compose 文件中没有定义服务")
	}
	
	return &compose, nil
}

// CreateFromCompose 从 Docker Compose 创建模板
func (ctm *CustomTemplateManager) CreateFromCompose(name, displayName, description, category, composeContent string) (*CustomTemplate, error) {
	// 解析 Compose 文件
	compose, err := ctm.ParseCompose(composeContent)
	if err != nil {
		return nil, err
	}
	
	// 提取端口
	var ports []PortConfig
	portRegex := regexp.MustCompile(`^(\d+):(\d+)(?:/(tcp|udp))?$`)
	
	for _, service := range compose.Services {
		for _, portStr := range service.Ports {
			matches := portRegex.FindStringSubmatch(portStr)
			if len(matches) >= 3 {
				var protocol string
				if len(matches) == 4 {
					protocol = matches[3]
				} else {
					protocol = "tcp"
				}
				ports = append(ports, PortConfig{
					Port:        parseInt(matches[2]),
					Protocol:    protocol,
					Description: "服务端口",
					Default:     parseInt(matches[1]),
				})
			}
		}
	}
	
	// 提取卷
	var volumes []VolumeConfig
	for _, service := range compose.Services {
		for _, volStr := range service.Volumes {
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				volumes = append(volumes, VolumeConfig{
					ContainerPath: parts[1],
					Description:   "数据卷",
					Default:       parts[0],
				})
			}
		}
	}
	
	// 创建模板
	template := &CustomTemplate{
		ID:          fmt.Sprintf("custom-%d", time.Now().UnixNano()),
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Category:    category,
		Icon:        "📦",
		Version:     "1.0.0",
		Author:      "user",
		Source:      "upload",
		Compose:     composeContent,
		Ports:       ports,
		Volumes:     volumes,
		Environment: extractEnvFromCompose(compose),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	ctm.templates[template.ID] = template
	if err := ctm.saveTemplate(template); err != nil {
		return nil, err
	}
	
	return template, nil
}

// CreateFromURL 从 URL 创建模板
func (ctm *CustomTemplateManager) CreateFromURL(name, displayName, description, category, url string) (*CustomTemplate, error) {
	// 获取内容
	resp, err := getHTTPClient().Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取 URL 失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("URL 返回状态码: %d", resp.StatusCode)
	}
	
	var buf strings.Builder
	buf.ReadFrom(resp.Body)
	composeContent := buf.String()
	
	template, err := ctm.CreateFromCompose(name, displayName, description, category, composeContent)
	if err != nil {
		return nil, err
	}
	
	template.Source = "url"
	template.SourceURL = url
	
	ctm.templates[template.ID] = template
	ctm.saveTemplate(template)
	
	return template, nil
}

// getHTTPClient 获取 HTTP 客户端
func getHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// ImportFromGitHub 从 GitHub 导入
func (ctm *CustomTemplateManager) ImportFromGitHub(repoURL, name, displayName, description, category string) (*CustomTemplate, error) {
	// 解析 GitHub URL
	// 格式: https://github.com/owner/repo 或 https://github.com/owner/repo/tree/branch/path
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)(?:/tree/([^/]+)/(.+))?`)
	matches := re.FindStringSubmatch(repoURL)
	
	if len(matches) < 3 {
		return nil, fmt.Errorf("无效的 GitHub URL")
	}
	
	owner := matches[1]
	repo := matches[2]
	branch := "main"
	path := ""
	
	if len(matches) >= 5 && matches[3] != "" {
		branch = matches[3]
		path = matches[4]
	}
	
	// 尝试获取 docker-compose 文件
	filenames := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	
	for _, filename := range filenames {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s%s", owner, repo, branch, path, filename)
		if path != "" {
			rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/%s", owner, repo, branch, path, filename)
		}
		
		resp, err := getHTTPClient().Get(rawURL)
		if err != nil {
			continue
		}
		
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			var buf strings.Builder
			buf.ReadFrom(resp.Body)
			
			template, err := ctm.CreateFromCompose(name, displayName, description, category, buf.String())
			if err != nil {
				resp.Body.Close()
				continue
			}
			
			template.Source = "github"
			template.SourceURL = repoURL
			ctm.templates[template.ID] = template
			ctm.saveTemplate(template)
			
			return template, nil
		}
		resp.Body.Close()
	}
	
	return nil, fmt.Errorf("未找到 docker-compose 文件")
}

// ListTemplates 列出所有自定义模板
func (ctm *CustomTemplateManager) ListTemplates() []*CustomTemplate {
	result := make([]*CustomTemplate, 0, len(ctm.templates))
	for _, t := range ctm.templates {
		result = append(result, t)
	}
	return result
}

// GetTemplate 获取模板
func (ctm *CustomTemplateManager) GetTemplate(id string) *CustomTemplate {
	return ctm.templates[id]
}

// UpdateTemplate 更新模板
func (ctm *CustomTemplateManager) UpdateTemplate(id string, updates map[string]interface{}) (*CustomTemplate, error) {
	template, ok := ctm.templates[id]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", id)
	}
	
	if displayName, ok := updates["displayName"].(string); ok {
		template.DisplayName = displayName
	}
	if description, ok := updates["description"].(string); ok {
		template.Description = description
	}
	if category, ok := updates["category"].(string); ok {
		template.Category = category
	}
	if icon, ok := updates["icon"].(string); ok {
		template.Icon = icon
	}
	if compose, ok := updates["compose"].(string); ok {
		// 重新解析 Compose
		parsed, err := ctm.ParseCompose(compose)
		if err != nil {
			return nil, err
		}
		template.Compose = compose
		template.Ports = extractPortsFromCompose(parsed)
		template.Volumes = extractVolumesFromCompose(parsed)
	}
	
	template.UpdatedAt = time.Now()
	
	if err := ctm.saveTemplate(template); err != nil {
		return nil, err
	}
	
	return template, nil
}

// DeleteTemplate 删除模板
func (ctm *CustomTemplateManager) DeleteTemplate(id string) error {
	if _, ok := ctm.templates[id]; !ok {
		return fmt.Errorf("模板不存在: %s", id)
	}
	
	delete(ctm.templates, id)
	
	path := filepath.Join(ctm.templateDir, id+".json")
	return os.Remove(path)
}

// ConvertToAppTemplate 转换为应用模板
func (ctm *CustomTemplateManager) ConvertToAppTemplate(custom *CustomTemplate) *AppTemplate {
	return &AppTemplate{
		ID:          custom.ID,
		Name:        custom.Name,
		DisplayName: custom.DisplayName,
		Description: custom.Description,
		Category:    custom.Category,
		Icon:        custom.Icon,
		Version:     custom.Version,
		Image:       extractImageFromCompose(custom.Compose),
		Ports:       custom.Ports,
		Volumes:     custom.Volumes,
		Environment: custom.Environment,
		Compose:     custom.Compose,
		Notes:       fmt.Sprintf("来源: %s", custom.Source),
		Source:      custom.SourceURL,
	}
}

// IncrementDownloads 增加下载次数
func (ctm *CustomTemplateManager) IncrementDownloads(id string) {
	if template, ok := ctm.templates[id]; ok {
		template.Downloads++
		ctm.saveTemplate(template)
	}
}

// 辅助函数

func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func extractEnvFromCompose(compose *ComposeFile) map[string]string {
	env := make(map[string]string)
	for _, service := range compose.Services {
		for k, v := range service.Environment {
			env[k] = v
		}
	}
	return env
}

func extractPortsFromCompose(compose *ComposeFile) []PortConfig {
	var ports []PortConfig
	portRegex := regexp.MustCompile(`^(\d+):(\d+)(?:/(tcp|udp))?$`)
	
	for _, service := range compose.Services {
		for _, portStr := range service.Ports {
			matches := portRegex.FindStringSubmatch(portStr)
			if len(matches) >= 3 {
				var protocol string
				if len(matches) == 4 {
					protocol = matches[3]
				} else {
					protocol = "tcp"
				}
				ports = append(ports, PortConfig{
					Port:        parseInt(matches[2]),
					Protocol:    protocol,
					Description: "服务端口",
					Default:     parseInt(matches[1]),
				})
			}
		}
	}
	return ports
}

func extractVolumesFromCompose(compose *ComposeFile) []VolumeConfig {
	var volumes []VolumeConfig
	for _, service := range compose.Services {
		for _, volStr := range service.Volumes {
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				volumes = append(volumes, VolumeConfig{
					ContainerPath: parts[1],
					Description:   "数据卷",
					Default:       parts[0],
				})
			}
		}
	}
	return volumes
}

func extractImageFromCompose(composeContent string) string {
	var compose ComposeFile
	if err := yaml.Unmarshal([]byte(composeContent), &compose); err != nil {
		return ""
	}
	
	for _, service := range compose.Services {
		if service.Image != "" {
			return service.Image
		}
	}
	return ""
}