// Package session 提供SMB/NFS会话监控和管理功能
package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-config.json")
	require.NoError(t, err)
	require.NotNil(t, manager)

	assert.NotNil(t, manager.sessions)
	assert.NotNil(t, manager.config)
	assert.True(t, manager.config.Enabled)
}

func TestManager_AddSession(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-add.json")
	require.NoError(t, err)

	session := &Session{
		ID:          "test-1",
		Type:        SessionTypeSMB,
		User:        "testuser",
		ClientIP:    "192.168.1.100",
		ShareName:   "share1",
		Status:      StatusActive,
		ConnectedAt: time.Now(),
	}

	err = manager.AddSession(session)
	require.NoError(t, err)

	// 验证会话已添加
	s, err := manager.GetSession("test-1")
	require.NoError(t, err)
	assert.Equal(t, "testuser", s.User)
	assert.Equal(t, SessionTypeSMB, s.Type)

	// 添加重复会话应该覆盖
	session2 := &Session{
		ID:       "test-1",
		Type:     SessionTypeSMB,
		User:     "updated",
		ClientIP: "192.168.1.100",
	}
	err = manager.AddSession(session2)
	require.NoError(t, err)

	s, _ = manager.GetSession("test-1")
	assert.Equal(t, "updated", s.User)

	// 测试无效输入
	err = manager.AddSession(nil)
	assert.Error(t, err)

	err = manager.AddSession(&Session{})
	assert.Error(t, err)
}

func TestManager_UpdateSession(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-update.json")
	require.NoError(t, err)

	// 先添加会话
	session := &Session{
		ID:       "test-2",
		Type:     SessionTypeSMB,
		User:     "testuser",
		ClientIP: "192.168.1.100",
	}
	err = manager.AddSession(session)
	require.NoError(t, err)

	// 更新会话
	updates := &Session{
		BytesRead:    1000,
		BytesWritten: 500,
		FilesOpen:    3,
	}
	err = manager.UpdateSession("test-2", updates)
	require.NoError(t, err)

	s, _ := manager.GetSession("test-2")
	assert.Equal(t, int64(1000), s.BytesRead)
	assert.Equal(t, int64(500), s.BytesWritten)
	assert.Equal(t, 3, s.FilesOpen)

	// 更新不存在的会话
	err = manager.UpdateSession("nonexistent", updates)
	assert.Error(t, err)
}

func TestManager_RemoveSession(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-remove.json")
	require.NoError(t, err)

	session := &Session{
		ID:       "test-3",
		Type:     SessionTypeSMB,
		User:     "testuser",
		ClientIP: "192.168.1.100",
	}
	err = manager.AddSession(session)
	require.NoError(t, err)

	// 移除会话
	err = manager.RemoveSession("test-3")
	require.NoError(t, err)

	// 验证会话已移除
	_, err = manager.GetSession("test-3")
	assert.Error(t, err)

	// 移除不存在的会话
	err = manager.RemoveSession("nonexistent")
	assert.Error(t, err)
}

func TestManager_ListSessions(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-list.json")
	require.NoError(t, err)

	// 添加多个会话
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:       "test-" + string(rune('0'+i)),
			Type:     SessionTypeSMB,
			User:     "user" + string(rune('0'+i%2)),
			ClientIP: "192.168.1.100",
		}
		_ = manager.AddSession(session)
	}

	// 列出所有会话
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 5)

	// 按类型列出
	smbSessions := manager.ListSessionsByType(SessionTypeSMB)
	assert.Len(t, smbSessions, 5)

	// 按用户列出
	user0Sessions := manager.ListSessionsByUser("user0")
	assert.Len(t, user0Sessions, 3) // 0, 2, 4
}

func TestManager_KickSession(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-kick.json")
	require.NoError(t, err)

	session := &Session{
		ID:       "test-kick",
		Type:     SessionTypeSMB,
		User:     "testuser",
		ClientIP: "192.168.1.100",
	}
	err = manager.AddSession(session)
	require.NoError(t, err)

	// 断开会话
	err = manager.KickSession("test-kick", "测试断开")
	require.NoError(t, err)

	// 验证会话已移除
	_, err = manager.GetSession("test-kick")
	assert.Error(t, err)

	// 验证事件记录
	events := manager.GetEvents(10)
	assert.NotEmpty(t, events)
	assert.Equal(t, "kick", events[len(events)-1].Type)

	// 断开不存在的会话
	err = manager.KickSession("nonexistent", "测试")
	assert.Error(t, err)
}

func TestManager_KickSessionsByUser(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-kick-user.json")
	require.NoError(t, err)

	// 添加多个用户的会话
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:       "test-user-" + string(rune('0'+i)),
			Type:     SessionTypeSMB,
			User:     "targetuser",
			ClientIP: "192.168.1." + string(rune('0'+i)),
		}
		_ = manager.AddSession(session)
	}

	// 添加其他用户的会话
	otherSession := &Session{
		ID:       "test-other",
		Type:     SessionTypeSMB,
		User:     "otheruser",
		ClientIP: "192.168.1.200",
	}
	_ = manager.AddSession(otherSession)

	// 断开用户所有会话
	count, err := manager.KickSessionsByUser("targetuser", "测试批量断开")
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	// 验证只剩其他用户的会话
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 1)
	assert.Equal(t, "otheruser", sessions[0].User)
}

func TestManager_GetStats(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-stats.json")
	require.NoError(t, err)

	// 添加不同类型的会话
	smbSession := &Session{
		ID:           "smb-1",
		Type:         SessionTypeSMB,
		User:         "user1",
		ClientIP:     "192.168.1.100",
		BytesRead:    1000,
		BytesWritten: 500,
	}
	_ = manager.AddSession(smbSession)

	nfsSession := &Session{
		ID:           "nfs-1",
		Type:         SessionTypeNFS,
		User:         "nfs-client",
		ClientIP:     "192.168.1.101",
		BytesRead:    2000,
		BytesWritten: 1000,
	}
	_ = manager.AddSession(nfsSession)

	stats := manager.GetStats()
	assert.Equal(t, 2, stats.TotalSessions)
	assert.Equal(t, 1, stats.SMBSessions)
	assert.Equal(t, 1, stats.NFSSessions)
	assert.Equal(t, int64(3000), stats.TotalBytesRead)
	assert.Equal(t, int64(1500), stats.TotalBytesWritten)
	assert.Equal(t, 2, stats.UniqueUsers)
	assert.Equal(t, 2, stats.UniqueClients)
}

func TestManager_SyncSessions(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-sync.json")
	require.NoError(t, err)

	// 添加初始会话
	oldSession := &Session{
		ID:       "old-1",
		Type:     SessionTypeSMB,
		User:     "olduser",
		ClientIP: "192.168.1.100",
	}
	_ = manager.AddSession(oldSession)

	// 同步新会话列表
	newSessions := []*Session{
		{
			ID:       "new-1",
			Type:     SessionTypeSMB,
			User:     "newuser1",
			ClientIP: "192.168.1.200",
		},
		{
			ID:       "new-2",
			Type:     SessionTypeNFS,
			User:     "newuser2",
			ClientIP: "192.168.1.201",
		},
	}

	manager.SyncSessions(newSessions)

	// 验证旧会话已移除
	_, err = manager.GetSession("old-1")
	assert.Error(t, err)

	// 验证新会话已添加
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 2)
}

func TestManager_Cleanup(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-cleanup.json")
	require.NoError(t, err)

	// 设置较短的过期时间
	manager.UpdateConfig(&Config{
		Enabled:         true,
		RefreshInterval: 10 * time.Second,
		IdleTimeout:     1 * time.Second,
		StaleTimeout:    2 * time.Second,
	})

	// 添加会话
	activeSession := &Session{
		ID:           "active",
		Type:         SessionTypeSMB,
		User:         "user",
		ClientIP:     "192.168.1.100",
		LastActiveAt: time.Now(),
	}
	_ = manager.AddSession(activeSession)

	staleSession := &Session{
		ID:           "stale",
		Type:         SessionTypeSMB,
		User:         "user",
		ClientIP:     "192.168.1.101",
		LastActiveAt: time.Now().Add(-10 * time.Second), // 很久之前
	}
	_ = manager.AddSession(staleSession)

	// 标记空闲
	idle := manager.MarkIdle()
	assert.Equal(t, 1, idle) // 只有 staleSession 会被标记为空闲

	// 清理过期
	time.Sleep(2 * time.Second) // 等待超过 staleTimeout
	cleaned := manager.CleanupStale()
	assert.GreaterOrEqual(t, cleaned, 1) // 至少有一个被清理
}

// ========== Monitor 测试 ==========

type mockSMBProvider struct {
	connections []*SMBConnection
	err         error
}

func (m *mockSMBProvider) Connections() ([]*SMBConnection, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.connections, nil
}

func (m *mockSMBProvider) KillConnection(pid int) error {
	return nil
}

type mockNFSProvider struct {
	clients []*NFSClient
	err     error
}

func (m *mockNFSProvider) GetClients() ([]*NFSClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clients, nil
}

func (m *mockNFSProvider) KillClient(clientID string) error {
	return nil
}

func TestMonitor_CollectSessions(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-monitor.json")
	require.NoError(t, err)

	monitor := NewMonitor(manager, nil)

	// 设置模拟提供者
	smbProvider := &mockSMBProvider{
		connections: []*SMBConnection{
			{
				PID:         1234,
				Username:    "testuser",
				ShareName:   "share1",
				ClientIP:    "192.168.1.100",
				Protocol:    "SMB3",
				ConnectedAt: time.Now(),
			},
		},
	}
	monitor.SetSMBProvider(smbProvider)

	nfsProvider := &mockNFSProvider{
		clients: []*NFSClient{
			{
				ID:          "1",
				ClientIP:    "192.168.1.200",
				SharePath:   "/export/data",
				ConnectedAt: time.Now(),
			},
		},
	}
	monitor.SetNFSProvider(nfsProvider)

	// 收集SMB会话
	smbSessions, err := monitor.collectSMBSessions()
	require.NoError(t, err)
	assert.Len(t, smbSessions, 1)
	assert.Equal(t, "testuser", smbSessions[0].User)
	assert.Equal(t, SessionTypeSMB, smbSessions[0].Type)

	// 收集NFS会话
	nfsSessions, err := monitor.collectNFSSessions()
	require.NoError(t, err)
	assert.Len(t, nfsSessions, 1)
	assert.Equal(t, SessionTypeNFS, nfsSessions[0].Type)
	assert.Equal(t, "192.168.1.200", nfsSessions[0].ClientIP)
}

func TestMonitor_StartStop(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-monitor-run.json")
	require.NoError(t, err)

	monitor := NewMonitor(manager, nil)
	monitor.SetPollInterval(100 * time.Millisecond)

	ctx := context.Background()

	// 启动监控
	err = monitor.Start(ctx)
	require.NoError(t, err)

	// 重复启动应该失败
	err = monitor.Start(ctx)
	assert.Error(t, err)

	// 等待一段时间
	time.Sleep(300 * time.Millisecond)

	// 停止监控
	monitor.Stop()

	// 验证状态
	status := monitor.GetStatus()
	assert.False(t, status["running"].(bool))
}

func TestMonitor_ForceRefresh(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-monitor-refresh.json")
	require.NoError(t, err)

	monitor := NewMonitor(manager, nil)

	// 设置模拟提供者
	smbProvider := &mockSMBProvider{
		connections: []*SMBConnection{
			{
				PID:         1234,
				Username:    "testuser",
				ShareName:   "share1",
				ClientIP:    "192.168.1.100",
				ConnectedAt: time.Now(),
			},
		},
	}
	monitor.SetSMBProvider(smbProvider)
	monitor.SetNFSProvider(&mockNFSProvider{})

	// 强制刷新
	err = monitor.ForceRefresh()
	require.NoError(t, err)

	// 验证会话已更新
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 1)
}

// ========== Config 测试 ==========

func TestConfig_SaveLoad(t *testing.T) {
	configPath := "/tmp/test-session-config-save.json"

	// 创建管理器并保存配置
	manager1, err := NewManager(configPath)
	require.NoError(t, err)

	config := &Config{
		Enabled:          true,
		RefreshInterval:  30 * time.Second,
		IdleTimeout:      1 * time.Hour,
		StaleTimeout:     2 * time.Hour,
		MaxSessions:      5000,
		HistoryRetention: 48 * time.Hour,
	}

	err = manager1.UpdateConfig(config)
	require.NoError(t, err)

	// 创建新的管理器加载配置
	manager2, err := NewManager(configPath)
	require.NoError(t, err)

	loadedConfig := manager2.GetConfig()
	assert.Equal(t, 30*time.Second, loadedConfig.RefreshInterval)
	assert.Equal(t, 1*time.Hour, loadedConfig.IdleTimeout)
	assert.Equal(t, 5000, loadedConfig.MaxSessions)
}

// ========== 事件测试 ==========

func TestManager_Events(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-events.json")
	require.NoError(t, err)

	var receivedEvent SessionEvent
	manager.SetEventHandler(func(e SessionEvent) {
		receivedEvent = e
	})

	// 添加会话应该触发事件
	session := &Session{
		ID:       "event-test",
		Type:     SessionTypeSMB,
		User:     "testuser",
		ClientIP: "192.168.1.100",
	}
	err = manager.AddSession(session)
	require.NoError(t, err)

	// 等待事件处理器
	time.Sleep(100 * time.Millisecond)

	// 验证事件
	assert.Equal(t, "connect", receivedEvent.Type)
	assert.Equal(t, "event-test", receivedEvent.SessionID)

	// 断开会话应该触发事件
	err = manager.KickSession("event-test", "测试断开")
	require.NoError(t, err)

	events := manager.GetEvents(10)
	assert.GreaterOrEqual(t, len(events), 2)
}

// ========== 并发测试 ==========

func TestManager_Concurrent(t *testing.T) {
	manager, err := NewManager("/tmp/test-session-concurrent.json")
	require.NoError(t, err)

	// 并发添加会话
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			session := &Session{
				ID:       "concurrent-" + string(rune(n)),
				Type:     SessionTypeSMB,
				User:     "user",
				ClientIP: "192.168.1.100",
			}
			_ = manager.AddSession(session)
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证会话数量
	sessions := manager.ListSessions()
	assert.Len(t, sessions, 100)
}

// ========== Benchmark ==========

func BenchmarkManager_AddSession(b *testing.B) {
	manager, _ := NewManager("/tmp/bench-session-add.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session := &Session{
			ID:       "bench-" + string(rune(i)),
			Type:     SessionTypeSMB,
			User:     "benchuser",
			ClientIP: "192.168.1.100",
		}
		_ = manager.AddSession(session)
	}
}

func BenchmarkManager_GetStats(b *testing.B) {
	manager, _ := NewManager("/tmp/bench-session-stats.json")

	// 添加大量会话
	for i := 0; i < 1000; i++ {
		session := &Session{
			ID:       "bench-" + string(rune(i)),
			Type:     SessionTypeSMB,
			User:     "user" + string(rune(i%10)),
			ClientIP: "192.168.1." + string(rune(i%255)),
		}
		_ = manager.AddSession(session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetStats()
	}
}

func BenchmarkManager_ListSessions(b *testing.B) {
	manager, _ := NewManager("/tmp/bench-session-list.json")

	// 添加大量会话
	for i := 0; i < 1000; i++ {
		session := &Session{
			ID:       "bench-" + string(rune(i)),
			Type:     SessionTypeSMB,
			User:     "user",
			ClientIP: "192.168.1.100",
		}
		_ = manager.AddSession(session)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.ListSessions()
	}
}
