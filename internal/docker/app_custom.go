package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CustomTemplate 自定义应用模板.
type CustomTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Compose     string    `json:"compose"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Source      string    `json:"source"` // "compose", "url", "github"
	SourceURL   string    `json:"source_url,omitempty"`
}

// CustomTemplateManager 自定义模板管理器.
type CustomTemplateManager struct {
	mu           sync.RWMutex
	templates    map[string]*CustomTemplate
	templatesDir string
}

// NewCustomTemplateManager 创建自定义模板管理器.
func NewCustomTemplateManager(templatesDir string) (*CustomTemplateManager, error) {
	ctm := &CustomTemplateManager{
		templates:    make(map[string]*CustomTemplate),
		templatesDir: templatesDir,
	}

	if templatesDir != "" {
		if err := os.MkdirAll(templatesDir, 0750); err != nil {
			return nil, err
		}
		if err := ctm.loadTemplates(); err != nil {
			return nil, err
		}
	}

	return ctm, nil
}

// ImportFromURL 从 URL 导入应用模板.
func (ctm *CustomTemplateManager) ImportFromURL(url, name, displayName, description, category string) (*CustomTemplate, error) {
	// 下载 compose 文件
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("URL 返回状态码：%d", resp.StatusCode)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}
	composeContent := buf.String()

	template, err := ctm.CreateFromCompose(name, displayName, description, category, composeContent)
	if err != nil {
		return nil, err
	}

	template.Source = "url"
	template.SourceURL = url

	ctm.templates[template.ID] = template
	if err := ctm.saveTemplate(template); err != nil {
		return nil, fmt.Errorf("保存模板失败：%w", err)
	}

	return template, nil
}

// getHTTPClient 获取 HTTP 客户端 - 保留用于未来需要 HTTP 请求的场景
// func getHTTPClient() *http.Client {
// 	return &http.Client{
// 		Timeout: 30 * time.Second,
// 	}
// }

// ImportFromGitHub 从 GitHub 导入.
func (ctm *CustomTemplateManager) ImportFromGitHub(owner, repo, path, ref, name, displayName, description string) (*CustomTemplate, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
	return ctm.ImportFromURL(url, name, displayName, description, "custom")
}

// CreateFromCompose 从 compose 内容创建模板.
func (ctm *CustomTemplateManager) CreateFromCompose(name, displayName, description, category, composeContent string) (*CustomTemplate, error) {
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}

	template := &CustomTemplate{
		ID:          generateTemplateID(name),
		Name:        name,
		DisplayName: displayName,
		Description: description,
		Category:    category,
		Compose:     composeContent,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Source:      "compose",
	}

	return template, nil
}

// generateTemplateID 生成模板 ID.
func generateTemplateID(name string) string {
	return "custom-" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

// saveTemplate 保存模板到文件.
func (ctm *CustomTemplateManager) saveTemplate(template *CustomTemplate) error {
	if ctm.templatesDir == "" {
		return nil
	}

	filePath := filepath.Join(ctm.templatesDir, template.ID+".json")

	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0640)
}

// loadTemplates 加载所有模板.
func (ctm *CustomTemplateManager) loadTemplates() error {
	if ctm.templatesDir == "" {
		return nil
	}

	files, err := os.ReadDir(ctm.templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(ctm.templatesDir, file.Name())
		data, err := os.ReadFile(filePath)
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

// ListTemplates 列出所有模板.
func (ctm *CustomTemplateManager) ListTemplates() []*CustomTemplate {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	result := make([]*CustomTemplate, 0, len(ctm.templates))
	for _, t := range ctm.templates {
		result = append(result, t)
	}
	return result
}

// GetTemplate 获取模板.
func (ctm *CustomTemplateManager) GetTemplate(id string) (*CustomTemplate, error) {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	t, ok := ctm.templates[id]
	if !ok {
		return nil, fmt.Errorf("模板不存在：%s", id)
	}
	return t, nil
}

// CreateFromURL 从 URL 创建模板（别名）.
func (ctm *CustomTemplateManager) CreateFromURL(url, name, displayName, description, category string) (*CustomTemplate, error) {
	return ctm.ImportFromURL(url, name, displayName, description, category)
}

// UpdateTemplate 更新模板.
func (ctm *CustomTemplateManager) UpdateTemplate(id string, updates map[string]interface{}) (*CustomTemplate, error) {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	t, ok := ctm.templates[id]
	if !ok {
		return nil, fmt.Errorf("模板不存在：%s", id)
	}

	for k, v := range updates {
		switch k {
		case "name":
			if s, ok := v.(string); ok {
				t.Name = s
			}
		case "display_name":
			if s, ok := v.(string); ok {
				t.DisplayName = s
			}
		case "description":
			if s, ok := v.(string); ok {
				t.Description = s
			}
		case "category":
			if s, ok := v.(string); ok {
				t.Category = s
			}
		case "compose":
			if s, ok := v.(string); ok {
				t.Compose = s
			}
		}
	}

	t.UpdatedAt = time.Now()
	if err := ctm.saveTemplate(t); err != nil {
		return nil, fmt.Errorf("保存模板失败：%w", err)
	}
	return t, nil
}

// DeleteTemplate 删除模板.
func (ctm *CustomTemplateManager) DeleteTemplate(id string) error {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	if _, ok := ctm.templates[id]; !ok {
		return fmt.Errorf("模板不存在：%s", id)
	}

	delete(ctm.templates, id)

	if ctm.templatesDir != "" {
		filePath := filepath.Join(ctm.templatesDir, id+".json")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除模板文件失败：%w", err)
		}
	}

	return nil
}

// IncrementDownloads 增加下载计数（占位实现）.
func (ctm *CustomTemplateManager) IncrementDownloads(id string) error {
	return nil
}
