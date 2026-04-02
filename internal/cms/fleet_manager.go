// Package cms 集群管理系统服务
// FleetManager - 多节点集中管理，类似群晖 CMS
package cms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ================== FleetManager 核心接口 ==================

// FleetManager 集群舰队管理器
// 负责多节点集中管理、任务调度、状态聚合
type FleetManager struct {
	registry       *DeviceRegistry
	nodeManager    *NodeManagementService
	taskDispatcher *TaskDispatcher
	statusAggr     *StatusAggregator
	config         FleetConfig
	logger         *zap.Logger
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

// FleetConfig 舰队配置
type FleetConfig struct {
	ClusterName       string        `json:"clusterName"`       // 集群名称
	FleetID           string        `json:"fleetId"`           // 舰队ID
	ControllerNode    string        `json:"controllerNode"`    // 控制节点地址
	DataDir           string        `json:"dataDir"`           // 数据目录
	HeartbeatTimeout  time.Duration `json:"heartbeatTimeout"`  // 心跳超时
	SyncInterval      time.Duration `json:"syncInterval"`      // 同步间隔
	MaxNodes          int           `json:"maxNodes"`          // 最大节点数
	EnableAutoDiscover bool         `json:"enableAutoDiscover"` // 启用自动发现
}

// DefaultFleetConfig 默认舰队配置
func DefaultFleetConfig() FleetConfig {
	return FleetConfig{
		ClusterName:       "nas-fleet",
		DataDir:           "/var/lib/nas-os/fleet",
		HeartbeatTimeout:  30 * time.Second,
		SyncInterval:      60 * time.Second,
		MaxNodes:          100,
		EnableAutoDiscover: true,
	}
}

// NewFleetManager 创建舰队管理器
func NewFleetManager(config FleetConfig, logger *zap.Logger) (*FleetManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 生成舰队ID
	if config.FleetID == "" {
		config.FleetID = uuid.New().String()
	}

	// 创建设备注册器
	registryConfig := &RegistryConfig{
		HeartbeatTimeout:    config.HeartbeatTimeout,
		HealthCheckInterval: config.SyncInterval,
		MaxDevices:          config.MaxNodes,
	}
	registry := NewDeviceRegistry(registryConfig, logger)

	// 创建节点管理服务
	nodeManager := NewNodeManagementService(config, logger)

	// 创建任务分发器
	taskDispatcher, err := NewTaskDispatcher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("创建任务分发器失败: %w", err)
	}

	// 创建状态聚合器
	statusAggr, err := NewStatusAggregator(config, logger)
	if err != nil {
		return nil, fmt.Errorf("创建状态聚合器失败: %w", err)
	}

	fm := &FleetManager{
		registry:       registry,
		nodeManager:    nodeManager,
		taskDispatcher: taskDispatcher,
		statusAggr:     statusAggr,
		config:         config,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}

	// 加载持久化数据
	if err := fm.loadState(); err != nil {
		logger.Warn("加载舰队状态失败", zap.Error(err))
	}

	return fm, nil
}

// Start 启动舰队管理器
func (fm *FleetManager) Start() error {
	fm.registry.Start()
	fm.nodeManager.Start()
	fm.taskDispatcher.Start()
	fm.statusAggr.Start()

	// 启动状态同步
	go fm.syncLoop()

	fm.logger.Info("舰队管理器启动",
		zap.String("fleet_id", fm.config.FleetID),
		zap.String("cluster", fm.config.ClusterName))

	return nil
}

// Stop 停止舰队管理器
func (fm *FleetManager) Stop() {
	fm.cancel()
	fm.registry.Stop()
	fm.nodeManager.Stop()
	fm.taskDispatcher.Stop()
	fm.statusAggr.Stop()
	fm.saveState()
	fm.logger.Info("舰队管理器停止")
}

// ================== 节点注册机制 ==================

// NodeRegistrationRequest 节点注册请求
type NodeRegistrationRequest struct {
	Name            string            `json:"name"`            // 节点名称
	Type            DeviceType        `json:"type"`            // 节点类型
	IPAddress       string            `json:"ipAddress"`       // IP地址
	Port            int               `json:"port"`            // API端口
	MACAddress      string            `json:"macAddress"`      // MAC地址
	SerialNumber    string            `json:"serialNumber"`    // 序列号
	Model           string            `json:"model"`           // 设备型号
	FirmwareVersion string            `json:"firmwareVersion"` // 固件版本
	Location        string            `json:"location"`        // 位置
	Labels          map[string]string `json:"labels"`          // 标签
	Capabilities    []string          `json:"capabilities"`    // 能力列表
	AuthToken       string            `json:"authToken"`       // 认证令牌（用于重新注册）
}

// NodeRegistrationResponse 节点注册响应
type NodeRegistrationResponse struct {
	DeviceID        string    `json:"deviceId"`        // 设备ID
	RegisterToken   string    `json:"registerToken"`   // 注册确认令牌
	ControllerAddr  string    `json:"controllerAddr"`  // 控制器地址
	HeartbeatURL    string    `json:"heartbeatUrl"`    // 心跳URL
	StatusURL       string    `json:"statusUrl"`       // 状态上报URL
	TaskPollURL     string    `json:"taskPollUrl"`     // 任务拉取URL
	ExpiresAt       time.Time `json:"expiresAt"`       // 令牌过期时间
	ClusterName     string    `json:"clusterName"`     // 集群名称
	Success         bool      `json:"success"`         // 是否成功
	Message         string    `json:"message"`         // 消息
}

// RegisterNode 注册新节点
func (fm *FleetManager) RegisterNode(req NodeRegistrationRequest, registeredBy string) (*NodeRegistrationResponse, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 检查节点数量限制
	stats := fm.registry.GetStats()
	if stats.TotalDevices >= fm.config.MaxNodes {
		return nil, errors.New("节点数量已达上限")
	}

	// 验证请求
	if req.Name == "" || req.IPAddress == "" {
		return nil, errors.New("节点名称和IP地址必须提供")
	}

	// 检查是否已注册
	if req.AuthToken != "" {
		// 尝试重新注册
		device, err := fm.registry.GetDeviceByIP(req.IPAddress, req.Port)
		if err == nil && device.RegisterToken == req.AuthToken {
			// 已注册，更新信息
			updates := map[string]interface{}{
				"firmware_version": req.FirmwareVersion,
				"location":         req.Location,
				"labels":           req.Labels,
			}
			_, _ = fm.registry.UpdateDevice(device.ID, updates)
			return fm.buildRegistrationResponse(device, false), nil
		}
	}

	// 新节点注册
	device, err := fm.registry.RegisterDevice(req.Type, req.Name, req.IPAddress, req.Port, registeredBy)
	if err != nil {
		return nil, err
	}

	// 设置额外信息
	updates := map[string]interface{}{
		"mac_address":      req.MACAddress,
		"serial_number":    req.SerialNumber,
		"model":            req.Model,
		"firmware_version": req.FirmwareVersion,
		"location":         req.Location,
		"group_id":         "",
	}
	_, _ = fm.registry.UpdateDevice(device.ID, updates)

	// 添加能力标签
	if len(req.Capabilities) > 0 {
		device.Labels = make(map[string]string)
		for _, cap := range req.Capabilities {
			device.Labels["cap_"+cap] = "true"
		}
	}

	fm.logger.Info("节点注册成功",
		zap.String("device_id", device.ID),
		zap.String("name", req.Name),
		zap.String("ip", req.IPAddress))

	// 持久化
	fm.saveState()

	return fm.buildRegistrationResponse(device, true), nil
}

// ConfirmNodeRegistration 确认节点注册
func (fm *FleetManager) ConfirmNodeRegistration(deviceID, token string) (*Device, error) {
	device, err := fm.registry.ConfirmRegistration(deviceID, token)
	if err != nil {
		return nil, err
	}

	fm.logger.Info("节点注册确认完成", zap.String("device_id", deviceID))
	fm.saveState()

	return device, nil
}

// UnregisterNode 注销节点
func (fm *FleetManager) UnregisterNode(deviceID string) error {
	err := fm.registry.UnregisterDevice(deviceID)
	if err != nil {
		return err
	}

	fm.logger.Info("节点已注销", zap.String("device_id", deviceID))
	fm.saveState()

	return nil
}

// buildRegistrationResponse 构建注册响应
func (fm *FleetManager) buildRegistrationResponse(device *Device, isNew bool) *NodeRegistrationResponse {
	controllerAddr := fm.config.ControllerNode
	if controllerAddr == "" {
		controllerAddr = "localhost:8080"
	}

	message := "节点已重新注册"
	if isNew {
		message = "节点注册成功，请确认注册"
	}

	return &NodeRegistrationResponse{
		DeviceID:       device.ID,
		RegisterToken:  device.RegisterToken,
		ControllerAddr: controllerAddr,
		HeartbeatURL:   fmt.Sprintf("http://%s/api/v1/fleet/heartbeat", controllerAddr),
		StatusURL:      fmt.Sprintf("http://%s/api/v1/fleet/status", controllerAddr),
		TaskPollURL:    fmt.Sprintf("http://%s/api/v1/fleet/tasks", controllerAddr),
		ExpiresAt:      time.Now().Add(5 * time.Minute),
		ClusterName:    fm.config.ClusterName,
		Success:        true,
		Message:        message,
	}
}

// ================== 状态查询接口 ==================

// GetNode 获取节点信息
func (fm *FleetManager) GetNode(deviceID string) (*Device, error) {
	return fm.registry.GetDevice(deviceID)
}

// ListNodes 列出所有节点
func (fm *FleetManager) ListNodes(filter DeviceFilter) []*Device {
	return fm.registry.ListDevices(filter)
}

// GetNodeStatus 获取节点详细状态
func (fm *FleetManager) GetNodeStatus(deviceID string) (*NodeDetailedStatus, error) {
	return fm.statusAggr.GetNodeStatus(deviceID)
}

// GetFleetStatus 获取舰队整体状态
func (fm *FleetManager) GetFleetStatus() *FleetStatus {
	return fm.statusAggr.GetFleetStatus()
}

// ================== 任务调度接口 ==================

// DispatchTask 分发任务到节点
func (fm *FleetManager) DispatchTask(req TaskDispatchRequest) (*TaskDispatchResult, error) {
	return fm.taskDispatcher.Dispatch(req)
}

// GetNodeTasks 获取节点任务列表
func (fm *FleetManager) GetNodeTasks(deviceID string) ([]NodeTask, error) {
	return fm.taskDispatcher.GetNodeTasks(deviceID)
}

// CancelTask 取消任务
func (fm *FleetManager) CancelTask(taskID string) error {
	return fm.taskDispatcher.CancelTask(taskID)
}

// ================== 心跳处理 ==================

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	DeviceID   string                 `json:"deviceId"`
	Token      string                 `json:"token"`
	Metrics    map[string]interface{} `json:"metrics"`
	Status     DeviceStatus           `json:"status"`
	Tasks      []TaskProgress         `json:"tasks"`
	Timestamp  time.Time              `json:"timestamp"`
}

// TaskProgress 任务进度
type TaskProgress struct {
	TaskID    string    `json:"taskId"`
	Status    string    `json:"status"`
	Progress  float64   `json:"progress"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success       bool              `json:"success"`
	Message       string            `json:"message"`
	Commands      []NodeCommand     `json:"commands"`      // 控制命令
	ConfigChanges map[string]string `json:"configChanges"` // 配置变更
	ServerTime    time.Time         `json:"serverTime"`
}

// NodeCommand 节点命令
type NodeCommand struct {
	Command   string                 `json:"command"`   // 命令类型
	Params    map[string]interface{} `json:"params"`    // 参数
	Priority  int                    `json:"priority"`  // 优先级
	ExpiresAt time.Time              `json:"expiresAt"` // 过期时间
}

// ProcessHeartbeat 处理节点心跳
func (fm *FleetManager) ProcessHeartbeat(req HeartbeatRequest) (*HeartbeatResponse, error) {
	// 验证设备
	_, err := fm.registry.GetDevice(req.DeviceID)
	if err != nil {
		return nil, err
	}

	// 更新心跳和指标
	err = fm.registry.UpdateHeartbeat(req.DeviceID, req.Metrics)
	if err != nil {
		return nil, err
	}

	// 更新状态
	if req.Status != "" {
		updates := map[string]interface{}{"status": req.Status}
		_, _ = fm.registry.UpdateDevice(req.DeviceID, updates)
	}

	// 处理任务进度
	for _, taskProg := range req.Tasks {
		fm.taskDispatcher.UpdateTaskProgress(req.DeviceID, taskProg)
	}

	// 获取待执行命令
	commands := fm.nodeManager.GetPendingCommands(req.DeviceID)

	// 更新状态聚合
	fm.statusAggr.UpdateNodeMetrics(req.DeviceID, req.Metrics)

	return &HeartbeatResponse{
		Success:    true,
		Message:    "心跳处理成功",
		Commands:   commands,
		ServerTime: time.Now(),
	}, nil
}

// ================== 内部方法 ==================

// syncLoop 状态同步循环
func (fm *FleetManager) syncLoop() {
	ticker := time.NewTicker(fm.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.syncFleetStatus()
		}
	}
}

// syncFleetStatus 同步舰队状态
func (fm *FleetManager) syncFleetStatus() {
	nodes := fm.ListNodes(DeviceFilter{})

	for _, node := range nodes {
		if node.Status == DeviceStatusOffline {
			continue
		}

		// 拉取节点状态
		go fm.pullNodeStatus(node)
	}
}

// pullNodeStatus 拉取节点状态
func (fm *FleetManager) pullNodeStatus(node *Device) {
	url := fmt.Sprintf("http://%s:%d/api/v1/status", node.IPAddress, node.Port)

	ctx, cancel := context.WithTimeout(fm.ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Set("X-Fleet-ID", fm.config.FleetID)
	req.Header.Set("X-Node-ID", node.ID)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fm.logger.Debug("拉取节点状态失败",
			zap.String("node_id", node.ID),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var status map[string]interface{}
		if json.NewDecoder(resp.Body).Decode(&status) == nil {
			fm.statusAggr.UpdateNodeMetrics(node.ID, status)
		}
	}
}

// loadState 加载状态
func (fm *FleetManager) loadState() error {
	stateFile := filepath.Join(fm.config.DataDir, "fleet_state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		FleetID string `json:"fleetId"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if state.FleetID != "" {
		fm.config.FleetID = state.FleetID
	}

	return nil
}

// saveState 保存状态
func (fm *FleetManager) saveState() {
	state := map[string]interface{}{
		"fleetId":    fm.config.FleetID,
		"clusterName": fm.config.ClusterName,
		"timestamp":  time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		fm.logger.Warn("序列化状态失败", zap.Error(err))
		return
	}

	stateFile := filepath.Join(fm.config.DataDir, "fleet_state.json")
	if err := os.WriteFile(stateFile, data, 0640); err != nil {
		fm.logger.Warn("保存状态失败", zap.Error(err))
	}
}

// ================== 辅助函数 ==================

// GenerateToken 生成随机令牌
func GenerateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}