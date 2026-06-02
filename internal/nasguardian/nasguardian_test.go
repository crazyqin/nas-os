// Package nasguardian 测试
package nasguardian

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	g := New(DefaultConfig())
	if g == nil {
		t.Fatal("实例不应为nil")
	}
	if g.IsRunning() {
		t.Error("新建实例不应处于运行状态")
	}
}

func TestNewWithCustomConfig(t *testing.T) {
	cfg := Config{
		ScanInterval:    10 * time.Minute,
		AlertThreshold:  50,
		AutoRepair:      false,
		MaxBlockedIPs:   500,
		BlockDuration:   12 * time.Hour,
		MaxThreatHistory: 5000,
		VulnScanEnabled: true,
	}
	g := New(cfg)
	c := g.GetConfig()
	if c.ScanInterval != 10*time.Minute {
		t.Errorf("扫描间隔不匹配: %v", c.ScanInterval)
	}
	if c.MaxBlockedIPs != 500 {
		t.Errorf("最大封锁IP数不匹配: %d", c.MaxBlockedIPs)
	}
}

func TestStartStop(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()

	if err := g.Start(ctx); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if !g.IsRunning() {
		t.Error("启动后应处于运行状态")
	}

	// 重复启动应报错
	if err := g.Start(ctx); err != ErrGuardianRunning {
		t.Errorf("重复启动应返回ErrGuardianRunning, 实际: %v", err)
	}

	if err := g.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if g.IsRunning() {
		t.Error("停止后不应处于运行状态")
	}

	// 重复停止应报错
	if err := g.Stop(); err != ErrGuardianNotRunning {
		t.Errorf("重复停止应返回ErrGuardianNotRunning, 实际: %v", err)
	}
}

func TestScanThreats(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	threats, err := g.ScanThreats(ctx)
	if err != nil {
		t.Fatalf("扫描威胁失败: %v", err)
	}
	if threats == nil {
		t.Error("扫描结果不应为nil")
	}
}

func TestScanThreatsNotRunning(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()

	_, err := g.ScanThreats(ctx)
	if err != ErrGuardianNotRunning {
		t.Errorf("未运行时扫描应返回ErrGuardianNotRunning, 实际: %v", err)
	}
}

func TestAddAndResolveThreat(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 添加威胁
	threatID := g.AddThreat(Threat{
		Type:        "brute_force",
		Level:       ThreatLevelHigh,
		Source:      "192.168.1.100",
		Description: "暴力破解尝试",
	})
	if threatID == "" {
		t.Fatal("威胁ID不应为空")
	}

	// 获取威胁
	threat, err := g.GetThreat(threatID)
	if err != nil {
		t.Fatalf("获取威胁失败: %v", err)
	}
	if threat.Type != "brute_force" {
		t.Errorf("威胁类型不匹配: %s", threat.Type)
	}
	if threat.Status != ThreatStatusActive {
		t.Errorf("威胁状态应为active, 实际: %s", threat.Status)
	}

	// 验证安全评分下降
	score := g.GetSecurityScore()
	if score.Overall >= 100 {
		t.Error("添加威胁后安全评分应下降")
	}
	if score.ThreatCount != 1 {
		t.Errorf("活跃威胁数应为1, 实际: %d", score.ThreatCount)
	}

	// 解决威胁
	if err := g.ResolveThreat(threatID); err != nil {
		t.Fatalf("解决威胁失败: %v", err)
	}

	threat, _ = g.GetThreat(threatID)
	if threat.Status != ThreatStatusResolved {
		t.Errorf("威胁状态应为resolved, 实际: %s", threat.Status)
	}
}

func TestGetThreatHistory(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 添加多个威胁
	for i := 0; i < 5; i++ {
		g.AddThreat(Threat{
			Type:        "test_threat",
			Level:       ThreatLevelLow,
			Description: "测试威胁",
		})
	}

	history := g.GetThreatHistory(3)
	if len(history) != 3 {
		t.Errorf("应返回3条历史记录, 实际: %d", len(history))
	}

	allHistory := g.GetThreatHistory(0)
	if len(allHistory) != 5 {
		t.Errorf("应返回全部5条记录, 实际: %d", len(allHistory))
	}
}

func TestVulnerabilityOperations(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 扫描漏洞（应无错误）
	_, err := g.ScanVulnerabilities(ctx)
	if err != nil {
		t.Fatalf("扫描漏洞失败: %v", err)
	}

	// 添加漏洞
	vulnID := g.AddVulnerability(Vulnerability{
		CVE:         "CVE-2024-1234",
		Severity:    VulnSeverityCritical,
		Title:       "远程代码执行漏洞",
		Description: "严重的远程代码执行漏洞",
		Affected:    "OpenSSL 1.1.1",
		FixVersion:  "OpenSSL 3.0.0",
	})
	if vulnID == "" {
		t.Fatal("漏洞ID不应为空")
	}

	vulns := g.GetVulnerabilities()
	if len(vulns) != 1 {
		t.Errorf("应有1个漏洞, 实际: %d", len(vulns))
	}

	// 验证安全评分下降
	score := g.GetSecurityScore()
	if score.VulnCount != 1 {
		t.Errorf("未修复漏洞数应为1, 实际: %d", score.VulnCount)
	}

	// 修复漏洞
	if err := g.FixVulnerability(vulnID); err != nil {
		t.Fatalf("修复漏洞失败: %v", err)
	}

	score = g.GetSecurityScore()
	if score.VulnCount != 0 {
		t.Errorf("修复后漏洞数应为0, 实际: %d", score.VulnCount)
	}
}

func TestBlockUnblockIP(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 封锁IP
	err := g.BlockIP(ctx, "10.0.0.1", time.Hour)
	if err != nil {
		t.Fatalf("封锁IP失败: %v", err)
	}

	if !g.IsIPBlocked("10.0.0.1") {
		t.Error("IP应被封锁")
	}

	blocked := g.GetBlockedIPs()
	if len(blocked) != 1 {
		t.Errorf("封锁列表应有1条, 实际: %d", len(blocked))
	}

	// 无效IP
	err = g.BlockIP(ctx, "invalid-ip", time.Hour)
	if err != ErrInvalidIP {
		t.Errorf("无效IP应返回ErrInvalidIP, 实际: %v", err)
	}

	// 解除封锁
	if err := g.UnblockIP("10.0.0.1"); err != nil {
		t.Fatalf("解除封锁失败: %v", err)
	}

	if g.IsIPBlocked("10.0.0.1") {
		t.Error("解除封锁后IP不应被封锁")
	}

	// 解除不存在的封锁
	if err := g.UnblockIP("10.0.0.99"); err != ErrIPNotBlocked {
		t.Errorf("不存在的IP应返回ErrIPNotBlocked, 实际: %v", err)
	}
}

func TestSecurityRules(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 添加规则
	ruleID := g.AddRule(SecurityRule{
		Name:        "暴力破解检测",
		Category:    HardeningAuth,
		Enabled:     true,
		Description: "检测连续登录失败",
		Condition:   "login_failures > 5",
		Action:      "block_ip",
		Severity:    ThreatLevelHigh,
	})
	if ruleID == "" {
		t.Fatal("规则ID不应为空")
	}

	rules := g.GetRules()
	if len(rules) != 1 {
		t.Errorf("应有1条规则, 实际: %d", len(rules))
	}

	// 评估规则
	triggered, err := g.EvaluateRules(ctx)
	if err != nil {
		t.Fatalf("评估规则失败: %v", err)
	}
	if triggered == nil {
		t.Error("评估结果不应为nil")
	}

	// 删除规则
	if err := g.RemoveRule(ruleID); err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}

	rules = g.GetRules()
	if len(rules) != 0 {
		t.Errorf("删除后应有0条规则, 实际: %d", len(rules))
	}

	// 删除不存在的规则
	if err := g.RemoveRule("nonexistent"); err != ErrRuleNotFound {
		t.Errorf("不存在的规则应返回ErrRuleNotFound, 实际: %v", err)
	}
}

func TestApplyHardening(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	err := g.ApplyHardening(ctx, HardeningTask{
		Name:          "启用防火墙",
		Category:      HardeningNetwork,
		Description:  "启用基础防火墙规则",
		Rollback:     true,
	})
	if err != nil {
		t.Fatalf("应用加固失败: %v", err)
	}

	tasks := g.GetHardeningTasks()
	if len(tasks) != 1 {
		t.Errorf("应有1个加固任务, 实际: %d", len(tasks))
	}
	if !tasks[0].Applied {
		t.Error("任务应已应用")
	}
}

func TestGenerateSecurityReport(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 添加一些数据
	g.AddThreat(Threat{
		Type:        "malware",
		Level:       ThreatLevelCritical,
		Description: "恶意软件检测",
	})
	g.AddVulnerability(Vulnerability{
		CVE:      "CVE-2024-5678",
		Severity: VulnSeverityHigh,
		Title:    "权限提升漏洞",
	})
	g.AddRule(SecurityRule{
		Name:    "测试规则",
		Enabled: true,
	})
	g.BlockIP(ctx, "192.168.1.50", time.Hour)

	report := g.GenerateSecurityReport()

	if report.Score.Overall >= 100 {
		t.Error("报告评分不应为满分")
	}
	if report.ActiveThreats != 1 {
		t.Errorf("活跃威胁数应为1, 实际: %d", report.ActiveThreats)
	}
	if report.TotalThreats != 1 {
		t.Errorf("总威胁数应为1, 实际: %d", report.TotalThreats)
	}
	if report.OpenVulns != 1 {
		t.Errorf("未修复漏洞应为1, 实际: %d", report.OpenVulns)
	}
	if report.BlockedIPs != 1 {
		t.Errorf("封锁IP数应为1, 实际: %d", report.BlockedIPs)
	}
	if report.GeneratedAt.IsZero() {
		t.Error("报告生成时间不应为零值")
	}
}

func TestExpiredBlockCleanup(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 添加一个已过期的封锁
	g.mu.Lock()
	g.blockedIPs["10.0.0.50"] = &BlockedIP{
		IP:        "10.0.0.50",
		Reason:    "测试",
		BlockedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}
	g.mu.Unlock()

	count := g.CleanupExpiredBlocks()
	if count != 1 {
		t.Errorf("应清理1个过期封锁, 实际: %d", count)
	}

	if g.IsIPBlocked("10.0.0.50") {
		t.Error("过期的封锁应已被清理")
	}
}

func TestSecurityScoreCalculation(t *testing.T) {
	g := New(DefaultConfig())
	ctx := context.Background()
	g.Start(ctx)
	defer g.Stop()

	// 初始评分应为满分
	score := g.GetSecurityScore()
	if score.Overall != 100 {
		t.Errorf("初始总分应为100, 实际: %d", score.Overall)
	}

	// 添加高危威胁
	g.AddThreat(Threat{
		Type:   "intrusion",
		Level:  ThreatLevelCritical,
		Status: ThreatStatusActive,
	})

	score = g.GetSecurityScore()
	if score.Overall >= 100 {
		t.Error("添加威胁后评分应下降")
	}
	if score.ThreatCount != 1 {
		t.Errorf("威胁计数应为1, 实际: %d", score.ThreatCount)
	}
}
