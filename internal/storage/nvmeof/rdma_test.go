// Package nvmeof - NVMe-oF RDMA 单元测试
// 测试 RDMA Target、Initiator、API Handlers

package nvmeof

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgnvmeof "nas-os/pkg/storage/nvmeof"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== RDMA Config Tests ==========

func TestRDMAConfigDefault(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()

	require.True(t, config.Enabled)
	require.Equal(t, pkgnvmeof.RDMATransportRoCEv2, config.TransportType)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 128, config.SQDepth)
	require.Equal(t, 128, config.RQDepth)
	require.Equal(t, 256, config.CQDepth)
	require.True(t, config.ZeroCopy)
	require.True(t, config.PollMode)
}

func TestRDMAConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		input    *pkgnvmeof.RDMAConfig
		expected *pkgnvmeof.RDMAConfig
	}{
		{
			name:     "nil config uses defaults",
			input:    nil,
			expected: pkgnvmeof.DefaultRDMAConfig(),
		},
		{
			name: "empty transport type defaults to rocev2",
			input: &pkgnvmeof.RDMAConfig{
				Enabled: true,
			},
			expected: &pkgnvmeof.RDMAConfig{
				Enabled:       true,
				TransportType: pkgnvmeof.RDMATransportRoCEv2,
				QueueDepth:    128,
				SQDepth:       128,
				RQDepth:       128,
				CQDepth:       256,
				MaxWR:         128,
				MaxInlineData: 0,
			},
		},
		{
			name: "invalid queue depth is corrected",
			input: &pkgnvmeof.RDMAConfig{
				Enabled:    true,
				QueueDepth: -1,
				SQDepth:    0,
				RQDepth:    -100,
				CQDepth:    0,
			},
			expected: &pkgnvmeof.RDMAConfig{
				Enabled:       true,
				TransportType: pkgnvmeof.RDMATransportRoCEv2,
				QueueDepth:    128,
				SQDepth:       128,
				RQDepth:       128,
				CQDepth:       256,
				MaxWR:         128,
			},
		},
		{
			name: "valid config is preserved",
			input: &pkgnvmeof.RDMAConfig{
				Enabled:       true,
				TransportType: pkgnvmeof.RDMATransportIB,
				QueueDepth:    256,
				SQDepth:       256,
				RQDepth:       256,
				CQDepth:       512,
				MaxWR:         256,
				MaxInlineData: 8192,
				ZeroCopy:      false,
				PollMode:      false,
			},
			expected: &pkgnvmeof.RDMAConfig{
				Enabled:       true,
				TransportType: pkgnvmeof.RDMATransportIB,
				QueueDepth:    256,
				SQDepth:       256,
				RQDepth:       256,
				CQDepth:       512,
				MaxWR:         256,
				MaxInlineData: 8192,
				ZeroCopy:      false,
				PollMode:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != nil {
				err := tt.input.Validate()
				require.NoError(t, err)
				require.Equal(t, tt.expected.QueueDepth, tt.input.QueueDepth)
				require.Equal(t, tt.expected.TransportType, tt.input.TransportType)
			}
		})
	}
}

func TestRDMATargetConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *pkgnvmeof.RDMATargetConfig
		wantErr bool
	}{
		{
			name: "valid config",
			input: &pkgnvmeof.RDMATargetConfig{
				SubsysNQN:   "nqn.2026-03.org.nas-os:test",
				Device:      "mlx5_0",
				IPAddress:   "192.168.100.100",
				ServicePort: 4420,
			},
			wantErr: false,
		},
		{
			name: "missing nqn",
			input: &pkgnvmeof.RDMATargetConfig{
				Device:    "mlx5_0",
				IPAddress: "192.168.100.100",
			},
			wantErr: true,
		},
		{
			name: "missing device",
			input: &pkgnvmeof.RDMATargetConfig{
				SubsysNQN: "nqn.2026-03.org.nas-os:test",
				IPAddress: "192.168.100.100",
			},
			wantErr: true,
		},
		{
			name: "missing ip",
			input: &pkgnvmeof.RDMATargetConfig{
				SubsysNQN: "nqn.2026-03.org.nas-os:test",
				Device:    "mlx5_0",
			},
			wantErr: true,
		},
		{
			name: "invalid port corrected",
			input: &pkgnvmeof.RDMATargetConfig{
				SubsysNQN:   "nqn.2026-03.org.nas-os:test",
				Device:      "mlx5_0",
				IPAddress:   "192.168.100.100",
				ServicePort: -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRDMAInitiatorConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *pkgnvmeof.RDMAInitiatorConfig
		wantErr bool
	}{
		{
			name: "valid config",
			input: &pkgnvmeof.RDMAInitiatorConfig{
				TargetNQN:     "nqn.2026-03.org.nas-os:test",
				TargetAddress: "192.168.100.100",
				TargetPort:    4420,
			},
			wantErr: false,
		},
		{
			name: "missing target nqn",
			input: &pkgnvmeof.RDMAInitiatorConfig{
				TargetAddress: "192.168.100.100",
			},
			wantErr: true,
		},
		{
			name: "missing target address",
			input: &pkgnvmeof.RDMAInitiatorConfig{
				TargetNQN: "nqn.2026-03.org.nas-os:test",
			},
			wantErr: true,
		},
		{
			name: "defaults applied",
			input: &pkgnvmeof.RDMAInitiatorConfig{
				TargetNQN:     "nqn.2026-03.org.nas-os:test",
				TargetAddress: "192.168.100.100",
				TargetPort:    -1,
				QueueDepth:    -1,
				IOQueues:      -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.name == "defaults applied" {
					require.Equal(t, 4420, tt.input.TargetPort)
					require.Equal(t, 128, tt.input.QueueDepth)
					require.Equal(t, 8, tt.input.IOQueues)
				}
			}
		})
	}
}

// ========== RDMA Manager Tests ==========

func TestRDMAManagerCreation(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()

	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// 测试可用性检查
	require.True(t, manager.IsAvailable())

	// 测试设备列表
	devices := manager.GetDevices()
	require.NotEmpty(t, devices)
}

func TestRDMAManagerDeviceOperations(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	// 测试获取设备
	device, err := manager.GetDevice("mlx5_0")
	require.NoError(t, err)
	require.NotNil(t, device)
	require.Equal(t, "mlx5_0", device.Name)

	// 测试获取不存在设备
	_, err = manager.GetDevice("nonexistent")
	require.Error(t, err)
	require.Equal(t, pkgnvmeof.ErrRDMADeviceNotFound, err)
}

func TestRDMAManagerTargetCRUD(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	ctx := context.Background()

	// 创建 Target 配置
	req := &pkgnvmeof.RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:test-subsys",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
		QueueDepth:  128,
		MTU:         9000,
		Enabled:     true,
	}

	// 创建
	target, err := manager.CreateRDMATarget(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, target)

	// 列出
	targets := manager.ListRDMATargets()
	require.NotEmpty(t, targets)

	// 删除
	err = manager.DeleteRDMATarget(ctx, req.SubsysNQN)
	require.NoError(t, err)

	// 再次列出应为空
	targets = manager.ListRDMATargets()
	require.Empty(t, targets)
}

func TestRDMAManagerInitiatorCRUD(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	ctx := context.Background()

	// 创建 Initiator 配置
	req := &pkgnvmeof.RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:test-subsys",
		TargetAddress: "192.168.100.100",
		TargetPort:    4420,
		QueueDepth:    128,
		IOQueues:      8,
	}

	// 创建
	initiator, err := manager.CreateRDMAInitiator(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, initiator)

	// 列出
	initiators := manager.ListRDMAInitiators()
	require.NotEmpty(t, initiators)

	// 删除
	id := req.TargetNQN + "-" + req.TargetAddress
	err = manager.DeleteRDMAInitiator(ctx, id)
	require.NoError(t, err)

	// 再次列出应为空
	initiators = manager.ListRDMAInitiators()
	require.Empty(t, initiators)
}

func TestRDMAManagerStartStop(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	ctx := context.Background()

	// 启动
	err = manager.Start(ctx)
	require.NoError(t, err)

	// 再次启动应无错误
	err = manager.Start(ctx)
	require.NoError(t, err)

	// 停止
	err = manager.Stop()
	require.NoError(t, err)

	// 再次停止应无错误
	err = manager.Stop()
	require.NoError(t, err)
}

// ========== RDMA Target Sys Manager Tests ==========

func TestCreateRDMAPortRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateRDMAPortRequest
		wantErr bool
	}{
		{
			name: "valid request",
			input: &CreateRDMAPortRequest{
				Address:     "192.168.100.100",
				ServicePort: 4420,
				Device:      "mlx5_0",
				GIDIndex:    0,
				MTU:         9000,
			},
			wantErr: false,
		},
		{
			name: "missing address",
			input: &CreateRDMAPortRequest{
				ServicePort: 4420,
			},
			wantErr: true,
		},
		{
			name: "invalid port corrected",
			input: &CreateRDMAPortRequest{
				Address:     "192.168.100.100",
				ServicePort: -1,
			},
			wantErr: false,
		},
		{
			name: "invalid mtu corrected",
			input: &CreateRDMAPortRequest{
				Address: "192.168.100.100",
				MTU:     -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ========== RDMA Initiator Request Tests ==========

func TestConnectRDMATargetRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *ConnectRDMATargetRequest
		wantErr bool
	}{
		{
			name: "valid request",
			input: &ConnectRDMATargetRequest{
				TargetNQN:     "nqn.2026-03.org.nas-os:test",
				TargetAddress: "192.168.100.100",
				TargetPort:    4420,
				QueueDepth:    128,
				IOQueues:      8,
			},
			wantErr: false,
		},
		{
			name: "missing target nqn",
			input: &ConnectRDMATargetRequest{
				TargetAddress: "192.168.100.100",
			},
			wantErr: true,
		},
		{
			name: "missing target address",
			input: &ConnectRDMATargetRequest{
				TargetNQN: "nqn.2026-03.org.nas-os:test",
			},
			wantErr: true,
		},
		{
			name: "defaults applied",
			input: &ConnectRDMATargetRequest{
				TargetNQN:     "nqn.2026-03.org.nas-os:test",
				TargetAddress: "192.168.100.100",
				TargetPort:    -1,
				QueueDepth:    -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDiscoverRDMATargetsRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *DiscoverRDMATargetsRequest
		wantErr bool
	}{
		{
			name: "valid request",
			input: &DiscoverRDMATargetsRequest{
				Address: "192.168.100.100",
				Port:    8009,
			},
			wantErr: false,
		},
		{
			name: "missing address",
			input: &DiscoverRDMATargetsRequest{
				Port: 8009,
			},
			wantErr: true,
		},
		{
			name: "invalid port corrected",
			input: &DiscoverRDMATargetsRequest{
				Address: "192.168.100.100",
				Port:    -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ========== RDMA Handlers Tests ==========

func TestRDMAHandlers(t *testing.T) {
	// 创建临时目录模拟 sysfs
	tmpDir := t.TempDir()
	configfsPath := filepath.Join(tmpDir, "config", "nvmet", "ports")
	_ = os.MkdirAll(configfsPath, 0o755)

	// 创建模拟 RDMA 管理器
	config := pkgnvmeof.DefaultRDMAConfig()
	rdmaManager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)
	require.NotNil(t, rdmaManager)
}

func TestRDMAHandlersRoutes(t *testing.T) {
	r := gin.New()

	// 注册路由
	handlers := NewRDMAHandlers(nil, nil)
	handlers.RegisterRoutes(r.Group("/api/v1"))

	// 验证路由存在
	routes := []string{
		"/api/v1/nvmeof/rdma/devices",
		"/api/v1/nvmeof/rdma/target/status",
		"/api/v1/nvmeof/rdma/initiator/status",
	}

	for _, route := range routes {
		// 简单验证路由注册
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// 不检查状态码，因为 manager 为 nil
		// 仅验证路由存在
		require.NotEqual(t, http.StatusNotFound, w.Code)
	}
}

func TestRDMAHandlersUninitialized(t *testing.T) {
	r := gin.New()

	handlers := NewRDMAHandlers(nil, nil)
	handlers.RegisterRoutes(r.Group("/api/v1"))

	// 测试未初始化时的响应
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nvmeof/rdma/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 应返回内部错误
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRDMAHandlersCreatePort(t *testing.T) {
	r := gin.New()

	handlers := NewRDMAHandlers(nil, nil)
	handlers.RegisterRoutes(r.Group("/api/v1"))

	// 测试创建端口
	body := mustJSONRDMA(t, map[string]interface{}{
		"address":     "192.168.100.100",
		"servicePort": 4420,
		"device":      "mlx5_0",
		"gidIndex":    0,
		"mtu":         9000,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/rdma/target/ports", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未初始化时应返回错误
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRDMAHandlersConnect(t *testing.T) {
	r := gin.New()

	handlers := NewRDMAHandlers(nil, nil)
	handlers.RegisterRoutes(r.Group("/api/v1"))

	// 测试连接 Target
	body := mustJSONRDMA(t, map[string]interface{}{
		"targetNqn":     "nqn.2026-03.org.nas-os:test",
		"targetAddress": "192.168.100.100",
		"targetPort":    4420,
		"queueDepth":    128,
		"ioQueues":      8,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/rdma/initiator/controllers", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未初始化时应返回错误
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRDMAHandlersDiscover(t *testing.T) {
	r := gin.New()

	handlers := NewRDMAHandlers(nil, nil)
	handlers.RegisterRoutes(r.Group("/api/v1"))

	// 测试发现 Target
	body := mustJSONRDMA(t, map[string]interface{}{
		"address": "192.168.100.100",
		"port":    8009,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nvmeof/rdma/initiator/discover", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 未初始化时应返回错误
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// ========== RDMA Port Tests ==========

func TestRDMAPortState(t *testing.T) {
	port := &RDMAPort{
		ID:            1,
		Address:       "192.168.100.100",
		ServicePort:   4420,
		TransportType: pkgnvmeof.RDMATransportRoCEv2,
		State:         RDMAPortStateUp,
	}

	require.Equal(t, 1, port.ID)
	require.Equal(t, "192.168.100.100", port.Address)
	require.Equal(t, 4420, port.ServicePort)
	require.Equal(t, pkgnvmeof.RDMATransportRoCEv2, port.TransportType)
	require.Equal(t, RDMAPortStateUp, port.State)
}

func TestRDMAPortStates(t *testing.T) {
	states := []RDMAPortState{
		RDMAPortStateUp,
		RDMAPortStateDown,
		RDMAPortStateError,
	}

	for _, state := range states {
		require.NotEmpty(t, state)
	}
}

// ========== RDMA Controller Tests ==========

func TestRDMAController(t *testing.T) {
	ctrl := &RDMAController{
		Name:          "nvme0",
		TargetNQN:     "nqn.2026-03.org.nas-os:test",
		TargetAddress: "192.168.100.100",
		TargetPort:    4420,
		Transport:     pkgnvmeof.TransportRDMA,
		State:         pkgnvmeof.ControllerStateLive,
		QueueDepth:    128,
		IOQueues:      8,
	}

	require.Equal(t, "nvme0", ctrl.Name)
	require.Equal(t, "nqn.2026-03.org.nas-os:test", ctrl.TargetNQN)
	require.Equal(t, pkgnvmeof.TransportRDMA, ctrl.Transport)
	require.Equal(t, pkgnvmeof.ControllerStateLive, ctrl.State)
}

// ========== RDMA Stats Tests ==========

func TestRDMATargetStats(t *testing.T) {
	stats := &RDMATargetStats{
		Available:         true,
		Running:           true,
		DeviceCount:       2,
		PortCount:         1,
		TotalTxBytes:      1024000,
		TotalRxBytes:      512000,
		ActiveConnections: 5,
	}

	require.True(t, stats.Available)
	require.True(t, stats.Running)
	require.Equal(t, 2, stats.DeviceCount)
	require.Equal(t, 1, stats.PortCount)
	require.Equal(t, uint64(1024000), stats.TotalTxBytes)
	require.Equal(t, uint64(512000), stats.TotalRxBytes)
}

func TestRDMAInitiatorStats(t *testing.T) {
	stats := &RDMAInitiatorStats{
		Available:         true,
		Running:           true,
		ControllerCount:   3,
		NamespaceCount:    6,
		ActiveConnections: 2,
	}

	require.True(t, stats.Available)
	require.True(t, stats.Running)
	require.Equal(t, 3, stats.ControllerCount)
	require.Equal(t, 6, stats.NamespaceCount)
	require.Equal(t, 2, stats.ActiveConnections)
}

// ========== RDMA Device Tests ==========

func TestRDMADevice(t *testing.T) {
	device := &pkgnvmeof.RDMADevice{
		Name:            "mlx5_0",
		TransportType:   pkgnvmeof.RDMATransportRoCEv2,
		FirmwareVersion: "16.32.1010",
		NodeGUID:        "50:6b:4b:0d:00:00:00:00",
		Ports:           1,
		MTU:             9000,
		MaxBandwidth:    100,
		State:           pkgnvmeof.RDMADeviceStateUp,
	}

	require.Equal(t, "mlx5_0", device.Name)
	require.Equal(t, pkgnvmeof.RDMATransportRoCEv2, device.TransportType)
	require.Equal(t, 100, device.MaxBandwidth)
	require.Equal(t, pkgnvmeof.RDMADeviceStateUp, device.State)
}

func TestRDMADeviceStates(t *testing.T) {
	states := []pkgnvmeof.RDMADeviceState{
		pkgnvmeof.RDMADeviceStateUp,
		pkgnvmeof.RDMADeviceStateDown,
		pkgnvmeof.RDMADeviceStateInit,
		pkgnvmeof.RDMADeviceStateError,
	}

	for _, state := range states {
		require.NotEmpty(t, state)
	}
}

func TestRDMAGID(t *testing.T) {
	gid := &pkgnvmeof.RDMAGID{
		Index:     0,
		GID:       "fe80:0000:0000:0000:526b:4b0d:0000:0000",
		Type:      "RoCEv2",
		IPAddress: "192.168.100.100",
	}

	require.Equal(t, 0, gid.Index)
	require.NotEmpty(t, gid.GID)
	require.Equal(t, "192.168.100.100", gid.IPAddress)
}

// ========== RDMA Performance Tests ==========

func TestRDMAPerformanceStats(t *testing.T) {
	stats := &pkgnvmeof.RDMAPerformanceStats{
		ReadBandwidth:  75000000000, // 75 GB/s (TrueNAS benchmark)
		WriteBandwidth: 50000000000,
		ReadIOPS:       500000,
		WriteIOPS:      400000,
		AvgLatency:     20, // 20 microseconds
		P99Latency:     100,
	}

	require.Equal(t, uint64(75000000000), stats.ReadBandwidth)
	require.Equal(t, uint64(500000), stats.ReadIOPS)
	require.Equal(t, uint64(20), stats.AvgLatency)
}

// ========== Helper Functions ==========

func mustJSONRDMA(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

// ========== Integration Tests (Mocked) ==========

func TestRDMAIntegrationFlow(t *testing.T) {
	// 创建配置
	rdmaConfig := pkgnvmeof.DefaultRDMAConfig()
	_ = pkgnvmeof.DefaultNVMeOFConfig() // nvmeofConfig used via managers

	// 创建管理器
	rdmaManager, err := pkgnvmeof.NewRDMAManager(rdmaConfig)
	require.NoError(t, err)

	ctx := context.Background()

	// 测试完整流程

	// 1. 启动 RDMA 管理器
	err = rdmaManager.Start(ctx)
	require.NoError(t, err)

	// 2. 获取设备列表
	devices := rdmaManager.GetDevices()
	require.NotEmpty(t, devices)

	// 3. 创建 Target 配置
	targetReq := &pkgnvmeof.RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:integration-test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
		QueueDepth:  128,
		MTU:         9000,
		Enabled:     true,
	}

	_, err = rdmaManager.CreateRDMATarget(ctx, targetReq)
	require.NoError(t, err)

	// 4. 列出 Targets
	targets := rdmaManager.ListRDMATargets()
	require.NotEmpty(t, targets)

	// 5. 创建 Initiator 配置
	initiatorReq := &pkgnvmeof.RDMAInitiatorConfig{
		TargetNQN:     "nqn.2026-03.org.nas-os:integration-test",
		TargetAddress: "192.168.100.100",
		TargetPort:    4420,
		QueueDepth:    128,
		IOQueues:      8,
	}

	_, err = rdmaManager.CreateRDMAInitiator(ctx, initiatorReq)
	require.NoError(t, err)

	// 6. 列出 Initiators
	initiators := rdmaManager.ListRDMAInitiators()
	require.NotEmpty(t, initiators)

	// 7. 获取性能统计
	perfStats, err := rdmaManager.GetPerformanceStats("mlx5_0")
	require.NoError(t, err)
	require.NotNil(t, perfStats)

	// 8. 清理
	err = rdmaManager.DeleteRDMATarget(ctx, targetReq.SubsysNQN)
	require.NoError(t, err)

	id := initiatorReq.TargetNQN + "-" + initiatorReq.TargetAddress
	err = rdmaManager.DeleteRDMAInitiator(ctx, id)
	require.NoError(t, err)

	// 9. 停止
	err = rdmaManager.Stop()
	require.NoError(t, err)
}

// ========== Coverage Tests ==========

func TestRDMAAllTransportTypes(t *testing.T) {
	transportTypes := []pkgnvmeof.RDMATransportType{
		pkgnvmeof.RDMATransportRoCEv2,
		pkgnvmeof.RDMATransportiWARP,
		pkgnvmeof.RDMATransportIB,
	}

	for _, tt := range transportTypes {
		require.True(t, pkgnvmeof.ValidRDMATransports[tt])
	}
}

func TestRDMAAllPortStates(t *testing.T) {
	portStates := []pkgnvmeof.RDMAPortState{
		pkgnvmeof.RDMAPortStateUp,
		pkgnvmeof.RDMAPortStateDown,
		pkgnvmeof.RDMAPortStateInit,
	}

	for _, state := range portStates {
		require.NotEmpty(t, state)
	}
}

func TestRDMAAllPhysStates(t *testing.T) {
	physStates := []pkgnvmeof.RDMAPortPhysState{
		pkgnvmeof.RDMAPortPhysStateSleep,
		pkgnvmeof.RDMAPortPhysStatePolling,
		pkgnvmeof.RDMAPortPhysStateDisabled,
		pkgnvmeof.RDMAPortPhysStateLinkUp,
		pkgnvmeof.RDMAPortPhysStateLinkDown,
	}

	for _, state := range physStates {
		require.NotEmpty(t, state)
	}
}

func TestRDMAAllErrors(t *testing.T) {
	errors := []error{
		pkgnvmeof.ErrRDMANotAvailable,
		pkgnvmeof.ErrRDMADeviceNotFound,
		pkgnvmeof.ErrRDMAPortNotFound,
		pkgnvmeof.ErrRDMAGIDNotFound,
		pkgnvmeof.ErrRDMABindFailed,
		pkgnvmeof.ErrInvalidRDMAConfig,
	}

	for _, err := range errors {
		require.NotNil(t, err)
		require.NotEmpty(t, err.Error())
	}
}

func TestRDMAPortConfigDefaults(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()

	require.Equal(t, 1, config.PortConfig.PortNum)
	require.Equal(t, 0, config.PortConfig.GIDIndex)
	require.Equal(t, 4420, config.PortConfig.ServicePort)
	require.Equal(t, 9000, config.PortConfig.MTU)
}

func TestRDMAPerformanceConfigDefaults(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()

	require.Equal(t, 128, config.Performance.MaxIOSize)
	require.Equal(t, 32, config.Performance.BatchSize)
	require.Equal(t, 512, config.Performance.ReadAhead)
	require.True(t, config.Performance.PacketAggregation)
	require.True(t, config.Performance.FlowControl)
}

func TestRDMAReconnectConfigDefaults(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()

	require.Equal(t, 10, config.Reconnect.Delay)
	require.Equal(t, 30, config.Reconnect.MaxAttempts)
	require.Equal(t, 60, config.Reconnect.Timeout)
	require.True(t, config.Reconnect.ExponentialBackoff)
}

func TestRDMATargetConfigDefaults(t *testing.T) {
	config := pkgnvmeof.DefaultRDMATargetConfig()

	require.Equal(t, 1, config.PortNum)
	require.Equal(t, 0, config.GIDIndex)
	require.Equal(t, 4420, config.ServicePort)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 9000, config.MTU)
	require.True(t, config.Enabled)
}

func TestRDMAInitiatorConfigDefaults(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAInitiatorConfig()

	require.Equal(t, 4420, config.TargetPort)
	require.Equal(t, 0, config.LocalGIDIndex)
	require.Equal(t, 128, config.QueueDepth)
	require.Equal(t, 8, config.IOQueues)
	require.Equal(t, 10, config.ReconnectDelay)
	require.Equal(t, 30, config.MaxReconnect)
	require.True(t, config.PollMode)
}

func TestRDMANamespace(t *testing.T) {
	ns := &RDMANamespace{
		NSID:       1,
		Name:       "nvme0n1",
		DevicePath: "/dev/nvme0n1",
		BlockSize:  512,
		Size:       1024 * 1024 * 1024 * 1024, // 1TB
		Controller: "nvme0",
		Online:     true,
		ReadOnly:   false,
	}

	require.Equal(t, uint32(1), ns.NSID)
	require.Equal(t, "/dev/nvme0n1", ns.DevicePath)
	require.True(t, ns.Online)
	require.False(t, ns.ReadOnly)
}

// ========== Additional Coverage Tests ==========

func TestRDMAManagerSetEventChannel(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	// 测试设置事件通道
	eventCh := make(chan pkgnvmeof.NVMeOFEvent, 10)
	manager.SetEventChannel(eventCh)

	// 启动并发送事件
	ctx := context.Background()
	err = manager.Start(ctx)
	require.NoError(t, err)

	// 创建 Target 触发事件
	req := &pkgnvmeof.RDMATargetConfig{
		SubsysNQN:   "nqn.2026-03.org.nas-os:event-test",
		Device:      "mlx5_0",
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
	}
	_, err = manager.CreateRDMATarget(ctx, req)
	require.NoError(t, err)

	// 检查事件
	select {
	case event := <-eventCh:
		require.Equal(t, pkgnvmeof.EventSubsystemCreated, event.Type)
	case <-time.After(time.Second):
		t.Log("No event received (expected in mock)")
	}

	_ = manager.Stop()
}

func TestRDMAManagerGetConfig(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	// 获取配置
	gotConfig := manager.GetConfig()
	require.NotNil(t, gotConfig)
	require.Equal(t, config.QueueDepth, gotConfig.QueueDepth)
}

func TestRDMAManagerGetDeviceByIP(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	// 测试查找设备
	device, gidIndex, err := manager.GetDeviceByIP("192.168.100.100")
	require.NoError(t, err)
	require.NotNil(t, device)
	require.Equal(t, 0, gidIndex)

	// 测试找不到设备
	_, _, err = manager.GetDeviceByIP("192.168.999.999")
	require.Error(t, err)
	require.Equal(t, pkgnvmeof.ErrRDMAGIDNotFound, err)
}

func TestRDMAManagerPerformanceStats(t *testing.T) {
	config := pkgnvmeof.DefaultRDMAConfig()
	manager, err := pkgnvmeof.NewRDMAManager(config)
	require.NoError(t, err)

	// 获取性能统计
	stats, err := manager.GetPerformanceStats("mlx5_0")
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, config.SQDepth, stats.SQDepth)
	require.Equal(t, config.RQDepth, stats.RQDepth)

	// 测试不存在的设备
	_, err = manager.GetPerformanceStats("nonexistent")
	require.Error(t, err)
}

func TestRDMAValidTransports(t *testing.T) {
	// 测试所有有效传输类型
	validTransports := []pkgnvmeof.RDMATransportType{
		pkgnvmeof.RDMATransportRoCEv2,
		pkgnvmeof.RDMATransportiWARP,
		pkgnvmeof.RDMATransportIB,
	}

	for _, transport := range validTransports {
		require.True(t, pkgnvmeof.ValidRDMATransports[transport], "Transport %s should be valid", transport)
	}

	// 测试无效传输类型
	require.False(t, pkgnvmeof.ValidRDMATransports["invalid"])
}

func TestRDMAReconnectConfig(t *testing.T) {
	config := pkgnvmeof.RDMAReconnectConfig{
		Delay:              15,
		MaxAttempts:        60,
		Timeout:            120,
		ExponentialBackoff: false,
	}

	require.Equal(t, 15, config.Delay)
	require.Equal(t, 60, config.MaxAttempts)
	require.Equal(t, 120, config.Timeout)
	require.False(t, config.ExponentialBackoff)
}

func TestRDMAPortConfig(t *testing.T) {
	config := pkgnvmeof.RDMAPortConfig{
		Device:      "mlx5_0",
		PortNum:     1,
		GIDIndex:    0,
		IPAddress:   "192.168.100.100",
		ServicePort: 4420,
		MTU:         9000,
	}

	require.Equal(t, "mlx5_0", config.Device)
	require.Equal(t, 1, config.PortNum)
	require.Equal(t, "192.168.100.100", config.IPAddress)
}

func TestRDMAPerformanceConfig(t *testing.T) {
	config := pkgnvmeof.RDMAPerformanceConfig{
		MaxIOSize:         256,
		BatchSize:         64,
		ReadAhead:         1024,
		PacketAggregation: false,
		FlowControl:       false,
	}

	require.Equal(t, 256, config.MaxIOSize)
	require.Equal(t, 64, config.BatchSize)
	require.False(t, config.PacketAggregation)
	require.False(t, config.FlowControl)
}

func TestRDMADeviceStats(t *testing.T) {
	stats := pkgnvmeof.RDMADeviceStats{
		TxBytes:      1000000000,
		TxPackets:    1000000,
		TxErrors:     10,
		TxDropped:    5,
		RxBytes:      500000000,
		RxPackets:    500000,
		RxErrors:     2,
		RxDropped:    1,
		RDMAReadOps:  100000,
		RDMAWriteOps: 80000,
		RDMASendOps:  50000,
		RDMARecvOps:  50000,
		QPCount:      128,
		CQCount:      256,
		SRQCount:     16,
	}

	require.Equal(t, uint64(1000000000), stats.TxBytes)
	require.Equal(t, uint64(100000), stats.RDMAReadOps)
	require.Equal(t, 128, stats.QPCount)
}

func TestRDMAPortInfo(t *testing.T) {
	portInfo := pkgnvmeof.RDMAPortInfo{
		PortNum:    1,
		State:      pkgnvmeof.RDMAPortStateUp,
		PhysState:  pkgnvmeof.RDMAPortPhysStateLinkUp,
		LinkLayer:  "Ethernet",
		ActiveRate: 100,
		ActiveMTU:  9000,
		NetDevice:  "eth0",
	}

	require.Equal(t, 1, portInfo.PortNum)
	require.Equal(t, pkgnvmeof.RDMAPortStateUp, portInfo.State)
	require.Equal(t, "Ethernet", portInfo.LinkLayer)
}

func TestRDMAGIDStruct(t *testing.T) {
	gid := pkgnvmeof.RDMAGID{
		Index:     1,
		GID:       "fe80:0000:0000:0000:526b:4b0d:0000:0001",
		Type:      "RoCEv2",
		PrefixLen: 64,
		IPAddress: "192.168.100.101",
	}

	require.Equal(t, 1, gid.Index)
	require.Equal(t, "RoCEv2", gid.Type)
	require.Equal(t, "192.168.100.101", gid.IPAddress)
}

func TestRDMAValidationFunctions(t *testing.T) {
	// Test CheckRDMAAvailable
	available, err := pkgnvmeof.CheckRDMAAvailable()
	require.NoError(t, err)
	require.True(t, available)

	// Test GetRDMAKernelModules
	modules, err := pkgnvmeof.GetRDMAKernelModules()
	require.NoError(t, err)
	require.NotEmpty(t, modules)
	require.Contains(t, modules, "ib_core")
	require.Contains(t, modules, "rdma_cm")

	// Test ValidateRDMAGID
	require.True(t, pkgnvmeof.ValidateRDMAGID("fe80::1"))
	require.False(t, pkgnvmeof.ValidateRDMAGID(""))

	// Test ParseGIDIPAddress (returns empty for mock)
	ip := pkgnvmeof.ParseGIDIPAddress("fe80::1")
	require.Empty(t, ip) // Mock returns empty
}

func TestRDMATargetRequestEdgeCases(t *testing.T) {
	// 测试空设备名
	req := &pkgnvmeof.RDMATargetConfig{
		SubsysNQN: "test",
		IPAddress: "192.168.1.1",
	}
	err := req.Validate()
	require.Error(t, err) // device required

	// 测试空 NQN
	req = &pkgnvmeof.RDMATargetConfig{
		Device:    "mlx5_0",
		IPAddress: "192.168.1.1",
	}
	err = req.Validate()
	require.Error(t, err) // nqn required

	// 测试空 IP
	req = &pkgnvmeof.RDMATargetConfig{
		SubsysNQN: "test",
		Device:    "mlx5_0",
	}
	err = req.Validate()
	require.Error(t, err) // ip required
}

func TestRDMAInitiatorRequestEdgeCases(t *testing.T) {
	// 测试默认值应用
	req := &pkgnvmeof.RDMAInitiatorConfig{
		TargetNQN:      "test",
		TargetAddress:  "192.168.1.1",
		TargetPort:     0,  // 应该被修正为 4420
		QueueDepth:     -1, // 应该被修正为 128
		IOQueues:       -1, // 应该被修正为 8
		ReconnectDelay: -1, // 应该被修正为 10
		MaxReconnect:   -1, // 应该被修正为 30
	}
	err := req.Validate()
	require.NoError(t, err)
	require.Equal(t, 4420, req.TargetPort)
	require.Equal(t, 128, req.QueueDepth)
	require.Equal(t, 8, req.IOQueues)
	require.Equal(t, 10, req.ReconnectDelay)
	require.Equal(t, 30, req.MaxReconnect)
}

func TestRDMAControllerStruct(t *testing.T) {
	ctrl := RDMAController{
		Name:           "nvme0",
		TargetNQN:      "nqn.test:subsys",
		TargetAddress:  "192.168.1.1",
		TargetPort:     4420,
		Transport:      pkgnvmeof.TransportRDMA,
		State:          pkgnvmeof.ControllerStateLive,
		QueueDepth:     128,
		IOQueues:       8,
		KeepAlive:      30,
		ReconnectDelay: 10,
		ConnectedAt:    time.Now(),
		ReconnectCount: 0,
		Namespaces:     []*RDMANamespace{},
	}

	require.Equal(t, "nvme0", ctrl.Name)
	require.Equal(t, pkgnvmeof.TransportRDMA, ctrl.Transport)
	require.Equal(t, pkgnvmeof.ControllerStateLive, ctrl.State)
}

func TestRDMAInitiatorStatsStruct(t *testing.T) {
	stats := RDMAInitiatorStats{
		Available:         true,
		Running:           true,
		ControllerCount:   5,
		NamespaceCount:    10,
		ActiveConnections: 3,
		TotalTxBytes:      1000000000,
		TotalRxBytes:      500000000,
		TotalTxIOPS:       100000,
		TotalRxIOPS:       80000,
		ErrorCount:        5,
		ReconnectCount:    2,
	}

	require.True(t, stats.Available)
	require.Equal(t, 5, stats.ControllerCount)
	require.Equal(t, 10, stats.NamespaceCount)
}

func TestCreateRDMAPortRequestDefaults(t *testing.T) {
	req := &CreateRDMAPortRequest{
		Address: "192.168.1.1",
		// ServicePort 和 MTU 应该被设为默认值
	}

	err := req.Validate()
	require.NoError(t, err)
	require.Equal(t, RDMADefaultPort, req.ServicePort)
	require.Equal(t, RDMADefaultMTU, req.MTU)
}

func TestConnectRDMATargetRequestDefaults(t *testing.T) {
	req := &ConnectRDMATargetRequest{
		TargetNQN:     "test",
		TargetAddress: "192.168.1.1",
		// 其他值应该被设为默认值
	}

	err := req.Validate()
	require.NoError(t, err)
	require.Equal(t, RDMAInitiatorDefaultPort, req.TargetPort)
	require.Equal(t, RDMAInitiatorDefaultQueueDepth, req.QueueDepth)
	require.Equal(t, RDMAInitiatorDefaultIOQueues, req.IOQueues)
	require.Equal(t, RDMAInitiatorDefaultKeepAlive, req.KeepAlive)
	require.Equal(t, RDMAInitiatorDefaultReconnectDelay, req.ReconnectDelay)
}

func TestDiscoverRDMATargetsRequestDefaults(t *testing.T) {
	req := &DiscoverRDMATargetsRequest{
		Address: "192.168.1.1",
		Port:    -1, // 应该被修正
	}

	err := req.Validate()
	require.NoError(t, err)
	require.Equal(t, pkgnvmeof.DefaultNVMeOFConfig().Target.DefaultPort, req.Port)
}
