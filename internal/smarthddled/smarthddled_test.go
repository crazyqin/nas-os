package smarthddled

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLEDState_String(t *testing.T) {
	tests := []struct {
		state    LEDState
		expected string
	}{
		{LEDStateOff, "Off"},
		{LEDStateOn, "On"},
		{LEDStateBlink, "Blink"},
		{LEDStateError, "Error"},
		{LEDState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestBlinkPolicy_Fields(t *testing.T) {
	policy := BlinkPolicy{
		Name:        "test",
		Reason:      BlinkReasonFault,
		OnDuration:  500 * time.Millisecond,
		OffDuration: 500 * time.Millisecond,
		MaxDuration: 10 * time.Minute,
		AutoStop:    true,
	}

	assert.Equal(t, "test", policy.Name)
	assert.Equal(t, BlinkReasonFault, policy.Reason)
	assert.Equal(t, 500*time.Millisecond, policy.OnDuration)
	assert.Equal(t, 10*time.Minute, policy.MaxDuration)
	assert.True(t, policy.AutoStop)
}

func TestPredefinedPolicies(t *testing.T) {
	// 测试故障策略
	assert.Equal(t, "fault", PolicyFault.Name)
	assert.Equal(t, BlinkReasonFault, PolicyFault.Reason)
	assert.Equal(t, 200*time.Millisecond, PolicyFault.OnDuration)
	assert.False(t, PolicyFault.AutoStop)

	// 测试定位策略
	assert.Equal(t, "locate", PolicyLocate.Name)
	assert.Equal(t, BlinkReasonLocate, PolicyLocate.Reason)
	assert.Equal(t, 15*time.Minute, PolicyLocate.MaxDuration)
	assert.True(t, PolicyLocate.AutoStop)

	// 测试重建策略
	assert.Equal(t, "rebuild", PolicyRebuild.Name)
	assert.Equal(t, BlinkReasonRebuild, PolicyRebuild.Reason)
	assert.Equal(t, 300*time.Millisecond, PolicyRebuild.OnDuration)
}

func TestDiskIdentifier_Fields(t *testing.T) {
	disk := DiskIdentifier{
		DevicePath:    "/dev/sda",
		SCSIAddress:   "0:0:0:0",
		SerialNumber:  "WD123456",
		WWN:           "0x50014ee20b123456",
		SlotNumber:    1,
		EnclosureID:   "enc0",
		HBAController: "hba0",
	}

	assert.Equal(t, "/dev/sda", disk.DevicePath)
	assert.Equal(t, "0:0:0:0", disk.SCSIAddress)
	assert.Equal(t, "WD123456", disk.SerialNumber)
	assert.Equal(t, 1, disk.SlotNumber)
	assert.Equal(t, "enc0", disk.EnclosureID)
}

func TestDiskLedInfo_Fields(t *testing.T) {
	now := time.Now()
	policy := &PolicyFault

	info := DiskLedInfo{
		Disk: DiskIdentifier{
			DevicePath: "/dev/sdb",
		},
		State:          LEDStateBlink,
		ControlMethod:  ControlMethodSCSIGeneric,
		BlinkPolicy:    policy,
		StartTime:      now,
		LastUpdateTime: now,
		Reason:         "disk failure detected",
	}

	assert.Equal(t, "/dev/sdb", info.Disk.DevicePath)
	assert.Equal(t, LEDStateBlink, info.State)
	assert.Equal(t, ControlMethodSCSIGeneric, info.ControlMethod)
	assert.NotNil(t, info.BlinkPolicy)
	assert.Equal(t, "disk failure detected", info.Reason)
}

func TestLedStateStore_SetAndGet(t *testing.T) {
	store := newLedStateStore()
	disk := DiskIdentifier{
		DevicePath:   "/dev/sdc",
		SerialNumber: "TEST123",
	}

	// 测试Set
	store.Set(disk, LEDStateOn, ControlMethodSCSIGeneric, nil, "test locate")

	// 测试Get
	info, exists := store.Get(disk)
	require.True(t, exists)
	assert.Equal(t, LEDStateOn, info.State)
	assert.Equal(t, ControlMethodSCSIGeneric, info.ControlMethod)
	assert.Equal(t, "test locate", info.Reason)
}

func TestLedStateStore_NotFound(t *testing.T) {
	store := newLedStateStore()
	disk := DiskIdentifier{
		DevicePath: "/dev/nonexistent",
	}

	info, exists := store.Get(disk)
	assert.False(t, exists)
	assert.Nil(t, info)
}

func TestLedStateStore_Delete(t *testing.T) {
	store := newLedStateStore()
	disk := DiskIdentifier{
		DevicePath: "/dev/sdd",
	}

	store.Set(disk, LEDStateOn, ControlMethodSCSIGeneric, nil, "test")

	// 确认存在
	_, exists := store.Get(disk)
	assert.True(t, exists)

	// 删除
	store.Delete(disk)

	// 确认不存在
	_, exists = store.Get(disk)
	assert.False(t, exists)
}

func TestLedStateStore_ListAll(t *testing.T) {
	store := newLedStateStore()

	disks := []DiskIdentifier{
		{DevicePath: "/dev/sda"},
		{DevicePath: "/dev/sdb"},
		{DevicePath: "/dev/sdc"},
	}

	for i, disk := range disks {
		state := LEDStateOff
		if i == 1 {
			state = LEDStateOn
		}
		store.Set(disk, state, ControlMethodSCSIGeneric, nil, "test")
	}

	all := store.ListAll()
	assert.Len(t, all, 3)
}

func TestLedStateStore_Update(t *testing.T) {
	store := newLedStateStore()
	disk := DiskIdentifier{
		DevicePath: "/dev/sde",
	}

	// 初始设置
	store.Set(disk, LEDStateOff, ControlMethodSCSIGeneric, nil, "initial")

	// 更新
	store.Set(disk, LEDStateOn, ControlMethodSCSIGeneric, nil, "updated")

	info, exists := store.Get(disk)
	require.True(t, exists)
	assert.Equal(t, LEDStateOn, info.State)
	assert.Equal(t, "updated", info.Reason)
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := newEventBus(100)

	received := make(chan LEDEvent, 1)

	// 订阅
	id := bus.Subscribe(func(event LEDEvent) {
		received <- event
	})

	assert.NotEmpty(t, string(id))

	// 发布事件
	event := LEDEvent{
		Timestamp: time.Now(),
		DiskID:    DiskIdentifier{DevicePath: "/dev/sda"},
		EventType: EventStateChanged,
		OldState:  LEDStateOff,
		NewState:  LEDStateOn,
		Reason:    "test",
	}

	bus.Publish(event)

	// 等待接收
	select {
	case e := <-received:
		assert.Equal(t, EventStateChanged, e.EventType)
		assert.Equal(t, LEDStateOff, e.OldState)
		assert.Equal(t, LEDStateOn, e.NewState)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := newEventBus(100)

	received := make(chan LEDEvent, 1)

	// 订阅
	id := bus.Subscribe(func(event LEDEvent) {
		received <- event
	})

	// 取消订阅
	bus.Unsubscribe(id)

	// 发布事件
	event := LEDEvent{
		Timestamp: time.Now(),
		EventType: EventStateChanged,
	}

	bus.Publish(event)

	// 应该没有收到
	select {
	case <-received:
		t.Fatal("should not receive event after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// 正常
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := newEventBus(100)

	received1 := make(chan LEDEvent, 1)
	received2 := make(chan LEDEvent, 1)

	bus.Subscribe(func(event LEDEvent) {
		received1 <- event
	})

	bus.Subscribe(func(event LEDEvent) {
		received2 <- event
	})

	event := LEDEvent{
		Timestamp: time.Now(),
		EventType: EventBlinkStart,
	}

	bus.Publish(event)

	// 两个订阅者都应该收到
	select {
	case e := <-received1:
		assert.Equal(t, EventBlinkStart, e.EventType)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event on subscriber 1")
	}

	select {
	case e := <-received2:
		assert.Equal(t, EventBlinkStart, e.EventType)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event on subscriber 2")
	}
}

func TestGetDiskKey(t *testing.T) {
	tests := []struct {
		name     string
		disk     DiskIdentifier
		expected string
	}{
		{
			name:     "by device path",
			disk:     DiskIdentifier{DevicePath: "/dev/sda"},
			expected: "/dev/sda",
		},
		{
			name:     "by WWN",
			disk:     DiskIdentifier{WWN: "0x50014ee20b123456"},
			expected: "wwn:0x50014ee20b123456",
		},
		{
			name:     "by serial number",
			disk:     DiskIdentifier{SerialNumber: "WD123456"},
			expected: "serial:WD123456",
		},
		{
			name: "by slot and enclosure",
			disk: DiskIdentifier{
				EnclosureID: "enc0",
				SlotNumber:  3,
			},
			expected: "slot:enc0:3",
		},
		{
			name: "device path has priority",
			disk: DiskIdentifier{
				DevicePath:   "/dev/sdb",
				SerialNumber: "TEST",
				WWN:          "0x123",
			},
			expected: "/dev/sdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getDiskKey(tt.disk))
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "device_path",
		Message: "cannot be empty",
	}

	assert.Contains(t, err.Error(), "device_path")
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestControllerError(t *testing.T) {
	cause := assert.AnError
	err := &ControllerError{
		Code:    "TEST_ERROR",
		Message: "test message",
		Cause:   cause,
	}

	assert.Contains(t, err.Error(), "TEST_ERROR")
	assert.Contains(t, err.Error(), "test message")
	assert.Equal(t, cause, err.Unwrap())

	// 测试无cause的情况
	errNoCause := &ControllerError{
		Code:    "NO_CAUSE",
		Message: "no cause",
	}

	assert.Contains(t, errNoCause.Error(), "NO_CAUSE")
	assert.Nil(t, errNoCause.Unwrap())
}

func TestPredefinedErrors(t *testing.T) {
	assert.Equal(t, "DISK_NOT_FOUND", ErrDiskNotFound.Code)
	assert.Equal(t, "LED_CONTROL_FAILED", ErrLEDControlFailed.Code)
	assert.Equal(t, "NOT_SUPPORTED", ErrNotSupported.Code)
	assert.Equal(t, "TIMEOUT", ErrTimeout.Code)
	assert.Equal(t, "ALREADY_BLINKING", ErrAlreadyBlinking.Code)
}

func TestDefaultControllerConfig(t *testing.T) {
	config := DefaultControllerConfig()

	assert.Equal(t, ControlMethodSCSIGeneric, config.DefaultMethod)
	assert.Equal(t, 15*time.Minute, config.LocateTimeout)
	assert.Equal(t, 1000, config.EventBufferSize)
	assert.Equal(t, PolicyLocate, config.DefaultBlinkPolicy)

	// 检查方法配置
	methods := config.Methods
	assert.True(t, methods[ControlMethodSCSIGeneric].Enabled)
	assert.Equal(t, 1, methods[ControlMethodSCSIGeneric].Priority)
	assert.False(t, methods[ControlMethodIPMI].Enabled)
	assert.Equal(t, 2, methods[ControlMethodIPMI].Priority)
}

func TestStoragePool_Fields(t *testing.T) {
	pool := StoragePool{
		ID:     "pool1",
		Name:   "MainPool",
		Status: "online",
		Disks: []DiskIdentifier{
			{DevicePath: "/dev/sda"},
			{DevicePath: "/dev/sdb"},
		},
		RAIDLevel: "raid1",
	}

	assert.Equal(t, "pool1", pool.ID)
	assert.Equal(t, "MainPool", pool.Name)
	assert.Len(t, pool.Disks, 2)
	assert.Equal(t, "raid1", pool.RAIDLevel)
}

func TestRAIDGroup_Fields(t *testing.T) {
	group := RAIDGroup{
		ID:     "rg1",
		Name:   "DataArray",
		Status: "optimal",
		Disks: []DiskIdentifier{
			{DevicePath: "/dev/sda"},
			{DevicePath: "/dev/sdb"},
			{DevicePath: "/dev/sdc"},
		},
		RAIDLevel: "raid5",
	}

	assert.Equal(t, "rg1", group.ID)
	assert.Equal(t, "DataArray", group.Name)
	assert.Equal(t, "optimal", group.Status)
	assert.Len(t, group.Disks, 3)
	assert.Equal(t, "raid5", group.RAIDLevel)
}

func TestBulkResult_Fields(t *testing.T) {
	result := &BulkResult{
		Total:   5,
		Success: 3,
		Failed:  2,
		Errors: []DiskError{
			{
				Disk:  DiskIdentifier{DevicePath: "/dev/sda"},
				Error: "timeout",
			},
			{
				Disk:  DiskIdentifier{DevicePath: "/dev/sdb"},
				Error: "device not found",
			},
		},
	}

	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 3, result.Success)
	assert.Equal(t, 2, result.Failed)
	assert.Len(t, result.Errors, 2)
}

func TestLedStateStore_ConcurrentAccess(t *testing.T) {
	store := newLedStateStore()
	disk := DiskIdentifier{DevicePath: "/dev/sdf"}

	// 并发写入
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			store.Set(disk, LEDStateOn, ControlMethodSCSIGeneric, nil, "concurrent")
			done <- true
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据一致性
	info, exists := store.Get(disk)
	assert.True(t, exists)
	assert.NotNil(t, info)
}

func TestLEDEvent_Fields(t *testing.T) {
	now := time.Now()
	event := LEDEvent{
		Timestamp: now,
		DiskID: DiskIdentifier{
			DevicePath: "/dev/sdg",
		},
		EventType: EventStateChanged,
		OldState:  LEDStateOff,
		NewState:  LEDStateBlink,
		Reason:    "disk failure",
	}

	assert.Equal(t, now, event.Timestamp)
	assert.Equal(t, "/dev/sdg", event.DiskID.DevicePath)
	assert.Equal(t, EventStateChanged, event.EventType)
	assert.Equal(t, LEDStateOff, event.OldState)
	assert.Equal(t, LEDStateBlink, event.NewState)
	assert.Equal(t, "disk failure", event.Reason)
}

func TestBlinkReason_Constants(t *testing.T) {
	assert.Equal(t, BlinkReason("fault"), BlinkReasonFault)
	assert.Equal(t, BlinkReason("locate"), BlinkReasonLocate)
	assert.Equal(t, BlinkReason("rebuild"), BlinkReasonRebuild)
	assert.Equal(t, BlinkReason("predictive"), BlinkReasonPredictive)
	assert.Equal(t, BlinkReason("hot_spare"), BlinkReasonHotSpare)
}

func TestLEDEventType_Constants(t *testing.T) {
	assert.Equal(t, LEDEventType("state_changed"), EventStateChanged)
	assert.Equal(t, LEDEventType("blink_start"), EventBlinkStart)
	assert.Equal(t, LEDEventType("blink_stop"), EventBlinkStop)
	assert.Equal(t, LEDEventType("control_error"), EventControlError)
}

func TestLEDControlMethod_Constants(t *testing.T) {
	assert.Equal(t, LEDControlMethod("scsi_generic"), ControlMethodSCSIGeneric)
	assert.Equal(t, LEDControlMethod("ipmi"), ControlMethodIPMI)
	assert.Equal(t, LEDControlMethod("megaraid"), ControlMethodMegaRAID)
	assert.Equal(t, LEDControlMethod("adaptec"), ControlMethodAdaptec)
	assert.Equal(t, LEDControlMethod("hba"), ControlMethodHBA)
}

func TestMethodConfig_Fields(t *testing.T) {
	config := MethodConfig{
		Enabled:    true,
		Priority:   1,
		DevicePath: "/dev/sg0",
		IPMIConfig: &IPMIConfig{
			Host:      "192.168.1.100",
			Username:  "admin",
			Password:  "password",
			Interface: "lanplus",
		},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, 1, config.Priority)
	assert.Equal(t, "/dev/sg0", config.DevicePath)
	assert.NotNil(t, config.IPMIConfig)
	assert.Equal(t, "192.168.1.100", config.IPMIConfig.Host)
}

func TestMegaRAIDConfig_Fields(t *testing.T) {
	config := MegaRAIDConfig{
		CLIPath:      "/opt/MegaRAID/storcli/storcli64",
		ControllerID: 0,
	}

	assert.Equal(t, "/opt/MegaRAID/storcli/storcli64", config.CLIPath)
	assert.Equal(t, 0, config.ControllerID)
}
