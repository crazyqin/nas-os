// Package app 应用模板解析
// 提供模板的解析、验证、渲染等功能
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"
)

// ========== 模板解析器 ==========

// TemplateParser 模板解析器
type TemplateParser struct {
	builtinFuncs template.FuncMap // 内置函数
}

// NewTemplateParser 创建模板解析器
func NewTemplateParser() *TemplateParser {
	return &TemplateParser{
		builtinFuncs: template.FuncMap{
			// 字符串处理
			"lower":    strings.ToLower,
			"upper":    strings.ToUpper,
			"title":    strings.Title,
			"trim":     strings.TrimSpace,
			"replace":  strings.ReplaceAll,
			"contains": strings.Contains,
			"split":    strings.Split,
			"join":     strings.Join,

			// 数值处理
			"add":      func(a, b int) int { return a + b },
			"sub":      func(a, b int) int { return a - b },
			"mul":      func(a, b int) int { return a * b },
			"div":      func(a, b int) int { return a / b },
			"default":  func(def, val interface{}) interface{} {
				if val == nil || val == "" || val == 0 {
					return def
				}
				return val
			},

			// 转义处理
			"escapeShell": escapeShellArg,
			"escapeYAML":  escapeYAMLString,
			"quote":       func(s string) string { return "\"" + s + "\"" },
			"singleQuote": func(s string) string { return "'" + s + "'" },
		},
	}
}

// ParseFile 从文件解析模板
func (p *TemplateParser) ParseFile(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取模板文件失败: %w", err)
	}

	return p.ParseJSON(data)
}

// ParseJSON 从JSON解析模板
func (p *TemplateParser) ParseJSON(data []byte) (*Template, error) {
	tmpl := &Template{}
	if err := json.Unmarshal(data, tmpl); err != nil {
		return nil, fmt.Errorf("解析模板JSON失败: %w", err)
	}

	if err := tmpl.Validate(); err != nil {
		return nil, fmt.Errorf("模板验证失败: %w", err)
	}

	return tmpl, nil
}

// ParseYAML 从YAML解析模板（简化实现，使用JSON转换）
func (p *TemplateParser) ParseYAML(data []byte) (*Template, error) {
	// 简化实现，实际应使用yaml库
	// 这里假设YAML已经被转换为JSON格式
	return p.ParseJSON(data)
}

// ========== Compose 模板渲染 ==========

// ComposeRenderData Compose模板渲染数据
type ComposeRenderData struct {
	// 基本信息
	AppID      string            `json:"appId"`
	AppName    string            `json:"appName"`
	TemplateID string            `json:"templateId"`

	// 用户自定义端口
	Ports      map[string]int    `json:"ports"`      // name -> hostPort

	// 用户自定义卷路径
	Volumes    map[string]string `json:"volumes"`    // name -> hostPath

	// 用户自定义环境变量
	Env        map[string]string `json:"env"`        // key -> value

	// 资源限制
	CPU        string            `json:"cpu"`        // CPU限制
	Memory     string            `json:"memory"`     // 内存限制

	// 网络设置
	Network    string            `json:"network"`    // 网络名称

	// 其他自定义字段
	Custom     map[string]interface{} `json:"custom"`
}

// RenderCompose 渲染Compose模板（使用模板引擎）
func (p *TemplateParser) RenderCompose(tmpl *Template, data *ComposeRenderData) (string, error) {
	// 如果模板有预定义的Compose内容，使用模板渲染
	if len(tmpl.Containers) > 0 && tmpl.Containers[0].ComposeTemplate != "" {
		return p.renderFromTemplate(tmpl.Containers[0].ComposeTemplate, data)
	}

	// 否则从容器规格生成
	return p.generateCompose(tmpl, data)
}

// renderFromTemplate 使用模板字符串渲染
func (p *TemplateParser) renderFromTemplate(templateStr string, data *ComposeRenderData) (string, error) {
	tmpl, err := template.New("compose").Funcs(p.builtinFuncs).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("解析Compose模板失败: %w", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("渲染Compose模板失败: %w", err)
	}

	return result.String(), nil
}

// generateCompose 从容器规格生成Compose文件
func (p *TemplateParser) generateCompose(tmpl *Template, data *ComposeRenderData) (string, error) {
	var compose strings.Builder

	compose.WriteString("version: '3.8'\n")
	compose.WriteString("\n")
	compose.WriteString("services:\n")

	for _, container := range tmpl.Containers {
		compose.WriteString(fmt.Sprintf("  %s:\n", container.Name))
		compose.WriteString(fmt.Sprintf("    image: %s\n", container.Image))

		// 主机名
		if container.Hostname != "" {
			compose.WriteString(fmt.Sprintf("    hostname: %s\n", container.Hostname))
		}

		// 特权模式
		if container.Privileged {
			compose.WriteString("    privileged: true\n")
		}

		// 网络模式
		if container.NetworkMode != "" {
			compose.WriteString(fmt.Sprintf("    network_mode: %s\n", container.NetworkMode))
		} else if data.Network != "" {
			compose.WriteString(fmt.Sprintf("    networks:\n      - %s\n", data.Network))
		}

		// 重启策略
		if container.RestartPolicy != "" {
			compose.WriteString(fmt.Sprintf("    restart: %s\n", container.RestartPolicy))
		} else {
			compose.WriteString("    restart: unless-stopped\n")
		}

		// 启动命令
		if len(container.Command) > 0 {
			compose.WriteString("    command:\n")
			for _, cmd := range container.Command {
				compose.WriteString(fmt.Sprintf("      - %s\n", cmd))
			}
		}

		// 端口映射
		if len(container.Ports) > 0 {
			compose.WriteString("    ports:\n")
			for _, port := range container.Ports {
				hostPort := data.Ports[port.Name]
				if hostPort == 0 {
					hostPort = port.DefaultHostPort
				}

				portStr := fmt.Sprintf("%d:%d", hostPort, port.ContainerPort)
				if port.Protocol != "" && port.Protocol != "tcp" {
					portStr += "/" + port.Protocol
				}
				compose.WriteString(fmt.Sprintf("      - %s\n", portStr))
			}
		}

		// 卷挂载
		if len(container.Volumes) > 0 {
			compose.WriteString("    volumes:\n")
			for _, vol := range container.Volumes {
				hostPath := data.Volumes[vol.Name]
				if hostPath == "" {
					hostPath = vol.DefaultHostPath
				}

				volStr := fmt.Sprintf("%s:%s", hostPath, vol.ContainerPath)
				if vol.ReadOnly {
					volStr += ":ro"
				}
				compose.WriteString(fmt.Sprintf("      - %s\n", volStr))
			}
		}

		// 环境变量（合并模板默认值和用户自定义）
		env := make(map[string]string)
		for k, v := range container.Environment {
			env[k] = v
		}
		for k, v := range data.Env {
			env[k] = v
		}

		if len(env) > 0 {
			compose.WriteString("    environment:\n")
			for k, v := range env {
				// 转义环境变量值
				compose.WriteString(fmt.Sprintf("      %s: %s\n", k, escapeYAMLString(v)))
			}
		}

		// 资源限制
		if data.CPU != "" || data.Memory != "" {
			compose.WriteString("    deploy:\n")
			compose.WriteString("      resources:\n")
			compose.WriteString("        limits:\n")
			if data.CPU != "" {
				compose.WriteString(fmt.Sprintf("          cpus: %s\n", data.CPU))
			}
			if data.Memory != "" {
				compose.WriteString(fmt.Sprintf("          memory: %s\n", data.Memory))
			}
		}

		// 健康检查
		if container.HealthCheck != nil {
			compose.WriteString("    healthcheck:\n")
			compose.WriteString("      test:\n")
			for _, cmd := range container.HealthCheck.Test {
				compose.WriteString(fmt.Sprintf("        - %s\n", cmd))
			}
			compose.WriteString(fmt.Sprintf("      interval: %ds\n", container.HealthCheck.Interval))
			compose.WriteString(fmt.Sprintf("      timeout: %ds\n", container.HealthCheck.Timeout))
			compose.WriteString(fmt.Sprintf("      start_period: %ds\n", container.HealthCheck.StartPeriod))
			compose.WriteString(fmt.Sprintf("      retries: %d\n", container.HealthCheck.Retries))
		}

		compose.WriteString("\n")
	}

	// 网络定义
	if data.Network != "" {
		compose.WriteString("networks:\n")
		compose.WriteString(fmt.Sprintf("  %s:\n", data.Network))
		compose.WriteString(fmt.Sprintf("    name: %s\n", data.Network))
		compose.WriteString("    external: true\n")
		compose.WriteString("\n")
	}

	return compose.String(), nil
}

// ========== 模板验证 ==========

// TemplateValidator 模板验证器
type TemplateValidator struct{}

// NewTemplateValidator 创建模板验证器
func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{}
}

// Validate 验证模板完整性
func (v *TemplateValidator) Validate(tmpl *Template) []error {
	errors := []error{}

	// 必填字段检查
	if tmpl.ID == "" {
		errors = append(errors, fmt.Errorf("模板ID不能为空"))
	}
	if tmpl.Name == "" {
		errors = append(errors, fmt.Errorf("应用名称不能为空"))
	}
	if tmpl.DisplayName == "" {
		errors = append(errors, fmt.Errorf("显示名称不能为空"))
	}
	if tmpl.Category == "" {
		errors = append(errors, fmt.Errorf("分类不能为空"))
	}

	// 容器检查
	if len(tmpl.Containers) == 0 {
		errors = append(errors, fmt.Errorf("至少需要定义一个容器"))
	} else {
		for _, c := range tmpl.Containers {
			errors = append(errors, v.validateContainer(c)...)
		}
	}

	// 端口冲突检查
	portMap := make(map[int]string)
	for _, c := range tmpl.Containers {
		for _, p := range c.Ports {
			if p.DefaultHostPort > 0 {
				if existing, ok := portMap[p.DefaultHostPort]; ok {
					errors = append(errors, fmt.Errorf("默认端口 %d 冲突: %s 与 %s", p.DefaultHostPort, existing, p.Name))
				}
				portMap[p.DefaultHostPort] = p.Name
			}
		}
	}

	return errors
}

// validateContainer 验证容器配置
func (v *TemplateValidator) validateContainer(c ContainerSpec) []error {
	errors := []error{}

	if c.Name == "" {
		errors = append(errors, fmt.Errorf("容器名称不能为空"))
	}
	if c.Image == "" {
		errors = append(errors, fmt.Errorf("容器 %s 镜像不能为空", c.Name))
	}

	// 端口检查
	for _, p := range c.Ports {
		if p.ContainerPort <= 0 || p.ContainerPort > 65535 {
			errors = append(errors, fmt.Errorf("容器端口 %d 无效", p.ContainerPort))
		}
		if p.DefaultHostPort < 0 || p.DefaultHostPort > 65535 {
			errors = append(errors, fmt.Errorf("默认主机端口 %d 无效", p.DefaultHostPort))
		}
	}

	// 卷检查
	for _, vol := range c.Volumes {
		if vol.ContainerPath == "" {
			errors = append(errors, fmt.Errorf("卷 %s 容器路径不能为空", vol.Name))
		}
	}

	return errors
}

// ========== 辅助函数 ==========

// escapeShellArg 转义Shell参数
func escapeShellArg(arg string) string {
	// 如果包含特殊字符，用单引号包裹并转义内部单引号
	if strings.ContainsAny(arg, " \t\n\"'$\\`") {
		return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return arg
}

// escapeYAMLString 转义YAML字符串
func escapeYAMLString(s string) string {
	// 如果字符串包含特殊字符或以数字/特殊字符开头，需要引号
	needsQuote := strings.ContainsAny(s, ":#{}[]&*|>?\"'`\n\r\t") ||
		strings.HasPrefix(s, " ") ||
		strings.HasPrefix(s, "-") ||
		strings.HasPrefix(s, "@") ||
		strings.HasPrefix(s, "!") ||
		len(s) == 0

	if needsQuote {
		return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, "\"", "\\\""), "\n", "\\n") + "\""
	}
	return s
}

// ========== 模板合并 ==========

// MergeTemplates 合并多个模板（用于多容器应用）
func MergeTemplates(templates []*Template) (*Template, error) {
	if len(templates) == 0 {
		return nil, fmt.Errorf("没有模板可合并")
	}

	base := templates[0]
	result := &Template{
		ID:          base.ID,
		Name:        base.Name,
		DisplayName: base.DisplayName,
		Description: base.Description,
		Category:    base.Category,
		Icon:        base.Icon,
		Version:     base.Version,
		Author:      base.Author,
		Website:     base.Website,
		Source:      base.Source,
		License:     base.License,
		Notes:       base.Notes,
		Tags:        base.Tags,
		Containers:  []ContainerSpec{},
	}

	// 合并所有容器
	for _, tmpl := range templates {
		result.Containers = append(result.Containers, tmpl.Containers...)
		// 合合标签
		for _, tag := range tmpl.Tags {
			found := false
			for _, existing := range result.Tags {
				if existing == tag {
					found = true
					break
				}
			}
			if !found {
				result.Tags = append(result.Tags, tag)
			}
		}
	}

	return result, nil
}

// CloneTemplate 克隆模板（用于自定义修改）
func CloneTemplate(tmpl *Template) *Template {
	result := &Template{
		ID:          tmpl.ID,
		Name:        tmpl.Name,
		DisplayName: tmpl.DisplayName,
		Description: tmpl.Description,
		Category:    tmpl.Category,
		Icon:        tmpl.Icon,
		Version:     tmpl.Version,
		Author:      tmpl.Author,
		Website:     tmpl.Website,
		Source:      tmpl.Source,
		License:     tmpl.License,
		Notes:       tmpl.Notes,
		Rating:      tmpl.Rating,
		Downloads:   tmpl.Downloads,
		Tags:        make([]string, len(tmpl.Tags)),
		Containers:  make([]ContainerSpec, len(tmpl.Containers)),
	}

	// 复制标签
	copy(result.Tags, tmpl.Tags)

	// 复制容器
	for i, c := range tmpl.Containers {
		result.Containers[i] = ContainerSpec{
			Name:          c.Name,
			Image:         c.Image,
			Hostname:      c.Hostname,
			Ports:         make([]PortSpec, len(c.Ports)),
			Volumes:       make([]VolumeSpec, len(c.Volumes)),
			Environment:   make(map[string]string),
			Command:       make([]string, len(c.Command)),
			Privileged:    c.Privileged,
			NetworkMode:   c.NetworkMode,
			RestartPolicy: c.RestartPolicy,
			HealthCheck:   nil,
		}

		copy(result.Containers[i].Ports, c.Ports)
		copy(result.Containers[i].Volumes, c.Volumes)
		copy(result.Containers[i].Command, c.Command)

		for k, v := range c.Environment {
			result.Containers[i].Environment[k] = v
		}

		if c.HealthCheck != nil {
			result.Containers[i].HealthCheck = &HealthCheckSpec{
				Test:        make([]string, len(c.HealthCheck.Test)),
				Interval:    c.HealthCheck.Interval,
				Timeout:     c.HealthCheck.Timeout,
				StartPeriod: c.HealthCheck.StartPeriod,
				Retries:     c.HealthCheck.Retries,
			}
			copy(result.Containers[i].HealthCheck.Test, c.HealthCheck.Test)
		}
	}

	return result
}

// ExtendTemplate 扩展模板（添加自定义容器）
func ExtendTemplate(base *Template, additionalContainers []ContainerSpec) *Template {
	result := CloneTemplate(base)
	result.Containers = append(result.Containers, additionalContainers...)
	return result
}