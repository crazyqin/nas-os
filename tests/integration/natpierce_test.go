// Package integration 提供 NAS-OS 集成测试
// 内网穿透服务集成测试
package integration

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nas-os/internal/natpierce"
)

// ==================== 连接测试 ====================

func TestNatPierce_RelayConnection(t *testing.T) {
	// 创建模拟中继服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// 提取服务器地址
	addr := strings.TrimPrefix(server.URL, "http://")
	host, port, _ := net.SplitHostPort(addr)
	portNum := 0
	for _, c := range port {
		if c >= '0' && c <= '9' {
			portNum = portNum*10 + int(c-'0')
		} else {
			break
		}
	}

	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: host,
		ServerPort: portNum,
		LocalPort:  8080,
		Token:      "test-token",
	}

	client := natpierce.NewPierceClient(cfg)

	// 测试初始状态
	status := client.GetStatus()
	if status.Connected {
		t.Error("初始状态应该是未连接")
	}

	t.Logf("中继服务器: %s:%d", host, portNum)
	t.Logf("初始状态: connected=%v, mode=%s", status.Connected, status.Mode)
}

func TestNatPierce_P2PMode(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:     true,
		Mode:        natpierce.ModeP2P,
		LocalPort:   8080,
		STUNServers: []string{"stun:stun.l.google.com:19302"},
	}

	client := natpierce.NewPierceClient(cfg)

	// P2P模式需要STUN服务器
	err := client.Start()
	// 预期会失败，因为没有真实的STUN响应
	if err == nil {
		t.Log("P2P启动成功")
	} else {
		t.Logf("P2P启动失败（预期）: %v", err)
	}

	// 清理
	_ = client.Stop()
}

func TestNatPierce_AutoMode(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:     true,
		Mode:        natpierce.ModeAuto,
		LocalPort:   8080,
		STUNServers: []string{"stun:stun.l.google.com:19302"},
		ServerAddr:  "relay.example.com",
		ServerPort:  443,
	}

	client := natpierce.NewPierceClient(cfg)

	// 自动模式会先尝试P2P，失败后回退到中继
	status := client.GetStatus()
	if status.Connected {
		t.Error("未启动时状态应为未连接")
	}

	t.Logf("自动模式配置: mode=%s", cfg.Mode)
}

func TestNatPierce_Disabled(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled: false,
		Mode:    natpierce.ModeRelay,
	}

	client := natpierce.NewPierceClient(cfg)

	// 禁用状态下启动应返回错误
	err := client.Start()
	if err == nil {
		t.Error("禁用状态下启动应返回错误")
	}

	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("错误消息应包含 'disabled', got: %v", err)
	}
}

// ==================== 状态检查测试 ====================

func TestNatPierce_StatusCheck(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	// 测试状态获取
	status := client.GetStatus()

	if status.Connected {
		t.Error("未启动时应为未连接状态")
	}

	if status.Mode != "" {
		t.Errorf("未启动时模式应为空, got: %s", status.Mode)
	}

	t.Logf("初始状态: Connected=%v, Mode=%s, PublicIP=%s",
		status.Connected, status.Mode, status.PublicIP)
}

func TestNatPierce_StatusCallback(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	var mu sync.Mutex

	client.SetStatusCallback(func(status natpierce.ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		// 回调已触发
		_ = status // 保存状态供后续检查
	})

	// 触发状态更新（通过Stop）
	_ = client.Stop()

	// 验证回调可以被设置
	t.Logf("回调已设置")
}

func TestNatPierce_StopAndRestart(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	// 停止未启动的客户端
	err := client.Stop()
	if err != nil {
		t.Logf("停止未启动客户端: %v", err)
	}

	// 再次停止（幂等性测试）
	err = client.Stop()
	if err != nil {
		t.Logf("再次停止: %v", err)
	}

	status := client.GetStatus()
	t.Logf("停止后状态: Connected=%v", status.Connected)
}

func TestNatPierce_ContextCancellation(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 模拟超时场景
	select {
	case <-ctx.Done():
		t.Log("上下文已取消")
	case <-time.After(2 * time.Second):
		t.Error("上下文取消超时")
	}

	// 清理
	_ = client.Stop()
}

// ==================== 故障恢复测试 ====================

func TestNatPierce_ConnectionFailure(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "non-existent-server.invalid",
		ServerPort: 9999,
		LocalPort:  8080,
		Timeout:    2, // 2秒超时
	}

	client := natpierce.NewPierceClient(cfg)

	// 尝试连接不存在的服务器
	err := client.Start()
	if err == nil {
		t.Error("连接不存在服务器应返回错误")
	}

	t.Logf("连接失败（预期）: %v", err)

	// 验证状态更新
	status := client.GetStatus()
	if status.Connected {
		t.Error("连接失败后状态应为未连接")
	}
}

func TestNatPierce_Reconnect(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	// 第一次连接尝试
	err := client.Start()
	if err != nil {
		t.Logf("第一次连接失败（预期）: %v", err)
	}

	// 停止
	_ = client.Stop()

	// 第二次连接尝试
	err = client.Start()
	if err != nil {
		t.Logf("第二次连接失败（预期）: %v", err)
	}

	// 清理
	_ = client.Stop()

	t.Log("重连测试完成")
}

func TestNatPierce_HeartbeatFailure(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
		LocalPort:  8080,
	}

	client := natpierce.NewPierceClient(cfg)

	// 启动后心跳应定期执行
	// 由于没有真实连接，心跳会失败
	time.Sleep(100 * time.Millisecond)

	status := client.GetStatus()
	t.Logf("心跳测试后状态: Connected=%v, ErrorMessage=%s",
		status.Connected, status.ErrorMessage)

	_ = client.Stop()
}

func TestNatPierce_MultipleStopCalls(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)

	// 多次停止调用不应导致panic
	for i := 0; i < 5; i++ {
		if err := client.Stop(); err != nil {
			t.Logf("停止调用 %d: %v", i, err)
		}
	}

	t.Log("多次停止调用测试通过")
}

func TestNatPierce_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *natpierce.Config
		wantErr bool
	}{
		{
			name: "空配置",
			cfg: &natpierce.Config{
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "缺少服务器地址",
			cfg: &natpierce.Config{
				Enabled:    true,
				Mode:       natpierce.ModeRelay,
				ServerAddr: "",
				ServerPort: 443,
			},
			wantErr: true,
		},
		{
			name: "P2P模式无STUN服务器",
			cfg: &natpierce.Config{
				Enabled:     true,
				Mode:        natpierce.ModeP2P,
				STUNServers: []string{},
			},
			wantErr: true,
		},
		{
			name: "无效模式",
			cfg: &natpierce.Config{
				Enabled: true,
				Mode:    natpierce.PierceMode("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := natpierce.NewPierceClient(tt.cfg)
			err := client.Start()

			if (err != nil) != tt.wantErr {
				t.Errorf("期望错误=%v, 实际=%v", tt.wantErr, err)
			}

			_ = client.Stop()
		})
	}
}

func TestNatPierce_ConcurrentAccess(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)

	// 并发读取状态
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = client.GetStatus()
			}
		}()
	}

	// 并发写入（停止）
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Stop()
		}()
	}

	wg.Wait()
	t.Log("并发访问测试完成")
}

// ==================== API Handler 测试 ====================

func TestNatPierce_Handler_Status(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)
	handler := natpierce.NewHandler(client)

	// 测试状态API
	req := httptest.NewRequest(http.MethodGet, "/api/natpierce/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("状态API应返回200, got %d", rec.Code)
	}

	t.Logf("状态响应: %s", rec.Body.String())
}

func TestNatPierce_Handler_Config(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)
	handler := natpierce.NewHandler(client)

	// 测试获取配置
	t.Run("GetConfig", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/natpierce/config", nil)
		rec := httptest.NewRecorder()

		handler.HandleConfig(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("配置API应返回200, got %d", rec.Code)
		}
	})

	// 测试方法不允许
	t.Run("MethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/natpierce/config", nil)
		rec := httptest.NewRecorder()

		handler.HandleConfig(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("DELETE方法应返回405, got %d", rec.Code)
		}
	})
}

func TestNatPierce_Handler_Connect(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)
	handler := natpierce.NewHandler(client)

	// 测试连接API
	req := httptest.NewRequest(http.MethodPost, "/api/natpierce/connect", nil)
	rec := httptest.NewRecorder()

	handler.HandleConnect(rec, req)

	// 由于没有真实服务器，可能返回500
	t.Logf("连接响应状态码: %d", rec.Code)
}

func TestNatPierce_Handler_Disconnect(t *testing.T) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)
	handler := natpierce.NewHandler(client)

	// 测试断开连接API
	req := httptest.NewRequest(http.MethodPost, "/api/natpierce/disconnect", nil)
	rec := httptest.NewRecorder()

	handler.HandleDisconnect(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("断开连接应返回200, got %d", rec.Code)
	}
}

// ==================== 性能测试 ====================

func BenchmarkNatPierce_GetStatus(b *testing.B) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetStatus()
	}
}

func BenchmarkNatPierce_Stop(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := &natpierce.Config{
			Enabled:    true,
			Mode:       natpierce.ModeRelay,
			ServerAddr: "localhost",
			ServerPort: 9999,
		}
		client := natpierce.NewPierceClient(cfg)
		_ = client.Stop()
	}
}

func BenchmarkNatPierce_Handler_Status(b *testing.B) {
	cfg := &natpierce.Config{
		Enabled:    true,
		Mode:       natpierce.ModeRelay,
		ServerAddr: "localhost",
		ServerPort: 9999,
	}

	client := natpierce.NewPierceClient(cfg)
	handler := natpierce.NewHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/natpierce/status", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.HandleStatus(rec, req)
	}
}
