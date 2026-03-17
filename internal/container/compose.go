package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ComposeService Compose 服务定义
type ComposeService struct {
	Name        string             `json:"name"`
	Image       string             `json:"image"`
	Container   string             `json:"container,omitempty"`
	Build       interface{}        `json:"build,omitempty"`
	Command     interface{}        `json:"command,omitempty"`
	Volumes     []string           `json:"volumes,omitempty"`
	Ports       []string           `json:"ports,omitempty"`
	Environment map[string]string  `json:"environment,omitempty"`
	EnvFile     []string           `json:"envFile,omitempty"`
	Networks    []string           `json:"networks,omitempty"`
	DependsOn   []string           `json:"dependsOn,omitempty"`
	Restart     string             `json:"restart,omitempty"`
	CPULimit    string             `json:"cpuLimit,omitempty"`
	MemLimit    string             `json:"memLimit,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Test        []string      `json:"test"`
	Interval    time.Duration `json:"interval"`
	Timeout     time.Duration `json:"timeout"`
	Retries     int           `json:"retries"`
	StartPeriod time.Duration `json:"startPeriod"`
}

// ComposeProject Compose 项目
type ComposeProject struct {
	Name         string                 `json:"name"`
	Path         string                 `json:"path"`
	Services     []*ComposeService      `json:"services"`
	Networks     map[string]interface{} `json:"networks,omitempty"`
	Volumes      map[string]interface{} `json:"volumes,omitempty"`
	Status       string                 `json:"status"`
	Containers   []string               `json:"containers"`
	LastDeployed time.Time              `json:"lastDeployed"`
}

// ComposeConfig Compose 配置
type ComposeConfig struct {
	Name     string                 `json:"name"`
	Services map[string]interface{} `json:"services"`
	Networks map[string]interface{} `json:"networks,omitempty"`
	Volumes  map[string]interface{} `json:"volumes,omitempty"`
}

// DeployProgress 部署进度
type DeployProgress struct {
	Current   int    `json:"current"`
	Total     int    `json:"total"`
	Service   string `json:"service"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Completed bool   `json:"completed"`
	Error     string `json:"error,omitempty"`
}

// ComposeManager Compose 管理器
type ComposeManager struct {
	manager *Manager
}

// NewComposeManager 创建 Compose 管理器
func NewComposeManager(mgr *Manager) *ComposeManager {
	return &ComposeManager{
		manager: mgr,
	}
}

// ParseComposeFile 解析 docker-compose.yml 文件
func (cm *ComposeManager) ParseComposeFile(path string) (*ComposeProject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Compose 文件失败：%w", err)
	}

	var config ComposeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 Compose 文件失败：%w", err)
	}

	project := &ComposeProject{
		Name:     config.Name,
		Path:     path,
		Services: make([]*ComposeService, 0),
		Networks: config.Networks,
		Volumes:  config.Volumes,
	}

	// 解析服务
	for name, serviceData := range config.Services {
		service, err := cm.parseService(name, serviceData)
		if err != nil {
			continue
		}
		project.Services = append(project.Services, service)
	}

	return project, nil
}

// parseService 解析服务定义
func (cm *ComposeManager) parseService(name string, data interface{}) (*ComposeService, error) {
	serviceMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的服务定义")
	}

	service := &ComposeService{
		Name:        name,
		Environment: make(map[string]string),
		Labels:      make(map[string]string),
		Volumes:     make([]string, 0),
		Ports:       make([]string, 0),
		Networks:    make([]string, 0),
		DependsOn:   make([]string, 0),
	}

	// 镜像
	if image, ok := serviceMap["image"].(string); ok {
		service.Image = image
	}

	// 容器名称
	if container, ok := serviceMap["container_name"].(string); ok {
		service.Container = container
	}

	// 构建配置
	if build, ok := serviceMap["build"]; ok {
		service.Build = build
	}

	// 命令
	if command, ok := serviceMap["command"]; ok {
		service.Command = command
	}

	// 卷
	if volumes, ok := serviceMap["volumes"].([]interface{}); ok {
		for _, v := range volumes {
			if vol, ok := v.(string); ok {
				service.Volumes = append(service.Volumes, vol)
			}
		}
	}

	// 端口
	if ports, ok := serviceMap["ports"].([]interface{}); ok {
		for _, p := range ports {
			if port, ok := p.(string); ok {
				service.Ports = append(service.Ports, port)
			}
		}
	}

	// 环境变量
	if env, ok := serviceMap["environment"]; ok {
		switch v := env.(type) {
		case map[string]interface{}:
			for k, val := range v {
				if valStr, ok := val.(string); ok {
					service.Environment[k] = valStr
				}
			}
		case []interface{}:
			for _, e := range v {
				if str, ok := e.(string); ok {
					parts := strings.SplitN(str, "=", 2)
					if len(parts) == 2 {
						service.Environment[parts[0]] = parts[1]
					}
				}
			}
		}
	}

	// 环境变量文件
	if envFile, ok := serviceMap["env_file"].([]interface{}); ok {
		for _, e := range envFile {
			if file, ok := e.(string); ok {
				service.EnvFile = append(service.EnvFile, file)
			}
		}
	}

	// 网络
	if networks, ok := serviceMap["networks"].([]interface{}); ok {
		for _, n := range networks {
			if net, ok := n.(string); ok {
				service.Networks = append(service.Networks, net)
			}
		}
	}

	// 依赖
	if dependsOn, ok := serviceMap["depends_on"].([]interface{}); ok {
		for _, d := range dependsOn {
			if dep, ok := d.(string); ok {
				service.DependsOn = append(service.DependsOn, dep)
			}
		}
	}

	// 重启策略
	if restart, ok := serviceMap["restart"].(string); ok {
		service.Restart = restart
	}

	// 资源限制
	if deploy, ok := serviceMap["deploy"].(map[string]interface{}); ok {
		if resources, ok := deploy["resources"].(map[string]interface{}); ok {
			if limits, ok := resources["limits"].(map[string]interface{}); ok {
				if cpus, ok := limits["cpus"].(string); ok {
					service.CPULimit = cpus
				}
				if memory, ok := limits["memory"].(string); ok {
					service.MemLimit = memory
				}
			}
		}
	}

	// 标签
	if labels, ok := serviceMap["labels"]; ok {
		switch v := labels.(type) {
		case map[string]interface{}:
			for k, val := range v {
				if valStr, ok := val.(string); ok {
					service.Labels[k] = valStr
				}
			}
		case []interface{}:
			for _, l := range v {
				if str, ok := l.(string); ok {
					parts := strings.SplitN(str, "=", 2)
					if len(parts) == 2 {
						service.Labels[parts[0]] = parts[1]
					}
				}
			}
		}
	}

	// 健康检查
	if healthcheck, ok := serviceMap["healthcheck"].(map[string]interface{}); ok {
		service.HealthCheck = &HealthCheckConfig{}
		if test, ok := healthcheck["test"].([]interface{}); ok {
			for _, t := range test {
				if str, ok := t.(string); ok {
					service.HealthCheck.Test = append(service.HealthCheck.Test, str)
				}
			}
		}
		if interval, ok := healthcheck["interval"].(string); ok {
			if dur, err := time.ParseDuration(interval); err == nil {
				service.HealthCheck.Interval = dur
			}
		}
		if timeout, ok := healthcheck["timeout"].(string); ok {
			if dur, err := time.ParseDuration(timeout); err == nil {
				service.HealthCheck.Timeout = dur
			}
		}
		if retries, ok := healthcheck["retries"].(int); ok {
			service.HealthCheck.Retries = retries
		}
	}

	return service, nil
}

// Deploy 部署 Compose 项目
func (cm *ComposeManager) Deploy(composePath string) error {
	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--build")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("部署 Compose 项目失败：%w, %s", err, string(output))
	}
	return nil
}

// DeployWithProgress 带进度跟踪的部署
func (cm *ComposeManager) DeployWithProgress(composePath string, progressChan chan<- *DeployProgress) error {
	project, err := cm.ParseComposeFile(composePath)
	if err != nil {
		return err
	}

	total := len(project.Services)

	// 发送开始进度
	progressChan <- &DeployProgress{
		Current: 0,
		Total:   total,
		Status:  "starting",
		Message: "开始部署...",
	}

	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--build")
	cmd.Dir = dir

	// 捕获输出
	output, err := cmd.CombinedOutput()

	current := 0
	for _, service := range project.Services {
		current++
		progressChan <- &DeployProgress{
			Current: current,
			Total:   total,
			Service: service.Name,
			Status:  "deployed",
			Message: fmt.Sprintf("服务 %s 已部署", service.Name),
		}
	}

	if err != nil {
		progressChan <- &DeployProgress{
			Current:   current,
			Total:     total,
			Status:    "error",
			Message:   "部署失败",
			Error:     fmt.Sprintf("%s: %s", err.Error(), string(output)),
			Completed: true,
		}
		close(progressChan)
		return fmt.Errorf("部署 Compose 项目失败：%w, %s", err, string(output))
	}

	progressChan <- &DeployProgress{
		Current:   total,
		Total:     total,
		Status:    "completed",
		Message:   "所有服务部署完成",
		Completed: true,
	}
	close(progressChan)

	return nil
}

// Stop 停止 Compose 项目
func (cm *ComposeManager) Stop(composePath string) error {
	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", "compose", "-f", composePath, "down")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止 Compose 项目失败：%w, %s", err, string(output))
	}
	return nil
}

// Restart 重启 Compose 项目
func (cm *ComposeManager) Restart(composePath string) error {
	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", "compose", "-f", composePath, "restart")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启 Compose 项目失败：%w, %s", err, string(output))
	}
	return nil
}

// Remove 删除 Compose 项目
func (cm *ComposeManager) Remove(composePath string, removeVolumes bool) error {
	args := []string{"compose", "-f", composePath, "down"}
	if removeVolumes {
		args = append(args, "-v")
	}

	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除 Compose 项目失败：%w, %s", err, string(output))
	}
	return nil
}

// GetServices 获取 Compose 项目服务状态
func (cm *ComposeManager) GetServices(composePath string) ([]*ComposeService, error) {
	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", "compose", "-f", composePath, "ps", "--format", "json")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取服务状态失败：%w", err)
	}

	var services []*ComposeService
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		var raw struct {
			Name    string `json:"Name"`
			Image   string `json:"Image"`
			State   string `json:"State"`
			Service string `json:"Service"`
		}

		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		services = append(services, &ComposeService{
			Name:  raw.Service,
			Image: raw.Image,
		})
	}

	return services, nil
}

// GetLogs 获取 Compose 项目日志
func (cm *ComposeManager) GetLogs(composePath string, service string, tail int) ([]string, error) {
	args := []string{"compose", "-f", composePath, "logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	if service != "" {
		args = append(args, service)
	}

	dir := filepath.Dir(composePath)
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取日志失败：%w", err)
	}

	var logs []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}

	return logs, nil
}

// ValidateComposeFile 验证 Compose 文件
func (cm *ComposeManager) ValidateComposeFile(path string) error {
	dir := filepath.Dir(path)
	cmd := exec.Command("docker", "compose", "-f", path, "config")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose 文件验证失败：%w, %s", err, string(output))
	}
	return nil
}

// CreateComposeFile 创建 Compose 文件
func (cm *ComposeManager) CreateComposeFile(path string, project *ComposeProject) error {
	config := ComposeConfig{
		Name:     project.Name,
		Services: make(map[string]interface{}),
		Networks: project.Networks,
		Volumes:  project.Volumes,
	}

	for _, service := range project.Services {
		serviceData := map[string]interface{}{
			"image": service.Image,
		}

		if service.Container != "" {
			serviceData["container_name"] = service.Container
		}
		if len(service.Volumes) > 0 {
			serviceData["volumes"] = service.Volumes
		}
		if len(service.Ports) > 0 {
			serviceData["ports"] = service.Ports
		}
		if len(service.Environment) > 0 {
			serviceData["environment"] = service.Environment
		}
		if len(service.Networks) > 0 {
			serviceData["networks"] = service.Networks
		}
		if len(service.DependsOn) > 0 {
			serviceData["depends_on"] = service.DependsOn
		}
		if service.Restart != "" {
			serviceData["restart"] = service.Restart
		}
		if service.CPULimit != "" || service.MemLimit != "" {
			deploy := map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{},
				},
			}
			if service.CPULimit != "" {
				if limits, ok := deploy["resources"].(map[string]interface{})["limits"].(map[string]interface{}); ok {
					limits["cpus"] = service.CPULimit
				}
			}
			if service.MemLimit != "" {
				if limits, ok := deploy["resources"].(map[string]interface{})["limits"].(map[string]interface{}); ok {
					limits["memory"] = service.MemLimit
				}
			}
			serviceData["deploy"] = deploy
		}

		config.Services[service.Name] = serviceData
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化 Compose 配置失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
