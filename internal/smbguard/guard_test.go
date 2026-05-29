package smbguard

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestGuard_BasicBan(t *testing.T) {
	config := DefaultConfig()
	config.MaxAttempts = 3
	config.WindowDuration = 1 * time.Minute
	config.TempBanDuration = 5 * time.Minute

	guard := NewGuard(config)
	ctx := context.Background()
	if err := guard.Start(ctx); err != nil {
		t.Fatalf("启动守卫失败: %v", err)
	}
	defer guard.Stop()

	ip := "192.168.1.100"

	// 记录失败尝试
	for i := 0; i < 2; i++ {
		entry := guard.RecordFailedAttempt(ip, "admin", "密码错误")
		if entry != nil {
			t.Fatalf("第 %d 次尝试不应被封禁", i+1)
		}
	}

	// 第 3 次应该触发封禁
	entry := guard.RecordFailedAttempt(ip, "admin", "密码错误")
	if entry == nil {
		t.Fatal("第 3 次尝试应该触发封禁")
	}

	if entry.Level != BanLevelTemp {
		t.Errorf("期望封禁级别 %v，实际 %v", BanLevelTemp, entry.Level)
	}

	// 检查封禁状态
	banned, banEntry := guard.IsBanned(ip)
	if !banned {
		t.Fatal("IP 应该被封禁")
	}
	if banEntry.IP != ip {
		t.Errorf("封禁 IP 不匹配: %s vs %s", banEntry.IP, ip)
	}
}

func TestGuard_Whitelist(t *testing.T) {
	config := DefaultConfig()
	config.MaxAttempts = 2
	config.WhitelistCIDRs = []string{"10.0.0.0/8"}

	guard := NewGuard(config)

	ip := "10.0.0.1"

	// 白名单 IP 不应被封禁
	for i := 0; i < 10; i++ {
		entry := guard.RecordFailedAttempt(ip, "admin", "密码错误")
		if entry != nil {
			t.Fatal("白名单 IP 不应被封禁")
		}
	}

	banned, _ := guard.IsBanned(ip)
	if banned {
		t.Fatal("白名单 IP 不应被封禁")
	}
}

func TestGuard_Escalation(t *testing.T) {
	config := DefaultConfig()
	config.MaxAttempts = 2
	config.WindowDuration = 1 * time.Hour
	config.EnableAutoEscalate = true

	guard := NewGuard(config)
	ctx := context.Background()
	guard.Start(ctx)
	defer guard.Stop()

	ip := "192.168.1.200"

	// 第一次封禁 - 应该是临时封禁
	for i := 0; i < 2; i++ {
		guard.RecordFailedAttempt(ip, "admin", "密码错误")
	}
	entry1 := guard.banned[ip]
	if entry1.Level != BanLevelTemp {
		t.Errorf("第一次封禁应为临时，实际 %v", entry1.Level)
	}

	// 解除封禁
	guard.ReleaseBan(ip)

	// 第二次封禁 - 应该升级为中期
	for i := 0; i < 2; i++ {
		guard.RecordFailedAttempt(ip, "admin", "密码错误")
	}
	entry2 := guard.banned[ip]
	if entry2.Level != BanLevelMedium {
		t.Errorf("第二次封禁应为中期，实际 %v", entry2.Level)
	}
}

func TestGuard_WhitelistAddRemove(t *testing.T) {
	config := DefaultConfig()
	guard := NewGuard(config)

	// 添加白名单
	err := guard.AddWhitelist("172.16.0.0/12")
	if err != nil {
		t.Fatalf("添加白名单失败: %v", err)
	}

	// 验证白名单
	ip := "172.16.0.100"
	banned, _ := guard.IsBanned(ip)
	if banned {
		t.Fatal("白名单 IP 不应被封禁")
	}

	// 移除白名单
	err = guard.RemoveWhitelist("172.16.0.0/12")
	if err != nil {
		t.Fatalf("移除白名单失败: %v", err)
	}
}

func TestGuard_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	config.MaxAttempts = 100

	guard := NewGuard(config)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := net.IPv4(192, 168, 1, byte(id))
			for j := 0; j < 50; j++ {
				guard.RecordFailedAttempt(ip.String(), "user", "test")
				guard.IsBanned(ip.String())
				guard.GetStats()
			}
		}(i)
	}
	wg.Wait()
}

func TestGuard_Stats(t *testing.T) {
	config := DefaultConfig()
	config.MaxAttempts = 100

	guard := NewGuard(config)

	// 记录一些尝试
	guard.RecordFailedAttempt("10.0.0.1", "admin", "密码错误")
	guard.RecordFailedAttempt("10.0.0.2", "user", "密码错误")
	guard.RecordFailedAttempt("10.0.0.1", "admin", "密码错误")

	stats := guard.GetStats()

	if stats.TotalAttempts != 3 {
		t.Errorf("期望总尝试次数 3，实际 %d", stats.TotalAttempts)
	}

	if len(stats.TopAttackers) == 0 {
		t.Error("TopAttackers 不应为空")
	}
}

func TestBanLevel_String(t *testing.T) {
	tests := []struct {
		level    BanLevel
		expected string
	}{
		{BanLevelWarn, "warn"},
		{BanLevelTemp, "temporary"},
		{BanLevelMedium, "medium"},
		{BanLevelPerm, "permanent"},
		{BanLevel(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("BanLevel(%d).String() = %s, 期望 %s", tt.level, got, tt.expected)
		}
	}
}
