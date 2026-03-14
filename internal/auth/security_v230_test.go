package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 密码策略测试 ==========

func TestPasswordValidator_Validate(t *testing.T) {
	validator := NewPasswordValidator(DefaultPasswordPolicy)

	tests := []struct {
		name     string
		password string
		userInfo []string
		wantValid bool
		wantErrors int
	}{
		{
			name:      "有效密码",
			password:  "SecurePass123!",
			wantValid: true,
			wantErrors: 0,
		},
		{
			name:      "太短",
			password:  "Ab1!",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "缺少大写字母",
			password:  "securepass123!",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "缺少小写字母",
			password:  "SECUREPASS123!",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "缺少数字",
			password:  "SecurePass!!!",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "缺少特殊字符",
			password:  "SecurePass123",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "常见弱密码",
			password:  "Password1!",
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "包含用户名",
			password:  "JohnDoe123!",
			userInfo: []string{"JohnDoe"},
			wantValid: false,
			wantErrors: 1,
		},
		{
			name:      "强密码",
			password:  "MyStr0ng@Pass#2024",
			wantValid: true,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.password, tt.userInfo...)
			assert.Equal(t, tt.wantValid, result.Valid)
			if tt.wantErrors > 0 {
				assert.GreaterOrEqual(t, len(result.Errors), tt.wantErrors)
			} else {
				assert.Len(t, result.Errors, tt.wantErrors)
			}
		})
	}
}

func TestPasswordValidator_CalculateScore(t *testing.T) {
	validator := NewPasswordValidator(DefaultPasswordPolicy)

	tests := []struct {
		name     string
		password string
		minScore int
		maxScore int
	}{
		{"弱密码", "abc", 0, 30},
		{"中等密码", "Password1", 30, 60},
		{"强密码", "MyStr0ng@Pass#2024", 70, 100},
		{"超长密码", "ThisIsAVeryLongAndSecurePassword123!@#", 80, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.password)
			assert.GreaterOrEqual(t, result.Score, tt.minScore)
			assert.LessOrEqual(t, result.Score, tt.maxScore)
		})
	}
}

func TestPasswordValidator_ValidateWithConfirm(t *testing.T) {
	validator := NewPasswordValidator(DefaultPasswordPolicy)

	// 密码不匹配
	result := validator.ValidateWithConfirm("SecurePass123!", "SecurePass456!")
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors, "两次输入的密码不一致")

	// 密码匹配
	result = validator.ValidateWithConfirm("SecurePass123!", "SecurePass123!")
	assert.True(t, result.Valid)
}

func TestPasswordHistory(t *testing.T) {
	history := NewPasswordHistory(5)

	hash1 := "hash1"
	hash2 := "hash2"
	hash3 := "hash3"

	// 添加历史记录
	history.Add(hash1)
	history.Add(hash2)

	assert.True(t, history.Contains(hash1))
	assert.True(t, history.Contains(hash2))
	assert.False(t, history.Contains(hash3))

	// 测试最大数量
	for i := 0; i < 10; i++ {
		history.Add(string(rune('a' + i)))
	}

	// hash1 应该被移除
	assert.False(t, history.Contains(hash1))
}

// ========== 登录尝试测试 ==========

func TestLoginAttemptTracker_RecordAttempt(t *testing.T) {
	config := DefaultLoginAttemptConfig
	config.MaxAttempts = 3
	tracker := NewLoginAttemptTracker(config)

	// 记录失败尝试
	for i := 0; i < 3; i++ {
		tracker.RecordAttempt("testuser", "192.168.1.1", false)
	}

	// 检查是否被锁定
	locked, _ := tracker.IsLocked("testuser")
	assert.True(t, locked)

	// 检查剩余尝试次数
	remaining := tracker.GetRemainingAttempts("testuser")
	assert.Equal(t, 0, remaining)
}

func TestLoginAttemptTracker_SuccessClearsAttempts(t *testing.T) {
	config := DefaultLoginAttemptConfig
	config.MaxAttempts = 3
	tracker := NewLoginAttemptTracker(config)

	// 记录 2 次失败
	tracker.RecordAttempt("testuser", "192.168.1.1", false)
	tracker.RecordAttempt("testuser", "192.168.1.1", false)

	assert.Equal(t, 2, tracker.GetFailedAttempts("testuser"))

	// 成功登录
	tracker.RecordAttempt("testuser", "192.168.1.1", true)

	// 失败计数应该被清除
	assert.Equal(t, 0, tracker.GetFailedAttempts("testuser"))
	locked, _ := tracker.IsLocked("testuser")
	assert.False(t, locked)
}

func TestLoginAttemptTracker_IPBlocking(t *testing.T) {
	config := DefaultLoginAttemptConfig
	config.IPMaxAttempts = 5
	tracker := NewLoginAttemptTracker(config)

	// 从同一 IP 记录多次失败
	for i := 0; i < 5; i++ {
		tracker.RecordAttempt("user1", "192.168.1.100", false)
	}

	// IP 应该被锁定
	locked, _ := tracker.IsIPLocked("192.168.1.100")
	assert.True(t, locked)
}

func TestLoginAttemptTracker_Unlock(t *testing.T) {
	config := DefaultLoginAttemptConfig
	config.MaxAttempts = 3
	tracker := NewLoginAttemptTracker(config)

	// 锁定用户
	for i := 0; i < 3; i++ {
		tracker.RecordAttempt("testuser", "192.168.1.1", false)
	}

	locked, _ := tracker.IsLocked("testuser")
	assert.True(t, locked)

	// 手动解锁
	tracker.Unlock("testuser")

	locked, _ = tracker.IsLocked("testuser")
	assert.False(t, locked)
}

func TestLoginAttemptTracker_Cleanup(t *testing.T) {
	config := DefaultLoginAttemptConfig
	config.AttemptWindow = 100 * time.Millisecond
	tracker := NewLoginAttemptTracker(config)

	// 记录失败
	tracker.RecordAttempt("testuser", "192.168.1.1", false)

	// 等待窗口过期
	time.Sleep(150 * time.Millisecond)

	// 清理
	tracker.Cleanup()

	// 失败计数应该被清除
	assert.Equal(t, 0, tracker.GetFailedAttempts("testuser"))
}

// ========== 会话管理测试 ==========

func TestSessionManager_CreateSession(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 5,
	}
	sm := NewSessionManager(config)

	session, err := sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})
	require.NoError(t, err)
	assert.NotEmpty(t, session.Token)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "user1", session.UserID)
	assert.Equal(t, "testuser", session.Username)
}

func TestSessionManager_ValidateSession(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 5,
	}
	sm := NewSessionManager(config)

	session, _ := sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})

	// 验证有效会话
	validSession, err := sm.ValidateSession(session.Token)
	require.NoError(t, err)
	assert.Equal(t, session.UserID, validSession.UserID)

	// 验证无效会话
	_, err = sm.ValidateSession("invalid-token")
	assert.Error(t, err)
}

func TestSessionManager_InvalidateSession(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 5,
	}
	sm := NewSessionManager(config)

	session, _ := sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})

	// 使会话失效
	err := sm.InvalidateSession(session.Token)
	require.NoError(t, err)

	// 验证会话已失效
	_, err = sm.ValidateSession(session.Token)
	assert.Error(t, err)
}

func TestSessionManager_MaxSessionsPerUser(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 2,
	}
	sm := NewSessionManager(config)

	// 创建 2 个会话
	sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})
	sm.CreateSession("user1", "testuser", "192.168.1.2", "Mozilla/5.0", []string{"user"}, []string{})

	// 获取用户会话
	sessions := sm.GetUserSessions("user1")
	assert.Len(t, sessions, 2)

	// 创建第 3 个会话，应该删除最旧的
	sm.CreateSession("user1", "testuser", "192.168.1.3", "Mozilla/5.0", []string{"user"}, []string{})

	sessions = sm.GetUserSessions("user1")
	assert.Len(t, sessions, 2)
}

func TestSessionManager_RefreshToken(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:         time.Hour,
		RefreshTokenExpiry:  24 * time.Hour,
		MaxSessionsPerUser:  5,
		EnableRefreshToken:  true,
	}
	sm := NewSessionManager(config)

	session, _ := sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})

	// 刷新令牌
	newSession, err := sm.RefreshSession(session.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, session.Token, newSession.Token)

	// 旧令牌应该无效
	_, err = sm.ValidateSession(session.Token)
	assert.Error(t, err)

	// 新令牌应该有效
	_, err = sm.ValidateSession(newSession.Token)
	assert.NoError(t, err)
}

func TestSessionManager_SetMFAVerified(t *testing.T) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 5,
	}
	sm := NewSessionManager(config)

	session, _ := sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})

	assert.False(t, session.MFAVerified)

	// 设置 MFA 验证状态
	err := sm.SetMFAVerified(session.Token, true)
	require.NoError(t, err)

	// 验证状态已更新
	validSession, _ := sm.ValidateSession(session.Token)
	assert.True(t, validSession.MFAVerified)
}

// ========== 基准测试 ==========

func BenchmarkPasswordValidator_Validate(b *testing.B) {
	validator := NewPasswordValidator(DefaultPasswordPolicy)
	password := "SecurePass123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.Validate(password)
	}
}

func BenchmarkLoginAttemptTracker_RecordAttempt(b *testing.B) {
	config := DefaultLoginAttemptConfig
	tracker := NewLoginAttemptTracker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.RecordAttempt("user1", "192.168.1.1", false)
	}
}

func BenchmarkSessionManager_CreateSession(b *testing.B) {
	config := SessionConfig{
		TokenExpiry:        time.Hour,
		MaxSessionsPerUser: 1000,
	}
	sm := NewSessionManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.CreateSession("user1", "testuser", "192.168.1.1", "Mozilla/5.0", []string{"user"}, []string{})
	}
}