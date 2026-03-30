package smb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ========== SecurityManager 测试 ==========

func TestNewSecurityManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	if sm == nil {
		t.Fatal("SecurityManager 不应为 nil")
	}

	if sm.config == nil {
		t.Error("config 不应为 nil")
	}

	if sm.bannedIPs == nil {
		t.Error("bannedIPs 不应为 nil")
	}

	if sm.failedAttempts == nil {
		t.Error("failedAttempts 不应为 nil")
	}
}

func TestCheckIPAllowed_BannedIP(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 手动封禁一个IP
	sm.mu.Lock()
	sm.bannedIPs["192.168.1.100"] = &IPBanEntry{
		IP:        "192.168.1.100",
		Reason:    "test",
		BannedAt:  time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	sm.mu.Unlock()

	// 检查封禁的IP
	allowed, reason := sm.CheckIPAllowed("192.168.1.100")
	if allowed {
		t.Error("封禁的IP应该被拒绝")
	}
	if reason == "" {
		t.Error("应该返回拒绝原因")
	}
}

func TestCheckIPAllowed_Blacklist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 设置黑名单
	sm.mu.Lock()
	sm.config.IPBlacklist = []string{"10.0.0.50", "10.0.0.51"}
	sm.mu.Unlock()

	allowed, _ := sm.CheckIPAllowed("10.0.0.50")
	if allowed {
		t.Error("黑名单IP应该被拒绝")
	}
}

func TestCheckIPAllowed_Whitelist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 设置白名单
	sm.mu.Lock()
	sm.config.IPWhitelist = []string{"192.168.1.0/24"}
	sm.mu.Unlock()

	// 白名单内的IP应该被允许
	allowed, _ := sm.CheckIPAllowed("192.168.1.100")
	if !allowed {
		t.Error("白名单IP应该被允许")
	}

	// 白名单外的IP应该被拒绝
	allowed, _ = sm.CheckIPAllowed("10.0.0.1")
	if allowed {
		t.Error("非白名单IP应该被拒绝")
	}
}

func TestCheckIPAllowed_CIDR(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		ip      string
		want    bool
	}{
		{"精确匹配", "192.168.1.100", "192.168.1.100", true},
		{"精确不匹配", "192.168.1.100", "192.168.1.101", false},
		{"CIDR /24", "192.168.1.0/24", "192.168.1.100", true},
		{"CIDR /24 外部", "192.168.1.0/24", "192.168.2.1", false},
		{"CIDR /16", "10.0.0.0/16", "10.0.1.1", true},
		{"无效CIDR当作精确匹配", "invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "security.json")
			logger := zap.NewNop().Sugar()
			sm := NewSecurityManager(configPath, logger)

			sm.mu.Lock()
			sm.config.IPWhitelist = []string{tt.pattern}
			sm.mu.Unlock()

			allowed, _ := sm.CheckIPAllowed(tt.ip)
			if allowed != tt.want {
				t.Errorf("CheckIPAllowed(%s, %s) = %v, want %v", tt.pattern, tt.ip, allowed, tt.want)
			}
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 启用限流，设置最大连接数为2
	sm.mu.Lock()
	sm.config.RateLimit.Enabled = true
	sm.config.RateLimit.MaxConnPerIP = 2
	sm.config.RateLimit.MaxConnTotal = 100
	sm.mu.Unlock()

	// 前两个连接应该成功
	for i := 0; i < 2; i++ {
		allowed, _ := sm.CheckRateLimit("192.168.1.100")
		if !allowed {
			t.Errorf("第 %d 个连接应该被允许", i+1)
		}
		sm.IncrementConnection("192.168.1.100")
	}

	// 第三个连接应该被拒绝
	allowed, _ := sm.CheckRateLimit("192.168.1.100")
	if allowed {
		t.Error("超过限流的连接应该被拒绝")
	}
}

func TestRecordFailedAttempt_AutoBan(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 设置自动封禁阈值为3
	sm.mu.Lock()
	sm.config.AutoBanEnabled = true
	sm.config.AutoBanThreshold = 3
	sm.config.AutoBanDurationMins = 30
	sm.config.AutoBanWindowMins = 5
	sm.mu.Unlock()

	// 记录失败尝试
	for i := 0; i < 3; i++ {
		sm.RecordFailedAttempt("192.168.1.200", "admin", "invalid password")
	}

	// 检查是否被封禁
	sm.mu.RLock()
	ban, exists := sm.bannedIPs["192.168.1.200"]
	sm.mu.RUnlock()

	if !exists {
		t.Fatal("IP应该被自动封禁")
	}

	if ban.Reason != "暴力破解检测" {
		t.Errorf("封禁原因错误: %s", ban.Reason)
	}
}

func TestRecordFailedAttempt_WhitelistNotBanned(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 白名单IP不应该被自动封禁
	sm.mu.Lock()
	sm.config.AutoBanEnabled = true
	sm.config.AutoBanThreshold = 3
	sm.config.IPWhitelist = []string{"192.168.1.250"}
	sm.mu.Unlock()

	// 记录大量失败尝试
	for i := 0; i < 5; i++ {
		sm.RecordFailedAttempt("192.168.1.250", "admin", "invalid password")
	}

	// 白名单IP不应该被封禁
	sm.mu.RLock()
	_, exists := sm.bannedIPs["192.168.1.250"]
	sm.mu.RUnlock()

	if exists {
		t.Error("白名单IP不应该被自动封禁")
	}
}

func TestBanIP(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 封禁一个IP
	err := sm.BanIP("192.168.1.100", "test ban", 60)
	if err != nil {
		t.Fatalf("BanIP 失败: %v", err)
	}

	// 验证封禁
	sm.mu.RLock()
	ban, exists := sm.bannedIPs["192.168.1.100"]
	sm.mu.RUnlock()

	if !exists {
		t.Fatal("IP应该被封禁")
	}

	if ban.Reason != "test ban" {
		t.Errorf("封禁原因错误: %s", ban.Reason)
	}
}

func TestBanIP_WhitelistForbidden(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 添加到白名单
	sm.mu.Lock()
	sm.config.IPWhitelist = []string{"192.168.1.100"}
	sm.mu.Unlock()

	// 尝试封禁白名单IP应该失败
	err := sm.BanIP("192.168.1.100", "test ban", 60)
	if err == nil {
		t.Error("封禁白名单IP应该失败")
	}
}

func TestUnbanIP(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 先封禁
	sm.mu.Lock()
	sm.bannedIPs["192.168.1.100"] = &IPBanEntry{
		IP:        "192.168.1.100",
		Reason:    "test",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	sm.mu.Unlock()

	// 解封
	err := sm.UnbanIP("192.168.1.100")
	if err != nil {
		t.Fatalf("UnbanIP 失败: %v", err)
	}

	// 验证解封
	sm.mu.RLock()
	_, exists := sm.bannedIPs["192.168.1.100"]
	sm.mu.RUnlock()

	if exists {
		t.Error("IP应该已被解封")
	}
}

func TestUnbanIP_NotBanned(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 尝试解封未封禁的IP
	err := sm.UnbanIP("192.168.1.100")
	if err == nil {
		t.Error("解封未封禁的IP应该返回错误")
	}
}

func TestGetBannedIPs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 添加封禁记录
	sm.mu.Lock()
	sm.bannedIPs["192.168.1.100"] = &IPBanEntry{
		IP:        "192.168.1.100",
		Reason:    "test",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	sm.bannedIPs["192.168.1.101"] = &IPBanEntry{
		IP:        "192.168.1.101",
		Reason:    "test",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}
	sm.mu.Unlock()

	banned := sm.GetBannedIPs()

	if len(banned) != 1 {
		t.Errorf("应该返回1个未过期的封禁记录，实际: %d", len(banned))
	}

	if len(banned) > 0 && banned[0].IP != "192.168.1.100" {
		t.Error("应该返回未过期的封禁记录")
	}
}

func TestAddToWhitelist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	err := sm.AddToWhitelist("192.168.1.100")
	if err != nil {
		t.Fatalf("AddToWhitelist 失败: %v", err)
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.config.IPWhitelist) != 1 {
		t.Error("白名单应该有1个IP")
	}

	if sm.config.IPWhitelist[0] != "192.168.1.100" {
		t.Error("白名单IP不正确")
	}
}

func TestAddToWhitelist_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 添加两次相同的IP
	sm.AddToWhitelist("192.168.1.100")
	sm.AddToWhitelist("192.168.1.100")

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.config.IPWhitelist) != 1 {
		t.Error("重复添加不应创建重复记录")
	}
}

func TestRemoveFromWhitelist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	sm.mu.Lock()
	sm.config.IPWhitelist = []string{"192.168.1.100", "192.168.1.101"}
	sm.mu.Unlock()

	err := sm.RemoveFromWhitelist("192.168.1.100")
	if err != nil {
		t.Fatalf("RemoveFromWhitelist 失败: %v", err)
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.config.IPWhitelist) != 1 {
		t.Error("白名单应该只剩1个IP")
	}

	if sm.config.IPWhitelist[0] != "192.168.1.101" {
		t.Error("剩余IP不正确")
	}
}

func TestAddToBlacklist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	err := sm.AddToBlacklist("10.0.0.1")
	if err != nil {
		t.Fatalf("AddToBlacklist 失败: %v", err)
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.config.IPBlacklist) != 1 {
		t.Error("黑名单应该有1个IP")
	}
}

func TestAddToBlacklist_WhitelistConflict(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 先添加到白名单
	sm.mu.Lock()
	sm.config.IPWhitelist = []string{"192.168.1.100"}
	sm.mu.Unlock()

	// 尝试添加到黑名单应该失败
	err := sm.AddToBlacklist("192.168.1.100")
	if err == nil {
		t.Error("白名单IP不能添加到黑名单")
	}
}

func TestRemoveFromBlacklist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	sm.mu.Lock()
	sm.config.IPBlacklist = []string{"10.0.0.1", "10.0.0.2"}
	sm.mu.Unlock()

	err := sm.RemoveFromBlacklist("10.0.0.1")
	if err != nil {
		t.Fatalf("RemoveFromBlacklist 失败: %v", err)
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.config.IPBlacklist) != 1 {
		t.Error("黑名单应该只剩1个IP")
	}
}

func TestIncrementDecrementConnection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 增加连接
	sm.IncrementConnection("192.168.1.100")
	sm.IncrementConnection("192.168.1.100")
	sm.IncrementConnection("192.168.1.101")

	sm.mu.RLock()
	if sm.connCounts["192.168.1.100"] != 2 {
		t.Errorf("IP连接计数错误: %d", sm.connCounts["192.168.1.100"])
	}
	if sm.totalConns != 3 {
		t.Errorf("总连接计数错误: %d", sm.totalConns)
	}
	sm.mu.RUnlock()

	// 减少连接
	sm.DecrementConnection("192.168.1.100")

	sm.mu.RLock()
	if sm.connCounts["192.168.1.100"] != 1 {
		t.Errorf("减少后IP连接计数错误: %d", sm.connCounts["192.168.1.100"])
	}
	if sm.totalConns != 2 {
		t.Errorf("减少后总连接计数错误: %d", sm.totalConns)
	}
	sm.mu.RUnlock()
}

func TestCleanupExpiredBans(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 添加过期和未过期的封禁
	sm.mu.Lock()
	sm.bannedIPs["192.168.1.100"] = &IPBanEntry{
		IP:        "192.168.1.100",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}
	sm.bannedIPs["192.168.1.101"] = &IPBanEntry{
		IP:        "192.168.1.101",
		ExpiresAt: time.Now().Add(1 * time.Hour), // 未过期
	}
	sm.mu.Unlock()

	count := sm.CleanupExpiredBans()

	if count != 1 {
		t.Errorf("应该清理1条过期记录，实际: %d", count)
	}

	sm.mu.RLock()
	_, exists1 := sm.bannedIPs["192.168.1.100"]
	_, exists2 := sm.bannedIPs["192.168.1.101"]
	sm.mu.RUnlock()

	if exists1 {
		t.Error("过期记录应该被清理")
	}
	if !exists2 {
		t.Error("未过期记录不应该被清理")
	}
}

func TestLoadSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 修改配置
	newConfig := &SMBSecurityConfig{
		IPWhitelist: []string{"192.168.1.0/24"},
		IPBlacklist: []string{"10.0.0.0/8"},
		RateLimit: RateLimitConfig{
			Enabled:       true,
			MaxConnPerIP:  5,
			MaxConnTotal:  500,
			WindowSeconds: 60,
		},
		AutoBanEnabled:      true,
		AutoBanThreshold:    5,
		AutoBanWindowMins:   5,
		AutoBanDurationMins: 30,
	}

	err := sm.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("UpdateConfig 失败: %v", err)
	}

	// 创建新的 SecurityManager 加载配置
	sm2 := NewSecurityManager(configPath, logger)

	sm2.mu.RLock()
	if len(sm2.config.IPWhitelist) != 1 {
		t.Error("白名单配置未保存")
	}
	if len(sm2.config.IPBlacklist) != 1 {
		t.Error("黑名单配置未保存")
	}
	if sm2.config.RateLimit.MaxConnPerIP != 5 {
		t.Error("限流配置未保存")
	}
	sm2.mu.RUnlock()
}

func TestGetSetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// GetConfig 应该返回非nil
	cfg := sm.GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig 不应返回 nil")
	}

	// 默认值检查
	if cfg.RateLimit.MaxConnPerIP != 10 {
		t.Errorf("默认 MaxConnPerIP 应为 10，实际: %d", cfg.RateLimit.MaxConnPerIP)
	}

	if cfg.AutoBanThreshold != 5 {
		t.Errorf("默认 AutoBanThreshold 应为 5，实际: %d", cfg.AutoBanThreshold)
	}
}

// ========== AuditLogger 测试 ==========

func TestAuditLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.json")

	logger := zap.NewNop().Sugar()
	auditLog, err := NewAuditLogger(logPath, logger)
	if err != nil {
		t.Fatalf("创建 AuditLogger 失败: %v", err)
	}
	defer auditLog.Close()

	// 记录日志
	entry := AuditLogEntry{
		Timestamp: time.Now(),
		EventType: "test",
		IP:        "192.168.1.100",
		Username:  "admin",
		Action:    "login",
		Result:    "success",
	}

	auditLog.Log(entry)

	// 验证日志已写入
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("日志文件不应为空")
	}
}

func TestAuditLogger_LogAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.json")

	logger := zap.NewNop().Sugar()
	auditLog, err := NewAuditLogger(logPath, logger)
	if err != nil {
		t.Fatalf("创建 AuditLogger 失败: %v", err)
	}

	// 记录多条日志
	for i := 0; i < 5; i++ {
		auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			EventType: "test",
			IP:        "192.168.1.100",
			Username:  "admin",
			Action:    "login",
			Result:    "success",
		})
	}

	auditLog.Close()

	// 重新打开并读取
	auditLog2, err := NewAuditLogger(logPath, logger)
	if err != nil {
		t.Fatalf("重新打开 AuditLogger 失败: %v", err)
	}
	defer auditLog2.Close()

	logs, err := auditLog2.ReadLogs(10, 0)
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}

	if len(logs) != 5 {
		t.Errorf("应该读取5条日志，实际: %d", len(logs))
	}
}

// ========== matchIP 测试 ==========

func TestMatchIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		pattern string
		want    bool
	}{
		{"精确匹配-成功", "192.168.1.1", "192.168.1.1", true},
		{"精确匹配-失败", "192.168.1.1", "192.168.1.2", false},
		{"CIDR /24-内部", "192.168.1.100", "192.168.1.0/24", true},
		{"CIDR /24-外部", "192.168.2.1", "192.168.1.0/24", false},
		{"CIDR /16-内部", "10.0.5.1", "10.0.0.0/16", true},
		{"CIDR /16-外部", "10.1.0.1", "10.0.0.0/16", false},
		{"CIDR /32-精确", "192.168.1.1", "192.168.1.1/32", true},
		{"无效CIDR-精确匹配", "invalid", "invalid", true},
		{"IPv6", "::1", "::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchIP(tt.ip, tt.pattern)
			if got != tt.want {
				t.Errorf("matchIP(%s, %s) = %v, want %v", tt.ip, tt.pattern, got, tt.want)
			}
		})
	}
}

// ========== ReverseScanner 测试 ==========

func TestReverseScanner(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// 创建测试文件
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := NewReverseScanner(file)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, string(scanner.Bytes()))
	}

	// 反向读取应该得到倒序
	expected := []string{"line5", "line4", "line3", "line2", "line1"}

	if len(lines) != len(expected) {
		t.Fatalf("行数错误: got %d, want %d", len(lines), len(expected))
	}

	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("第 %d 行: got %q, want %q", i, lines[i], exp)
		}
	}
}

// ========== 并发安全测试 ==========

func TestSecurityManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	var wg sync.WaitGroup

	// 并发读
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				sm.CheckIPAllowed("192.168.1.100")
				sm.GetConfig()
			}
		}()
	}

	// 并发写
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				sm.BanIP("192.168.1.100", "test", 60)
				sm.UnbanIP("192.168.1.100")
			}
		}(i)
	}

	wg.Wait()
}

// ========== 边界条件测试 ==========

func TestSecurityManager_EmptyIP(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	// 空IP不会被特殊处理，正常检查
	allowed, reason := sm.CheckIPAllowed("")
	// 无白名单时空IP会被允许（因为matchIP("", "")返回true）
	_ = allowed
	_ = reason
}

func TestSecurityManager_ManyFailedAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.json")

	logger := zap.NewNop().Sugar()
	sm := NewSecurityManager(configPath, logger)

	sm.mu.Lock()
	sm.config.AutoBanEnabled = true
	sm.config.AutoBanThreshold = 3
	sm.config.AutoBanWindowMins = 5
	sm.config.AutoBanDurationMins = 30
	sm.mu.Unlock()

	// 记录超过阈值的失败尝试
	for i := 0; i < 10; i++ {
		sm.RecordFailedAttempt("192.168.1.200", "admin", "invalid password")
	}

	// 应该只被封禁一次
	sm.mu.RLock()
	ban, exists := sm.bannedIPs["192.168.1.200"]
	sm.mu.RUnlock()

	if !exists {
		t.Fatal("IP应该被封禁")
	}

	// 清理过期记录后重试应该触发新的封禁
	sm.mu.Lock()
	sm.cleanupFailedAttemptsLocked("192.168.1.200")
	sm.mu.Unlock()

	// 重新积累失败尝试
	for i := 0; i < 3; i++ {
		sm.RecordFailedAttempt("192.168.1.200", "admin", "invalid password")
	}

	// 应该仍然被封禁（因为是幂等的检查）
	sm.mu.RLock()
	ban2, exists2 := sm.bannedIPs["192.168.1.200"]
	sm.mu.RUnlock()

	if !exists2 {
		t.Error("IP应该仍然被封禁")
	}
	_ = ban
	_ = ban2
}
