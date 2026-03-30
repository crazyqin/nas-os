// Package cms 提供集中管理系统，协调多设备管理
package cms

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 错误定义
var (
	ErrManagerNotReady   = errors.New("manager not ready")
	ErrOperationTimeout  = errors.New("operation timeout")
	ErrNoDevicesSelected = errors.New("no devices selected")
)

// OperationType 操作类型
type OperationType string

const (
	OpTypeDeploy    OperationType = "deploy"    // 部署操作
	OpTypeRestart   OperationType = "restart"   // 重启服务
	OpTypeUpdate    OperationType = "update"    // 更新固件
	OpTypeConfigure OperationType = "configure" // 配置更改
	OpTypeBackup    OperationType = "backup"    // 备份操作
	OpTypeSync      OperationType = "sync"      // 同步数据
	OpTypeStatus    OperationType = "status"    // 状态查询
)

// OperationStatus 操作状态
type OperationStatus string

const (
	OpStatusPending   OperationStatus = "pending"   // 待执行
	OpStatusRunning   OperationStatus = "running"   // 执行中
	OpStatusCompleted OperationStatus = "completed" // 已完成
	OpStatusFailed    OperationStatus = "failed"    // 失败
	OpStatusCancelled OperationStatus = "cancelled" // 已取消
	OpStatusPartial   OperationStatus = "partial"   // 部分成功
)

// Operation 批量操作
type Operation struct {
	ID          string            `json:"id"`
	Type        OperationType     `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	DeviceIDs   []string          `json:"device_ids"`
	Status      OperationStatus   `json:"status"`
	
	// 配置参数
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	
	// 执行进度
	Progress    int               `json:"progress"` // 0-100
	TotalTasks  int               `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	FailedTasks int               `json:"failed_tasks"`
	
	// 时间记录
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	CreatedBy   string            `json:"created_by"`
	
	// 结果汇总
	Results     map[string]*TaskResult `json:"results"` // deviceID -> result
	Errors      []OperationError       `json:"errors,omitempty"`
}

// TaskResult 任务结果
type TaskResult struct {
	DeviceID    string    `json:"device_id"`
	Status      string    `json:"status"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	DurationMs  int64     `json:"duration_ms"`
	CompletedAt time.Time `json:"completed_at"`
}

// OperationError 操作错误
type OperationError struct {
	DeviceID string `json:"device_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// OperationTemplate 操作模板
type OperationTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        OperationType          `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"default_parameters"`
	CreatedAt   time.Time              `json:"created_at"`
}

// CMSManager CMS 管理器
type CMSManager struct {
	mu            sync.RWMutex
	registry      *DeviceRegistry
	operations    map[string]*Operation
	templates     map[string]*OperationTemplate
	deployer      *BatchDeployer
	
	// 操作执行器
	executors     map[OperationType]OperationExecutor
	
	// 配置
	config        *ManagerConfig
	
	// 生命周期
	ctx           context.Context
	cancel        context.CancelFunc
	logger        *zap.Logger
	
	// 操作队列
	opQueue       chan *Operation
	concurrency   int
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxConcurrentOps  int           `json:"max_concurrent_ops"`
	OperationTimeout  time.Duration `json:"operation_timeout"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
}

// OperationExecutor 操作执行器接口
type OperationExecutor interface {
	Execute(ctx context.Context, device *Device, params map[string]interface{}) (*TaskResult, error)
	Validate(params map[string]interface{}) error
}

// NewCMSManager 创建 CMS 管理器
func NewCMSManager(registry *DeviceRegistry, config *ManagerConfig, logger *zap.Logger) *CMSManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	if config == nil {
	 config = &ManagerConfig{
	  MaxConcurrentOps: 10,
	  OperationTimeout: 5 * time.Minute,
	  RetryAttempts:    3,
	  RetryDelay:       30 * time.Second,
	 }
	}
	
	mgr := &CMSManager{
	 registry:    registry,
	 operations:  make(map[string]*Operation),
	 templates:   make(map[string]*OperationTemplate),
	 executors:   make(map[OperationType]OperationExecutor),
	 ctx:         ctx,
	 cancel:      cancel,
	 logger:      logger,
	 config:      config,
	 opQueue:     make(chan *Operation, 100),
	 concurrency: config.MaxConcurrentOps,
	}
	
	// 初始化部署器
	mgr.deployer = NewBatchDeployer(registry, logger)
	
	return mgr
}

// Start 启动管理器
func (m *CMSManager) Start() {
	// 启动操作处理循环
	for i := 0; i < m.concurrency; i++ {
	 go m.operationWorker(i)
	}
	
	m.logger.Info("CMS manager started", zap.Int("workers", m.concurrency))
}

// Stop 停止管理器
func (m *CMSManager) Stop() {
	m.cancel()
	m.logger.Info("CMS manager stopped")
}

// RegisterExecutor 注册操作执行器
func (m *CMSManager) RegisterExecutor(opType OperationType, executor OperationExecutor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executors[opType] = executor
}

// CreateOperation 创建操作
func (m *CMSManager) CreateOperation(opType OperationType, name, description string, deviceIDs []string, params map[string]interface{}, createdBy string) (*Operation, error) {
	if len(deviceIDs) == 0 {
	 return nil, ErrNoDevicesSelected
	}
	
	// 验证参数
	if executor, exists := m.executors[opType]; exists {
	 if err := executor.Validate(params); err != nil {
	  return nil, err
	 }
	}
	
	// 验证设备存在且在线
	for _, deviceID := range deviceIDs {
	 device, err := m.registry.GetDevice(deviceID)
	 if err != nil {
	  return nil, err
	 }
	 if device.Status == DeviceStatusOffline {
	  return nil, ErrDeviceOffline
	 }
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	op := &Operation{
	 ID:          uuid.New().String(),
	 Type:        opType,
	 Name:        name,
	 Description: description,
	 DeviceIDs:   deviceIDs,
	 Status:      OpStatusPending,
	 Parameters:  params,
	 Progress:    0,
	 TotalTasks:  len(deviceIDs),
	 Results:     make(map[string]*TaskResult),
	 CreatedAt:   now,
	 CreatedBy:   createdBy,
	}
	
	m.operations[op.ID] = op
	
	// 加入执行队列
	m.opQueue <- op
	
	m.logger.Info("Operation created",
	 zap.String("op_id", op.ID),
	 zap.String("type", string(opType)),
	 zap.Int("devices", len(deviceIDs)),
	)
	
	return op, nil
}

// GetOperation 获取操作
func (m *CMSManager) GetOperation(opID string) (*Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	op, exists := m.operations[opID]
	if !exists {
	 return nil, errors.New("operation not found")
	}
	return op, nil
}

// CancelOperation 取消操作
func (m *CMSManager) CancelOperation(opID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	op, exists := m.operations[opID]
	if !exists {
	 return errors.New("operation not found")
	}
	
	if op.Status == OpStatusCompleted || op.Status == OpStatusCancelled {
	 return errors.New("operation already finished")
	}
	
	op.Status = OpStatusCancelled
	now := time.Now()
 op.CompletedAt = &now
	
	m.logger.Info("Operation cancelled", zap.String("op_id", opID))
	return nil
}

// ListOperations 列出操作
func (m *CMSManager) ListOperations(filter OperationFilter) []*Operation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*Operation, 0)
	for _, op := range m.operations {
	 if m.matchesOpFilter(op, filter) {
	  result = append(result, op)
	 }
	}
	
	return result
}

// OperationFilter 操作筛选条件
type OperationFilter struct {
	Type   []OperationType   `json:"type,omitempty"`
	Status []OperationStatus `json:"status,omitempty"`
	User   string            `json:"user,omitempty"`
}

// matchesOpFilter 检查操作是否匹配筛选条件
func (m *CMSManager) matchesOpFilter(op *Operation, filter OperationFilter) bool {
	if len(filter.Type) > 0 {
	 matched := false
	 for _, t := range filter.Type {
	  if op.Type == t {
	   matched = true
	   break
	  }
	 }
	 if !matched {
	  return false
	 }
	}
	
	if len(filter.Status) > 0 {
	 matched := false
	 for _, s := range filter.Status {
	  if op.Status == s {
	   matched = true
	   break
	  }
	 }
	 if !matched {
	  return false
	 }
	}
	
	if filter.User != "" && op.CreatedBy != filter.User {
	 return false
	}
	
	return true
}

// operationWorker 操作处理工作线程
func (m *CMSManager) operationWorker(workerID int) {
	for {
	 select {
	 case <-m.ctx.Done():
	  return
	 case op := <-m.opQueue:
	  m.executeOperation(op, workerID)
	 }
	}
}

// executeOperation 执行操作
func (m *CMSManager) executeOperation(op *Operation, workerID int) {
	m.mu.Lock()
	if op.Status == OpStatusCancelled {
	 m.mu.Unlock()
	 return
	}
	
	now := time.Now()
	op.Status = OpStatusRunning
	op.StartedAt = &now
	m.mu.Unlock()
	
	m.logger.Info("Operation started",
	 zap.String("op_id", op.ID),
	 zap.Int("worker", workerID),
	)
	
	// 获取执行器
	executor, exists := m.executors[op.Type]
	if !exists {
	 m.completeOperation(op, OpStatusFailed, "No executor registered for operation type")
	 return
	}
	
	// 执行每个设备的任务
	var wg sync.WaitGroup
	resultChan := make(chan *TaskResult, len(op.DeviceIDs))
	
	for _, deviceID := range op.DeviceIDs {
	 wg.Add(1)
	 go func(dID string) {
	  defer wg.Done()
	  
	  device, err := m.registry.GetDevice(dID)
	  if err != nil {
	   resultChan <- &TaskResult{
	    DeviceID: dID,
	    Status:   "failed",
	    Error:    err.Error(),
	   }
	   return
	  }
	  
	  // 执行任务，支持重试
	  var result *TaskResult
	  for attempt := 0; attempt < m.config.RetryAttempts; attempt++ {
	   ctx, cancel := context.WithTimeout(m.ctx, m.config.OperationTimeout)
	   
	   result, err = executor.Execute(ctx, device, op.Parameters)
	   cancel()
	   
	   if err == nil {
	    break
	   }
	   
	   if attempt < m.config.RetryAttempts-1 {
	    time.Sleep(m.config.RetryDelay)
	    m.logger.Debug("Retrying task",
	     zap.String("device_id", dID),
	     zap.Int("attempt", attempt+1),
	    )
	   }
	  }
	  
	  if err != nil {
	   result = &TaskResult{
	    DeviceID: dID,
	    Status:   "failed",
	    Error:    err.Error(),
	   }
	  }
	  
	  resultChan <- result
	 }(deviceID)
	}
	
	wg.Wait()
	close(resultChan)
	
	// 处理结果
	m.mu.Lock()
	for result := range resultChan {
	 op.Results[result.DeviceID] = result
	 
	 if result.Status == "success" {
	  op.CompletedTasks++
	 } else {
	  op.FailedTasks++
	  op.Errors = append(op.Errors, OperationError{
	   DeviceID: result.DeviceID,
	   Code:     "execution_error",
	   Message:  result.Error,
	  })
	 }
	}
	
	// 更新进度
	if op.TotalTasks > 0 {
	 op.Progress = (op.CompletedTasks + op.FailedTasks) * 100 / op.TotalTasks
	}
	m.mu.Unlock()
	
	// 确定最终状态
	finalStatus := OpStatusCompleted
	if op.FailedTasks > 0 {
	 if op.CompletedTasks > 0 {
	  finalStatus = OpStatusPartial
	 } else {
	  finalStatus = OpStatusFailed
	 }
	}
	
	m.completeOperation(op, finalStatus, "")
}

// completeOperation 完成操作
func (m *CMSManager) completeOperation(op *Operation, status OperationStatus, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	op.Status = status
	op.CompletedAt = &now
	op.Progress = 100
	
	if errMsg != "" {
	 op.Errors = append(op.Errors, OperationError{
	  Code:    "execution_error",
	  Message: errMsg,
	 })
	}
	
	m.logger.Info("Operation completed",
	 zap.String("op_id", op.ID),
	 zap.String("status", string(status)),
	 zap.Int("success", op.CompletedTasks),
	 zap.Int("failed", op.FailedTasks),
	)
}

// CreateTemplate 创建操作模板
func (m *CMSManager) CreateTemplate(name string, opType OperationType, description string, defaultParams map[string]interface{}) (*OperationTemplate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	template := &OperationTemplate{
	 ID:          uuid.New().String(),
	 Name:        name,
	 Type:        opType,
	 Description: description,
	 Parameters:  defaultParams,
	 CreatedAt:   time.Now(),
	}
	
	m.templates[template.ID] = template
	return template, nil
}

// GetTemplate 获取模板
func (m *CMSManager) GetTemplate(templateID string) (*OperationTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	template, exists := m.templates[templateID]
	if !exists {
	 return nil, errors.New("template not found")
	}
	return template, nil
}

// ListTemplates 列出模板
func (m *CMSManager) ListTemplates(opType OperationType) []*OperationTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*OperationTemplate, 0)
	for _, template := range m.templates {
	 if opType == "" || template.Type == opType {
	  result = append(result, template)
	 }
	}
	
	return result
}

// ExecuteFromTemplate 从模板执行操作
func (m *CMSManager) ExecuteFromTemplate(templateID string, deviceIDs []string, customParams map[string]interface{}, createdBy string) (*Operation, error) {
	template, err := m.GetTemplate(templateID)
	if err != nil {
	 return nil, err
	}
	
	// 合并参数
	params := make(map[string]interface{})
	for k, v := range template.Parameters {
	 params[k] = v
	}
	for k, v := range customParams {
	 params[k] = v
	}
	
	return m.CreateOperation(template.Type, template.Name, template.Description, deviceIDs, params, createdBy)
}

// GetDeployer 获取部署器
func (m *CMSManager) GetDeployer() *BatchDeployer {
	return m.deployer
}

// GetRegistry 获取设备注册器
func (m *CMSManager) GetRegistry() *DeviceRegistry {
	return m.registry
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalOperations    int               `json:"total_operations"`
	RunningOperations  int               `json:"running_operations"`
	CompletedOperations int              `json:"completed_operations"`
	FailedOperations   int               `json:"failed_operations"`
	ByType             map[string]int    `json:"by_type"`
	TemplateCount      int               `json:"template_count"`
}

// GetStats 获取统计
func (m *CMSManager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := ManagerStats{
	 ByType: make(map[string]int),
	}
	
	for _, op := range m.operations {
	 stats.TotalOperations++
	 stats.ByType[string(op.Type)]++
	 
	 switch op.Status {
	 case OpStatusRunning:
	  stats.RunningOperations++
	 case OpStatusCompleted, OpStatusPartial:
	  stats.CompletedOperations++
	 case OpStatusFailed:
	  stats.FailedOperations++
	 }
	}
	
 stats.TemplateCount = len(m.templates)
	
	return stats
}

// SelectDevicesByCriteria 按条件选择设备
func (m *CMSManager) SelectDevicesByCriteria(criteria DeviceSelectionCriteria) ([]string, error) {
	filter := DeviceFilter{
	 Type:     criteria.Types,
	 Status:   criteria.Statuses,
	 GroupID:  criteria.GroupID,
	 Location: criteria.Location,
	}
	
	devices := m.registry.ListDevices(filter)
	
	if criteria.MinHealthScore > 0 {
	 filtered := make([]*Device, 0)
	 for _, d := range devices {
	  if d.HealthScore >= criteria.MinHealthScore {
	   filtered = append(filtered, d)
	  }
	 }
	 devices = filtered
	}
	
	ids := make([]string, 0, len(devices))
	for _, d := range devices {
	 ids = append(ids, d.ID)
	}
	
	if len(ids) == 0 {
	 return nil, ErrNoDevicesSelected
	}
	
	return ids, nil
}

// DeviceSelectionCriteria 设备选择条件
type DeviceSelectionCriteria struct {
	Types         []DeviceType   `json:"types,omitempty"`
	Statuses      []DeviceStatus `json:"statuses,omitempty"`
	GroupID       string         `json:"group_id,omitempty"`
	Location      string         `json:"location,omitempty"`
	MinHealthScore int           `json:"min_health_score,omitempty"`
	MaxCPUUsage   float64        `json:"max_cpu_usage,omitempty"`
	MaxDiskUsage  float64        `json:"max_disk_usage,omitempty"`
}

// BroadcastCommand 广播命令到所有在线设备
func (m *CMSManager) BroadcastCommand(opType OperationType, params map[string]interface{}, createdBy string) (*Operation, error) {
	// 选择所有在线设备
	ids, err := m.SelectDevicesByCriteria(DeviceSelectionCriteria{
	 Statuses: []DeviceStatus{DeviceStatusOnline},
	})
	
	if err != nil {
	 return nil, err
	}
	
	return m.CreateOperation(opType, "Broadcast: "+string(opType), "Broadcast operation to all online devices", ids, params, createdBy)
}

// ScheduleOperation 调度操作（延迟执行）
func (m *CMSManager) ScheduleOperation(opType OperationType, name string, deviceIDs []string, params map[string]interface{}, scheduledTime time.Time, createdBy string) (*ScheduledOperation, error) {
	scheduled := &ScheduledOperation{
	 ID:            uuid.New().String(),
	 Type:          opType,
	 Name:          name,
	 DeviceIDs:     deviceIDs,
	 Parameters:    params,
	 ScheduledTime: scheduledTime,
	 CreatedBy:     createdBy,
	 CreatedAt:     time.Now(),
	 Status:        "pending",
	}
	
	// 在实际实现中，这里应该将调度任务存储到持久化存储
	// 并启动定时器在 scheduledTime 触发操作
	
	m.logger.Info("Operation scheduled",
	 zap.String("scheduled_id", scheduled.ID),
	 zap.Time("scheduled_time", scheduledTime),
	)
	
	return scheduled, nil
}

// ScheduledOperation 调度操作
type ScheduledOperation struct {
	ID            string                 `json:"id"`
	Type          OperationType          `json:"type"`
	Name          string                 `json:"name"`
	DeviceIDs     []string               `json:"device_ids"`
	Parameters    map[string]interface{} `json:"parameters"`
	ScheduledTime time.Time              `json:"scheduled_time"`
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	Status        string                 `json:"status"`
	OperationID   string                 `json:"operation_id,omitempty"` // 实际执行的操作 ID
}