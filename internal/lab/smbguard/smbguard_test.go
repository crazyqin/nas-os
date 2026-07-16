package smbguard

import (
	"net"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if len(engine.policies) != 0 {
		t.Fatalf("expected 0 policies, got %d", len(engine.policies))
	}
}

func TestOnConnectAndDisconnect(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ip := net.ParseIP("192.168.1.100")
	if err := engine.OnConnect(ip, 445, "user1"); err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	conn := engine.GetConnection(ip)
	if conn == nil {
		t.Fatal("expected connection record")
	}
	if conn.State != StateActive {
		t.Errorf("expected state active, got %s", conn.State)
	}

	engine.OnDisconnect(ip)
	conn = engine.GetConnection(ip)
	if conn != nil {
		t.Error("expected nil connection after disconnect")
	}
}

func TestBlockedIPReject(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	// 添加黑名单
	if err := engine.AddToBlacklist("10.0.0.1", "known attacker", "admin", 0); err != nil {
		t.Fatalf("AddToBlacklist failed: %v", err)
	}

	ip := net.ParseIP("10.0.0.1")
	err := engine.OnConnect(ip, 445, "user1")
	if err != ErrIPBlocked {
		t.Errorf("expected ErrIPBlocked, got %v", err)
	}
}

func TestAutoBlock(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ip := net.ParseIP("192.168.1.200")

	// 建立连接
	engine.OnConnect(ip, 445, "attacker")

	// 模拟5次认证失败
	for i := 0; i < engine.config.MaxFailedAttempts; i++ {
		engine.OnAuthFailure(ip, "attacker")
	}

	// 检查是否被封锁
	if !engine.IsIPBlocked("192.168.1.200") {
		t.Error("expected IP to be blocked after max failed attempts")
	}

	// 再连接应失败
	err := engine.OnConnect(ip, 445, "attacker")
	if err != ErrIPBlocked {
		t.Errorf("expected ErrIPBlocked on reconnect, got %v", err)
	}
}

func TestWhitelistBypass(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ipStr := "192.168.1.50"
	ip := net.ParseIP(ipStr)

	// 添加白名单
	if err := engine.AddToWhitelist(ipStr, "trusted admin IP", "admin"); err != nil {
		t.Fatalf("AddToWhitelist failed: %v", err)
	}

	// 建立连接
	engine.OnConnect(ip, 445, "admin")

	// 超过失败次数
	for i := 0; i < engine.config.MaxFailedAttempts+5; i++ {
		engine.OnAuthFailure(ip, "admin")
	}

	// 白名单 IP 不应被封锁
	if engine.IsIPBlocked(ipStr) {
		t.Error("whitelisted IP should not be auto-blocked")
	}
}

func TestWhitelistUnblocks(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ipStr := "192.168.1.60"
	ip := net.ParseIP(ipStr)

	engine.OnConnect(ip, 445, "user1")
	for i := 0; i < engine.config.MaxFailedAttempts; i++ {
		engine.OnAuthFailure(ip, "user1")
	}

	// 应被封锁
	if !engine.IsIPBlocked(ipStr) {
		t.Fatal("expected IP to be blocked")
	}

	// 加入白名单应解除封锁
	engine.AddToWhitelist(ipStr, "forgive", "admin")
	if engine.IsIPBlocked(ipStr) {
		t.Error("adding to whitelist should unblock IP")
	}
}

func TestBlacklist(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	if err := engine.AddToBlacklist("10.0.0.5", "malicious", "admin", 0); err != nil {
		t.Fatalf("AddToBlacklist failed: %v", err)
	}

	if !engine.IsBlacklisted("10.0.0.5") {
		t.Error("expected IP to be blacklisted")
	}

	bl := engine.ListBlacklist()
	if len(bl) != 1 {
		t.Errorf("expected 1 blacklist entry, got %d", len(bl))
	}

	if err := engine.RemoveFromBlacklist("10.0.0.5"); err != nil {
		t.Fatalf("RemoveFromBlacklist failed: %v", err)
	}

	if engine.IsBlacklisted("10.0.0.5") {
		t.Error("expected IP to be removed from blacklist")
	}
}

func TestWhitelistManagement(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	if err := engine.AddToWhitelist("192.168.1.1", "gateway", "admin"); err != nil {
		t.Fatalf("AddToWhitelist failed: %v", err)
	}

	if !engine.IsWhitelisted("192.168.1.1") {
		t.Error("expected IP to be whitelisted")
	}

	wl := engine.ListWhitelist()
	if len(wl) != 1 {
		t.Errorf("expected 1 whitelist entry, got %d", len(wl))
	}

	if err := engine.RemoveFromWhitelist("192.168.1.1"); err != nil {
		t.Fatalf("RemoveFromWhitelist failed: %v", err)
	}

	if engine.IsWhitelisted("192.168.1.1") {
		t.Error("expected IP to be removed from whitelist")
	}

	// 移除不存在的 IP 应报错
	if err := engine.RemoveFromWhitelist("192.168.1.1"); err != ErrIPNotFound {
		t.Errorf("expected ErrIPNotFound, got %v", err)
	}
}

func TestInvalidIP(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	if err := engine.AddToWhitelist("not-an-ip", "test", "admin"); err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}

	if err := engine.AddToBlacklist("also-not-ip", "test", "admin", 0); err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

func TestCreatePolicy(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	policy := &BlockPolicy{
		ID:                "bp-001",
		Name:              "Strict SMB Protection",
		Enabled:           true,
		MaxFailedAttempts: 3,
		WindowSeconds:     60,
		BlockDuration:     3600,
		BlockAction:       BlockActionTemp,
		Priority:          10,
	}

	if err := engine.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}

	if policy.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// 重复创建
	if err := engine.CreatePolicy(policy); err != ErrPolicyExists {
		t.Errorf("expected ErrPolicyExists, got %v", err)
	}
}

func TestUpdateDeletePolicy(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	policy := &BlockPolicy{
		ID:                "bp-002",
		Name:              "Moderate",
		Enabled:           true,
		MaxFailedAttempts: 10,
	}
	engine.CreatePolicy(policy)

	updated := &BlockPolicy{
		Name:              "Moderate Updated",
		Enabled:           true,
		MaxFailedAttempts: 15,
	}
	if err := engine.UpdatePolicy("bp-002", updated); err != nil {
		t.Fatalf("UpdatePolicy failed: %v", err)
	}

	got, err := engine.GetPolicy("bp-002")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if got.Name != "Moderate Updated" {
		t.Errorf("expected updated name, got %s", got.Name)
	}
	if got.MaxFailedAttempts != 15 {
		t.Errorf("expected 15, got %d", got.MaxFailedAttempts)
	}

	if err := engine.DeletePolicy("bp-002"); err != nil {
		t.Fatalf("DeletePolicy failed: %v", err)
	}

	if _, err := engine.GetPolicy("bp-002"); err != ErrPolicyNotFound {
		t.Errorf("expected ErrPolicyNotFound, got %v", err)
	}
}

func TestUnblockIP(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ip := net.ParseIP("172.16.0.1")
	engine.OnConnect(ip, 445, "user")
	for i := 0; i < engine.config.MaxFailedAttempts; i++ {
		engine.OnAuthFailure(ip, "user")
	}

	if !engine.IsIPBlocked("172.16.0.1") {
		t.Fatal("expected IP to be blocked")
	}

	if err := engine.UnblockIP("172.16.0.1"); err != nil {
		t.Fatalf("UnblockIP failed: %v", err)
	}

	if engine.IsIPBlocked("172.16.0.1") {
		t.Error("expected IP to be unblocked")
	}
}

func TestAlerts(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ip := net.ParseIP("192.168.2.1")
	engine.OnConnect(ip, 445, "test")
	for i := 0; i < engine.config.MaxFailedAttempts; i++ {
		engine.OnAuthFailure(ip, "test")
	}

	alerts := engine.GetAlerts(10, false)
	if len(alerts) == 0 {
		t.Fatal("expected at least 1 alert")
	}

	// 检查告警内容
	found := false
	for _, a := range alerts {
		if a.Type == "brute_force" && a.ClientIP == "192.168.2.1" {
			found = true
			if !a.Acknowledged {
				// 确认告警
				if err := engine.AcknowledgeAlert(a.ID); err != nil {
					t.Fatalf("AcknowledgeAlert failed: %v", err)
				}
			}
		}
	}
	if !found {
		t.Error("expected brute_force alert for 192.168.2.1")
	}

	// 未确认的告警应为0
	unackAlerts := engine.GetAlerts(10, true)
	for _, a := range unackAlerts {
		if a.Type == "brute_force" && a.ClientIP == "192.168.2.1" {
			t.Error("expected no unacknowledged brute_force alerts")
		}
	}
}

func TestConnectionLimit(t *testing.T) {
	config := DefaultGuardConfig()
	config.MaxConnections = 2
	engine := NewEngine(config)

	engine.OnConnect(net.ParseIP("10.0.0.1"), 445, "u1")
	engine.OnConnect(net.ParseIP("10.0.0.2"), 445, "u2")

	err := engine.OnConnect(net.ParseIP("10.0.0.3"), 445, "u3")
	if err != ErrConnectionLimit {
		t.Errorf("expected ErrConnectionLimit, got %v", err)
	}
}

func TestAuthSuccess(t *testing.T) {
	engine := NewEngine(DefaultGuardConfig())

	ip := net.ParseIP("192.168.3.1")
	engine.OnConnect(ip, 445, "user")

	engine.OnAuthFailure(ip, "user")
	engine.OnAuthSuccess(ip, "user")

	conn := engine.GetConnection(ip)
	if conn.AuthAttempts != 2 {
		t.Errorf("expected 2 auth attempts, got %d", conn.AuthAttempts)
	}
	if conn.FailedAttempts != 1 {
		t.Errorf("expected 1 failed attempt, got %d", conn.FailedAttempts)
	}
}

func TestCleanupExpired(t *testing.T) {
	config := DefaultGuardConfig()
	config.DefaultBlockDur = 1 // 1 second
	engine := NewEngine(config)

	ip := net.ParseIP("192.168.4.1")
	engine.OnConnect(ip, 445, "user")
	for i := 0; i < config.MaxFailedAttempts; i++ {
		engine.OnAuthFailure(ip, "user")
	}

	if !engine.IsIPBlocked("192.168.4.1") {
		t.Fatal("expected IP blocked")
	}

	// 等待过期
	time.Sleep(2 * time.Second)

	count := engine.CleanupExpired()
	if count == 0 {
		t.Error("expected at least 1 expired entry to be cleaned")
	}
}
