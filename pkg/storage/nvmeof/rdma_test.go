// Package nvmeof - NVMe over RDMA (NVMe/RDMA) 单元测试
// pkg 层 RDMA 功能测试

package nvmeof

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ========== RDMA Config Tests ==========

func TestRDMAConfigDefaultValues(t *testing.T) {
	config := DefaultRDMAConfig()

	require.True(t, config.Enabled)
	require.Equal(t, RDMATransportRoCEv2, config.TransportType)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 128, config.SQDepth)
	require.Equal(t, 128, config.RQDepth)
	require.Equal(t, 256, config.CQDepth)
	require.Equal(t, 128, config.MaxWR)
	require.Equal(t, 4096, config.MaxInlineData)
	require.True(t, config.ZeroCopy)
	require.True(t, config.PollMode)
	require.Empty(t, config.CPUAffinity)
}

func TestRDMAConfigValidateNil(t *testing.T) {
	config := &RDMAConfig{}
	err := config.Validate()
	require.NoError(t, err)
	require.Equal(t, RDMATransportRoCEv2, config.TransportType)
	require.Equal(t, 128, config.QueueDepth)
}

func TestRDMAConfigValidateInvalidTransport(t *testing.T) {
	config := &RDMAConfig{
		TransportType: "invalid",
		QueueDepth:    64,
	}
	err := config.Validate()
	require.NoError(t, err)
	require.Equal(t, RDMATransportRoCEv2, config.TransportType)
}

func TestRDMAConfigValidateNegativeValues(t *testing.T) {
	config := &RDMAConfig{
		QueueDepth:    -1,
		SQDepth:       -1,
		RQDepth:       -1,
		CQDepth:       -1,
		MaxWR:         -1,
		MaxInlineData: -1,
		PortConfig: RDMAPortConfig{
			PortNum:     -1,
			GIDIndex:    -1,
			ServicePort: -1,
			MTU:         -1,
		},
		Reconnect: RDMAReconnectConfig{
			Delay:       -1,
			MaxAttempts: -1,
			Timeout:     -1,
		},
	}
	err := config.Validate()
	require.NoError(t, err)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 128, config.SQDepth)
	require.Equal(t, 128, config.RQDepth)
	require.Equal(t, 256, config.CQDepth)
	require.Equal(t, 128, config.MaxWR)
	require.Equal(t, 0, config.MaxInlineData)
	require.Equal(t, 1, config.PortConfig.PortNum)
	require.Equal(t, 0, config.PortConfig.GIDIndex)
	require.Equal(t, 4420, config.PortConfig.ServicePort)
	require.Equal(t, 9000, config.PortConfig.MTU)
	require.Equal(t, 10, config.Reconnect.Delay)
	require.Equal(t, 30, config.Reconnect.MaxAttempts)
	require.Equal(t, 60, config.Reconnect.Timeout)
}

// ========== RDMA Target Config Tests ==========

func TestRDMATargetConfigDefaultValues(t *testing.T) {
	config := DefaultRDMATargetConfig()

	require.Equal(t, 1, config.PortNum)
	require.Equal(t, 0, config.GIDIndex)
	require.Equal(t, 4420, config.ServicePort)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 9000, config.MTU)
	require.True(t, config.Enabled)
}

func TestRDMATargetConfigValidateValid(t *testing.T) {
	config := &RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
		QueueDepth:  128,
		MTU:         9000,
		Enabled:     true,
	}
	err := config.Validate()
	require.NoError(t, err)
}

func TestRDMATargetConfigValidateMissingNQN(t *testing.T) {
	config := &RDMATargetConfig{
		Device:    "mlx5_0",
		IPAddress: "192.168.100.100",
	}
	err := config.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "subsys_nqn")
}

func TestRDMATargetConfigValidateMissingDevice(t *testing.T) {
	config := &RDMATargetConfig{
		SubsysNQN: "nqn.2026-03.org.nas-os:test",
		IPAddress: "192.168.100.100",
	}
	err := config.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "device")
}

func TestRDMATargetConfigValidateMissingIP(t *testing.T) {
	config := &RDMATargetConfig{
		SubsysNQN: "nqn.2026-03.org.nas-os:test",
		Device:    "mlx5_0",
	}
	err := config.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ip_address")
}

func TestRDMATargetConfigValidateDefaults(t *testing.T) {
	config := &RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: -1,
		PortNum:     -1,
		GIDIndex:    -2,
		QueueDepth:  -1,
		MTU:         -1,
	}
	err := config.Validate()
	require.NoError(t, err)
	require.Equal(t, 4420, config.ServicePort)
	require.Equal(t, 1, config.PortNum)
	require.Equal(t, 0, config.GIDIndex)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 9000, config.MTU)
}

// ========== RDMA Initiator Config Tests ==========

func TestRDMAInitiatorConfigDefaultValues(t *testing.T) {
	config := DefaultRDMAInitiatorConfig()

	require.Equal(t, 4420, config.TargetPort)
	require.Equal(t, 0, config.LocalGIDIndex)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 8, config.IOQueues)
	require.Equal(t, 10, config.ReconnectDelay)
	require.Equal(t, 30, config.MaxReconnect)
	require.True(t, config.PollMode)
}

func TestRDMAInitiatorConfigValidateValid(t *testing.T) {
	config := &RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:test",
		TargetAddress: "192.168.100.100",
		TargetPort:    4420,
		QueueDepth:    128,
		IOQueues:      8,
	}
	err := config.Validate()
	require.NoError(t, err)
}

func TestRDMAInitiatorConfigValidateMissingNQN(t *testing.T) {
	config := &RDMAInitiatorConfig{
		TargetAddress: "192.168.100.100",
	}
	err := config.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "target_nqn")
}

func TestRDMAInitiatorConfigValidateMissingAddress(t *testing.T) {
	config := &RDMAInitiatorConfig{
		TargetNQN: "nqn.2026-03.org.nas-os:test",
	}
	err := config.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "target_address")
}

func TestRDMAInitiatorConfigValidateDefaults(t *testing.T) {
	config := &RDMAInitiatorConfig{
		TargetNQN:      "nqn.2026-03.org.nas-os:test",
		TargetAddress:  "192.168.100.100",
		TargetPort:     -1,
		QueueDepth:     -1,
		IOQueues:       -1,
		ReconnectDelay: -1,
		MaxReconnect:   -1,
	}
	err := config.Validate()
	require.NoError(t, err)
	require.Equal(t, 4420, config.TargetPort)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 8, config.IOQueues)
	require.Equal(t, 10, config.ReconnectDelay)
	require.Equal(t, 30, config.MaxReconnect)
}

// ========== RDMA Manager Tests ==========

func TestNewRDMAManagerNilConfig(t *testing.T) {
	manager, err := NewRDMAManager(nil)
	require.NoError(t, err)
	require.NotNil(t, manager)
	require.True(t, manager.IsAvailable())
}

func TestNewRDMAManagerWithConfig(t *testing.T) {
	config := DefaultRDMAConfig()
	manager, err := NewRDMAManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)
	require.Equal(t, config, manager.GetConfig())
}

func TestRDMAManagerIsAvailable(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	require.True(t, manager.IsAvailable())
}

func TestRDMAManagerGetDevices(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	devices := manager.GetDevices()
	require.NotEmpty(t, devices)
}

func TestRDMAManagerGetDevice(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	device, err := manager.GetDevice("mlx5_0")
	require.NoError(t, err)
	require.NotNil(t, device)
	require.Equal(t, "mlx5_0", device.Name)
}

func TestRDMAManagerGetDeviceNotFound(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	_, err := manager.GetDevice("nonexistent")
	require.Error(t, err)
	require.Equal(t, ErrRDMADeviceNotFound, err)
}

func TestRDMAManagerGetDeviceByIP(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	device, gidIndex, err := manager.GetDeviceByIP("192.168.100.100")
	require.NoError(t, err)
	require.NotNil(t, device)
	require.Equal(t, 0, gidIndex)
}

func TestRDMAManagerGetDeviceByIPNotFound(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	_, _, err := manager.GetDeviceByIP("192.168.999.999")
	require.Error(t, err)
	require.Equal(t, ErrRDMAGIDNotFound, err)
}

func TestRDMAManagerCreateTarget(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	req := &RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
		QueueDepth:  128,
		MTU:         9000,
		Enabled:     true,
	}

	target, err := manager.CreateRDMATarget(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, target)

	// 验证已存储
	targets := manager.ListRDMATargets()
	require.Len(t, targets, 1)
}

func TestRDMAManagerDeleteTarget(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	req := &RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:test-delete",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
	}

	_, _ = manager.CreateRDMATarget(ctx, req)
	err := manager.DeleteRDMATarget(ctx, req.SubsysNQN)
	require.NoError(t, err)

	targets := manager.ListRDMATargets()
	require.Empty(t, targets)
}

func TestRDMAManagerDeleteTargetNotFound(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	err := manager.DeleteRDMATarget(ctx, "nonexistent")
	require.Error(t, err)
	require.Equal(t, ErrSubsystemNotFound, err)
}

func TestRDMAManagerCreateInitiator(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	req := &RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:test",
		TargetAddress: "192.168.100.100",
		TargetPort:    4420,
		QueueDepth:    128,
		IOQueues:      8,
	}

	initiator, err := manager.CreateRDMAInitiator(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, initiator)

	initiators := manager.ListRDMAInitiators()
	require.Len(t, initiators, 1)
}

func TestRDMAManagerDeleteInitiator(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	req := &RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:test-del",
		TargetAddress: "192.168.100.100",
	}

	_, _ = manager.CreateRDMAInitiator(ctx, req)
	id := req.TargetNQN + "-" + req.TargetAddress
	err := manager.DeleteRDMAInitiator(ctx, id)
	require.NoError(t, err)

	initiators := manager.ListRDMAInitiators()
	require.Empty(t, initiators)
}

func TestRDMAManagerGetPerformanceStats(t *testing.T) {
	manager, _ := NewRDMAManager(nil)

	stats, err := manager.GetPerformanceStats("mlx5_0")
	require.NoError(t, err)
	require.NotNil(t, stats)
}

func TestRDMAManagerGetPerformanceStatsNotFound(t *testing.T) {
	manager, _ := NewRDMAManager(nil)

	_, err := manager.GetPerformanceStats("nonexistent")
	require.Error(t, err)
	require.Equal(t, ErrRDMADeviceNotFound, err)
}

func TestRDMAManagerStartStop(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	err := manager.Start(ctx)
	require.NoError(t, err)

	err = manager.Start(ctx) // 再次启动应无错误
	require.NoError(t, err)

	err = manager.Stop()
	require.NoError(t, err)

	err = manager.Stop() // 再次停止应无错误
	require.NoError(t, err)
}

func TestRDMAManagerSetEventChannel(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ch := make(chan NVMeOFEvent, 10)
	manager.SetEventChannel(ch)
}

// ========== RDMA Helper Functions Tests ==========

func TestCheckRDMAAvailable(t *testing.T) {
	available, err := CheckRDMAAvailable()
	require.NoError(t, err)
	require.True(t, available)
}

func TestGetRDMAKernelModules(t *testing.T) {
	modules, err := GetRDMAKernelModules()
	require.NoError(t, err)
	require.NotEmpty(t, modules)
	require.Contains(t, modules, "ib_core")
	require.Contains(t, modules, "rdma_cm")
}

func TestLoadRDMAKernelModules(t *testing.T) {
	err := LoadRDMAKernelModules()
	require.NoError(t, err)
}

func TestValidateRDMAGID(t *testing.T) {
	require.True(t, ValidateRDMAGID("fe80::1"))
	require.True(t, ValidateRDMAGID("any-non-empty-string"))
	require.False(t, ValidateRDMAGID(""))
}

func TestParseGIDIPAddress(t *testing.T) {
	ip := ParseGIDIPAddress("fe80::1")
	require.Empty(t, ip) // Mock 实现返回空
}

// ========== RDMA Device Tests ==========

func TestRDMADeviceState(t *testing.T) {
	states := []RDMADeviceState{
		RDMADeviceStateUp,
		RDMADeviceStateDown,
		RDMADeviceStateInit,
		RDMADeviceStateError,
	}
	for _, state := range states {
		require.NotEmpty(t, string(state))
	}
}

func TestRDMAPortStateValues(t *testing.T) {
	states := []RDMAPortState{
		RDMAPortStateUp,
		RDMAPortStateDown,
		RDMAPortStateInit,
	}
	for _, state := range states {
		require.NotEmpty(t, string(state))
	}
}

func TestRDMAPortPhysStateValues(t *testing.T) {
	states := []RDMAPortPhysState{
		RDMAPortPhysStateSleep,
		RDMAPortPhysStatePolling,
		RDMAPortPhysStateDisabled,
		RDMAPortPhysStatePortConfiguration,
		RDMAPortPhysStateLinkUp,
		RDMAPortPhysStateLinkDown,
		RDMAPortPhysStateUnknown,
	}
	for _, state := range states {
		require.NotEmpty(t, string(state))
	}
}

func TestValidRDMATransports(t *testing.T) {
	require.True(t, ValidRDMATransports[RDMATransportRoCEv2])
	require.True(t, ValidRDMATransports[RDMATransportiWARP])
	require.True(t, ValidRDMATransports[RDMATransportIB])
	require.False(t, ValidRDMATransports["invalid"])
}

// ========== RDMA Performance Stats Tests ==========

func TestRDMAPerformanceStatsStruct(t *testing.T) {
	stats := &RDMAPerformanceStats{
		ReadBandwidth:  75000000000, // 75 GB/s
		WriteBandwidth: 50000000000,
		ReadIOPS:       500000,
		WriteIOPS:      400000,
		AvgLatency:     20,
		P50Latency:     15,
		P95Latency:     50,
		P99Latency:     100,
		MaxLatency:     500,
		SQDepth:        128,
		RQDepth:        128,
		CQDepth:        256,
		ActiveQP:       64,
		RDMAErrors:     0,
		TxErrors:       0,
		RxErrors:       0,
		Timeouts:       0,
		ReconnectCount: 0,
		Timestamp:      time.Now(),
	}

	require.Equal(t, uint64(75000000000), stats.ReadBandwidth)
	require.Equal(t, uint64(500000), stats.ReadIOPS)
	require.Equal(t, 128, stats.SQDepth)
}

// ========== NVMeOF Config Tests ==========

func TestNVMeOFConfigWithRDMA(t *testing.T) {
	config := DefaultNVMeOFConfig()
	require.Equal(t, TransportTCP, config.DefaultTransport)

	// 验证可以设置 RDMA
	config.DefaultTransport = TransportRDMA
	require.Equal(t, TransportRDMA, config.DefaultTransport)
}

func TestValidTransportsIncludeRDMA(t *testing.T) {
	require.True(t, ValidTransports[TransportRDMA])
	require.True(t, ValidTransports[TransportTCP])
}

// ========== Integration Tests ==========

func TestRDMAManagerFullFlow(t *testing.T) {
	manager, _ := NewRDMAManager(nil)
	ctx := context.Background()

	// 1. 启动
	_ = manager.Start(ctx)

	// 2. 检查设备
	devices := manager.GetDevices()
	require.NotEmpty(t, devices)

	// 3. 创建 Target
	targetReq := &RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:flow-test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
	}
	_, _ = manager.CreateRDMATarget(ctx, targetReq)

	// 4. 创建 Initiator
	initiatorReq := &RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:flow-test",
		TargetAddress: "192.168.100.100",
	}
	_, _ = manager.CreateRDMAInitiator(ctx, initiatorReq)

	// 5. 获取统计
	_, _ = manager.GetPerformanceStats("mlx5_0")

	// 6. 清理
	_ = manager.DeleteRDMATarget(ctx, targetReq.SubsysNQN)
	id := initiatorReq.TargetNQN + "-" + initiatorReq.TargetAddress
	_ = manager.DeleteRDMAInitiator(ctx, id)

	// 7. 停止
	_ = manager.Stop()
}
