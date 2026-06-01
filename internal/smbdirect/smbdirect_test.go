package smbdirect

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name  string
		check func(*Config)
	}{
		{
			name: "默认端口为5445",
			check: func(c *Config) {
				if c.ListenAddr != "0.0.0.0:5445" {
					t.Errorf("期望监听地址 0.0.0.0:5445, 实际 %s", c.ListenAddr)
				}
			},
		},
		{
			name: "默认最大连接数为1000",
			check: func(c *Config) {
				if c.MaxConnections != 1000 {
					t.Errorf("期望MaxConnections 1000, 实际 %d", c.MaxConnections)
				}
			},
		},
		{
			name: "默认传输类型为RoCEv2",
			check: func(c *Config) {
				if c.Transport != TransportRoCEv2 {
					t.Errorf("期望传输类型 %s, 实际 %s", TransportRoCEv2, c.Transport)
				}
			},
		},
		{
			name: "默认启用TCP降级",
			check: func(c *Config) {
				if !c.FallbackToTCP {
					t.Error("期望FallbackToTCP为true")
				}
			},
		},
		{
			name: "默认MTU为4096",
			check: func(c *Config) {
				if c.MTU != 4096 {
					t.Errorf("期望MTU 4096, 实际 %d", c.MTU)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.check(cfg)
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{name: "nil配置使用默认值", config: nil},
		{name: "自定义配置", config: &Config{
			Enabled:        true,
			ListenAddr:     "0.0.0.0:9999",
			MaxConnections: 100,
			MaxQueuePairs:  32,
			QPType:         QPTypeRC,
			Transport:      TransportTCP,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := New(tt.config)
			if mgr == nil {
				t.Fatal("New返回nil")
			}
			if mgr.connections == nil {
				t.Error("connections未初始化")
			}
			if mgr.queuePairs == nil {
				t.Error("queuePairs未初始化")
			}
			if mgr.memoryRegions == nil {
				t.Error("memoryRegions未初始化")
			}
			if mgr.stats == nil {
				t.Error("stats未初始化")
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	mgr := New(nil)

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start失败: %v", err)
	}

	// 重复Start应报错
	if err := mgr.Start(); err == nil {
		t.Error("重复Start应返回错误")
	}

	mgr.Stop()

	// 等待goroutine退出
	time.Sleep(200 * time.Millisecond)
}

func TestGetStatus(t *testing.T) {
	mgr := New(nil)

	// 启动前状态
	status := mgr.GetStatus()
	if status == nil {
		t.Fatal("GetStatus返回nil")
	}
	if status.State != "stopped" {
		t.Errorf("启动前状态应为stopped, 实际 %s", status.State)
	}

	// 启动后状态
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start失败: %v", err)
	}
	defer mgr.Stop()

	status = mgr.GetStatus()
	if status == nil {
		t.Fatal("启动后GetStatus返回nil")
	}
	if status.State != "running" {
		t.Errorf("运行后状态应为running, 实际 %s", status.State)
	}
	if status.Transport != TransportRoCEv2 {
		t.Errorf("传输类型应为roce_v2, 实际 %s", status.Transport)
	}
}

func TestCreateConnectionWhenStopped(t *testing.T) {
	mgr := New(nil)

	_, err := mgr.CreateConnection("127.0.0.1:1000", "127.0.0.1:2000")
	if err == nil {
		t.Error("未启动时创建连接应返回错误")
	}
}

func TestGetStats(t *testing.T) {
	mgr := New(nil)

	stats := mgr.GetStats()
	if stats == nil {
		t.Fatal("GetStats返回nil")
	}
	if stats.TotalConnections != 0 {
		t.Errorf("初始连接数应为0, 实际 %d", stats.TotalConnections)
	}
}

func TestGetConnections(t *testing.T) {
	mgr := New(nil)

	conns := mgr.GetConnections()
	if len(conns) != 0 {
		t.Errorf("初始连接列表应为空, 实际 %d", len(conns))
	}
}

func TestGetQueuePairs(t *testing.T) {
	mgr := New(&Config{
		Enabled:       true,
		MaxQueuePairs: 8,
		QPType:        QPTypeRC,
		SendQueueSize: 256,
		RecvQueueSize: 256,
	})

	// 启动后获取队列对
	mgr.Start()
	defer mgr.Stop()

	qps := mgr.GetQueuePairs()
	if qps == nil {
		t.Error("GetQueuePairs返回nil")
	}
}

func TestGetMemoryRegions(t *testing.T) {
	mgr := New(nil)

	mrs := mgr.GetMemoryRegions()
	if len(mrs) != 0 {
		t.Errorf("初始内存区域应为空, 实际 %d", len(mrs))
	}
}

func TestCreateQueuePair(t *testing.T) {
	mgr := New(&Config{
		Enabled:       true,
		MaxQueuePairs: 4,
		QPType:        QPTypeRC,
		SendQueueSize: 512,
		RecvQueueSize: 512,
	})

	qp, err := mgr.CreateQueuePair()
	if err != nil {
		t.Fatalf("CreateQueuePair失败: %v", err)
	}
	if qp.State != QPStateInit {
		t.Errorf("初始QP状态应为init, 实际 %s", qp.State)
	}
	if qp.Type != QPTypeRC {
		t.Errorf("QP类型应为rc, 实际 %s", qp.Type)
	}
}

func TestRegisterMemory(t *testing.T) {
	mgr := New(&Config{
		Enabled:   true,
		MaxMRSize: 1 << 30,
	})

	tests := []struct {
		name      string
		length    int
		wantError bool
	}{
		{name: "有效大小", length: 4096, wantError: false},
		{name: "零大小", length: 0, wantError: true},
		{name: "负大小", length: -1, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, err := mgr.RegisterMemory(0, tt.length, MRAccessLocalWrite)
			if (err != nil) != tt.wantError {
				t.Errorf("期望错误=%v, 实际=%v", tt.wantError, err)
			}
			if !tt.wantError && mr == nil {
				t.Error("成功注册时不应返回nil")
			}
		})
	}
}

func TestGetConnectionNotFound(t *testing.T) {
	mgr := New(nil)

	_, err := mgr.GetConnection("nonexistent")
	if err == nil {
		t.Error("获取不存在的连接应返回错误")
	}
}

func TestCloseConnectionNotFound(t *testing.T) {
	mgr := New(nil)

	err := mgr.CloseConnection("nonexistent")
	if err == nil {
		t.Error("关闭不存在的连接应返回错误")
	}
}

func TestDefaultConfigQPType(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.QPType != QPTypeRC {
		t.Errorf("默认QP类型应为rc, 实际 %s", cfg.QPType)
	}
	if cfg.SendQueueSize != 1024 {
		t.Errorf("默认发送队列大小应为1024, 实际 %d", cfg.SendQueueSize)
	}
	if cfg.RecvQueueSize != 1024 {
		t.Errorf("默认接收队列大小应为1024, 实际 %d", cfg.RecvQueueSize)
	}
}

func TestNewWithNilConfigPortsInitialized(t *testing.T) {
	mgr := New(nil)
	// 应有模拟端口
	if len(mgr.ports) == 0 {
		t.Error("应至少有1个模拟RDMA端口")
	}
}
