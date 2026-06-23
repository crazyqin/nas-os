// Package ups 扩展管理器测试
package ups

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExtendedManager(t *testing.T) {
	config := DefaultUPSConfig()
	manager := NewExtendedManager(config)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.devices)
	assert.NotNil(t, manager.events)
	assert.NotNil(t, manager.tasks)
}

func TestAddDevice(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	device := &Device{
		Name:         "UPS-001",
		Model:        "Smart-UPS 1500",
		Manufacturer: "APC",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)
	assert.NotEmpty(t, device.ID)
	assert.Equal(t, DeviceStatusOnline, device.Status)
}

func TestAddDeviceDuplicate(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	device := &Device{
		ID:   "dev1",
		Name: "UPS-001",
	}

	err := manager.AddDevice(device)
	require.NoError(t, err)

	err = manager.AddDevice(device)
	assert.ErrorIs(t, err, ErrDeviceExists)
}

func TestRemoveDevice(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	device := &Device{ID: "dev1", Name: "UPS-001"}
	_ = manager.AddDevice(device)

	err := manager.RemoveDevice("dev1")
	require.NoError(t, err)

	_, err = manager.GetDevice("dev1")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestRemoveDeviceNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	err := manager.RemoveDevice("nonexistent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestGetDevice(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	device := &Device{ID: "dev1", Name: "UPS-001"}
	_ = manager.AddDevice(device)

	result, err := manager.GetDevice("dev1")
	require.NoError(t, err)
	assert.Equal(t, "UPS-001", result.Name)
}

func TestGetDeviceNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_, err := manager.GetDevice("nonexistent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestListDevices(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})
	_ = manager.AddDevice(&Device{ID: "dev2", Name: "UPS-2"})

	devices := manager.ListDevices()
	assert.Len(t, devices, 2)
}

func TestCreateShutdownTask(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})

	task, err := manager.CreateShutdownTask("dev1", "手动关机", 120)
	require.NoError(t, err)
	assert.Equal(t, TaskStatusPending, task.Status)
	assert.Equal(t, 120, task.Delay)
}

func TestCreateShutdownTaskDeviceNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_, err := manager.CreateShutdownTask("nonexistent", "测试", 60)
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestGetTask(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})
	task, _ := manager.CreateShutdownTask("dev1", "测试", 60)

	result, err := manager.GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "测试", result.Reason)
}

func TestGetTaskNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_, err := manager.GetTask("nonexistent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestListTasks(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})
	_ = manager.CreateShutdownTask("dev1", "任务1", 60)
	_ = manager.CreateShutdownTask("dev1", "任务2", 120)

	tasks := manager.ListTasks()
	assert.Len(t, tasks, 2)
}

func TestCancelTask(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})
	task, _ := manager.CreateShutdownTask("dev1", "测试", 60)

	err := manager.CancelTask(task.ID)
	require.NoError(t, err)

	result, _ := manager.GetTask(task.ID)
	assert.Equal(t, TaskStatusCancelled, result.Status)
}

func TestCancelTaskNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	err := manager.CancelTask("nonexistent")
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestCancelTaskNotCancellable(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})
	task, _ := manager.CreateShutdownTask("dev1", "测试", 60)

	// 模拟任务正在执行
	manager.mu.Lock()
	task.Status = TaskStatusExecuting
	manager.mu.Unlock()

	err := manager.CancelTask(task.ID)
	assert.ErrorIs(t, err, ErrTaskNotCancellable)
}

func TestGetEvents(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	// 添加设备触发事件
	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})

	events := manager.GetEvents(10)
	assert.NotEmpty(t, events)
	assert.Equal(t, EventDeviceConnected, events[0].Type)
}

func TestAcknowledgeEvent(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})

	events := manager.GetEvents(1)
	require.NotEmpty(t, events)

	err := manager.AcknowledgeEvent(events[0].ID)
	require.NoError(t, err)

	events = manager.GetEvents(1)
	assert.True(t, events[0].Acknowledged)
}

func TestRunSelfTest(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})

	result, err := manager.RunSelfTest("dev1")
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestRunSelfTestDeviceNotFound(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	_, err := manager.RunSelfTest("nonexistent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestRegisterEventCallback(t *testing.T) {
	manager := NewExtendedManager(DefaultUPSConfig())

	var receivedEvent Event
	callbackCalled := false

	manager.RegisterEventCallback(func(e Event) {
		receivedEvent = e
		callbackCalled = true
	})

	_ = manager.AddDevice(&Device{ID: "dev1", Name: "UPS-1"})

	// 等待回调执行
	time.Sleep(100 * time.Millisecond)

	assert.True(t, callbackCalled)
	assert.Equal(t, EventDeviceConnected, receivedEvent.Type)
}
