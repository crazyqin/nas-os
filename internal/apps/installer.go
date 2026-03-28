// Package apps 应用安装器
// 处理应用的安装、卸载、配置更新等操作
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nas-os/pkg/app"
)

// Installer 应用安装器
type Installer struct {
	installDir string          // 安装目录（Compose文件存放）
	manager    ContainerManager // 容器管理器
}

// NewInstaller 创建应用安装器
func NewInstaller(installDir string, manager ContainerManager) (*Installer, error) {
	if err := os.MkdirAll(installDir, 0750); err != nil {
		return nil, fmt.Errorf("创建安装目录失败: %w", err)
	}

	return &Installer{
		installDir: installDir,
		manager:    manager,
	}, nil
}

// Install 安装应用
func (i *Installer) Install(ctx context.Context, template *app.Template, opts *app.InstallOptions) (*app.InstalledApp, error) {
	if opts == nil {
		opts = &app.InstallOptions{}
	}

	// 验证模板
	if err := template.Validate(); err != nil {
		return nil, fmt.Errorf("模板验证失败: %w", err)
	}

	// 生成应用ID
	appID := template.ID
	if opts.InstanceName != "" {
		appID = opts.InstanceName
	}

	// 创建应用目录
	appDir := filepath.Join(i.installDir, appID)
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return nil, fmt.Errorf("创建应用目录失败: %w", err)
	}

	// 生成 Compose 文件
	composePath := filepath.Join(appDir, "docker-compose.yml")
	composeContent, err := i.generateCompose(template, opts)
	if err != nil {
		return nil, fmt.Errorf("生成Compose文件失败: %w", err)
	}

	// 写入 Compose 文件
	if err := os.WriteFile(composePath, composeContent, 0644); err != nil {
		return nil, fmt.Errorf("写入Compose文件失败: %w", err)
	}

	// 写入配置文件（用于后续更新）
	configPath := filepath.Join(appDir, "config.json")
	config := &app.InstallConfig{
		TemplateID: template.ID,
		Options:    opts,
		InstalledAt: time.Now(),
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return nil, fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 创建数据卷目录
	for _, container := range template.Containers {
		for _, vol := range container.Volumes {
			hostPath := opts.VolumePaths[vol.Name]
			if hostPath == "" {
				hostPath = vol.DefaultHostPath
			}
			if hostPath != "" {
				if err := os.MkdirAll(hostPath, 0750); err != nil {
					fmt.Printf("创建数据目录 %s 失败: %v\n", hostPath, err)
				}
			}
		}
	}

	// 启动应用
	if opts.SkipStart == false {
		if err := i.manager.ComposeUp(ctx, composePath); err != nil {
			return nil, fmt.Errorf("启动应用失败: %w", err)
		}
	}

	// 获取服务状态
	services, err := i.manager.ComposePS(ctx, composePath)
	if err != nil {
		fmt.Printf("获取服务状态失败: %v\n", err)
		services = []app.ComposeService{}
	}

	// 构建安装记录
	installed := &app.InstalledApp{
		ID:          appID,
		Name:        template.Name,
		DisplayName: template.DisplayName,
		TemplateID:  template.ID,
		Version:     template.Version,
		Status:      app.AppStatusRunning,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
		ComposePath: composePath,
		ConfigPath:  configPath,
		Config:      opts.Env,
		PortMappings: i.extractPortMappings(template, opts),
		VolumeMappings: i.extractVolumeMappings(template, opts),
		Services:    services,
	}

	return installed, nil
}

// Uninstall 卸载应用
func (i *Installer) Uninstall(ctx context.Context, installed *app.InstalledApp, opts *app.UninstallOptions) error {
	if opts == nil {
		opts = &app.UninstallOptions{}
	}

	// 停止并移除容器
	if installed.ComposePath != "" {
		if err := i.manager.ComposeDown(ctx, installed.ComposePath); err != nil {
			if !opts.Force {
				return fmt.Errorf("停止应用失败: %w", err)
			}
			fmt.Printf("强制卸载: 停止应用失败 %v\n", err)
		}
	}

	// 删除数据卷（可选）
	if opts.RemoveVolumes {
		for _, mapping := range installed.VolumeMappings {
			if mapping.HostPath != "" {
				fmt.Printf("删除数据目录: %s\n", mapping.HostPath)
				if err := os.RemoveAll(mapping.HostPath); err != nil {
					fmt.Printf("删除数据目录失败: %v\n", err)
				}
			}
		}
	}

	// 删除应用目录
	appDir := filepath.Dir(installed.ComposePath)
	if opts.RemoveConfig {
		if err := os.RemoveAll(appDir); err != nil {
			fmt.Printf("删除应用目录失败: %v\n", err)
		}
	}

	return nil
}

// Start 启动应用
func (i *Installer) Start(ctx context.Context, installed *app.InstalledApp) error {
	if installed.ComposePath == "" {
		return fmt.Errorf("应用Compose文件不存在")
	}

	return i.manager.ComposeUp(ctx, installed.ComposePath)
}

// Stop 停止应用
func (i *Installer) Stop(ctx context.Context, installed *app.InstalledApp, timeout int) error {
	if installed.ComposePath == "" {
		return fmt.Errorf("应用Compose文件不存在")
	}

	return i.manager.ComposeDown(ctx, installed.ComposePath)
}

// Restart 重启应用
func (i *Installer) Restart(ctx context.Context, installed *app.InstalledApp, timeout int) error {
	if installed.ComposePath == "" {
		return fmt.Errorf("应用Compose文件不存在")
	}

	// 停止
	if err := i.manager.ComposeDown(ctx, installed.ComposePath); err != nil {
		fmt.Printf("停止应用失败: %v\n", err)
	}

	// 启动
	return i.manager.ComposeUp(ctx, installed.ComposePath)
}

// GetStatus 获取应用状态
func (i *Installer) GetStatus(ctx context.Context, installed *app.InstalledApp) (*app.AppStatus, error) {
	if installed.ComposePath == "" {
		return nil, fmt.Errorf("应用Compose文件不存在")
	}

	// 获取服务列表
	services, err := i.manager.ComposePS(ctx, installed.ComposePath)
	if err != nil {
		return &app.AppStatus{
			State:   app.AppStateError,
			Message: err.Error(),
		}, nil
	}

	// 计算整体状态
	status := &app.AppStatus{
		State:      app.AppStateRunning,
		Services:   services,
		Message:    "运行正常",
		UpdatedAt:  time.Now(),
	}

	allRunning := true
	anyError := false
	for _, svc := range services {
		if svc.State != "running" {
			allRunning = false
		}
		if svc.State == "error" || svc.State == "exited" {
			anyError = true
		}
	}

	if len(services) == 0 {
		status.State = app.AppStateStopped
		status.Message = "应用已停止"
	} else if anyError {
		status.State = app.AppStateError
		status.Message = "部分服务异常"
	} else if allRunning {
		status.State = app.AppStateRunning
		status.Message = "所有服务运行正常"
	} else {
		status.State = app.AppStateStarting
		status.Message = "应用正在启动"
	}

	return status, nil
}

// GetConfig 获取应用配置
func (i *Installer) GetConfig(installed *app.InstalledApp) (map[string]string, error) {
	if installed.ConfigPath == "" {
		return installed.Config, nil
	}

	data, err := os.ReadFile(installed.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return installed.Config, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config app.InstallConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return config.Options.Env, nil
}

// UpdateConfig 更新应用配置（需要重启应用生效）
func (i *Installer) UpdateConfig(ctx context.Context, installed *app.InstalledApp, newConfig map[string]string) error {
	// 读取原配置
	config := &app.InstallConfig{
		TemplateID: installed.TemplateID,
		Options:    &app.InstallOptions{},
		InstalledAt: installed.InstalledAt,
	}

	if installed.ConfigPath != "" {
		data, err := os.ReadFile(installed.ConfigPath)
		if err == nil {
			json.Unmarshal(data, config)
		}
	}

	// 合并新配置
	for k, v := range newConfig {
		if config.Options.Env == nil {
			config.Options.Env = make(map[string]string)
		}
		config.Options.Env[k] = v
	}
	config.UpdatedAt = time.Now()

	// 保存配置
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.WriteFile(installed.ConfigPath, configData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 重新生成Compose文件
	template, err := i.getTemplate(installed.TemplateID)
	if err != nil {
		return fmt.Errorf("获取模板失败: %w", err)
	}

	composeContent, err := i.generateCompose(template, config.Options)
	if err != nil {
		return fmt.Errorf("生成Compose文件失败: %w", err)
	}

	if err := os.WriteFile(installed.ComposePath, composeContent, 0644); err != nil {
		return fmt.Errorf("写入Compose文件失败: %w", err)
	}

	// 重启应用
	return i.Restart(ctx, installed, 10)
}

// ========== 内部方法 ==========

// generateCompose 生成 Docker Compose 文件
func (i *Installer) generateCompose(template *app.Template, opts *app.InstallOptions) ([]byte, error) {
	compose := &ComposeFile{
		Version: "3.8",
		Services: make(map[string]ComposeService),
	}

	for _, container := range template.Containers {
		service := i.containerToComposeService(container, opts)
		compose.Services[container.Name] = service
	}

	// 添加网络（可选）
	if opts.Network != "" {
		compose.Networks = map[string]ComposeNetwork{
			opts.Network: {Name: opts.Network, External: true},
		}
		for name := range compose.Services {
			svc := compose.Services[name]
			svc.Networks = []string{opts.Network}
			compose.Services[name] = svc
		}
	}

	// YAML 序列化
	return composeToYAML(compose), nil
}

// containerToComposeService 转换容器规格为Compose服务
func (i *Installer) containerToComposeService(spec app.ContainerSpec, opts *app.InstallOptions) ComposeService {
	service := ComposeService{
		Image:         spec.Image,
		ContainerName: spec.Name,
		Restart:       spec.RestartPolicy,
	}

	// hostname
	if spec.Hostname != "" {
		service.Hostname = spec.Hostname
	}

	// 特权模式
	if spec.Privileged {
		service.Privileged = true
	}

	// 网络模式
	if spec.NetworkMode != "" {
		service.NetworkMode = spec.NetworkMode
	}

	// 命令
	if len(spec.Command) > 0 {
		service.Command = spec.Command
	}

	// 端口映射
	for _, port := range spec.Ports {
		hostPort := opts.PortMappings[port.Name]
		if hostPort == 0 {
			hostPort = port.DefaultHostPort
		}
		portStr := fmt.Sprintf("%d:%d", hostPort, port.ContainerPort)
		if port.Protocol != "" && port.Protocol != "tcp" {
			portStr += "/" + port.Protocol
		}
		service.Ports = append(service.Ports, portStr)
	}

	// 卷挂载
	for _, vol := range spec.Volumes {
		hostPath := opts.VolumePaths[vol.Name]
		if hostPath == "" {
			hostPath = vol.DefaultHostPath
		}
		if hostPath != "" {
			volStr := fmt.Sprintf("%s:%s", hostPath, vol.ContainerPath)
			if vol.ReadOnly {
				volStr += ":ro"
			}
			service.Volumes = append(service.Volumes, volStr)
		}
	}

	// 环境变量（合并模板和用户配置）
	env := make(map[string]string)
	for k, v := range spec.Environment {
		env[k] = v
	}
	for k, v := range opts.Env {
		env[k] = v
	}
	service.Environment = env

	// CPU限制
	if opts.CPULimit != "" {
		service.Deploy = &ComposeDeploy{
			Resources: ComposeResources{
				Limits: ComposeResourceLimits{
					CPUs: opts.CPULimit,
				},
			},
		}
	}

	// 内存限制
	if opts.MemoryLimit != "" {
		if service.Deploy == nil {
			service.Deploy = &ComposeDeploy{}
		}
		service.Deploy.Resources.Limits.Memory = opts.MemoryLimit
	}

	return service
}

// extractPortMappings 提取端口映射
func (i *Installer) extractPortMappings(template *app.Template, opts *app.InstallOptions) []app.PortMapping {
	mappings := []app.PortMapping{}
	for _, container := range template.Containers {
		for _, port := range container.Ports {
			hostPort := opts.PortMappings[port.Name]
			if hostPort == 0 {
				hostPort = port.DefaultHostPort
			}
			mappings = append(mappings, app.PortMapping{
				Name:          port.Name,
				HostPort:      hostPort,
				ContainerPort: port.ContainerPort,
				Protocol:      port.Protocol,
				Description:   port.Description,
			})
		}
	}
	return mappings
}

// extractVolumeMappings 提取卷映射
func (i *Installer) extractVolumeMappings(template *app.Template, opts *app.InstallOptions) []app.VolumeMapping {
	mappings := []app.VolumeMapping{}
	for _, container := range template.Containers {
		for _, vol := range container.Volumes {
			hostPath := opts.VolumePaths[vol.Name]
			if hostPath == "" {
				hostPath = vol.DefaultHostPath
			}
			mappings = append(mappings, app.VolumeMapping{
				Name:          vol.Name,
				HostPath:      hostPath,
				ContainerPath: vol.ContainerPath,
				Description:   vol.Description,
				ReadOnly:      vol.ReadOnly,
			})
		}
	}
	return mappings
}

// getTemplate 获取模板（简化实现，实际应从Catalog获取）
func (i *Installer) getTemplate(templateID string) (*app.Template, error) {
	// 这里应该从Catalog获取，暂时返回错误
	return nil, fmt.Errorf("需要从Catalog获取模板")
}

// ========== Docker Compose 执行 ==========

// ComposeUp 启动Compose项目
func (i *Installer) composeUp(ctx context.Context, composePath string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "up", "-d")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose启动失败: %w, %s", err, string(output))
	}
	return nil
}

// ComposeDown 停止Compose项目
func (i *Installer) composeDown(ctx context.Context, composePath string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "down")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Compose停止失败: %w, %s", err, string(output))
	}
	return nil
}

// composePS 获取Compose服务状态
func (i *Installer) composePS(ctx context.Context, composePath string) ([]app.ComposeService, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "ps", "--format", "json")
	cmd.Dir = filepath.Dir(composePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取服务状态失败: %w", err)
	}

	var services []app.ComposeService
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var svc app.ComposeService
		if err := json.Unmarshal([]byte(line), &svc); err != nil {
			continue
		}
		services = append(services, svc)
	}

	return services, nil
}

// ========== Compose 数据结构 ==========

// ComposeFile Docker Compose 文件结构
type ComposeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
	Networks map[string]ComposeNetwork `yaml:"networks,omitempty"`
	Volumes  map[string]ComposeVolume  `yaml:"volumes,omitempty"`
}

// ComposeService Compose 服务定义
type ComposeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name,omitempty"`
	Hostname      string            `yaml:"hostname,omitempty"`
	Restart       string            `yaml:"restart,omitempty"`
	Privileged    bool              `yaml:"privileged,omitempty"`
	NetworkMode   string            `yaml:"network_mode,omitempty"`
	Command       []string          `yaml:"command,omitempty"`
	Ports         []string          `yaml:"ports,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
	Environment   map[string]string `yaml:"environment,omitempty"`
	Networks      []string          `yaml:"networks,omitempty"`
	DependsOn     []string          `yaml:"depends_on,omitempty"`
	Deploy        *ComposeDeploy    `yaml:"deploy,omitempty"`
}

// ComposeDeploy 资源部署配置
type ComposeDeploy struct {
	Resources ComposeResources `yaml:"resources,omitempty"`
}

// ComposeResources 资源限制
type ComposeResources struct {
	Limits ComposeResourceLimits `yaml:"limits,omitempty"`
}

// ComposeResourceLimits 资源限制值
type ComposeResourceLimits struct {
	CPUs   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// ComposeNetwork 网络定义
type ComposeNetwork struct {
	Name     string `yaml:"name,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

// ComposeVolume 卷定义
type ComposeVolume struct {
	Name     string `yaml:"name,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

// composeToYAML 将Compose文件转换为YAML
func composeToYAML(compose *ComposeFile) []byte {
	var yaml strings.Builder

	yaml.WriteString(fmt.Sprintf("version: '%s'\n", compose.Version))
	yaml.WriteString("\n")

	if len(compose.Services) > 0 {
		yaml.WriteString("services:\n")
		for name, svc := range compose.Services {
			yaml.WriteString(fmt.Sprintf("  %s:\n", name))
			yaml.WriteString(fmt.Sprintf("    image: %s\n", svc.Image))
			if svc.ContainerName != "" {
				yaml.WriteString(fmt.Sprintf("    container_name: %s\n", svc.ContainerName))
			}
			if svc.Hostname != "" {
				yaml.WriteString(fmt.Sprintf("    hostname: %s\n", svc.Hostname))
			}
			if svc.Restart != "" {
				yaml.WriteString(fmt.Sprintf("    restart: %s\n", svc.Restart))
			}
			if svc.Privileged {
				yaml.WriteString("    privileged: true\n")
			}
			if svc.NetworkMode != "" {
				yaml.WriteString(fmt.Sprintf("    network_mode: %s\n", svc.NetworkMode))
			}
			if len(svc.Command) > 0 {
				yaml.WriteString("    command:\n")
				for _, cmd := range svc.Command {
					yaml.WriteString(fmt.Sprintf("      - %s\n", cmd))
				}
			}
			if len(svc.Ports) > 0 {
				yaml.WriteString("    ports:\n")
				for _, port := range svc.Ports {
					yaml.WriteString(fmt.Sprintf("      - %s\n", port))
				}
			}
			if len(svc.Volumes) > 0 {
				yaml.WriteString("    volumes:\n")
				for _, vol := range svc.Volumes {
					yaml.WriteString(fmt.Sprintf("      - %s\n", vol))
				}
			}
			if len(svc.Environment) > 0 {
				yaml.WriteString("    environment:\n")
				for k, v := range svc.Environment {
					yaml.WriteString(fmt.Sprintf("      %s: %s\n", k, v))
				}
			}
			if len(svc.Networks) > 0 {
				yaml.WriteString("    networks:\n")
				for _, net := range svc.Networks {
					yaml.WriteString(fmt.Sprintf("      - %s\n", net))
				}
			}
			if len(svc.DependsOn) > 0 {
				yaml.WriteString("    depends_on:\n")
				for _, dep := range svc.DependsOn {
					yaml.WriteString(fmt.Sprintf("      - %s\n", dep))
				}
			}
			if svc.Deploy != nil {
				yaml.WriteString("    deploy:\n")
				yaml.WriteString("      resources:\n")
				yaml.WriteString("        limits:\n")
				if svc.Deploy.Resources.Limits.CPUs != "" {
					yaml.WriteString(fmt.Sprintf("          cpus: %s\n", svc.Deploy.Resources.Limits.CPUs))
				}
				if svc.Deploy.Resources.Limits.Memory != "" {
					yaml.WriteString(fmt.Sprintf("          memory: %s\n", svc.Deploy.Resources.Limits.Memory))
				}
			}
		}
	}

	if len(compose.Networks) > 0 {
		yaml.WriteString("\nnetworks:\n")
		for name, net := range compose.Networks {
			yaml.WriteString(fmt.Sprintf("  %s:\n", name))
			if net.Name != "" {
				yaml.WriteString(fmt.Sprintf("    name: %s\n", net.Name))
			}
			if net.External {
				yaml.WriteString("    external: true\n")
			}
		}
	}

	if len(compose.Volumes) > 0 {
		yaml.WriteString("\nvolumes:\n")
		for name, vol := range compose.Volumes {
			yaml.WriteString(fmt.Sprintf("  %s:\n", name))
			if vol.Name != "" {
				yaml.WriteString(fmt.Sprintf("    name: %s\n", vol.Name))
			}
			if vol.External {
				yaml.WriteString("    external: true\n")
			}
		}
	}

	return []byte(yaml.String())
}