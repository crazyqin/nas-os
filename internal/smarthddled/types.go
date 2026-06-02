// Package smarthddled 硬盘LED指示灯管理系统
//
// 提供硬盘LED指示灯的统一控制接口，支持多种硬件平台：
// - SCSI Generic (sg_raw) 方式控制
// - IPMI Chassis Identify
// - Vendor-specific (LSI/Broadcom MegaRAID, Adaptec, etc.)
//
// 主要功能：
// - 定位硬盘（点亮LED）
// - 熄灭LED
// - 批量控制
// - 基于存储池/RAID组关联硬盘LED
// - 故障硬盘自动闪烁告警
package smarthddled

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LEDState LED状态枚举
type LEDState int

const (
	// LEDStateOff LED关闭
	LEDStateOff LEDState = iota
	// LEDStateOn LED常亮（定位模式）
	LEDStateOn
	// LEDStateBlink LED闪烁（故障/告警）
	LEDStateBlink
	// LEDStateError LED错误状态（无法控制）
	LEDStateError
)

// String 返回LED状态的字符串表示
func (s LEDState) String() string {
	switch s {
	case LEDStateOff:
		return "Off"
	case LEDStateOn:
		return "On"
	case LEDStateBlink:
		return "Blink"
	case LEDStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// LEDControlMethod LED控制方式
type LEDControlMethod string

const (
	// ControlMethodSCSIGeneric SCSI Generic 方式 (sg_raw)
	ControlMethodSCSIGeneric LEDControlMethod = "scsi_generic"
	// ControlMethodIPMI IPMI Chassis Identify
	ControlMethodIPMI LEDControlMethod = "ipmi"
	// ControlMethodMegaRAID LSI/Broadcom MegaRAID CLI
	ControlMethodMegaRAID LEDControlMethod = "megaraid"
	// ControlMethodAdaptec Adaptec ARCCONF
	ControlMethodAdaptec LEDControlMethod = "adaptec"
	// ControlMethodHBA 通用HBA控制
	ControlMethodHBA LEDControlMethod = "hba"
)

// BlinkPolicy 闪烁策略
type BlinkPolicy struct {
	// Name 策略名称
	Name string `json:"name"`
	// Reason 触发原因
	Reason BlinkReason `json:"reason"`
	// OnDuration 亮灯持续时间
	OnDuration time.Duration `json:"on_duration"`
	// OffDuration 灭灯持续时间
	OffDuration time.Duration `json:"off_duration"`
	// MaxDuration 最大闪烁持续时间（0表示无限）
	MaxDuration time.Duration `json:"max_duration"`
	// AutoStop 自动停止（到达MaxDuration后停止闪烁）
	AutoStop bool `json:"auto_stop"`
}

// BlinkReason 闪烁原因
type BlinkReason string

const (
	// BlinkReasonFault 硬盘故障
	BlinkReasonFault BlinkReason = "fault"
	// BlinkReasonLocate 定位硬盘
	BlinkReasonLocate BlinkReason = "locate"
	// BlinkReasonRebuild RAID重建中
	BlinkReasonRebuild BlinkReason = "rebuild"
	// BlinkReasonPredictive 预测性故障
	BlinkReasonPredictive BlinkReason = "predictive"
	// BlinkReasonHotSpare 热备盘激活
	BlinkReasonHotSpare BlinkReason = "hot_spare"
)

// 预定义闪烁策略
var (
	// PolicyFault 故障闪烁策略：快闪
	PolicyFault = BlinkPolicy{
		Name:        "fault",
		Reason:      BlinkReasonFault,
		OnDuration:  200 * time.Millisecond,
		OffDuration: 200 * time.Millisecond,
		MaxDuration: 0, // 无限
		AutoStop:    false,
	}

	// PolicyLocate 定位闪烁策略：慢闪
	PolicyLocate = BlinkPolicy{
		Name:        "locate",
		Reason:      BlinkReasonLocate,
		OnDuration:  500 * time.Millisecond,
		OffDuration: 500 * time.Millisecond,
		MaxDuration: 15 * time.Minute,
		AutoStop:    true,
	}

	// PolicyRebuild 重建闪烁策略：中速闪
	PolicyRebuild = BlinkPolicy{
		Name:        "rebuild",
		Reason:      BlinkReasonRebuild,
		OnDuration:  300 * time.Millisecond,
		OffDuration: 300 * time.Millisecond,
		MaxDuration: 0,
		AutoStop:    false,
	}

	// PolicyPredictive 预测性故障策略：慢速间隔闪
	PolicyPredictive = BlinkPolicy{
		Name:        "predictive",
		Reason:      BlinkReasonPredictive,
		OnDuration:  1 * time.Second,
		OffDuration: 2 * time.Second,
		MaxDuration: 0,
		AutoStop:    false,
	}
)

// DiskIdentifier 磁盘标识
type DiskIdentifier struct {
	// DevicePath 设备路径，例如 /dev/sda
	DevicePath string `json:"device_path"`
	// SCSIAddress SCSI地址 (host:channel:id:lun)
	SCSIAddress string `json:"scsi_address"`
	// SerialNumber 序列号
	SerialNumber string `json:"serial_number"`
	// WWN World Wide Name
	WWN string `json:"wwn"`
	// SlotNumber 盘位号
	SlotNumber int `json:"slot_number"`
	// EnclosureID 磁盘柜ID
	EnclosureID string `json:"enclosure_id"`
	// HBAController HBA控制器标识
	HBAController string `json:"hba_controller"`
}

// DiskLedInfo 磁盘LED信息
type DiskLedInfo struct {
	// Disk 磁盘标识
	Disk DiskIdentifier `json:"disk"`
	// State LED状态
	State LEDState `json:"state"`
	// ControlMethod 使用的控制方式
	ControlMethod LEDControlMethod `json:"control_method"`
	// BlinkPolicy 当前闪烁策略（如果正在闪烁）
	BlinkPolicy *BlinkPolicy `json:"blink_policy,omitempty"`
	// StartTime LED状态开始时间
	StartTime time.Time `json:"start_time"`
	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time `json:"last_update_time"`
	// Reason 操作原因
	Reason string `json:"reason,omitempty"`
	// Error 错误信息（如果状态为Error）
	Error string `json:"error,omitempty"`
}

// StoragePool 存储池信息
type StoragePool struct {
	// ID 池ID
	ID string `json:"id"`
	// Name 池名称
	Name string `json:"name"`
	// Status 池状态
	Status string `json:"status"`
	// Disks 池中的磁盘列表
	Disks []DiskIdentifier `json:"disks"`
	// RAIDLevel RAID级别
	RAIDLevel string `json:"raid_level,omitempty"`
}

// RAIDGroup RAID组信息
type RAIDGroup struct {
	// ID RAID组ID
	ID string `json:"id"`
	// Name RAID组名称
	Name string `json:"name"`
	// Status RAID组状态 (optimal, degraded, rebuilding, failed)
	Status string `json:"status"`
	// Disks RAID组中的磁盘列表
	Disks []DiskIdentifier `json:"disks"`
	// RAIDLevel RAID级别
	RAIDLevel string `json:"raid_level"`
}

// LEDEvent LED事件
type LEDEvent struct {
	// Timestamp 事件时间
	Timestamp time.Time `json:"timestamp"`
	// DiskID 磁盘标识
	DiskID DiskIdentifier `json:"disk_id"`
	// EventType 事件类型
	EventType LEDEventType `json:"event_type"`
	// OldState 之前的状态
	OldState LEDState `json:"old_state"`
	// NewState 新状态
	NewState LEDState `json:"new_state"`
	// Reason 原因
	Reason string `json:"reason,omitempty"`
}

// LEDEventType LED事件类型
type LEDEventType string

const (
	// EventStateChanged 状态变化
	EventStateChanged LEDEventType = "state_changed"
	// EventBlinkStart 闪烁开始
	EventBlinkStart LEDEventType = "blink_start"
	// EventBlinkStop 闪烁停止
	EventBlinkStop LEDEventType = "blink_stop"
	// EventControlError 控制错误
	EventControlError LEDEventType = "control_error"
)

// HDDLedController 硬盘LED控制器接口
type HDDLedController interface {
	// Init 初始化控制器
	Init(ctx context.Context) error

	// Close 关闭控制器
	Close() error

	// SetLED 设置指定磁盘的LED状态
	SetLED(ctx context.Context, disk DiskIdentifier, state LEDState) error

	// GetLED 获取指定磁盘的LED状态
	GetLED(ctx context.Context, disk DiskIdentifier) (*DiskLedInfo, error)

	// LocateDisk 定位磁盘（点亮LED）
	LocateDisk(ctx context.Context, disk DiskIdentifier, duration time.Duration) error

	// StopLocate 停止定位（熄灭LED）
	StopLocate(ctx context.Context, disk DiskIdentifier) error

	// StartBlink 开始闪烁
	StartBlink(ctx context.Context, disk DiskIdentifier, policy BlinkPolicy) error

	// StopBlink 停止闪烁
	StopBlink(ctx context.Context, disk DiskIdentifier) error

	// SetBulkLED 批量设置LED状态
	SetBulkLED(ctx context.Context, disks []DiskIdentifier, state LEDState) (*BulkResult, error)

	// LocateBulkDisks 批量定位磁盘
	LocateBulkDisks(ctx context.Context, disks []DiskIdentifier, duration time.Duration) (*BulkResult, error)

	// StopAllBlink 停止所有闪烁
	StopAllBlink(ctx context.Context) error

	// GetPoolDisksLED 获取存储池所有磁盘的LED状态
	GetPoolDisksLED(ctx context.Context, pool StoragePool) ([]DiskLedInfo, error)

	// LocatePoolDisks 定位存储池所有磁盘
	LocatePoolDisks(ctx context.Context, pool StoragePool, duration time.Duration) (*BulkResult, error)

	// GetRAIDGroupDisksLED 获取RAID组所有磁盘的LED状态
	GetRAIDGroupDisksLED(ctx context.Context, group RAIDGroup) ([]DiskLedInfo, error)

	// LocateRAIDGroupDisks 定位RAID组所有磁盘
	LocateRAIDGroupDisks(ctx context.Context, group RAIDGroup, duration time.Duration) (*BulkResult, error)

	// ListAllLEDs 列出所有磁盘的LED状态
	ListAllLEDs(ctx context.Context) ([]DiskLedInfo, error)

	// GetSupportedMethods 获取支持的控制方式
	GetSupportedMethods() []LEDControlMethod

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Subscribe 订阅LED事件
	Subscribe(ctx context.Context, handler EventHandler) (SubscriptionID, error)

	// Unsubscribe 取消订阅
	Unsubscribe(id SubscriptionID)
}

// BulkResult 批量操作结果
type BulkResult struct {
	// Total 总数
	Total int `json:"total"`
	// Success 成功数
	Success int `json:"success"`
	// Failed 失败数
	Failed int `json:"failed"`
	// Errors 错误详情
	Errors []DiskError `json:"errors,omitempty"`
}

// DiskError 磁盘操作错误
type DiskError struct {
	// Disk 磁盘标识
	Disk DiskIdentifier `json:"disk"`
	// Error 错误信息
	Error string `json:"error"`
}

// EventHandler 事件处理函数
type EventHandler func(event LEDEvent)

// SubscriptionID 订阅ID
type SubscriptionID string

// ControllerConfig 控制器配置
type ControllerConfig struct {
	// DefaultMethod 默认控制方式
	DefaultMethod LEDControlMethod `json:"default_method"`
	// Methods 可用的控制方式配置
	Methods map[LEDControlMethod]MethodConfig `json:"methods"`
	// DefaultBlinkPolicy 默认闪烁策略
	DefaultBlinkPolicy BlinkPolicy `json:"default_blink_policy"`
	// LocateTimeout 定位超时时间
	LocateTimeout time.Duration `json:"locate_timeout"`
	// EventBufferSize 事件缓冲区大小
	EventBufferSize int `json:"event_buffer_size"`
}

// MethodConfig 控制方式配置
type MethodConfig struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// Priority 优先级（数字越小优先级越高）
	Priority int `json:"priority"`
	// DevicePath 设备路径（如 /dev/sg0 for SCSI）
	DevicePath string `json:"device_path,omitempty"`
	// IPMIConfig IPMI配置
	IPMIConfig *IPMIConfig `json:"ipmi_config,omitempty"`
	// MegaRAIDConfig MegaRAID配置
	MegaRAIDConfig *MegaRAIDConfig `json:"megaraid_config,omitempty"`
}

// IPMIConfig IPMI配置
type IPMIConfig struct {
	// Host IPMI主机地址
	Host string `json:"host"`
	// Username 用户名
	Username string `json:"username"`
	// Password 密码
	Password string `json:"password"`
	// Interface 接口类型 (lan, lanplus, etc.)
	Interface string `json:"interface"`
}

// MegaRAIDConfig MegaRAID配置
type MegaRAIDConfig struct {
	// CLIPath MegaCLI/StorCLI路径
	CLIPath string `json:"cli_path"`
	// ControllerID 控制器ID
	ControllerID int `json:"controller_id"`
}

// DefaultControllerConfig 返回默认控制器配置
func DefaultControllerConfig() ControllerConfig {
	return ControllerConfig{
		DefaultMethod:      ControlMethodSCSIGeneric,
		DefaultBlinkPolicy: PolicyLocate,
		LocateTimeout:      15 * time.Minute,
		EventBufferSize:    1000,
		Methods: map[LEDControlMethod]MethodConfig{
			ControlMethodSCSIGeneric: {
				Enabled:  true,
				Priority: 1,
			},
			ControlMethodIPMI: {
				Enabled:  false,
				Priority: 2,
			},
			ControlMethodMegaRAID: {
				Enabled:  false,
				Priority: 3,
			},
		},
	}
}

// ValidationError 参数验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// ControllerError 控制器错误
type ControllerError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ControllerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ControllerError) Unwrap() error {
	return e.Cause
}

// 常见错误码
var (
	ErrDiskNotFound     = &ControllerError{Code: "DISK_NOT_FOUND", Message: "disk not found"}
	ErrLEDControlFailed = &ControllerError{Code: "LED_CONTROL_FAILED", Message: "LED control failed"}
	ErrNotSupported     = &ControllerError{Code: "NOT_SUPPORTED", Message: "operation not supported"}
	ErrTimeout          = &ControllerError{Code: "TIMEOUT", Message: "operation timeout"}
	ErrAlreadyBlinking  = &ControllerError{Code: "ALREADY_BLINKING", Message: "LED is already blinking"}
)

// ledStateStore LED状态存储（内存实现）
type ledStateStore struct {
	mu    sync.RWMutex
	store map[string]*DiskLedInfo // key: device_path or slot identifier
}

// newLedStateStore 创建新的LED状态存储
func newLedStateStore() *ledStateStore {
	return &ledStateStore{
		store: make(map[string]*DiskLedInfo),
	}
}

// getDiskKey 获取磁盘的唯一键
func getDiskKey(disk DiskIdentifier) string {
	if disk.DevicePath != "" {
		return disk.DevicePath
	}
	if disk.WWN != "" {
		return "wwn:" + disk.WWN
	}
	if disk.SerialNumber != "" {
		return "serial:" + disk.SerialNumber
	}
	return fmt.Sprintf("slot:%s:%d", disk.EnclosureID, disk.SlotNumber)
}

// Set 设置LED状态
func (s *ledStateStore) Set(disk DiskIdentifier, state LEDState, method LEDControlMethod, policy *BlinkPolicy, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := getDiskKey(disk)
	now := time.Now()

	info := &DiskLedInfo{
		Disk:           disk,
		State:          state,
		ControlMethod:  method,
		BlinkPolicy:    policy,
		StartTime:      now,
		LastUpdateTime: now,
		Reason:         reason,
	}

	s.store[key] = info
}

// Get 获取LED状态
func (s *ledStateStore) Get(disk DiskIdentifier) (*DiskLedInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := getDiskKey(disk)
	info, exists := s.store[key]
	return info, exists
}

// Delete 删除LED状态
func (s *ledStateStore) Delete(disk DiskIdentifier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := getDiskKey(disk)
	delete(s.store, key)
}

// ListAll 列出所有LED状态
func (s *ledStateStore) ListAll() []*DiskLedInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*DiskLedInfo, 0, len(s.store))
	for _, info := range s.store {
		result = append(result, info)
	}
	return result
}

// eventBus 事件总线
type eventBus struct {
	mu          sync.RWMutex
	subscribers map[SubscriptionID]EventHandler
	counter     int64
	bufferSize  int
}

// newEventBus 创建新的事件总线
func newEventBus(bufferSize int) *eventBus {
	return &eventBus{
		subscribers: make(map[SubscriptionID]EventHandler),
		bufferSize:  bufferSize,
	}
}

// Subscribe 订阅事件
func (b *eventBus) Subscribe(handler EventHandler) SubscriptionID {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.counter++
	id := SubscriptionID(fmt.Sprintf("sub_%d", b.counter))
	b.subscribers[id] = handler
	return id
}

// Unsubscribe 取消订阅
func (b *eventBus) Unsubscribe(id SubscriptionID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subscribers, id)
}

// Publish 发布事件
func (b *eventBus) Publish(event LEDEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, handler := range b.subscribers {
		go handler(event)
	}
}
