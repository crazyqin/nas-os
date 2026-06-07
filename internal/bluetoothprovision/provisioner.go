package bluetoothprovision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultProvisioner 实现默认配网引擎
type DefaultProvisioner struct {
	mu         sync.RWMutex
	sessions   map[string]*ProvisionSession
	history    []ProvisionHistory
	maxHistory int
	timeout    time.Duration
	onEvent    func(ProvisionEvent)
}

// NewDefaultProvisioner 创建默认配网引擎
func NewDefaultProvisioner(opts ...ProvisionerOption) *DefaultProvisioner {
	p := &DefaultProvisioner{
		sessions:   make(map[string]*ProvisionSession),
		history:    make([]ProvisionHistory, 0),
		maxHistory: 100,
		timeout:    60 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ProvisionerOption 配网器配置选项
type ProvisionerOption func(*DefaultProvisioner)

// WithProvisionerTimeout 设置配网超时
func WithProvisionerTimeout(d time.Duration) ProvisionerOption {
	return func(p *DefaultProvisioner) {
		p.timeout = d
	}
}

// WithProvisionerMaxHistory 设置最大历史记录数
func WithProvisionerMaxHistory(n int) ProvisionerOption {
	return func(p *DefaultProvisioner) {
		p.maxHistory = n
	}
}

// WithProvisionerEventCallback 设置事件回调
func WithProvisionerEventCallback(fn func(ProvisionEvent)) ProvisionerOption {
	return func(p *DefaultProvisioner) {
		p.onEvent = fn
	}
}

// StartProvision 启动配网流程
func (p *DefaultProvisioner) StartProvision(req ProvisionRequest) (*ProvisionSession, error) {
	// 参数校验
	if req.DeviceID == "" {
		return nil, fmt.Errorf("设备ID不能为空")
	}
	if req.WiFiConfig.SSID == "" {
		return nil, fmt.Errorf("WiFi SSID不能为空")
	}

	timeout := p.timeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	// 创建会话
	session := &ProvisionSession{
		ID:         uuid.New().String(),
		DeviceID:   req.DeviceID,
		WiFiConfig: req.WiFiConfig,
		Status:     StatusPending,
		Progress:   0,
		StartTime:  time.Now(),
		Steps: []ProvisionStep{
			{Name: "连接设备", Status: "pending"},
			{Name: "身份认证", Status: "pending"},
			{Name: "发送WiFi配置", Status: "pending"},
			{Name: "验证连接", Status: "pending"},
		},
	}

	p.mu.Lock()
	p.sessions[session.ID] = session
	p.mu.Unlock()

	// 异步执行配网
	go p.executeProvision(session, timeout, req.RetryCount)

	return session, nil
}

// executeProvision 执行配网流程
func (p *DefaultProvisioner) executeProvision(session *ProvisionSession, timeout time.Duration, retryCount int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()

	// 步骤1: 连接设备
	p.updateStep(session, 0, "running", "")
	p.updateStatus(session, StatusConnecting, 10)
	p.publishEvent(session, "status_change", "正在连接设备...")

	select {
	case <-ctx.Done():
		p.failSession(session, "连接设备超时", startTime)
		return
	default:
		time.Sleep(500 * time.Millisecond) // 模拟连接
	}
	p.updateStep(session, 0, "success", "")
	p.updateProgress(session, 25)

	// 步骤2: 身份认证
	p.updateStep(session, 1, "running", "")
	p.updateStatus(session, StatusAuth, 25)
	p.publishEvent(session, "status_change", "正在认证...")

	select {
	case <-ctx.Done():
		p.failSession(session, "认证超时", startTime)
		return
	default:
		time.Sleep(300 * time.Millisecond) // 模拟认证
	}
	p.updateStep(session, 1, "success", "")
	p.updateProgress(session, 50)

	// 步骤3: 发送WiFi配置
	p.updateStep(session, 2, "running", "")
	p.updateStatus(session, StatusSending, 50)
	p.publishEvent(session, "status_change", "正在发送WiFi配置...")

	select {
	case <-ctx.Done():
		p.failSession(session, "发送配置超时", startTime)
		return
	default:
		time.Sleep(800 * time.Millisecond) // 模拟发送
	}
	p.updateStep(session, 2, "success", "")
	p.updateProgress(session, 75)

	// 步骤4: 验证连接
	p.updateStep(session, 3, "running", "")
	p.updateStatus(session, StatusVerifying, 75)
	p.publishEvent(session, "status_change", "正在验证连接...")

	select {
	case <-ctx.Done():
		p.failSession(session, "验证超时", startTime)
		return
	default:
		time.Sleep(500 * time.Millisecond) // 模拟验证
	}
	p.updateStep(session, 3, "success", "")

	// 配网成功
	endTime := time.Now()
	session.EndTime = &endTime
	session.Progress = 100
	session.Status = StatusSuccess
	session.NetworkInfo = &NetworkInfo{
		SSID:        session.WiFiConfig.SSID,
		IP:          "192.168.1.100",
		MAC:         "AA:BB:CC:DD:EE:FF",
		Gateway:     "192.168.1.1",
		DNS:         "8.8.8.8",
		Signal:      -45,
		Speed:       72,
		Connected:   true,
		ConnectedAt: endTime,
	}

	p.publishEvent(session, "complete", "配网成功")

	// 添加到历史记录
	p.addHistory(session, startTime)
	log.Printf("[Provisioner] 配网成功: 设备=%s, SSID=%s, 耗时=%v",
		session.DeviceName, session.WiFiConfig.SSID, endTime.Sub(startTime))
}

// CancelProvision 取消配网
func (p *DefaultProvisioner) CancelProvision(sessionID string) error {
	p.mu.Lock()
	session, ok := p.sessions[sessionID]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 只能取消进行中的会话
	if session.Status == StatusSuccess || session.Status == StatusFailed || session.Status == StatusCancelled {
		p.mu.Unlock()
		return fmt.Errorf("会话已结束，无法取消")
	}
	p.mu.Unlock()

	endTime := time.Now()
	session.EndTime = &endTime
	session.Status = StatusCancelled
	session.Error = "用户取消"

	p.publishEvent(session, "status_change", "配网已取消")

	// 添加到历史记录
	p.addHistory(session, session.StartTime)

	return nil
}

// GetSession 获取配网会话
func (p *DefaultProvisioner) GetSession(sessionID string) (*ProvisionSession, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session, ok := p.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}
	return session, nil
}

// GetHistory 获取配网历史记录
func (p *DefaultProvisioner) GetHistory(limit int) ([]ProvisionHistory, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 || limit > len(p.history) {
		limit = len(p.history)
	}

	// 返回最新的记录
	start := len(p.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ProvisionHistory, limit)
	copy(result, p.history[start:])
	return result, nil
}

// ClearHistory 清空历史记录
func (p *DefaultProvisioner) ClearHistory() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = make([]ProvisionHistory, 0)
	return nil
}

// updateStep 更新配网步骤状态
func (p *DefaultProvisioner) updateStep(session *ProvisionSession, index int, status, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(session.Steps) {
		return
	}

	session.Steps[index].Status = status
	if status == "running" {
		session.Steps[index].StartTime = time.Now()
	} else {
		session.Steps[index].EndTime = time.Now()
	}
	if errMsg != "" {
		session.Steps[index].Error = errMsg
	}
}

// updateStatus 更新会话状态
func (p *DefaultProvisioner) updateStatus(session *ProvisionSession, status ProvisionStatus, progress int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	session.Status = status
	session.Progress = progress
}

// updateProgress 更新进度
func (p *DefaultProvisioner) updateProgress(session *ProvisionSession, progress int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	session.Progress = progress
}

// failSession 标记会话失败
func (p *DefaultProvisioner) failSession(session *ProvisionSession, errMsg string, startTime time.Time) {
	endTime := time.Now()
	p.mu.Lock()
	session.EndTime = &endTime
	session.Status = StatusFailed
	session.Error = errMsg
	p.mu.Unlock()

	p.publishEvent(session, "error", errMsg)
	p.addHistory(session, startTime)
	log.Printf("[Provisioner] 配网失败: 设备=%s, 错误=%s", session.DeviceName, errMsg)
}

// addHistory 添加历史记录
func (p *DefaultProvisioner) addHistory(session *ProvisionSession, startTime time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	record := ProvisionHistory{
		ID:         uuid.New().String(),
		DeviceName: session.DeviceName,
		DeviceMAC:  session.DeviceID,
		SSID:       session.WiFiConfig.SSID,
		Status:     session.Status,
		Error:      session.Error,
		StartTime:  startTime,
		Duration:   time.Since(startTime),
	}

	p.history = append(p.history, record)

	// 超过最大记录数时删除旧记录
	if len(p.history) > p.maxHistory {
		p.history = p.history[len(p.history)-p.maxHistory:]
	}
}

// publishEvent 发布事件
func (p *DefaultProvisioner) publishEvent(session *ProvisionSession, eventType, message string) {
	if p.onEvent == nil {
		return
	}

	event := ProvisionEvent{
		Type:      eventType,
		SessionID: session.ID,
		DeviceID:  session.DeviceID,
		Status:    session.Status,
		Progress:  session.Progress,
		Message:   message,
		Timestamp: time.Now(),
	}

	p.onEvent(event)
}
