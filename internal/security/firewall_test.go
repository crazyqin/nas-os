// Package security 提供防火墙功能测试
package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFirewallManager_Firewall(t *testing.T) {
	fm := NewFirewallManager()
	require.NotNil(t, fm)
	assert.NotNil(t, fm.rules)
	assert.NotNil(t, fm.ipBlacklist)
	assert.NotNil(t, fm.ipWhitelist)
	assert.True(t, fm.config.Enabled)
}

func TestFirewallManager_GetConfig(t *testing.T) {
	fm := NewFirewallManager()
	config := fm.GetConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, "deny", config.DefaultPolicy)
	assert.True(t, config.IPv6Enabled)
	assert.True(t, config.LogDropped)
}

func TestFirewallManager_UpdateConfig(t *testing.T) {
	fm := NewFirewallManager()

	newConfig := FirewallConfig{
		Enabled:       false,
		DefaultPolicy: "allow",
		IPv6Enabled:   false,
		LogDropped:    false,
	}

	err := fm.UpdateConfig(newConfig)
	// 可能因为没有权限而失败，但不应崩溃
	assert.NoError(t, err)

	config := fm.GetConfig()
	assert.False(t, config.Enabled)
	assert.Equal(t, "allow", config.DefaultPolicy)
}

func TestFirewallManager_ListRules(t *testing.T) {
	fm := NewFirewallManager()

	// 初始应该没有规则
	rules := fm.ListRules()
	assert.Empty(t, rules)

	// 添加规则
	fm.rules["rule-1"] = &FirewallRule{
		ID:       "rule-1",
		Name:     "Test Rule",
		Priority: 100,
	}
	fm.rules["rule-2"] = &FirewallRule{
		ID:       "rule-2",
		Name:     "Another Rule",
		Priority: 50,
	}

	rules = fm.ListRules()
	assert.Len(t, rules, 2)
}

func TestFirewallManager_ValidateRule(t *testing.T) {
	fm := NewFirewallManager()

	tests := []struct {
		name    string
		rule    *FirewallRule
		wantErr bool
	}{
		{
			name: "有效规则",
			rule: &FirewallRule{
				Name:      "Allow HTTP",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "inbound",
				DestPort:  "80",
			},
			wantErr: false,
		},
		{
			name: "无效动作",
			rule: &FirewallRule{
				Name:      "Invalid Action",
				Action:    "invalid",
				Protocol:  "tcp",
				Direction: "inbound",
			},
			wantErr: true,
		},
		{
			name: "无效协议",
			rule: &FirewallRule{
				Name:      "Invalid Protocol",
				Action:    "allow",
				Protocol:  "sctp",
				Direction: "inbound",
			},
			wantErr: true,
		},
		{
			name: "无效方向",
			rule: &FirewallRule{
				Name:      "Invalid Direction",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "sideways",
			},
			wantErr: true,
		},
		{
			name: "有效源IP",
			rule: &FirewallRule{
				Name:      "Source IP",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "inbound",
				SourceIP:  "192.168.1.0/24",
			},
			wantErr: false,
		},
		{
			name: "无效源IP",
			rule: &FirewallRule{
				Name:      "Invalid Source IP",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "inbound",
				SourceIP:  "invalid-ip",
			},
			wantErr: true,
		},
		{
			name: "有效端口范围",
			rule: &FirewallRule{
				Name:      "Port Range",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "inbound",
				DestPort:  "8000-9000",
			},
			wantErr: false,
		},
		{
			name: "无效端口",
			rule: &FirewallRule{
				Name:      "Invalid Port",
				Action:    "allow",
				Protocol:  "tcp",
				Direction: "inbound",
				DestPort:  "99999",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fm.validateRule(tt.rule)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFirewallManager_ValidatePort(t *testing.T) {
	fm := NewFirewallManager()

	tests := []struct {
		port    string
		wantErr bool
	}{
		{"80", false},
		{"443", false},
		{"22", false},
		{"8080", false},
		{"80-443", false},
		{"80,443,8080", false},
		{"0", true},
		{"65536", true},
		{"100-50", true}, // 起始大于结束
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			err := fm.validatePort(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFirewallManager_AddToBlacklist(t *testing.T) {
	fm := NewFirewallManager()

	err := fm.AddToBlacklist("10.0.0.1", "恶意攻击", 0)
	assert.NoError(t, err)

	// 验证添加成功
	blacklist := fm.GetBlacklist()
	found := false
	for _, e := range blacklist {
		if e.IP == "10.0.0.1" {
			found = true
			assert.Equal(t, "恶意攻击", e.Reason)
			break
		}
	}
	assert.True(t, found)
}

func TestFirewallManager_RemoveFromBlacklist(t *testing.T) {
	fm := NewFirewallManager()

	// 先添加
	fm.AddToBlacklist("10.0.0.2", "测试", 0)

	// 然后移除
	err := fm.RemoveFromBlacklist("10.0.0.2")
	assert.NoError(t, err)

	// 验证已移除
	blacklist := fm.GetBlacklist()
	for _, e := range blacklist {
		assert.NotEqual(t, "10.0.0.2", e.IP)
	}
}

func TestFirewallManager_AddToWhitelist(t *testing.T) {
	fm := NewFirewallManager()

	err := fm.AddToWhitelist("192.168.1.100", "管理员IP")
	assert.NoError(t, err)

	whitelist := fm.GetWhitelist()
	found := false
	for _, e := range whitelist {
		if e.IP == "192.168.1.100" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestFirewallManager_RemoveFromWhitelist(t *testing.T) {
	fm := NewFirewallManager()

	// 先添加
	fm.AddToWhitelist("192.168.1.101", "临时白名单")

	// 然后移除
	err := fm.RemoveFromWhitelist("192.168.1.101")
	assert.NoError(t, err)

	whitelist := fm.GetWhitelist()
	for _, e := range whitelist {
		assert.NotEqual(t, "192.168.1.101", e.IP)
	}
}

func TestFirewallManager_IsBlacklisted(t *testing.T) {
	fm := NewFirewallManager()

	// 添加黑名单
	fm.AddToBlacklist("10.0.0.100", "测试黑名单", 0)

	// 检查黑名单IP
	blocked := fm.IsBlacklisted("10.0.0.100")
	assert.True(t, blocked)

	// 检查非黑名单IP
	blocked = fm.IsBlacklisted("10.0.0.200")
	assert.False(t, blocked)
}

func TestFirewallManager_CleanupExpired(t *testing.T) {
	fm := NewFirewallManager()

	// 添加过期的黑名单条目
	pastTime := time.Now().Add(-24 * time.Hour)
	expiredEntry := &IPBlacklistEntry{
		IP:        "10.0.0.50",
		Reason:    "过期条目",
		CreatedAt: pastTime,
		ExpiresAt: &pastTime,
	}
	fm.ipBlacklist["10.0.0.50"] = expiredEntry

	// 添加未过期的黑名单条目
	futureTime := time.Now().Add(24 * time.Hour)
	validEntry := &IPBlacklistEntry{
		IP:        "10.0.0.51",
		Reason:    "有效条目",
		CreatedAt: time.Now(),
		ExpiresAt: &futureTime,
	}
	fm.ipBlacklist["10.0.0.51"] = validEntry

	// 清理过期条目
	fm.CleanupExpired()

	// 验证结果
	blacklist := fm.GetBlacklist()
	assert.Len(t, blacklist, 1)
	assert.Equal(t, "10.0.0.51", blacklist[0].IP)
}

func TestFirewallRule_Fields(t *testing.T) {
	now := time.Now()
	rule := &FirewallRule{
		ID:          "rule-123",
		Name:        "Test Rule",
		Enabled:     true,
		Action:      "allow",
		Protocol:    "tcp",
		SourceIP:    "192.168.1.0/24",
		DestPort:    "443",
		Direction:   "inbound",
		Interface:   "eth0",
		GeoLocation: "CN",
		Priority:    100,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "rule-123", rule.ID)
	assert.Equal(t, "Test Rule", rule.Name)
	assert.True(t, rule.Enabled)
	assert.Equal(t, "allow", rule.Action)
	assert.Equal(t, "tcp", rule.Protocol)
	assert.Equal(t, "192.168.1.0/24", rule.SourceIP)
	assert.Equal(t, "443", rule.DestPort)
	assert.Equal(t, "inbound", rule.Direction)
	assert.Equal(t, 100, rule.Priority)
}

func TestFirewallManager_ConcurrentAccess(t *testing.T) {
	fm := NewFirewallManager()

	done := make(chan bool, 10)

	// 并发添加黑名单
	for i := 0; i < 5; i++ {
		go func(idx int) {
			ip := "10.0.0." + string(rune('0'+idx))
			fm.AddToBlacklist(ip, "并发测试", 0)
			done <- true
		}(i)
	}

	// 并发添加白名单
	for i := 0; i < 5; i++ {
		go func(idx int) {
			ip := "192.168.1." + string(rune('0'+idx))
			fm.AddToWhitelist(ip, "并发测试")
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	blacklist := fm.GetBlacklist()
	whitelist := fm.GetWhitelist()
	assert.Len(t, blacklist, 5)
	assert.Len(t, whitelist, 5)
}
