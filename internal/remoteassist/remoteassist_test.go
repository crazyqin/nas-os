// remoteassist_test.go - 完整单元测试
package remoteassist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "0.0.0.0", cfg.BindAddress)
	assert.Equal(t, 8443, cfg.BindPort)
	assert.Equal(t, 100, cfg.MaxSessions)
	assert.Equal(t, 86400, cfg.MaxDuration)
	assert.NotNil(t, cfg.Recording)
	assert.NotNil(t, cfg.Security)
	assert.NotNil(t, cfg.RateLimit)
	assert.NotNil(t, cfg.Storage)
}

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxSessions = 10

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	require.NotNil(t, mgr)

	defer mgr.Close()

	stats := mgr.GetStats()
	assert.Equal(t, 0, stats.TotalSessions)
	assert.Equal(t, 0, stats.ActiveSessions)
}

func TestManagerCreateSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxSessions = 5

	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	req := &AssistRequest{
		HostID:     "host_1",
		GuestID:    "guest_1",
		Type:       AssistTypeScreen,
		Permission: PermissionView,
		ExpiresIn:  3600,
	}

	session, err := mgr.CreateSession(req)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.NotEmpty(t, session.ID)
	assert.Equal(t, StatusPending, session.Status)
	assert.Equal(t, AssistTypeScreen, session.Type)
	assert.Equal(t, PermissionView, session.Permission)
}

func TestManagerSessionLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	req := &AssistRequest{
		HostID:     "host_1",
		GuestID:    "guest_1",
		Type:       AssistTypeFull,
		Permission: PermissionControl,
		ExpiresIn:  3600,
	}

	session, err := mgr.CreateSession(req)
	require.NoError(t, err)

	// 激活会话
	err = mgr.ActivateSession(session.ID)
	require.NoError(t, err)

	updatedSession, _ := mgr.GetSession(session.ID)
	assert.Equal(t, StatusActive, updatedSession.Status)

	// 暂停会话
	err = mgr.PauseSession(session.ID)
	require.NoError(t, err)

	updatedSession, _ = mgr.GetSession(session.ID)
	assert.Equal(t, StatusPaused, updatedSession.Status)

	// 恢复会话
	err = mgr.ResumeSession(session.ID)
	require.NoError(t, err)

	updatedSession, _ = mgr.GetSession(session.ID)
	assert.Equal(t, StatusActive, updatedSession.Status)

	// 结束会话
	err = mgr.EndSession(session.ID)
	require.NoError(t, err)

	updatedSession, _ = mgr.GetSession(session.ID)
	assert.Equal(t, StatusCompleted, updatedSession.Status)
}

func TestManagerListSessions(t *testing.T) {
	cfg := DefaultConfig()
	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	// 创建多个会话
	for i := 0; i < 3; i++ {
		req := &AssistRequest{
			HostID:     "host_1",
			GuestID:    "guest_1",
			Type:       AssistTypeScreen,
			Permission: PermissionView,
			ExpiresIn:  3600,
		}
		mgr.CreateSession(req)
	}

	sessions := mgr.ListSessions("", "")
	assert.Equal(t, 3, len(sessions))

	sessions = mgr.ListSessions("host_1", "")
	assert.Equal(t, 3, len(sessions))

	sessions = mgr.ListSessions("", StatusPending)
	assert.Equal(t, 3, len(sessions))
}

func TestManagerDeleteSession(t *testing.T) {
	cfg := DefaultConfig()
	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	req := &AssistRequest{
		HostID:     "host_1",
		GuestID:    "guest_1",
		Type:       AssistTypeScreen,
		Permission: PermissionView,
		ExpiresIn:  3600,
	}

	session, _ := mgr.CreateSession(req)

	err = mgr.DeleteSession(session.ID)
	require.NoError(t, err)

	_, err = mgr.GetSession(session.ID)
	assert.Error(t, err)
}

func TestScreenEngineStartStop(t *testing.T) {
	engine := NewScreenEngine()

	options := &ScreenShareOptions{
		Width:   1920,
		Height:  1080,
		FPS:     30,
		Quality: 80,
		Bitrate: 4000,
		Codec:   "h264",
	}

	share, err := engine.StartSharing("session_1", options)
	require.NoError(t, err)
	require.NotNil(t, share)

	assert.Equal(t, 1920, share.Width)
	assert.Equal(t, 1080, share.Height)
	assert.Equal(t, "active", share.Status)

	// 获取共享
	got, err := engine.GetSharing("session_1")
	require.NoError(t, err)
	assert.Equal(t, share.ID, got.ID)

	// 停止共享
	err = engine.StopSharing("session_1")
	require.NoError(t, err)

	_, err = engine.GetSharing("session_1")
	assert.Error(t, err)
}

func TestScreenEngineSendFrame(t *testing.T) {
	engine := NewScreenEngine()

	options := &ScreenShareOptions{
		Width:  1920,
		Height: 1080,
		FPS:    30,
	}

	engine.StartSharing("session_1", options)

	frame := &ScreenFrame{
		ID:        "frame_1",
		SessionID: "session_1",
		Sequence:  1,
		Data:      []byte("frame_data"),
		Width:     1920,
		Height:    1080,
		Format:    "h264",
		Timestamp: time.Now().UnixMilli(),
		Size:      10,
	}

	err := engine.SendFrame("session_1", frame)
	require.NoError(t, err)

	// 获取帧
	got, err := engine.GetFrame("session_1")
	require.NoError(t, err)
	assert.Equal(t, "frame_1", got.ID)
}

func TestScreenEngineCursor(t *testing.T) {
	engine := NewScreenEngine()

	options := &ScreenShareOptions{
		Width:  1920,
		Height: 1080,
	}

	engine.StartSharing("session_1", options)

	pos := &CursorPosition{
		X:    100,
		Y:    200,
		Type: "normal",
	}

	err := engine.UpdateCursor("session_1", pos)
	require.NoError(t, err)

	share, _ := engine.GetSharing("session_1")
	assert.Equal(t, 100, share.Cursor.X)
	assert.Equal(t, 200, share.Cursor.Y)
}

func TestTerminalManagerCreateClose(t *testing.T) {
	mgr := NewTerminalManager()

	options := &TerminalOptions{
		Shell:      "/bin/bash",
		Rows:       24,
		Cols:       80,
		WorkingDir: "/home",
		Env:        map[string]string{"TERM": "xterm"},
	}

	term, err := mgr.CreateTerminal("session_1", options)
	require.NoError(t, err)
	require.NotNil(t, term)

	assert.Equal(t, "/bin/bash", term.Shell)
	assert.Equal(t, 24, term.Rows)
	assert.Equal(t, 80, term.Cols)
	assert.Equal(t, "active", term.Status)

	err = mgr.CloseTerminal("session_1")
	require.NoError(t, err)

	_, err = mgr.GetTerminal("session_1")
	assert.Error(t, err)
}

func TestTerminalManagerExecuteCommand(t *testing.T) {
	mgr := NewTerminalManager()

	options := &TerminalOptions{
		Shell: "/bin/bash",
		Rows:  24,
		Cols:  80,
	}

	mgr.CreateTerminal("session_1", options)

	cmd, err := mgr.ExecuteCommand("session_1", "echo hello")
	require.NoError(t, err)
	require.NotNil(t, cmd)

	assert.Equal(t, "echo hello", cmd.Command)
	assert.Equal(t, 0, cmd.ExitCode)
	assert.NotEmpty(t, cmd.Output)
}

func TestTerminalManagerCommandHistory(t *testing.T) {
	mgr := NewTerminalManager()

	options := &TerminalOptions{
		Shell: "/bin/bash",
		Rows:  24,
		Cols:  80,
	}

	mgr.CreateTerminal("session_1", options)

	// 执行多个命令
	mgr.ExecuteCommand("session_1", "echo 1")
	mgr.ExecuteCommand("session_1", "echo 2")
	mgr.ExecuteCommand("session_1", "echo 3")

	history, err := mgr.GetCommandHistory("session_1", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(history))
	assert.Equal(t, "echo 2", history[0].Command)
	assert.Equal(t, "echo 3", history[1].Command)
}

func TestTerminalManagerResize(t *testing.T) {
	mgr := NewTerminalManager()

	options := &TerminalOptions{
		Shell: "/bin/bash",
		Rows:  24,
		Cols:  80,
	}

	mgr.CreateTerminal("session_1", options)

	err := mgr.ResizeTerminal("session_1", 50, 120)
	require.NoError(t, err)

	term, _ := mgr.GetTerminal("session_1")
	assert.Equal(t, 50, term.Rows)
	assert.Equal(t, 120, term.Cols)
}

func TestFileTransferManagerUpload(t *testing.T) {
	mgr := NewFileTransferManager()

	transfer, err := mgr.StartUpload("session_1", "test.pdf", 1024*1024)
	require.NoError(t, err)
	require.NotNil(t, transfer)

	assert.Equal(t, "upload", transfer.Direction)
	assert.Equal(t, "test.pdf", transfer.FileName)
	assert.Equal(t, int64(1024*1024), transfer.FileSize)
	assert.Equal(t, "pending", transfer.Status)

	// 更新进度
	err = mgr.UpdateProgress(transfer.ID, 512*1024)
	require.NoError(t, err)

	got, _ := mgr.GetTransfer(transfer.ID)
	assert.Equal(t, "transferring", got.Status)
	assert.True(t, got.Progress > 0)

	// 完成传输
	err = mgr.CompleteTransfer(transfer.ID, "abc123hash")
	require.NoError(t, err)

	got, _ = mgr.GetTransfer(transfer.ID)
	assert.Equal(t, "completed", got.Status)
	assert.Equal(t, 100.0, got.Progress)
}

func TestFileTransferManagerCancel(t *testing.T) {
	mgr := NewFileTransferManager()

	transfer, _ := mgr.StartUpload("session_1", "test.pdf", 1024)

	err := mgr.CancelTransfer(transfer.ID)
	require.NoError(t, err)

	got, _ := mgr.GetTransfer(transfer.ID)
	assert.Equal(t, "cancelled", got.Status)
}

func TestFileTransferManagerListTransfers(t *testing.T) {
	mgr := NewFileTransferManager()

	mgr.StartUpload("session_1", "file1.pdf", 1024)
	mgr.StartUpload("session_1", "file2.pdf", 2048)
	mgr.StartUpload("session_2", "file3.pdf", 512)

	transfers := mgr.ListTransfers("session_1")
	assert.Equal(t, 2, len(transfers))

	transfers = mgr.ListTransfers("")
	assert.Equal(t, 3, len(transfers))
}

func TestFileTransferManagerStats(t *testing.T) {
	mgr := NewFileTransferManager()

	mgr.StartUpload("session_1", "file1.pdf", 1024)
	mgr.StartUpload("session_1", "file2.pdf", 2048)

	stats := mgr.GetTransferStats()
	assert.Equal(t, 2, stats["total_transfers"])
}

func TestAuthManagerAuthenticate(t *testing.T) {
	mgr := NewAuthManager(nil)

	// 注册用户
	cred, err := mgr.RegisterUser("testuser", "password123", []string{"view", "control"})
	require.NoError(t, err)
	require.NotNil(t, cred)

	// 认证
	result, err := mgr.Authenticate("testuser", "password123", "192.168.1.1")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.Token)
	assert.NotEmpty(t, result.RefreshToken)
}

func TestAuthManagerTokenValidation(t *testing.T) {
	mgr := NewAuthManager(nil)

	mgr.RegisterUser("testuser", "password123", []string{"view"})
	cred, _ := mgr.Authenticate("testuser", "password123", "192.168.1.1")

	// 验证令牌
	validated, err := mgr.ValidateToken(cred.Token)
	require.NoError(t, err)
	assert.Equal(t, "testuser", validated.Username)

	// 无效令牌
	_, err = mgr.ValidateToken("invalid_token")
	assert.Error(t, err)
}

func TestAuthManagerTokenRefresh(t *testing.T) {
	mgr := NewAuthManager(nil)

	mgr.RegisterUser("testuser", "password123", []string{"view"})
	cred, _ := mgr.Authenticate("testuser", "password123", "192.168.1.1")

	oldToken := cred.Token

	// 刷新令牌
	refreshed, err := mgr.RefreshToken(cred.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, oldToken, refreshed.Token)
	assert.NotEmpty(t, refreshed.RefreshToken)
}

func TestAuthManagerRevokeToken(t *testing.T) {
	mgr := NewAuthManager(nil)

	mgr.RegisterUser("testuser", "password123", []string{"view"})
	cred, _ := mgr.Authenticate("testuser", "password123", "192.168.1.1")

	err := mgr.RevokeToken(cred.Token)
	require.NoError(t, err)

	_, err = mgr.ValidateToken(cred.Token)
	assert.Error(t, err)
}

func TestAuthManagerAccessPolicy(t *testing.T) {
	mgr := NewAuthManager(nil)

	// 注册用户
	mgr.RegisterUser("user_1", "password123", []string{"view"})
	mgr.RegisterUser("user_2", "password456", []string{"view"})

	// 获取用户ID
	users := mgr.ListUsers()
	var user1ID, user2ID string
	for _, u := range users {
		if u.Username == "user_1" {
			user1ID = u.UserID
		}
		if u.Username == "user_2" {
			user2ID = u.UserID
		}
	}

	policy := &AccessPolicy{
		ID:         "policy_1",
		Name:       "测试策略",
		Users:      []string{"user_1"},
		Permission: PermissionControl,
		Resources:  []string{"session_1"},
		Enabled:    true,
	}

	mgr.AddAccessPolicy(policy)

	// 检查访问权限
	hasAccess := mgr.CheckAccess(user1ID, "session_1", "view")
	assert.True(t, hasAccess)

	hasAccess = mgr.CheckAccess(user2ID, "session_1", "view")
	assert.False(t, hasAccess)

	// 移除策略
	mgr.RemoveAccessPolicy("policy_1")
}

func TestRecorderStartStop(t *testing.T) {
	cfg := &RecordingConfig{
		Enabled:       true,
		StoragePath:   "/tmp/test_recordings",
		Format:        "webm",
		RetentionDays: 7,
	}

	recorder := NewRecorder(cfg)

	recording, err := recorder.StartRecording("session_1", "user_1")
	require.NoError(t, err)
	require.NotNil(t, recording)

	assert.Equal(t, "recording", recording.Status)
	assert.Equal(t, "session_1", recording.SessionID)

	// 录制事件
	err = recorder.RecordMouseEvent("session_1", &MouseEvent{
		X:      100,
		Y:      200,
		Button: "left",
		Action: "click",
	})
	require.NoError(t, err)

	// 停止录制
	result, err := recorder.StopRecording("session_1")
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
}

func TestRecorderEvents(t *testing.T) {
	cfg := &RecordingConfig{
		Enabled:     true,
		StoragePath: "/tmp/test_recordings",
	}

	recorder := NewRecorder(cfg)

	recording, _ := recorder.StartRecording("session_1", "user_1")

	// 录制多种事件
	recorder.RecordMouseEvent("session_1", &MouseEvent{X: 100, Y: 200})
	recorder.RecordKeyboardEvent("session_1", &KeyboardEvent{Key: "a", Action: "press"})
	recorder.RecordScreenFrame("session_1", &ScreenFrame{Sequence: 1, Data: []byte("frame")})

	recorder.StopRecording("session_1")

	// 获取事件
	events, err := recorder.GetRecordingEvents(recording.ID, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(events))
}

func TestChatServiceCreateChannel(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 50)

	channels := svc.ListChannels()
	assert.Equal(t, 1, len(channels))
	assert.Equal(t, "session_1", channels[0])
}

func TestChatServiceSendMessage(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	msg, err := svc.SendMessage("session_1", "user_1", "测试用户", "你好，世界！", "text")
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "user_1", msg.SenderID)
	assert.Equal(t, "测试用户", msg.SenderName)
	assert.Equal(t, "你好，世界！", msg.Content)
	assert.Equal(t, "text", msg.Type)
}

func TestChatServiceGetMessages(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	svc.SendMessage("session_1", "user_1", "用户1", "消息1", "text")
	svc.SendMessage("session_1", "user_2", "用户2", "消息2", "text")
	svc.SendMessage("session_1", "user_1", "用户1", "消息3", "text")

	messages, err := svc.GetMessages("session_1", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(messages))
	assert.Equal(t, "消息2", messages[0].Content)
	assert.Equal(t, "消息3", messages[1].Content)
}

func TestChatServiceSystemMessage(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	msg, err := svc.SendSystemMessage("session_1", "会话已开始")
	require.NoError(t, err)
	assert.Equal(t, "system", msg.Type)
	assert.Equal(t, "系统", msg.SenderName)
}

func TestChatServiceSearchMessages(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	svc.SendMessage("session_1", "user_1", "用户1", "今天天气真好", "text")
	svc.SendMessage("session_1", "user_2", "用户2", "明天要下雨", "text")
	svc.SendMessage("session_1", "user_1", "用户1", "天气预报说后天晴天", "text")

	results, err := svc.SearchMessages("session_1", "天气")
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))
}

func TestChatServiceDeleteMessage(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	msg, _ := svc.SendMessage("session_1", "user_1", "用户1", "要删除的消息", "text")

	err := svc.DeleteMessage("session_1", msg.ID)
	require.NoError(t, err)

	messages, _ := svc.GetMessages("session_1", 100)
	assert.Equal(t, 0, len(messages))
}

func TestChatServiceStats(t *testing.T) {
	svc := NewChatService()

	svc.CreateChannel("session_1", 100)

	svc.SendMessage("session_1", "user_1", "用户1", "消息1", "text")
	svc.SendMessage("session_1", "user_2", "用户2", "消息2", "text")
	svc.SendSystemMessage("session_1", "系统消息")

	stats, err := svc.GetMessageStats("session_1")
	require.NoError(t, err)

	assert.Equal(t, 3, stats["total_messages"])
	byType := stats["by_type"].(map[string]int)
	assert.Equal(t, 2, byType["text"])
	assert.Equal(t, 1, byType["system"])
}

func TestAuditServiceLogEvent(t *testing.T) {
	svc := NewAuditService()

	svc.LogEvent(&AuditEvent{
		SessionID: "session_1",
		UserID:    "user_1",
		Username:  "测试用户",
		Action:    "test_action",
		Resource:  "resource_1",
		Status:    "success",
		RiskLevel: "low",
	})

	stats := svc.GetAuditStats()
	assert.Equal(t, 1, stats["total_events"])
}

func TestAuditServiceQueryEvents(t *testing.T) {
	svc := NewAuditService()

	svc.LogEvent(&AuditEvent{
		SessionID: "session_1",
		UserID:    "user_1",
		Action:    "action_1",
		RiskLevel: "low",
	})

	svc.LogEvent(&AuditEvent{
		SessionID: "session_1",
		UserID:    "user_2",
		Action:    "action_2",
		RiskLevel: "high",
	})

	svc.LogEvent(&AuditEvent{
		SessionID: "session_2",
		UserID:    "user_1",
		Action:    "action_3",
		RiskLevel: "medium",
	})

	// 查询会话事件
	query := &AuditQuery{
		SessionID: "session_1",
	}

	events, err := svc.QueryEvents(query)
	require.NoError(t, err)
	assert.Equal(t, 2, len(events))

	// 查询高风险事件
	query = &AuditQuery{
		RiskLevel: "high",
	}

	events, err = svc.QueryEvents(query)
	require.NoError(t, err)
	assert.Equal(t, 1, len(events))
	assert.Equal(t, "action_2", events[0].Action)
}

func TestAuditServiceHelperMethods(t *testing.T) {
	svc := NewAuditService()

	// 测试各种日志方法
	svc.LogAuth("testuser", "192.168.1.1", true, "login")
	svc.LogAuth("testuser", "192.168.1.1", false, "wrong password")

	svc.LogSessionCreated(&Session{ID: "session_1"}, "user_1")
	svc.LogSessionActivated(&Session{ID: "session_1"}, "user_1")
	svc.LogSessionEnded(&Session{ID: "session_1", Duration: 3600}, "user_1", "manual")

	svc.LogScreenShareStarted("session_1", "user_1")
	svc.LogScreenShareStopped("session_1", "user_1")

	svc.LogTerminalCommand("session_1", "user_1", "ls -la", 0)
	svc.LogTerminalCommand("session_1", "user_1", "rm -rf /", 1)

	svc.LogFileTransfer("session_1", "user_1", &FileTransfer{
		ID:        "transfer_1",
		Direction: "upload",
		FileName:  "test.pdf",
		FileSize:  1024,
	})

	svc.LogRecording("session_1", "user_1", "start")
	svc.LogRecording("session_1", "user_1", "stop")

	stats := svc.GetAuditStats()
	assert.True(t, stats["total_events"].(int) > 0)
}

func TestAuditServiceHighRiskEvents(t *testing.T) {
	svc := NewAuditService()

	svc.LogEvent(&AuditEvent{Action: "low_1", RiskLevel: "low"})
	svc.LogEvent(&AuditEvent{Action: "high_1", RiskLevel: "high"})
	svc.LogEvent(&AuditEvent{Action: "medium_1", RiskLevel: "medium"})
	svc.LogEvent(&AuditEvent{Action: "high_2", RiskLevel: "high"})

	highRisk := svc.GetHighRiskEvents(10)
	assert.Equal(t, 2, len(highRisk))
}

func TestManagerGetSessionHistory(t *testing.T) {
	cfg := DefaultConfig()
	mgr, err := NewManager(cfg)
	require.NoError(t, err)
	defer mgr.Close()

	req := &AssistRequest{
		HostID:     "host_1",
		GuestID:    "guest_1",
		Type:       AssistTypeScreen,
		Permission: PermissionView,
		ExpiresIn:  3600,
	}

	session, _ := mgr.CreateSession(req)
	mgr.ActivateSession(session.ID)
	mgr.EndSession(session.ID)

	history, err := mgr.GetSessionHistory(session.ID)
	require.NoError(t, err)
	assert.True(t, len(history) > 0)
}
