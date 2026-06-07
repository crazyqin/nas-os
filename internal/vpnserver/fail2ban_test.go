package vpnserver

import (
	"testing"
	"time"
)

// ==================== 基础封禁测试 ====================

func TestNewFail2Ban(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	cfg := fb.GetConfig()
	if !cfg.Enabled {
		t.Fatal("Fail2Ban 默认应启用")
	}
	if cfg.MaxAttempts != DefaultMaxAttempts {
		t.Fatalf("MaxAttempts 期望 %d，实际 %d", DefaultMaxAttempts, cfg.MaxAttempts)
	}
	if cfg.WindowSeconds != DefaultWindowSeconds {
		t.Fatalf("WindowSeconds 期望 %d，实际 %d", DefaultWindowSeconds, cfg.WindowSeconds)
	}
	if cfg.BanDurationSeconds != DefaultBanDurationSeconds {
		t.Fatalf("BanDurationSeconds 期望 %d，实际 %d", DefaultBanDurationSeconds, cfg.BanDurationSeconds)
	}
}

func TestRecordFailAttempt_NotBannedBeforeThreshold(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.100"
	// 记录4次失败（未达阈值5）
	for i := 0; i < 4; i++ {
		fb.RecordFailAttempt(ip, "testuser")
	}

	if fb.IsBanned(ip) {
		t.Fatal("4次失败不应被封禁")
	}

	attempts := fb.GetFailAttempts(ip)
	if len(attempts) != 4 {
		t.Fatalf("失败记录数期望 4，实际 %d", len(attempts))
	}
}

func TestRecordFailAttempt_AutoBanAtThreshold(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.100"
	// 记录5次失败（达到阈值）
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt(ip, "testuser")
	}

	if !fb.IsBanned(ip) {
		t.Fatal("5次失败应被自动封禁")
	}

	entry, exists := fb.GetBanEntry(ip)
	if !exists {
		t.Fatal("封禁记录应存在")
	}
	if !entry.Active {
		t.Fatal("封禁记录应为活跃状态")
	}
	if entry.BanCount != 1 {
		t.Fatalf("BanCount 期望 1，实际 %d", entry.BanCount)
	}
}

func TestRecordFailAttempt_WindowExpiry(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	// 设置很短的窗口用于测试
	fb.UpdateConfig(Fail2BanConfig{
		Enabled:                true,
		MaxAttempts:            5,
		WindowSeconds:          1, // 1秒窗口
		BanDurationSeconds:     1800,
		CleanupIntervalSeconds: 60,
	})

	ip := "192.168.1.101"
	// 记录3次
	for i := 0; i < 3; i++ {
		fb.RecordFailAttempt(ip, "user1")
	}

	// 等待窗口过期
	time.Sleep(1100 * time.Millisecond)

	// 再记录3次（之前的3次已过期，总计从新开始应为3次）
	for i := 0; i < 3; i++ {
		fb.RecordFailAttempt(ip, "user1")
	}

	// 3次不应封禁（窗口内的记录只有3次）
	if fb.IsBanned(ip) {
		t.Fatal("窗口过期后的失败记录不应累计到旧记录")
	}
}

func TestRecordFailAttempt_MultipleUsersSameIP(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "10.0.0.1"
	// 不同用户从同一IP失败
	fb.RecordFailAttempt(ip, "user1")
	fb.RecordFailAttempt(ip, "user2")
	fb.RecordFailAttempt(ip, "user3")
	fb.RecordFailAttempt(ip, "user4")

	if fb.IsBanned(ip) {
		t.Fatal("4次失败（不同用户）不应封禁")
	}

	fb.RecordFailAttempt(ip, "user5")

	if !fb.IsBanned(ip) {
		t.Fatal("同一IP 5次失败应被封禁，即使用户不同")
	}
}

// ==================== 解封测试 ====================

func TestUnblock(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.100"
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt(ip, "testuser")
	}

	if !fb.IsBanned(ip) {
		t.Fatal("应已被封禁")
	}

	// 手动解封
	err := fb.Unblock(ip)
	if err != nil {
		t.Fatalf("解封失败: %v", err)
	}

	if fb.IsBanned(ip) {
		t.Fatal("解封后不应被封禁")
	}

	// 解封不存在的IP应返回错误
	err = fb.Unblock("10.99.99.99")
	if err == nil {
		t.Fatal("解封不存在的IP应返回错误")
	}
}

func TestBanExpiry(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	// 设置很短的封禁时间
	fb.UpdateConfig(Fail2BanConfig{
		Enabled:                true,
		MaxAttempts:            3,
		WindowSeconds:          300,
		BanDurationSeconds:     1, // 1秒封禁
		CleanupIntervalSeconds: 1,
	})

	ip := "192.168.1.102"
	for i := 0; i < 3; i++ {
		fb.RecordFailAttempt(ip, "user")
	}

	if !fb.IsBanned(ip) {
		t.Fatal("应已被封禁")
	}

	// 等待封禁过期
	time.Sleep(1200 * time.Millisecond)

	if fb.IsBanned(ip) {
		t.Fatal("封禁过期后不应被封禁")
	}
}

// ==================== 白名单测试 ====================

func TestWhiteList_AddAndRemove(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "10.0.0.1"

	// 加入白名单
	fb.AddToWhiteList(ip)
	if !fb.IsWhiteListed(ip) {
		t.Fatal("应已在白名单中")
	}

	// 白名单IP不应被封禁
	for i := 0; i < 10; i++ {
		fb.RecordFailAttempt(ip, "admin")
	}
	if fb.IsBanned(ip) {
		t.Fatal("白名单IP不应被封禁")
	}

	// 移出白名单
	err := fb.RemoveFromWhiteList(ip)
	if err != nil {
		t.Fatalf("移出白名单失败: %v", err)
	}
	if fb.IsWhiteListed(ip) {
		t.Fatal("移出后不应在白名单中")
	}

	// 移出不在白名单中的IP应返回错误
	err = fb.RemoveFromWhiteList("10.99.99.99")
	if err == nil {
		t.Fatal("移出不在白名单中的IP应返回错误")
	}
}

func TestWhiteList_UnbanOnAdd(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.200"
	// 先封禁
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt(ip, "user")
	}
	if !fb.IsBanned(ip) {
		t.Fatal("应已被封禁")
	}

	// 加入白名单应同时解除封禁
	fb.AddToWhiteList(ip)
	if fb.IsBanned(ip) {
		t.Fatal("加入白名单后应解除封禁")
	}
	if !fb.IsWhiteListed(ip) {
		t.Fatal("应已在白名单中")
	}
}

// ==================== 状态查询测试 ====================

func TestGetStatus(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	// 设置白名单
	fb.AddToWhiteList("10.0.0.1")
	fb.AddToWhiteList("10.0.0.2")

	// 封禁一个IP
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt("192.168.1.1", "user1")
	}

	status := fb.GetStatus()

	if status.TotalBanned != 1 {
		t.Fatalf("TotalBanned 期望 1，实际 %d", status.TotalBanned)
	}
	if len(status.WhiteList) != 2 {
		t.Fatalf("WhiteList长度 期望 2，实际 %d", len(status.WhiteList))
	}
	if status.TotalEvents == 0 {
		t.Fatal("应有事件记录")
	}
	if len(status.RecentEvents) == 0 {
		t.Fatal("应有最近事件")
	}
}

func TestGetBannedIPs(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	// 封禁两个IP
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt("192.168.1.1", "user1")
		fb.RecordFailAttempt("192.168.1.2", "user2")
	}

	banned := fb.GetBannedIPs()
	if len(banned) != 2 {
		t.Fatalf("被封禁IP数 期望 2，实际 %d", len(banned))
	}
}

// ==================== 事件日志测试 ====================

func TestEventLog_Records(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.50"
	fb.RecordFailAttempt(ip, "user")

	status := fb.GetStatus()
	if len(status.RecentEvents) < 1 {
		t.Fatal("应至少有1条事件记录")
	}

	event := status.RecentEvents[len(status.RecentEvents)-1]
	if event.EventType != "fail_attempt" {
		t.Fatalf("事件类型期望 fail_attempt，实际 %s", event.EventType)
	}
	if event.IP != ip {
		t.Fatalf("事件IP期望 %s，实际 %s", ip, event.IP)
	}
}

func TestEventLog_BanEvent(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.51"
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt(ip, "user")
	}

	status := fb.GetStatus()
	found := false
	for _, event := range status.RecentEvents {
		if event.EventType == "banned" && event.IP == ip {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("应有封禁事件记录")
	}
}

func TestEventLog_UnbanEvent(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	ip := "192.168.1.52"
	for i := 0; i < 5; i++ {
		fb.RecordFailAttempt(ip, "user")
	}
	fb.Unblock(ip)

	status := fb.GetStatus()
	found := false
	for _, event := range status.RecentEvents {
		if event.EventType == "unbanned" && event.IP == ip {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("应有解封事件记录")
	}
}

// ==================== 配置测试 ====================

func TestUpdateConfig(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	newCfg := Fail2BanConfig{
		Enabled:                false,
		MaxAttempts:            10,
		WindowSeconds:          600,
		BanDurationSeconds:     3600,
		CleanupIntervalSeconds: 120,
	}
	fb.UpdateConfig(newCfg)

	cfg := fb.GetConfig()
	if cfg.Enabled {
		t.Fatal("Enabled 应为 false")
	}
	if cfg.MaxAttempts != 10 {
		t.Fatalf("MaxAttempts 期望 10，实际 %d", cfg.MaxAttempts)
	}
	if cfg.WindowSeconds != 600 {
		t.Fatalf("WindowSeconds 期望 600，实际 %d", cfg.WindowSeconds)
	}
	if cfg.BanDurationSeconds != 3600 {
		t.Fatalf("BanDurationSeconds 期望 3600，实际 %d", cfg.BanDurationSeconds)
	}
}

func TestDisabledConfig_NoBan(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	fb.UpdateConfig(Fail2BanConfig{
		Enabled:                false,
		MaxAttempts:            3,
		WindowSeconds:          300,
		BanDurationSeconds:     1800,
		CleanupIntervalSeconds: 60,
	})

	ip := "192.168.1.100"
	for i := 0; i < 10; i++ {
		fb.RecordFailAttempt(ip, "user")
	}

	if fb.IsBanned(ip) {
		t.Fatal("禁用状态下不应封禁")
	}
}

// ==================== 累计封禁测试 ====================

func TestBanCount_Increments(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	fb.UpdateConfig(Fail2BanConfig{
		Enabled:                true,
		MaxAttempts:            3,
		WindowSeconds:          300,
		BanDurationSeconds:     1, // 1秒封禁
		CleanupIntervalSeconds: 1,
	})

	ip := "192.168.1.200"

	// 第一次封禁
	for i := 0; i < 3; i++ {
		fb.RecordFailAttempt(ip, "user")
	}
	entry, _ := fb.GetBanEntry(ip)
	if entry.BanCount != 1 {
		t.Fatalf("首次封禁 BanCount 期望 1，实际 %d", entry.BanCount)
	}

	// 等待过期
	time.Sleep(1200 * time.Millisecond)

	// 第二次封禁
	for i := 0; i < 3; i++ {
		fb.RecordFailAttempt(ip, "user")
	}
	entry, _ = fb.GetBanEntry(ip)
	if entry.BanCount != 2 {
		t.Fatalf("第二次封禁 BanCount 期望 2，实际 %d", entry.BanCount)
	}
}

// ==================== 边界情况测试 ====================

func TestIsBanned_NonExistentIP(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	if fb.IsBanned("10.99.99.99") {
		t.Fatal("不存在的IP不应被封禁")
	}
}

func TestGetBanEntry_NonExistentIP(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	_, exists := fb.GetBanEntry("10.99.99.99")
	if exists {
		t.Fatal("不存在的IP不应有封禁记录")
	}
}

func TestGetFailAttempts_NonExistentIP(t *testing.T) {
	fb := NewFail2Ban()
	defer fb.Stop()

	attempts := fb.GetFailAttempts("10.99.99.99")
	if attempts != nil {
		t.Fatal("不存在的IP不应有失败记录")
	}
}
