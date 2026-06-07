// Package cms 节点管理服务
// NodeManagementService - 节点生命周期管理、命令下发、配置同步
package cms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// NodeManagementService 节点管理服务
// 处理节点生命周期、命令下发、配置管理
type NodeManagementService struct {
	fleetConfig FleetConfig
	logger      *zap.Logger
	commands    map[string][]NodeCommand // deviceID -> commands
	configs     map[string]*NodeConfig   // deviceID -> config
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NodeConfig 节点配置
type NodeConfig struct {
	DeviceID         string            `json:"deviceId"`
	Name             string            `json:"name"`
	Role             string            `json:"role"`             // master, worker, storage, edge
	GroupID          string            `json:"groupId"`          // 分组ID
	Priority         int               `json:"priority"`         // 优先级
	SyncEnabled      bool              `json:"syncEnabled"`      // 启用同步
	TaskEnabled      bool              `json:"taskEnabled"`      // 启用任务
	HealthThresholds HealthThresholds  `json:"healthThresholds"` // 健康阈值
	Tags             []string          `json:"tags"`             // 标签
	Labels           map[string]string `json:"labels"`           // 自定义标签
	LastSync         time.Time         `json:"lastSync"`         // 最后同步时间
	Version          int               `json:"version"`          // 配置版本
}

// HealthThresholds 健康阈值
type HealthThresholds struct {
	CPUHigh        float64 `json:"cpuHigh"`        // CPU高阈值
	MemoryHigh     float64 `json:"memoryHigh"`     // 内存高阈值
	DiskHigh       float64 `json:"diskHigh"`       // 磁盘高阈值
	TempHigh       float64 `json:"tempHigh"`       // 温度高阈值
	HealthLow      int     `json:"healthLow"`      // 健康分低阈值
	OfflineTimeout int     `json:"offlineTimeout"` // 离线超时（秒）
}

// DefaultHealthThresholds 默认健康阈值
func DefaultHealthThresholds() HealthThresholds {
	return HealthThresholds{
		CPUHigh:        85.0,
		MemoryHigh:     85.0,
		DiskHigh:       90.0,
		TempHigh:       75.0,
		HealthLow:      60,
		OfflineTimeout: 30,
	}
}

// NewNodeManagementService 创建节点管理服务
func NewNodeManagementService(config FleetConfig, logger *zap.Logger) *NodeManagementService {
	ctx, cancel := context.WithCancel(context.Background())

	return &NodeManagementService{
		fleetConfig: config,
		logger:      logger,
		commands:    make(map[string][]NodeCommand),
		configs:     make(map[string]*NodeConfig),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动服务
func (nms *NodeManagementService) Start() {
	nms.logger.Info("节点管理服务启动")
}

// Stop 停止服务
func (nms *NodeManagementService) Stop() {
	nms.cancel()
	nms.logger.Info("节点管理服务停止")
}

// ================== 配置管理 ==================

// SetNodeConfig 设置节点配置
func (nms *NodeManagementService) SetNodeConfig(deviceID string, config NodeConfig) error {
	nms.mu.Lock()
	defer nms.mu.Unlock()

	config.DeviceID = deviceID
	config.Version++
	config.LastSync = time.Now()

	nms.configs[deviceID] = &config

	nms.logger.Info("更新节点配置",
		zap.String("device_id", deviceID),
		zap.Int("version", config.Version))

	return nil
}

// GetNodeConfig 获取节点配置
func (nms *NodeManagementService) GetNodeConfig(deviceID string) (*NodeConfig, error) {
	nms.mu.RLock()
	defer nms.mu.RUnlock()

	config, exists := nms.configs[deviceID]
	if !exists {
		return nil, errors.New("节点配置不存在")
	}

	return config, nil
}

// GetDefaultConfig 获取默认配置
func (nms *NodeManagementService) GetDefaultConfig(deviceID string) *NodeConfig {
	return &NodeConfig{
		DeviceID:         deviceID,
		Role:             "worker",
		SyncEnabled:      true,
		TaskEnabled:      true,
		HealthThresholds: DefaultHealthThresholds(),
		Tags:             []string{},
		Labels:           make(map[string]string),
		Version:          1,
	}
}

// ================== 命令下发 ==================

// AddCommand 添加节点命令
func (nms *NodeManagementService) AddCommand(deviceID string, command NodeCommand) error {
	nms.mu.Lock()
	defer nms.mu.Unlock()

	// 设置默认优先级
	if command.Priority == 0 {
		command.Priority = 5
	}

	nms.commands[deviceID] = append(nms.commands[deviceID], command)

	nms.logger.Info("添加节点命令",
		zap.String("device_id", deviceID),
		zap.String("command", command.Command))

	return nil
}

// GetPendingCommands 获取待执行命令
func (nms *NodeManagementService) GetPendingCommands(deviceID string) []NodeCommand {
	nms.mu.Lock()
	defer nms.mu.Unlock()

	commands := nms.commands[deviceID]
	if len(commands) == 0 {
		return []NodeCommand{}
	}

	// 过滤过期命令
	validCommands := make([]NodeCommand, 0)
	for _, cmd := range commands {
		if cmd.ExpiresAt.IsZero() || time.Now().Before(cmd.ExpiresAt) {
			validCommands = append(validCommands, cmd)
		}
	}

	// 清除已获取的命令
	nms.commands[deviceID] = []NodeCommand{}

	return validCommands
}

// ClearCommands 清除节点命令
func (nms *NodeManagementService) ClearCommands(deviceID string) {
	nms.mu.Lock()
	defer nms.mu.Unlock()
	nms.commands[deviceID] = []NodeCommand{}
}

// ================== 批量命令 ==================

// BroadcastCommand 广播命令到所有节点
func (nms *NodeManagementService) BroadcastCommand(command NodeCommand, filter DeviceFilter) int {
	nms.mu.Lock()
	defer nms.mu.Unlock()

	count := 0
	for deviceID := range nms.configs {
		if command.ExpiresAt.IsZero() {
			command.ExpiresAt = time.Now().Add(5 * time.Minute)
		}
		nms.commands[deviceID] = append(nms.commands[deviceID], command)
		count++
	}

	nms.logger.Info("广播命令",
		zap.String("command", command.Command),
		zap.Int("nodes", count))

	return count
}

// ================== 常用命令生成 ==================

// CreateSyncCommand 创建同步命令
func CreateSyncCommand(paths []string) NodeCommand {
	return NodeCommand{
		Command:   "sync",
		Params:    map[string]interface{}{"paths": paths},
		Priority:  7,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}

// CreateBackupCommand 创建备份命令
func CreateBackupCommand(sourcePath, targetPath string, schedule string) NodeCommand {
	return NodeCommand{
		Command: "backup",
		Params: map[string]interface{}{
			"source":   sourcePath,
			"target":   targetPath,
			"schedule": schedule,
		},
		Priority:  8,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
}

// CreateRestartCommand 创建重启命令
func CreateRestartCommand(reason string) NodeCommand {
	return NodeCommand{
		Command:   "restart",
		Params:    map[string]interface{}{"reason": reason},
		Priority:  10, // 最高优先级
		ExpiresAt: time.Now().Add(2 * time.Minute),
	}
}

// CreateUpdateCommand 创建更新命令
func CreateUpdateCommand(version string, force bool) NodeCommand {
	return NodeCommand{
		Command: "update",
		Params: map[string]interface{}{
			"version": version,
			"force":   force,
		},
		Priority:  9,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
}

// CreateHealthCheckCommand 创建健康检查命令
func CreateHealthCheckCommand(full bool) NodeCommand {
	return NodeCommand{
		Command:   "health_check",
		Params:    map[string]interface{}{"full": full},
		Priority:  5,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

// ================== 配置同步 ==================

// SyncConfig 同步配置到节点
func (nms *NodeManagementService) SyncConfig(deviceID string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		// 使用默认配置
		config = nms.GetDefaultConfig(deviceID)
	}

	// 构建配置同步命令
	cmd := NodeCommand{
		Command:   "config_sync",
		Params:    map[string]interface{}{"config": config},
		Priority:  6,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	return nms.AddCommand(deviceID, cmd)
}

// ================== 节点操作 ==================

// EnableNode 启用节点
func (nms *NodeManagementService) EnableNode(deviceID string) error {
	config := nms.GetDefaultConfig(deviceID)
	config.TaskEnabled = true
	config.SyncEnabled = true
	return nms.SetNodeConfig(deviceID, *config)
}

// DisableNode 禁用节点
func (nms *NodeManagementService) DisableNode(deviceID string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		config = nms.GetDefaultConfig(deviceID)
	}
	config.TaskEnabled = false
	config.SyncEnabled = false
	return nms.SetNodeConfig(deviceID, *config)
}

// SetNodeRole 设置节点角色
func (nms *NodeManagementService) SetNodeRole(deviceID, role string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		config = nms.GetDefaultConfig(deviceID)
	}
	config.Role = role
	return nms.SetNodeConfig(deviceID, *config)
}

// SetNodeGroup 设置节点分组
func (nms *NodeManagementService) SetNodeGroup(deviceID, groupID string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		config = nms.GetDefaultConfig(deviceID)
	}
	config.GroupID = groupID
	return nms.SetNodeConfig(deviceID, *config)
}

// AddNodeTags 添加节点标签
func (nms *NodeManagementService) AddNodeTags(deviceID string, tags []string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		config = nms.GetDefaultConfig(deviceID)
	}

	for _, tag := range tags {
		found := false
		for _, t := range config.Tags {
			if t == tag {
				found = true
				break
			}
		}
		if !found {
			config.Tags = append(config.Tags, tag)
		}
	}

	return nms.SetNodeConfig(deviceID, *config)
}

// SetNodeLabels 设置节点自定义标签
func (nms *NodeManagementService) SetNodeLabels(deviceID string, labels map[string]string) error {
	config, err := nms.GetNodeConfig(deviceID)
	if err != nil {
		config = nms.GetDefaultConfig(deviceID)
	}

	if config.Labels == nil {
		config.Labels = make(map[string]string)
	}

	for k, v := range labels {
		config.Labels[k] = v
	}

	return nms.SetNodeConfig(deviceID, *config)
}

// ================== 健康检查 ==================

// CheckNodeHealth 检查节点健康状态
func (nms *NodeManagementService) CheckNodeHealth(device *Device) NodeHealthReport {
	config, err := nms.GetNodeConfig(device.ID)
	if err != nil {
		config = nms.GetDefaultConfig(device.ID)
	}

	report := NodeHealthReport{
		DeviceID: device.ID,
		Status:   "healthy",
		Issues:   make([]HealthIssue, 0),
	}

	// CPU 检查
	if device.CPUUsage > config.HealthThresholds.CPUHigh {
		report.Status = "warning"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "cpu_high",
			Message:  fmt.Sprintf("CPU使用率 %.1f%% 超过阈值 %.1f%%", device.CPUUsage, config.HealthThresholds.CPUHigh),
			Severity: "medium",
			Value:    device.CPUUsage,
		})
	}

	// 内存检查
	if device.MemoryUsage > config.HealthThresholds.MemoryHigh {
		report.Status = "warning"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "memory_high",
			Message:  fmt.Sprintf("内存使用率 %.1f%% 超过阈值 %.1f%%", device.MemoryUsage, config.HealthThresholds.MemoryHigh),
			Severity: "medium",
			Value:    device.MemoryUsage,
		})
	}

	// 磁盘检查
	if device.DiskUsage > config.HealthThresholds.DiskHigh {
		report.Status = "warning"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "disk_high",
			Message:  fmt.Sprintf("磁盘使用率 %.1f%% 超过阈值 %.1f%%", device.DiskUsage, config.HealthThresholds.DiskHigh),
			Severity: "high",
			Value:    device.DiskUsage,
		})
	}

	// 温度检查
	if device.Temperature > config.HealthThresholds.TempHigh {
		report.Status = "warning"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "temperature_high",
			Message:  fmt.Sprintf("温度 %.1f°C 超过阈值 %.1f°C", device.Temperature, config.HealthThresholds.TempHigh),
			Severity: "medium",
			Value:    device.Temperature,
		})
	}

	// 健康分检查
	if device.HealthScore < config.HealthThresholds.HealthLow {
		report.Status = "critical"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "health_low",
			Message:  fmt.Sprintf("健康分 %d 低于阈值 %d", device.HealthScore, config.HealthThresholds.HealthLow),
			Severity: "high",
			Value:    float64(device.HealthScore),
		})
	}

	// 离线检查
	if device.Status == DeviceStatusOffline {
		report.Status = "offline"
		report.Issues = append(report.Issues, HealthIssue{
			Type:     "offline",
			Message:  "节点离线",
			Severity: "critical",
		})
	}

	report.Score = device.HealthScore
	report.CheckedAt = time.Now()

	return report
}

// NodeHealthReport 节点健康报告
type NodeHealthReport struct {
	DeviceID  string        `json:"deviceId"`
	Status    string        `json:"status"`    // healthy, warning, critical, offline
	Score     int           `json:"score"`     // 0-100
	Issues    []HealthIssue `json:"issues"`    // 问题列表
	CheckedAt time.Time     `json:"checkedAt"` // 检查时间
}

// HealthIssue 健康问题
type HealthIssue struct {
	Type     string  `json:"type"`     // 问题类型
	Message  string  `json:"message"`  // 描述
	Severity string  `json:"severity"` // low, medium, high, critical
	Value    float64 `json:"value"`    // 当前值
}

// ================== HTTP 处理器 ==================

// HTTPHandler HTTP 处理器
type HTTPHandler struct {
	fleetManager *FleetManager
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(fm *FleetManager) *HTTPHandler {
	return &HTTPHandler{fleetManager: fm}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(r interface{}) {
	// 使用 gin.RouterGroup 或其他路由注册器
	// 具体路由注册由调用方实现
}

// HandleHeartbeat 处理心跳请求
func (h *HTTPHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.fleetManager.ProcessHeartbeat(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleRegister 处理注册请求
func (h *HTTPHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req NodeRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.fleetManager.RegisterNode(req, "system")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleConfirm 处理确认注册
func (h *HTTPHandler) HandleConfirm(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	token := r.URL.Query().Get("token")

	if deviceID == "" || token == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}

	device, err := h.fleetManager.ConfirmNodeRegistration(deviceID, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

// HandleStatus 处理状态查询
func (h *HTTPHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")

	if deviceID != "" {
		status, err := h.fleetManager.GetNodeStatus(deviceID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	} else {
		status := h.fleetManager.GetFleetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
