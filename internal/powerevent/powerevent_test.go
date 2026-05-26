package powerevent

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	logger := zap.NewNop()

	manager := NewManager(config, logger)

	assert.NotNil(t, manager)
	assert.Equal(t, config.LowBatteryThreshold, manager.config.LowBatteryThreshold)
	assert.Equal(t, ShutdownPolicyGraceful, manager.policy)
	assert.NotNil(t, manager.schedules)
	assert.NotNil(t, manager.eventChan)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 20, config.LowBatteryThreshold)
	assert.Equal(t, 10, config.CriticalBatteryThreshold)
	assert.Equal(t, 30*time.Second, config.UPSCheckInterval)
	assert.Equal(t, 5*time.Minute, config.ShutdownDelay)
	assert.Equal(t, 3, config.WOLRetryCount)
	assert.Equal(t, 5*time.Second, config.WOLRetryInterval)
	assert.Equal(t, 1000, config.MaxHistorySize)
}

func TestManager_StartStop(t *testing.T) {
	config := DefaultConfig()
	config.UPSCheckInterval = 100 * time.Millisecond // 加快测试
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	require.NoError(t, err)

	// 等待一下让goroutine启动
	time.Sleep(50 * time.Millisecond)

	manager.Stop()
}

func TestSchedulePowerOn(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduledAt := time.Now().Add(1 * time.Hour)
	mac := "00:11:22:33:44:55"
	ip := "192.168.1.100"

	event, err := manager.SchedulePowerOn(ctx, scheduledAt, mac, ip)
	require.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, PowerEventPowerOn, event.Type)
	assert.Equal(t, StatePending, event.State)
	assert.Equal(t, mac, event.TargetMAC)
	assert.Equal(t, ip, event.TargetIP)
	assert.NotNil(t, event.ScheduledAt)
}

func TestSchedulePowerOn_EmptyMAC(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduledAt := time.Now().Add(1 * time.Hour)

	_, err := manager.SchedulePowerOn(ctx, scheduledAt, "", "192.168.1.100")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MAC地址不能为空")
}

func TestSchedulePowerOff(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduledAt := time.Now().Add(1 * time.Hour)

	event, err := manager.SchedulePowerOff(ctx, scheduledAt, ShutdownPolicyGraceful, 30)
	require.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, PowerEventPowerOff, event.Type)
	assert.Equal(t, StatePending, event.State)
}

func TestScheduleRestart(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduledAt := time.Now().Add(1 * time.Hour)

	event, err := manager.ScheduleRestart(ctx, scheduledAt, ShutdownPolicyDelayed, 60)
	require.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, PowerEventRestart, event.Type)
	assert.Equal(t, StatePending, event.State)
}

func TestHandleUPSEvent_OnBattery(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upsStatus := UPSStatus{
		Online:       false,
		BatteryLevel: 50,
		BatteryHealth: "good",
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnBattery, upsStatus)
	assert.NoError(t, err)

	// 验证UPS状态已更新
	status := manager.CheckBatteryStatus()
	assert.Equal(t, 50, status.BatteryLevel)
	assert.False(t, status.Online)
}

func TestHandleUPSEvent_LowBattery(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upsStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 15, // 低于低电量阈值20%
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnBattery, upsStatus)
	assert.NoError(t, err)
}

func TestHandleUPSEvent_CriticalBattery(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upsStatus := UPSStatus{
		Online:        true,
		BatteryLevel:  5, // 低于临界电量阈值10%
		EstimatedMin:  5,
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnBattery, upsStatus)
	assert.NoError(t, err)
}

func TestHandleUPSEvent_OnLine(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upsStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 100,
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnLine, upsStatus)
	assert.NoError(t, err)
}

func TestHandleUPSEvent_Shutdown(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upsStatus := UPSStatus{
		Online:       false,
		BatteryLevel: 0,
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSShutdown, upsStatus)
	assert.NoError(t, err)
}

func TestSetShutdownPolicy(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	assert.Equal(t, ShutdownPolicyGraceful, manager.GetShutdownPolicy())

	manager.SetShutdownPolicy(ShutdownPolicyImmediate)
	assert.Equal(t, ShutdownPolicyImmediate, manager.GetShutdownPolicy())

	manager.SetShutdownPolicy(ShutdownPolicyDelayed)
	assert.Equal(t, ShutdownPolicyDelayed, manager.GetShutdownPolicy())
}

func TestGetPowerHistory(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 添加一些事件
	_, _ = manager.SchedulePowerOn(ctx, time.Now(), "00:11:22:33:44:55", "192.168.1.1")
	_, _ = manager.SchedulePowerOn(ctx, time.Now(), "00:11:22:33:44:66", "192.168.1.2")
	_, _ = manager.SchedulePowerOn(ctx, time.Now(), "00:11:22:33:44:77", "192.168.1.3")

	// 获取历史记录
	history := manager.GetPowerHistory(10)
	assert.Len(t, history, 3)

	// 获取限制数量
	history = manager.GetPowerHistory(2)
	assert.Len(t, history, 2)
}

func TestTriggerWakeOnLan(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ip := "192.168.1.255"

	// 注意：实际WOL测试需要网络环境，这里只测试错误情况
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 测试无效MAC
	err := manager.TriggerWakeOnLan(ctx, "invalid-mac", ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的MAC地址")
}

func TestCheckBatteryStatus(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	// 默认状态
	status := manager.CheckBatteryStatus()
	assert.False(t, status.Online)
	assert.Equal(t, 0, status.BatteryLevel)

	// 更新状态
	newStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 85,
		Temperature:  35.5,
	}
	manager.UpdateUPSStatus(newStatus)

	status = manager.CheckBatteryStatus()
	assert.True(t, status.Online)
	assert.Equal(t, 85, status.BatteryLevel)
	assert.Equal(t, 35.5, status.Temperature)
}

func TestScheduleManagement(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	// 添加调度
	schedule := &PowerSchedule{
		Name:      "每日关机",
		Enabled:   true,
		EventType: PowerEventPowerOff,
		CronExpr:  "0 2 * * *",
	}

	err := manager.AddSchedule(schedule)
	require.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)

	// 获取调度列表
	schedules := manager.GetSchedules()
	assert.Len(t, schedules, 1)
	assert.Equal(t, "每日关机", schedules[0].Name)

	// 更新调度
	schedule.Name = "每日凌晨关机"
	err = manager.UpdateSchedule(schedule)
	require.NoError(t, err)

	schedules = manager.GetSchedules()
	assert.Equal(t, "每日凌晨关机", schedules[0].Name)

	// 删除调度
	err = manager.RemoveSchedule(schedule.ID)
	require.NoError(t, err)

	schedules = manager.GetSchedules()
	assert.Len(t, schedules, 0)

	// 删除不存在的调度
	err = manager.RemoveSchedule("non-existent")
	assert.Error(t, err)
}

func TestBuildMagicPacket(t *testing.T) {
	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	packet := buildMagicPacket(mac)

	// 魔术包应该是 6 + 16*6 = 102 字节
	assert.Len(t, packet, 102)

	// 前6字节应该是0xFF
	for i := 0; i < 6; i++ {
		assert.Equal(t, byte(0xFF), packet[i])
	}

	// 检查MAC地址重复
	for i := 0; i < 16; i++ {
		offset := 6 + i*6
		assert.Equal(t, mac, net.HardwareAddr(packet[offset:offset+6]))
	}
}

func TestMaxHistorySize(t *testing.T) {
	config := DefaultConfig()
	config.MaxHistorySize = 5
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 添加超过限制的事件
	for i := 0; i < 10; i++ {
		_, _ = manager.SchedulePowerOn(ctx, time.Now(), "00:11:22:33:44:55", "192.168.1.1")
	}

	history := manager.GetPowerHistory(100)
	assert.Len(t, history, 5) // 应该只保留最新的5个
}

func TestExecutePowerOff_WithDelay(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 测试延迟关机（会超时）
	err := manager.executePowerOff(ctx, ShutdownPolicyDelayed, 10)
	assert.Error(t, err) // 应该因为context超时而失败
}

func TestExecutePowerOff_ContextCancelled(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	// 立即取消context
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := manager.executePowerOff(ctx, ShutdownPolicyDelayed, 100)
	assert.Error(t, err)
}

func TestExecuteRestart(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx := context.Background()

	// 测试立即重启（无延迟）
	err := manager.executeRestart(ctx, ShutdownPolicyImmediate, 0)
	assert.NoError(t, err)
}

func TestSchedulePowerOff_DefaultPolicy(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 不指定策略，应该使用默认策略
	event, err := manager.SchedulePowerOff(ctx, time.Now().Add(1*time.Hour), "", 0)
	require.NoError(t, err)
	assert.NotNil(t, event)
}

func TestScheduleRestart_DefaultPolicy(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	event, err := manager.ScheduleRestart(ctx, time.Now().Add(1*time.Hour), "", 0)
	require.NoError(t, err)
	assert.NotNil(t, event)
}

func TestHandleUPSEvent_LowBatteryProtection(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 模拟低电量场景（但未达到临界）
	upsStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 15,
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnBattery, upsStatus)
	assert.NoError(t, err)

	// 验证事件被记录
	history := manager.GetPowerHistory(10)
	assert.True(t, len(history) > 0)
}

func TestHandleUPSEvent_CriticalBatteryShutdown(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 模拟临界电量场景
	upsStatus := UPSStatus{
		Online:        true,
		BatteryLevel:  5,
		EstimatedMin:  2,
	}

	err := manager.HandleUPSEvent(ctx, PowerEventUPSOnBattery, upsStatus)
	// 可能会因为context超时而返回错误，这是预期的
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
}

func TestBuildMagicPacket_DifferentMACs(t *testing.T) {
	tests := []struct {
		mac    string
		valid  bool
	}{
		{"00:11:22:33:44:55", true},
		{"AA:BB:CC:DD:EE:FF", true},
		{"ff:ff:ff:ff:ff:ff", true},
	}

	for _, tt := range tests {
		t.Run(tt.mac, func(t *testing.T) {
			parsedMAC, err := net.ParseMAC(tt.mac)
			require.NoError(t, err)

			packet := buildMagicPacket(parsedMAC)
			assert.Len(t, packet, 102)

			// 验证前6字节是0xFF
			for i := 0; i < 6; i++ {
				assert.Equal(t, byte(0xFF), packet[i])
			}
		})
	}
}

func TestUpdateSchedule_NotFound(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	schedule := &PowerSchedule{
		ID:        "non-existent",
		Name:      "test",
		EventType: PowerEventPowerOff,
	}

	err := manager.UpdateSchedule(schedule)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "调度不存在")
}

func TestAddSchedule_AutoID(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	schedule := &PowerSchedule{
		Name:      "test",
		EventType: PowerEventPowerOff,
	}

	err := manager.AddSchedule(schedule)
	require.NoError(t, err)
	assert.NotEmpty(t, schedule.ID)
}

func TestGetPowerHistory_EmptyHistory(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	history := manager.GetPowerHistory(10)
	assert.Len(t, history, 0)
}

func TestGetPowerHistory_LimitZero(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _ = manager.SchedulePowerOn(ctx, time.Now(), "00:11:22:33:44:55", "192.168.1.1")

	// limit为0应该返回所有
	history := manager.GetPowerHistory(0)
	assert.Len(t, history, 1)
}

func TestCheckUPSBattery_CriticalLevel(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	// 设置UPS状态为临界电量
	upsStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 8,
	}
	manager.UpdateUPSStatus(upsStatus)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 直接调用checkUPSBattery
	manager.checkUPSBattery(ctx)

	// 验证事件被记录
	history := manager.GetPowerHistory(10)
	assert.True(t, len(history) > 0)
}

func TestCheckUPSBattery_NormalLevel(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, zap.NewNop())

	// 设置UPS状态为正常电量
	upsStatus := UPSStatus{
		Online:       true,
		BatteryLevel: 80,
	}
	manager.UpdateUPSStatus(upsStatus)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 调用checkUPSBattery
	manager.checkUPSBattery(ctx)

	// 正常电量不应该触发事件
	history := manager.GetPowerHistory(10)
	assert.Len(t, history, 0)
}
