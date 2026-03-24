// Package storage 提供 Hot Spare (热备盘) 管理功能测试
package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 测试辅助函数 ==========

// mockManager 创建模拟的存储管理器
func mockManager() *Manager {
	return &Manager{
		volumes:   make(map[string]*Volume),
		mountBase: "/tmp/test-mnt",
	}
}

// mockHotSpareManager 创建模拟的热备盘管理器
func mockHotSpareManager() *HotSpareManager {
	m := mockManager()
	return NewHotSpareManager(m)
}

// ========== 配置测试 ==========

func TestHotSpareConfig_Default(t *testing.T) {
	config := DefaultHotSpareConfig

	assert.True(t, config.Enabled)
	assert.Equal(t, 30*time.Second, config.CheckInterval)
	assert.True(t, config.AutoRebuild)
	assert.True(t, config.NotifyOnStart)
	assert.True(t, config.NotifyOnComplete)
	assert.True(t, config.NotifyOnFailure)
	assert.Equal(t, 24*time.Hour, config.RebuildTimeout)
	assert.Equal(t, 5, config.MinCapacityMargin)
}

func TestHotSpareManager_SetGetConfig(t *testing.T) {
	h := mockHotSpareManager()

	// 测试默认配置
	config := h.GetConfig()
	assert.Equal(t, DefaultHotSpareConfig, config)

	// 测试设置新配置
	newConfig := HotSpareConfig{
		Enabled:          false,
		CheckInterval:    60 * time.Second,
		AutoRebuild:      false,
		RebuildTimeout:   12 * time.Hour,
		MinCapacityMargin: 10,
	}
	h.SetConfig(newConfig)

	gotConfig := h.GetConfig()
	assert.Equal(t, newConfig, gotConfig)
}

// ========== 热备盘管理测试 ==========

func TestHotSpareManager_AddHotSpare(t *testing.T) {
	// 跳过需要实际设备的测试
	t.Skip("需要实际设备")
}

func TestHotSpareManager_AddHotSpare_Duplicate(t *testing.T) {
	h := mockHotSpareManager()

	// 测试重复添加（模拟）
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:     "hs-1",
		Device: "/dev/sdb",
		Status: "available",
	}

	// 再次添加相同设备应该失败
	_, err := h.AddHotSpare("/dev/sdb", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

func TestHotSpareManager_RemoveHotSpare(t *testing.T) {
	h := mockHotSpareManager()

	// 添加测试热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:     "hs-1",
		Device: "/dev/sdb",
		Status: "available",
	}

	// 移除热备盘
	err := h.RemoveHotSpare("/dev/sdb")
	require.NoError(t, err)

	// 验证已移除
	_, exists := h.hotSpares["/dev/sdb"]
	assert.False(t, exists)
}

func TestHotSpareManager_RemoveHotSpare_NotExist(t *testing.T) {
	h := mockHotSpareManager()

	err := h.RemoveHotSpare("/dev/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestHotSpareManager_RemoveHotSpare_Rebuilding(t *testing.T) {
	h := mockHotSpareManager()

	// 添加正在重建的热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:     "hs-1",
		Device: "/dev/sdb",
		Status: "rebuilding",
	}

	// 正在重建的热备盘不能移除
	err := h.RemoveHotSpare("/dev/sdb")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "正在重建")
}

func TestHotSpareManager_ListHotSpares(t *testing.T) {
	h := mockHotSpareManager()

	// 添加测试热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:         "hs-1",
		Device:     "/dev/sdb",
		VolumeName: "vol1",
		Status:     "available",
	}
	h.hotSpares["/dev/sdc"] = &HotSpare{
		ID:         "hs-2",
		Device:     "/dev/sdc",
		VolumeName: "",
		Status:     "available",
	}
	h.hotSpares["/dev/sdd"] = &HotSpare{
		ID:         "hs-3",
		Device:     "/dev/sdd",
		VolumeName: "vol2",
		Status:     "available",
	}

	// 列出所有
	all := h.ListHotSpares("")
	assert.Len(t, all, 3)

	// 列出 vol1 的热备盘
	vol1 := h.ListHotSpares("vol1")
	assert.Len(t, vol1, 2) // vol1 + 全局

	// 列出 vol2 的热备盘
	vol2 := h.ListHotSpares("vol2")
	assert.Len(t, vol2, 2) // vol2 + 全局
}

func TestHotSpareManager_GetHotSpare(t *testing.T) {
	h := mockHotSpareManager()

	// 添加测试热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:     "hs-1",
		Device: "/dev/sdb",
		Status: "available",
	}

	// 获取存在的热备盘
	hs, err := h.GetHotSpare("/dev/sdb")
	require.NoError(t, err)
	assert.Equal(t, "/dev/sdb", hs.Device)

	// 获取不存在的热备盘
	_, err = h.GetHotSpare("/dev/nonexistent")
	assert.Error(t, err)
}

func TestHotSpareManager_GetStatus(t *testing.T) {
	h := mockHotSpareManager()

	// 添加不同状态的热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		Device: "/dev/sdb",
		Status: "available",
	}
	h.hotSpares["/dev/sdc"] = &HotSpare{
		Device: "/dev/sdc",
		Status: "rebuilding",
	}
	h.hotSpares["/dev/sdd"] = &HotSpare{
		Device: "/dev/sdd",
		Status: "failed",
	}
	h.hotSpares["/dev/sde"] = &HotSpare{
		Device: "/dev/sde",
		Status: "standby",
	}

	status := h.GetStatus()
	assert.Equal(t, 4, status.TotalHotSpares)
	assert.Equal(t, 2, status.AvailableCount) // available + standby
	assert.Equal(t, 1, status.RebuildingCount)
	assert.Equal(t, 1, status.FailedCount)
}

// ========== 事件测试 ==========

func TestHotSpareManager_EventNotification(t *testing.T) {
	h := mockHotSpareManager()

	var receivedEvent HotSpareEvent
	var wg sync.WaitGroup
	wg.Add(1)

	// 设置通知回调
	h.SetNotificationFunc(func(event HotSpareEvent) {
		receivedEvent = event
		wg.Done()
	})

	// 发送测试事件
	testEvent := HotSpareEvent{
		Type:      "test",
		Device:    "/dev/sdb",
		Message:   "测试事件",
		Timestamp: time.Now(),
	}

	// 启动事件循环
	go h.eventLoop()
	defer h.Stop()

	h.emitEvent(testEvent)

	// 等待事件处理
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, "test", receivedEvent.Type)
		assert.Equal(t, "/dev/sdb", receivedEvent.Device)
	case <-time.After(2 * time.Second):
		t.Fatal("事件通知超时")
	}
}

// ========== 重建状态测试 ==========

func TestHotSpareManager_RebuildStatus(t *testing.T) {
	h := mockHotSpareManager()

	now := time.Now()
	// 添加正在重建的热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		ID:              "hs-1",
		Device:          "/dev/sdb",
		VolumeName:      "vol1",
		Status:          "rebuilding",
		FailedDevice:    "/dev/sda",
		RebuildProgress: 50.0,
		RebuildStarted:  now.Add(-10 * time.Minute),
	}

	// 获取重建状态
	status, err := h.GetRebuildStatus("/dev/sdb")
	require.NoError(t, err)

	assert.Equal(t, "/dev/sdb", status.Device)
	assert.Equal(t, "vol1", status.VolumeName)
	assert.Equal(t, "/dev/sda", status.FailedDevice)
	assert.Equal(t, 50.0, status.Progress)
	assert.Equal(t, "rebuilding", status.Status)
	assert.False(t, status.EstimatedEnd.IsZero())
}

func TestHotSpareManager_ListRebuilding(t *testing.T) {
	h := mockHotSpareManager()

	// 添加不同状态的热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		Device: "/dev/sdb",
		Status: "available",
	}
	h.hotSpares["/dev/sdc"] = &HotSpare{
		Device:          "/dev/sdc",
		Status:          "rebuilding",
		VolumeName:      "vol1",
		FailedDevice:    "/dev/sda",
		RebuildProgress: 30.0,
	}
	h.hotSpares["/dev/sdd"] = &HotSpare{
		Device:          "/dev/sdd",
		Status:          "rebuilding",
		VolumeName:      "vol2",
		FailedDevice:    "/dev/sde",
		RebuildProgress: 60.0,
	}

	rebuilding := h.ListRebuilding()
	assert.Len(t, rebuilding, 2)
}

// ========== 并发测试 ==========

func TestHotSpareManager_Concurrency(t *testing.T) {
	h := mockHotSpareManager()

	var wg sync.WaitGroup
	numOps := 100

	// 并发添加
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			device := "/dev/sd" + string(rune('a'+i%26))
			h.mu.Lock()
			h.hotSpares[device] = &HotSpare{
				ID:     "hs-" + string(rune(i)),
				Device: device,
				Status: "available",
			}
			h.mu.Unlock()
		}(i)
	}

	// 并发读取
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.ListHotSpares("")
			_ = h.GetStatus()
		}()
	}

	wg.Wait()

	// 验证最终状态
	status := h.GetStatus()
	assert.Equal(t, status.TotalHotSpares, status.AvailableCount+status.RebuildingCount+status.FailedCount)
}

// ========== 边界条件测试 ==========

func TestHotSpareManager_EmptyList(t *testing.T) {
	h := mockHotSpareManager()

	// 空列表
	all := h.ListHotSpares("")
	assert.NotNil(t, all)
	assert.Len(t, all, 0)

	status := h.GetStatus()
	assert.Equal(t, 0, status.TotalHotSpares)
}

func TestHotSpareManager_CancelRebuild_NotRebuilding(t *testing.T) {
	h := mockHotSpareManager()

	// 添加非重建状态的热备盘
	h.hotSpares["/dev/sdb"] = &HotSpare{
		Device: "/dev/sdb",
		Status: "available",
	}

	err := h.CancelRebuild("/dev/sdb")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不在重建状态")
}

func TestHotSpareManager_CancelRebuild_NotExist(t *testing.T) {
	h := mockHotSpareManager()

	err := h.CancelRebuild("/dev/nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== 配置验证测试 ==========

func TestHotSpareConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config HotSpareConfig
		valid  bool
	}{
		{
			name: "默认配置",
			config: DefaultHotSpareConfig,
			valid: true,
		},
		{
			name: "禁用自动重建",
			config: HotSpareConfig{
				Enabled:       true,
				AutoRebuild:   false,
				CheckInterval: 30 * time.Second,
			},
			valid: true,
		},
		{
			name: "极短检查间隔",
			config: HotSpareConfig{
				Enabled:       true,
				CheckInterval: 1 * time.Second,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 配置本身不做严格验证，只测试是否能正常设置
			h := mockHotSpareManager()
			h.SetConfig(tt.config)
			got := h.GetConfig()
			assert.Equal(t, tt.config, got)
		})
	}
}

// ========== 事件类型测试 ==========

func TestHotSpareEvent_Types(t *testing.T) {
	eventTypes := []string{
		"added",
		"removed",
		"activated",
		"rebuild_start",
		"rebuild_complete",
		"rebuild_failed",
		"rebuild_cancelled",
		"error",
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			event := HotSpareEvent{
				Type:        eventType,
				HotSpareID:  "hs-1",
				Device:      "/dev/sdb",
				VolumeName:  "vol1",
				FailedDevice: "/dev/sda",
				Message:     "测试消息",
				Timestamp:   time.Now(),
			}

			// 验证事件字段
			assert.Equal(t, eventType, event.Type)
			assert.NotEmpty(t, event.Message)
			assert.False(t, event.Timestamp.IsZero())
		})
	}
}

// ========== ID 生成测试 ==========

func TestGenerateID(t *testing.T) {
	id1 := generateID("/dev/sdb")
	id2 := generateID("/dev/sdb")

	// 两个 ID 应该不同（因为包含时间戳）
	assert.NotEqual(t, id1, id2)

	// ID 格式检查
	assert.Contains(t, id1, "hs-")
}

// ========== 热备盘状态转换测试 ==========

func TestHotSpare_StatusTransitions(t *testing.T) {
	h := mockHotSpareManager()

	// 初始状态：available
	hs := &HotSpare{
		ID:     "hs-1",
		Device: "/dev/sdb",
		Status: "available",
	}
	h.hotSpares["/dev/sdb"] = hs

	// 模拟状态转换：available -> rebuilding
	hs.Status = "rebuilding"
	hs.FailedDevice = "/dev/sda"
	hs.RebuildStarted = time.Now()

	// 模拟重建完成：rebuilding -> standby
	hs.Status = "standby"
	hs.RebuildProgress = 100
	hs.RebuildEnded = time.Now()

	// 验证最终状态
	got, err := h.GetHotSpare("/dev/sdb")
	require.NoError(t, err)
	assert.Equal(t, "standby", got.Status)
	assert.Equal(t, 100.0, got.RebuildProgress)
	assert.False(t, got.RebuildEnded.IsZero())
}