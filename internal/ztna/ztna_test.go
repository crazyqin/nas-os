package ztna

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 策略测试 ==========

func TestCreatePolicy(t *testing.T) {
	mgr := NewManager()

	req := CreatePolicyRequest{
		Name:        "测试策略",
		Description: "用于测试的访问策略",
		Priority:    10,
		MinTrust:    70,
		Action:      ActionAllow,
		Rules: []AccessRule{
			{
				Name:     "允许读取",
				Enabled:  true,
				Identity: "user1",
				Resource: "api/*",
				Actions:  []string{"read"},
				MinTrust: 60,
			},
		},
	}

	policy, err := mgr.CreatePolicy(req)
	require.NoError(t, err)
	assert.NotEmpty(t, policy.ID)
	assert.Equal(t, "测试策略", policy.Name)
	assert.Equal(t, 10, policy.Priority)
	assert.Equal(t, 70, policy.MinTrust)
	assert.Equal(t, ActionAllow, policy.Action)
	assert.Len(t, policy.Rules, 1)
	assert.NotEmpty(t, policy.Rules[0].ID) // ID 应该自动生成
}

func TestGetPolicy(t *testing.T) {
	mgr := NewManager()

	// 创建策略
	req := CreatePolicyRequest{
		Name:   "策略1",
		Action: ActionAllow,
		MinTrust: 50,
	}
	created, err := mgr.CreatePolicy(req)
	require.NoError(t, err)

	// 获取策略
	policy, err := mgr.GetPolicy(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, policy.ID)
	assert.Equal(t, "策略1", policy.Name)
}

func TestGetPolicyNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.GetPolicy("nonexistent")
	assert.ErrorIs(t, err, ErrPolicyNotFound)
}

func TestListPolicies(t *testing.T) {
	mgr := NewManager()

	// 创建多个策略
	mgr.CreatePolicy(CreatePolicyRequest{Name: "策略1", Action: ActionAllow})
	mgr.CreatePolicy(CreatePolicyRequest{Name: "策略2", Action: ActionDeny})
	mgr.CreatePolicy(CreatePolicyRequest{Name: "策略3", Action: ActionStepUp})

	policies := mgr.ListPolicies()
	assert.Len(t, policies, 3)
}

func TestDeletePolicy(t *testing.T) {
	mgr := NewManager()

	// 创建策略
	policy, _ := mgr.CreatePolicy(CreatePolicyRequest{Name: "待删除", Action: ActionDeny})

	// 删除策略
	err := mgr.DeletePolicy(policy.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = mgr.GetPolicy(policy.ID)
	assert.ErrorIs(t, err, ErrPolicyNotFound)
}

func TestDeletePolicyNotFound(t *testing.T) {
	mgr := NewManager()

	err := mgr.DeletePolicy("nonexistent")
	assert.ErrorIs(t, err, ErrPolicyNotFound)
}

// ========== 设备信任测试 ==========

func TestVerifyDevice(t *testing.T) {
	mgr := NewManager()

	req := VerifyRequest{
		DeviceID:   "device-1",
		UserID:     "user-1",
		DeviceName: "我的电脑",
		DeviceType: "desktop",
		OS:         "windows",
		OSVersion:  "11",
		PatchLevel: "latest",
	}

	device, err := mgr.VerifyDevice(req)
	require.NoError(t, err)
	assert.Equal(t, "device-1", device.DeviceID)
	assert.Equal(t, "user-1", device.UserID)
	assert.Equal(t, "windows", device.OS)
	assert.GreaterOrEqual(t, device.TrustScore, 70) // 最新系统+补丁应该有较高分
	assert.NotEmpty(t, device.TrustFactors)
}

func TestVerifyDeviceMobile(t *testing.T) {
	mgr := NewManager()

	req := VerifyRequest{
		DeviceID:   "device-2",
		UserID:     "user-1",
		DeviceName: "我的手机",
		DeviceType: "mobile",
		OS:         "ios",
		OSVersion:  "17",
		PatchLevel: "current",
	}

	device, err := mgr.VerifyDevice(req)
	require.NoError(t, err)
	assert.Equal(t, DeviceStatusTrusted, device.Status)
	assert.True(t, device.Compliant)
}

func TestVerifyDeviceOutdated(t *testing.T) {
	mgr := NewManager()

	req := VerifyRequest{
		DeviceID:   "device-3",
		UserID:     "user-2",
		DeviceName: "旧设备",
		DeviceType: "other",
		OS:         "unknown",
		OSVersion:  "1",
		PatchLevel: "outdated",
	}

	device, err := mgr.VerifyDevice(req)
	require.NoError(t, err)
	assert.Less(t, device.TrustScore, 70) // 过时系统应该有较低分
}

func TestGetDeviceTrust(t *testing.T) {
	mgr := NewManager()

	// 先验证设备
	req := VerifyRequest{
		DeviceID:   "device-4",
		UserID:     "user-1",
		DeviceType: "desktop",
		OS:         "macos",
		PatchLevel: "latest",
	}
	verified, _ := mgr.VerifyDevice(req)

	// 获取信任信息
	device, err := mgr.GetDeviceTrust("device-4")
	require.NoError(t, err)
	assert.Equal(t, verified.TrustScore, device.TrustScore)
}

func TestGetDeviceTrustNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.GetDeviceTrust("nonexistent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestCheckTrustScore(t *testing.T) {
	mgr := NewManager()

	// 验证设备
	req := VerifyRequest{
		DeviceID:   "device-5",
		UserID:     "user-1",
		DeviceType: "desktop",
		OS:         "linux",
		PatchLevel: "latest",
	}
	device, _ := mgr.VerifyDevice(req)

	// 检查信任分
	pass, score, err := mgr.CheckTrustScore("device-5", 70)
	require.NoError(t, err)
	assert.Equal(t, device.TrustScore, score)
	assert.True(t, pass) // 最新 Linux 应该通过 70 分要求

	// 检查过高要求
	pass, score, err = mgr.CheckTrustScore("device-5", 99)
	require.NoError(t, err)
	assert.False(t, pass)
}

func TestCheckTrustScoreDeviceNotFound(t *testing.T) {
	mgr := NewManager()

	_, _, err := mgr.CheckTrustScore("nonexistent", 50)
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

// ========== 策略评估测试 ==========

func TestEvaluatePolicy(t *testing.T) {
	mgr := NewManager()

	// 创建策略：用户 user1 可以读取 api/*
	mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "API 读取策略",
		MinTrust: 50,
		Action:   ActionAllow,
		Rules: []AccessRule{
			{
				Enabled:  true,
				Identity: "user1",
				Resource: "api/*",
				Actions:  []string{"read"},
				MinTrust: 50,
			},
		},
	})

	// 验证设备
	mgr.VerifyDevice(VerifyRequest{
		DeviceID:   "device-1",
		UserID:     "user1",
		DeviceType: "desktop",
		OS:         "windows",
		PatchLevel: "latest",
	})

	// 评估策略
	policy, err := mgr.EvaluatePolicy("user1", "device-1", "api/users", "read")
	require.NoError(t, err)
	assert.Equal(t, "API 读取策略", policy.Name)
}

func TestEvaluatePolicyAccessDenied(t *testing.T) {
	mgr := NewManager()

	// 创建策略：只允许 user1
	mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "限制策略",
		MinTrust: 50,
		Action:   ActionAllow,
		Rules: []AccessRule{
			{
				Enabled:  true,
				Identity: "user1",
				Resource: "api/*",
				Actions:  []string{"read"},
			},
		},
	})

	// user2 请求应该被拒绝
	_, err := mgr.EvaluatePolicy("user2", "device-1", "api/users", "read")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

func TestEvaluatePolicyWildcard(t *testing.T) {
	mgr := NewManager()

	// 创建通配符策略
	mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "通配符策略",
		MinTrust: 0,
		Action:   ActionAllow,
		Rules: []AccessRule{
			{
				Enabled:  true,
				Resource: "*",
				Actions:  []string{"read", "write"},
			},
		},
	})

	// 应该匹配任何资源
	policy, err := mgr.EvaluatePolicy("user1", "device-1", "any/resource", "read")
	require.NoError(t, err)
	assert.Equal(t, "通配符策略", policy.Name)
}

func TestEvaluatePolicyPriority(t *testing.T) {
	mgr := NewManager()

	// 创建低优先级策略
	mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "低优先级",
		Priority: 100,
		MinTrust: 0,
		Action:   ActionDeny,
		Rules: []AccessRule{
			{Enabled: true, Resource: "api/*"},
		},
	})

	// 创建高优先级策略
	mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "高优先级",
		Priority: 1,
		MinTrust: 0,
		Action:   ActionAllow,
		Rules: []AccessRule{
			{Enabled: true, Resource: "api/*"},
		},
	})

	// 应该返回高优先级（数字小 = 优先级高）
	policy, err := mgr.EvaluatePolicy("user1", "device-1", "api/users", "read")
	require.NoError(t, err)
	assert.Equal(t, "高优先级", policy.Name)
}

// ========== 会话管理测试 ==========

func TestCreateSession(t *testing.T) {
	mgr := NewManager()

	session, err := mgr.CreateSession(
		"user1", "device-1", "api/users",
		[]string{"read"}, "policy-1", 85,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "user1", session.UserID)
	assert.Equal(t, "device-1", session.DeviceID)
	assert.Equal(t, SessionStatusActive, session.Status)
	assert.True(t, session.ExpiresAt.After(time.Now()))
}

func TestGetSession(t *testing.T) {
	mgr := NewManager()

	// 创建会话
	created, _ := mgr.CreateSession(
		"user1", "device-1", "api/users",
		[]string{"read"}, "policy-1", 85,
	)

	// 获取会话
	session, err := mgr.GetSession(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, session.ID)
	assert.Equal(t, "user1", session.UserID)
}

func TestGetSessionNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.GetSession("nonexistent")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestListSessions(t *testing.T) {
	mgr := NewManager()

	// 创建多个会话
	mgr.CreateSession("user1", "device-1", "api/a", []string{"read"}, "p1", 80)
	mgr.CreateSession("user2", "device-2", "api/b", []string{"write"}, "p2", 70)
	mgr.CreateSession("user3", "device-3", "api/c", []string{"read"}, "p3", 90)

	sessions := mgr.ListSessions()
	assert.Len(t, sessions, 3)
}

func TestRevokeAccess(t *testing.T) {
	mgr := NewManager()

	// 创建会话
	session, _ := mgr.CreateSession(
		"user1", "device-1", "api/users",
		[]string{"read"}, "policy-1", 85,
	)

	// 撤销访问
	err := mgr.RevokeAccess(session.ID)
	assert.NoError(t, err)

	// 验证会话已撤销
	revoked, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusRevoked, revoked.Status)
}

func TestRevokeAccessNotFound(t *testing.T) {
	mgr := NewManager()

	err := mgr.RevokeAccess("nonexistent")
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

func TestRevokeAllUserSessions(t *testing.T) {
	mgr := NewManager()

	// 为同一用户创建多个会话
	mgr.CreateSession("user1", "device-1", "api/a", []string{"read"}, "p1", 80)
	mgr.CreateSession("user1", "device-2", "api/b", []string{"write"}, "p2", 70)
	mgr.CreateSession("user2", "device-3", "api/c", []string{"read"}, "p3", 90)

	// 撤销 user1 的所有会话
	count := mgr.RevokeAllUserSessions("user1")
	assert.Equal(t, 2, count)

	// 验证 user1 的会话已撤销
	sessions := mgr.ListSessions()
	for _, s := range sessions {
		if s.UserID == "user1" {
			assert.Equal(t, SessionStatusRevoked, s.Status)
		} else {
			assert.Equal(t, SessionStatusActive, s.Status)
		}
	}
}

// ========== 信任分计算测试 ==========

func TestTrustScoreWindowsLatest(t *testing.T) {
	mgr := NewManager()

	req := VerifyRequest{
		DeviceID:   "test-1",
		UserID:     "user1",
		DeviceType: "desktop",
		OS:         "windows",
		PatchLevel: "latest",
	}

	device, err := mgr.VerifyDevice(req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, device.TrustScore, 80)
	assert.Equal(t, DeviceStatusTrusted, device.Status)
}

func TestTrustScoreOutdatedOS(t *testing.T) {
	mgr := NewManager()

	req := VerifyRequest{
		DeviceID:   "test-2",
		UserID:     "user1",
		DeviceType: "desktop",
		OS:         "unknown",
		PatchLevel: "outdated",
	}

	device, err := mgr.VerifyDevice(req)
	require.NoError(t, err)
	assert.Less(t, device.TrustScore, 70)
}

// ========== 条件评估测试 ==========

func TestTimeCondition(t *testing.T) {
	// 测试时间条件评估
	cond := Condition{
		Type:     ConditionTime,
		Operator: "gte",
		Value:    "9",
	}

	// 工作时间应该通过
	workTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	assert.True(t, evaluateTimeCondition(cond, workTime))

	// 早晨应该不通过
	morning := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	assert.False(t, evaluateTimeCondition(cond, morning))
}

func TestTimeConditionBetween(t *testing.T) {
	cond := Condition{
		Type:     ConditionTime,
		Operator: "between",
		Value:    "9-17",
	}

	// 工作时间
	workTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)
	assert.True(t, evaluateTimeCondition(cond, workTime))

	// 非工作时间
	nightTime := time.Date(2024, 1, 15, 22, 0, 0, 0, time.UTC)
	assert.False(t, evaluateTimeCondition(cond, nightTime))
}

// ========== 集成测试 ==========

func TestFullZTNAFlow(t *testing.T) {
	mgr := NewManager()

	// 1. 创建策略
	policy, err := mgr.CreatePolicy(CreatePolicyRequest{
		Name:     "集成测试策略",
		MinTrust: 60,
		Action:   ActionAllow,
		Rules: []AccessRule{
			{
				Enabled:  true,
				Identity: "user1",
				Resource: "api/*",
				Actions:  []string{"read", "write"},
				MinTrust: 60,
			},
		},
	})
	require.NoError(t, err)

	// 2. 验证设备
	device, err := mgr.VerifyDevice(VerifyRequest{
		DeviceID:   "device-1",
		UserID:     "user1",
		DeviceType: "desktop",
		OS:         "macos",
		PatchLevel: "latest",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, device.TrustScore, 60)

	// 3. 评估策略
	evaluatedPolicy, err := mgr.EvaluatePolicy("user1", "device-1", "api/users", "read")
	require.NoError(t, err)
	assert.Equal(t, policy.ID, evaluatedPolicy.ID)

	// 4. 创建会话
	session, err := mgr.CreateSession(
		"user1", "device-1", "api/users",
		[]string{"read", "write"}, policy.ID, device.TrustScore,
	)
	require.NoError(t, err)
	assert.Equal(t, SessionStatusActive, session.Status)

	// 5. 检查信任分
	pass, _, err := mgr.CheckTrustScore("device-1", 60)
	require.NoError(t, err)
	assert.True(t, pass)

	// 6. 撤销访问
	err = mgr.RevokeAccess(session.ID)
	require.NoError(t, err)

	// 7. 验证撤销
	revoked, _ := mgr.GetSession(session.ID)
	assert.Equal(t, SessionStatusRevoked, revoked.Status)
}
