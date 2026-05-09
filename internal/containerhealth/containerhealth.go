package containerhealth

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// ContainerHealth 容器健康状态信息
type ContainerHealth struct {
	ContainerID  string        `json:"container_id"`
	Name         string        `json:"name"`
	Status       string        `json:"status"` // healthy/unhealthy/starting/stopped
	HealthCheckType string     `json:"health_check_type"` // http/tcp/cmd
	LastCheck    time.Time     `json:"last_check"`
	LastHealthy  time.Time     `json:"last_healthy"`
	FailCount    int           `json:"fail_count"`
	MaxFailCount int           `json:"max_fail_count"` // 默认3
	AutoRestart  bool          `json:"auto_restart"`
	RestartCount int           `json:"restart_count"`
	Uptime       time.Duration `json:"uptime"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Type     string `json:"type"`     // http/tcp/cmd
	Endpoint string `json:"endpoint"` // HTTP URL
	Port     int    `json:"port"`     // TCP端口
	Command  string `json:"command"`  // 自定义命令
	Interval int    `json:"interval"` // 检查间隔（秒）
	Timeout  int    `json:"timeout"`  // 超时时间（秒）
}

// Manager 容器健康管理器
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*ContainerHealth
	configs    map[string]*HealthCheckConfig
	stopCh     chan struct{}
}

// NewManager 创建容器健康管理器
func NewManager() *Manager {
	return &Manager{
		containers: make(map[string]*ContainerHealth),
		configs:    make(map[string]*HealthCheckConfig),
		stopCh:     make(chan struct{}),
	}
}

// RegisterContainer 注册容器到健康监控
func (m *Manager) RegisterContainer(containerID, name string, cfg HealthCheckConfig, autoRestart bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("container ID 不能为空")
	}
	if _, exists := m.containers[containerID]; exists {
		return fmt.Errorf("容器 %s 已注册", containerID)
	}

	maxFail := 3
	m.containers[containerID] = &ContainerHealth{
		ContainerID:  containerID,
		Name:         name,
		Status:       "starting",
		HealthCheckType: cfg.Type,
		MaxFailCount: maxFail,
		AutoRestart:  autoRestart,
	}
	m.configs[containerID] = &cfg
	log.Printf("容器健康监控已注册: %s (%s)", name, containerID)
	return nil
}

// UnregisterContainer 注销容器健康监控
func (m *Manager) UnregisterContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}

	delete(m.containers, containerID)
	delete(m.configs, containerID)
	log.Printf("容器健康监控已注销: %s", containerID)
	return nil
}

// CheckHealth 检查指定容器健康状态
func (m *Manager) CheckHealth(containerID string) (*ContainerHealth, error) {
	m.mu.RLock()
	container, exists := m.containers[containerID]
	cfg := m.configs[containerID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}

	healthy := m.doHealthCheck(cfg)

	m.mu.Lock()
	container.LastCheck = time.Now()
	if healthy {
		container.Status = "healthy"
		container.LastHealthy = time.Now()
		container.FailCount = 0
		container.ErrorMessage = ""
	} else {
		container.FailCount++
		container.ErrorMessage = "健康检查失败"
		if container.FailCount >= container.MaxFailCount {
			container.Status = "unhealthy"
			if container.AutoRestart {
				if err := m.restartContainerLocked(containerID); err != nil {
					container.ErrorMessage = fmt.Sprintf("自动重启失败: %v", err)
				}
			}
		}
	}
	result := *container
	m.mu.Unlock()

	return &result, nil
}

// CheckAllHealth 检查所有容器健康状态
func (m *Manager) CheckAllHealth() []ContainerHealth {
	m.mu.RLock()
	ids := make([]string, 0, len(m.containers))
	for id := range m.containers {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	var results []ContainerHealth
	for _, id := range ids {
		ch, err := m.CheckHealth(id)
		if err != nil {
			log.Printf("检查容器 %s 失败: %v", id, err)
			continue
		}
		results = append(results, *ch)
	}
	return results
}

// GetContainer 获取容器健康状态详情
func (m *Manager) GetContainer(containerID string) (*ContainerHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器 %s 未注册", containerID)
	}
	result := *container
	return &result, nil
}

// ListContainers 列出所有监控中的容器
func (m *Manager) ListContainers() []ContainerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ContainerHealth, 0, len(m.containers))
	for _, c := range m.containers {
		result = append(result, *c)
	}
	return result
}

// SetAutoRestart 设置容器自动重启开关
func (m *Manager) SetAutoRestart(containerID string, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerID]
	if !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}
	container.AutoRestart = enable
	log.Printf("容器 %s 自动重启设置为: %v", containerID, enable)
	return nil
}

// RestartContainer 手动重启指定容器（模拟）
func (m *Manager) RestartContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.containers[containerID]; !exists {
		return fmt.Errorf("容器 %s 未注册", containerID)
	}
	return m.restartContainerLocked(containerID)
}

// restartContainerLocked 重启容器（需持有写锁）
func (m *Manager) restartContainerLocked(containerID string) error {
	container := m.containers[containerID]
	log.Printf("正在重启容器: %s (%s)", container.Name, containerID)

	// 模拟重启延迟
	time.Sleep(100 * time.Millisecond)

	container.RestartCount++
	container.Status = "starting"
	container.FailCount = 0
	container.ErrorMessage = ""
	log.Printf("容器 %s 重启完成 (第%d次)", container.Name, container.RestartCount)
	return nil
}

// GetHealthReport 生成健康统计报告
func (m *Manager) GetHealthReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.containers)
	healthy := 0
	unhealthy := 0
	starting := 0
	stopped := 0
	totalRestarts := 0

	for _, c := range m.containers {
		switch c.Status {
		case "healthy":
			healthy++
		case "unhealthy":
			unhealthy++
		case "starting":
			starting++
		case "stopped":
			stopped++
		}
		totalRestarts += c.RestartCount
	}

	return map[string]interface{}{
		"total_containers": total,
		"healthy":          healthy,
		"unhealthy":        unhealthy,
		"starting":         starting,
		"stopped":          stopped,
		"total_restarts":   totalRestarts,
		"timestamp":        time.Now(),
	}
}

// doHealthCheck 执行健康检查
func (m *Manager) doHealthCheck(cfg *HealthCheckConfig) bool {
	timeout := 5 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	switch cfg.Type {
	case "http":
		return checkHTTP(cfg.Endpoint, timeout)
	case "tcp":
		return checkTCP(cfg.Endpoint, cfg.Port, timeout)
	case "cmd":
		return checkCommand(cfg.Command, timeout)
	default:
		return false
	}
}

// checkHTTP HTTP健康检查
func checkHTTP(endpoint string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// checkTCP TCP健康检查
func checkTCP(host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkCommand 命令健康检查
func checkCommand(command string, timeout time.Duration) bool {
	ctx := fmt.Sprintf("timeout %ds", int(timeout.Seconds()))
	cmd := exec.Command("sh", "-c", ctx+" "+command)
	err := cmd.Run()
	return err == nil
}
