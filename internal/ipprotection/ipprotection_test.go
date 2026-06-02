package ipprotection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestManagerWithConfig(t *testing.T, config *IPProtectionConfig) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), config)
}

func testConfig() *IPProtectionConfig {
	cfg := DefaultIPProtectionConfig()
	cfg.LoginFailureThreshold = 3
	cfg.LoginFailureWindow = 5 * time.Minute
	cfg.AutoBanDuration = 30 * time.Minute
	cfg.RateLimitRequestsPerSecond = 100
	cfg.RateLimitBurst = 200
	cfg.PortScanThreshold = 5
	cfg.PortScanWindow = 1 * time.Minute
	cfg.BruteForceThreshold = 5
	cfg.BruteForceWindow = 1 * time.Minute
	cfg.WhitelistedIPs = []string{"127.0.0.1", "::1"}
	cfg.BlacklistedIPs = []string{}
	return cfg
}

func TestDefaultIPProtectionConfig(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 5, cfg.LoginFailureThreshold)
	assert.Equal(t, 10*time.Minute, cfg.LoginFailureWindow)
	assert.Equal(t, 1*time.Hour, cfg.AutoBanDuration)
	assert.True(t, cfg.EnableAutoBan)
	assert.Equal(t, float64(10), cfg.RateLimitRequestsPerSecond)
	assert.Equal(t, 20, cfg.RateLimitBurst)
	assert.Equal(t, 100, cfg.InitialReputationScore)
	assert.Equal(t, 20, cfg.MinReputationScore)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, ListType("allow"), ListTypeAllow)
	assert.Equal(t, ListType("deny"), ListTypeDeny)
	assert.Equal(t, BanReason("login_failure"), BanReasonLoginFailure)
	assert.Equal(t, BanReason("brute_force"), BanReasonBruteForce)
	assert.Equal(t, BanReason("port_scan"), BanReasonPortScan)
	assert.Equal(t, BanReason("rate_limit"), BanReasonRateLimit)
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	assert.NotNil(t, m)
	assert.NotNil(t, m.detector)
	assert.NotNil(t, m.rateLimiter)
	assert.NotNil(t, m.records)
	assert.NotNil(t, m.allowList)
	assert.NotNil(t, m.denyList)
}

func TestNewManagerNilConfig(t *testing.T) {
	m := NewManager(nil, nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.config)
}

func TestManagerStartStop(t *testing.T) {
	m := setupTestManager(t)
	m.Start()
	assert.True(t, m.running)
	m.Start()
	m.Stop()
	assert.False(t, m.running)
	m.Stop()
}

func TestManagerInitLists(t *testing.T) {
	cfg := testConfig()
	cfg.WhitelistedIPs = []string{"10.0.0.1", "192.168.1.1"}
	cfg.BlacklistedIPs = []string{"172.16.0.1"}
	m := setupTestManagerWithConfig(t, cfg)
	allowList := m.GetAllowList()
	assert.Len(t, allowList, 2)
	denyList := m.GetDenyList()
	assert.Len(t, denyList, 1)
}

func TestAddToAllowList(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToAllowList("10.0.0.1", "测试白名单", 0)
	require.NoError(t, err)
	assert.True(t, m.isWhitelisted("10.0.0.1"))
}

func TestAddToAllowListIPv6(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToAllowList("2001:db8::1", "IPv6 白名单", 0)
	require.NoError(t, err)
	assert.True(t, m.isWhitelisted("2001:db8::1"))
}

func TestAddToAllowListInvalidIP(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToAllowList("invalid-ip", "测试", 0)
	assert.Error(t, err)
}

func TestAddToAllowListWithDuration(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToAllowList("10.0.0.2", "临时白名单", 1*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	assert.False(t, m.isWhitelisted("10.0.0.2"))
}

func TestRemoveFromAllowList(t *testing.T) {
	m := setupTestManager(t)
	m.AddToAllowList("10.0.0.3", "测试", 0)
	assert.True(t, m.isWhitelisted("10.0.0.3"))
	m.RemoveFromAllowList("10.0.0.3")
	assert.False(t, m.isWhitelisted("10.0.0.3"))
}

func TestAddToDenyList(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToDenyList("10.0.0.10", BanReasonManual, "恶意 IP", 0)
	require.NoError(t, err)
	assert.True(t, m.isBlacklisted("10.0.0.10"))
}

func TestAddToDenyListIPv6(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToDenyList("2001:db8::10", BanReasonManual, "恶意 IPv6", 0)
	require.NoError(t, err)
	assert.True(t, m.isBlacklisted("2001:db8::10"))
}

func TestAddToDenyListInvalidIP(t *testing.T) {
	m := setupTestManager(t)
	err := m.AddToDenyList("not-an-ip", BanReasonManual, "", 0)
	assert.Error(t, err)
}

func TestDenyListCannotBanWhitelistedIP(t *testing.T) {
	m := setupTestManager(t)
	m.AddToAllowList("10.0.0.5", "白名单", 0)
	err := m.AddToDenyList("10.0.0.5", BanReasonManual, "尝试封禁", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "白名单")
}

func TestRemoveFromDenyList(t *testing.T) {
	m := setupTestManager(t)
	m.AddToDenyList("10.0.0.11", BanReasonManual, "测试", 0)
	assert.True(t, m.isBlacklisted("10.0.0.11"))
	m.RemoveFromDenyList("10.0.0.11")
	assert.False(t, m.isBlacklisted("10.0.0.11"))
}

func TestGetAllowList(t *testing.T) {
	m := setupTestManager(t)
	m.AddToAllowList("10.0.0.1", "A", 0)
	m.AddToAllowList("10.0.0.2", "B", 0)
	list := m.GetAllowList()
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestGetDenyList(t *testing.T) {
	m := setupTestManager(t)
	m.AddToDenyList("10.0.0.20", BanReasonManual, "A", 0)
	m.AddToDenyList("10.0.0.21", BanReasonManual, "B", 0)
	list := m.GetDenyList()
	assert.GreaterOrEqual(t, len(list), 2)
}

func TestCheckRequestWhitelisted(t *testing.T) {
	m := setupTestManager(t)
	allowed, reason := m.CheckRequest("127.0.0.1")
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckRequestBlacklisted(t *testing.T) {
	m := setupTestManager(t)
	m.AddToDenyList("10.0.0.50", BanReasonManual, "测试", 0)
	allowed, reason := m.CheckRequest("10.0.0.50")
	assert.False(t, allowed)
	assert.Equal(t, BanReasonManual, reason)
}

func TestCheckRequestNormalIP(t *testing.T) {
	m := setupTestManager(t)
	allowed, _ := m.CheckRequest("192.168.1.100")
	assert.True(t, allowed)
}

func TestCheckRequestBannedIP(t *testing.T) {
	cfg := testConfig()
	cfg.EnableAutoBan = true
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	for i := 0; i < cfg.LoginFailureThreshold; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP:        "10.0.0.60",
			Success:   false,
			Timestamp: time.Now(),
		})
	}

	allowed, reason := m.CheckRequest("10.0.0.60")
	assert.False(t, allowed)
	assert.Equal(t, BanReasonLoginFailure, reason)
}

func TestProcessLoginAttemptSuccess(t *testing.T) {
	m := setupTestManager(t)
	m.ProcessLoginAttempt(&LoginAttempt{
		IP: "10.0.0.70", Username: "admin", Success: true, Timestamp: time.Now(),
	})
	stats := m.GetIPStats("10.0.0.70")
	assert.NotNil(t, stats)
}

func TestProcessLoginAttemptFailure(t *testing.T) {
	m := setupTestManager(t)
	m.ProcessLoginAttempt(&LoginAttempt{
		IP: "10.0.0.71", Username: "admin", Success: false, Timestamp: time.Now(),
	})
	stats := m.GetIPStats("10.0.0.71")
	assert.NotNil(t, stats)
	assert.True(t, stats.ReputationScore < 100)
}

func TestAutoBanOnLoginFailure(t *testing.T) {
	cfg := testConfig()
	cfg.LoginFailureThreshold = 3
	cfg.EnableAutoBan = true
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	for i := 0; i < 3; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP: "10.0.0.72", Username: "admin", Success: false, Timestamp: time.Now(),
		})
	}

	stats := m.GetIPStats("10.0.0.72")
	assert.True(t, stats.IsBanned)
	assert.Equal(t, BanReasonLoginFailure, stats.BanReason)
	assert.True(t, stats.BanCount > 0)
}

func TestNoAutoBanWhenDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.LoginFailureThreshold = 3
	cfg.EnableAutoBan = false
	m := setupTestManagerWithConfig(t, cfg)

	for i := 0; i < 5; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP: "10.0.0.73", Username: "admin", Success: false, Timestamp: time.Now(),
		})
	}

	stats := m.GetIPStats("10.0.0.73")
	assert.False(t, stats.IsBanned)
}

func TestWhitelistedIPNotBanned(t *testing.T) {
	cfg := testConfig()
	cfg.EnableAutoBan = true
	cfg.WhitelistedIPs = []string{"127.0.0.1", "::1", "10.0.0.74"}
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	for i := 0; i < 10; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP: "10.0.0.74", Success: false, Timestamp: time.Now(),
		})
	}

	allowed, _ := m.CheckRequest("10.0.0.74")
	assert.True(t, allowed)
}

func TestLoginSuccessRecoversReputation(t *testing.T) {
	m := setupTestManager(t)

	for i := 0; i < 3; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP: "10.0.0.75", Success: false, Timestamp: time.Now(),
		})
	}

	statsBefore := m.GetIPStats("10.0.0.75")
	scoreBefore := statsBefore.ReputationScore

	m.ProcessLoginAttempt(&LoginAttempt{
		IP: "10.0.0.75", Success: true, Timestamp: time.Now(),
	})

	statsAfter := m.GetIPStats("10.0.0.75")
	assert.True(t, statsAfter.ReputationScore > scoreBefore)
}

func TestRecordPortAccess(t *testing.T) {
	m := setupTestManager(t)
	for port := 1; port <= 3; port++ {
		m.RecordPortAccess("10.0.0.80", port)
	}
	stats := m.GetIPStats("10.0.0.80")
	assert.NotNil(t, stats)
}

func TestPortScanDetection(t *testing.T) {
	cfg := testConfig()
	cfg.PortScanThreshold = 5
	cfg.EnableAutoBan = true
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	for port := 1; port <= 6; port++ {
		m.RecordPortAccess("10.0.0.81", port)
	}

	stats := m.GetIPStats("10.0.0.81")
	assert.True(t, stats.IsBanned)
	assert.Equal(t, BanReasonPortScan, stats.BanReason)
}

func TestRecordHTTPAccess(t *testing.T) {
	m := setupTestManager(t)
	m.RecordHTTPAccess("10.0.0.90", "/api/v1/login", "Mozilla/5.0")
	stats := m.GetIPStats("10.0.0.90")
	assert.NotNil(t, stats)
}

func TestSuspiciousUserAgentDetection(t *testing.T) {
	m := setupTestManager(t)
	m.RecordHTTPAccess("10.0.0.91", "/api/v1/login", "sqlmap/1.0")
	stats := m.GetIPStats("10.0.0.91")
	assert.True(t, stats.ReputationScore < 100)
}

func TestBotDetection(t *testing.T) {
	m := setupTestManager(t)
	m.RecordHTTPAccess("10.0.0.92", "/", "python-requests/2.28.0")
	stats := m.GetIPStats("10.0.0.92")
	assert.NotNil(t, stats)
}

func TestRateLimiting(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimitRequestsPerSecond = 2
	cfg.RateLimitBurst = 3
	m := setupTestManagerWithConfig(t, cfg)

	for i := 0; i < 3; i++ {
		allowed, _ := m.CheckRequest("10.0.0.100")
		assert.True(t, allowed, "request %d should be allowed", i)
	}

	allowed, reason := m.CheckRequest("10.0.0.100")
	assert.False(t, allowed)
	assert.Equal(t, BanReasonRateLimit, reason)
}

func TestRateLimitRefill(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimitRequestsPerSecond = 100
	cfg.RateLimitBurst = 1
	m := setupTestManagerWithConfig(t, cfg)

	allowed, _ := m.CheckRequest("10.0.0.101")
	assert.True(t, allowed)

	allowed, _ = m.CheckRequest("10.0.0.101")
	assert.False(t, allowed)

	time.Sleep(20 * time.Millisecond)
	allowed, _ = m.CheckRequest("10.0.0.101")
	assert.True(t, allowed)
}

func TestRateLimiterManager(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	cfg.RateLimitRequestsPerSecond = 10
	cfg.RateLimitBurst = 5

	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	ip := "10.0.0.200"

	for i := 0; i < 5; i++ {
		assert.True(t, rlm.Allow(ip))
	}

	assert.False(t, rlm.Allow(ip))

	rlm.Reset(ip)
	assert.True(t, rlm.Allow(ip))
}

func TestRateLimiterAllowN(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	cfg.RateLimitRequestsPerSecond = 10
	cfg.RateLimitBurst = 10

	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	ip := "10.0.0.201"

	assert.True(t, rlm.AllowN(ip, 5))
	assert.True(t, rlm.AllowN(ip, 5))
	assert.False(t, rlm.AllowN(ip, 1))
}

func TestRateLimiterTokens(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	cfg.RateLimitRequestsPerSecond = 10
	cfg.RateLimitBurst = 10

	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	ip := "10.0.0.202"

	tokens := rlm.Tokens(ip)
	assert.InDelta(t, 10, tokens, 0.1)

	rlm.Allow(ip)
	tokens = rlm.Tokens(ip)
	assert.InDelta(t, 9, tokens, 0.1)
}

func TestRateLimiterTrackedIPs(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	assert.Equal(t, 0, rlm.TrackedIPs())

	rlm.Allow("10.0.0.1")
	rlm.Allow("10.0.0.2")
	assert.Equal(t, 2, rlm.TrackedIPs())
}

func TestRateLimiterRemove(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	rlm.Allow("10.0.0.1")
	assert.Equal(t, 1, rlm.TrackedIPs())

	rlm.Remove("10.0.0.1")
	assert.Equal(t, 0, rlm.TrackedIPs())
}

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(5, 10)

	for i := 0; i < 5; i++ {
		assert.True(t, tb.Allow())
	}
	assert.False(t, tb.Allow())
}

func TestTokenBucketRefill(t *testing.T) {
	tb := NewTokenBucket(10, 100)

	for i := 0; i < 10; i++ {
		tb.Allow()
	}
	assert.False(t, tb.Allow())

	time.Sleep(20 * time.Millisecond)
	assert.True(t, tb.Allow())
}

func TestTokenBucketAllowN(t *testing.T) {
	tb := NewTokenBucket(10, 10)

	assert.True(t, tb.AllowN(5))
	assert.True(t, tb.AllowN(5))
	assert.False(t, tb.AllowN(1))
}

func TestTokenBucketTokens(t *testing.T) {
	tb := NewTokenBucket(10, 10)

	tokens := tb.Tokens()
	assert.InDelta(t, 10, tokens, 0.1)

	tb.Allow()
	tokens = tb.Tokens()
	assert.InDelta(t, 9, tokens, 0.1)
}

func TestTokenBucketReset(t *testing.T) {
	tb := NewTokenBucket(10, 10)

	for i := 0; i < 10; i++ {
		tb.Allow()
	}
	assert.False(t, tb.Allow())

	tb.Reset()
	assert.True(t, tb.Allow())
	tokens := tb.Tokens()
	assert.InDelta(t, 9, tokens, 0.1)
}

func TestDetectorLoginFailure(t *testing.T) {
	d := NewDetector(nil, nil)

	for i := 0; i < 6; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "10.0.0.10", Success: false, Timestamp: time.Now(),
		})
	}

	triggered, count, threshold := d.DetectLoginFailure("10.0.0.10")
	assert.True(t, triggered)
	assert.Equal(t, 6, count)
	assert.Equal(t, 5, threshold)
}

func TestDetectorLoginFailureNotTriggered(t *testing.T) {
	d := NewDetector(nil, nil)

	for i := 0; i < 3; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "10.0.0.11", Success: false, Timestamp: time.Now(),
		})
	}

	triggered, count, _ := d.DetectLoginFailure("10.0.0.11")
	assert.False(t, triggered)
	assert.Equal(t, 3, count)
}

func TestDetectorLoginFailureWithSuccess(t *testing.T) {
	d := NewDetector(nil, nil)

	d.RecordLoginAttempt(&LoginAttempt{IP: "10.0.0.12", Success: false, Timestamp: time.Now()})
	d.RecordLoginAttempt(&LoginAttempt{IP: "10.0.0.12", Success: true, Timestamp: time.Now()})
	d.RecordLoginAttempt(&LoginAttempt{IP: "10.0.0.12", Success: false, Timestamp: time.Now()})
	d.RecordLoginAttempt(&LoginAttempt{IP: "10.0.0.12", Success: false, Timestamp: time.Now()})

	triggered, count, _ := d.DetectLoginFailure("10.0.0.12")
	assert.False(t, triggered)
	assert.Equal(t, 3, count)
}

func TestDetectorPortScan(t *testing.T) {
	cfg := &IPProtectionConfig{
		PortScanThreshold: 5,
		PortScanWindow:    1 * time.Minute,
	}
	d := NewDetector(nil, cfg)

	for port := 1; port <= 6; port++ {
		d.RecordAccess(&AccessRecord{
			IP: "10.0.0.20", Port: port, Timestamp: time.Now(),
		})
	}

	result := d.DetectPortScan("10.0.0.20")
	assert.True(t, result.Detected)
	assert.Equal(t, DetectionPortScan, result.Type)
	assert.Equal(t, ThreatLevelHigh, result.ThreatLevel)
}

func TestDetectorPortScanNotTriggered(t *testing.T) {
	cfg := &IPProtectionConfig{
		PortScanThreshold: 10,
		PortScanWindow:    1 * time.Minute,
	}
	d := NewDetector(nil, cfg)

	for port := 1; port <= 5; port++ {
		d.RecordAccess(&AccessRecord{
			IP: "10.0.0.21", Port: port, Timestamp: time.Now(),
		})
	}

	result := d.DetectPortScan("10.0.0.21")
	assert.False(t, result.Detected)
}

func TestDetectorBruteForce(t *testing.T) {
	cfg := &IPProtectionConfig{
		BruteForceThreshold: 5,
		BruteForceWindow:    1 * time.Minute,
	}
	d := NewDetector(nil, cfg)

	for i := 0; i < 6; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "10.0.0.30", Username: "admin", Success: false, Timestamp: time.Now(),
		})
	}

	result := d.DetectBruteForce("10.0.0.30")
	assert.True(t, result.Detected)
	assert.Equal(t, DetectionBruteForce, result.Type)
	assert.Equal(t, ThreatLevelCritical, result.ThreatLevel)
}

func TestDetectorBruteForceDictionaryAttack(t *testing.T) {
	cfg := &IPProtectionConfig{
		BruteForceThreshold: 5,
		BruteForceWindow:    1 * time.Minute,
	}
	d := NewDetector(nil, cfg)

	users := []string{"admin", "root", "user", "test"}
	for i := 0; i < 6; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "10.0.0.31", Username: users[i%len(users)], Success: false, Timestamp: time.Now(),
		})
	}

	result := d.DetectBruteForce("10.0.0.31")
	assert.True(t, result.Detected)
	assert.Contains(t, result.Details, "字典")
}

func TestDetectorSuspiciousUserAgent(t *testing.T) {
	d := NewDetector(nil, nil)

	tests := []struct {
		ua     string
		detect bool
	}{
		{"sqlmap/1.0", true},
		{"nikto/2.1", true},
		{"nmap", true},
		{"Mozilla/5.0", false},
		{"", true},
	}

	for _, tt := range tests {
		result := d.DetectSuspiciousUserAgent("10.0.0.40", tt.ua)
		assert.Equal(t, tt.detect, result.Detected, "UA: %s", tt.ua)
	}
}

func TestDetectorBotPattern(t *testing.T) {
	d := NewDetector(nil, nil)

	tests := []struct {
		ua     string
		detect bool
	}{
		{"Googlebot/2.1", true},
		{"python-requests/2.28", true},
		{"curl/7.68", true},
		{"Mozilla/5.0", false},
	}

	for _, tt := range tests {
		result := d.DetectBotPattern("10.0.0.41", tt.ua)
		assert.Equal(t, tt.detect, result.Detected, "UA: %s", tt.ua)
	}
}

func TestDetectorGetRecentLoginFailures(t *testing.T) {
	d := NewDetector(nil, nil)

	for i := 0; i < 3; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "10.0.0.50", Success: false, Timestamp: time.Now(),
		})
	}

	count := d.GetRecentLoginFailures("10.0.0.50")
	assert.Equal(t, 3, count)
}

func TestDetectorGetRecentPortCount(t *testing.T) {
	cfg := &IPProtectionConfig{PortScanWindow: 1 * time.Minute}
	d := NewDetector(nil, cfg)

	for port := 80; port < 85; port++ {
		d.RecordAccess(&AccessRecord{
			IP: "10.0.0.51", Port: port, Timestamp: time.Now(),
		})
	}

	count := d.GetRecentPortCount("10.0.0.51")
	assert.Equal(t, 5, count)
}

func TestGetIPStatsNoRecord(t *testing.T) {
	m := setupTestManager(t)

	stats := m.GetIPStats("10.0.0.99")
	assert.NotNil(t, stats)
	assert.Equal(t, "10.0.0.99", stats.IP)
	assert.Equal(t, 100, stats.ReputationScore)
	assert.Equal(t, ThreatLevelLow, stats.ThreatLevel)
}

func TestGetIPStatsWithRecord(t *testing.T) {
	m := setupTestManager(t)

	m.CheckRequest("10.0.0.100")
	m.CheckRequest("10.0.0.100")

	stats := m.GetIPStats("10.0.0.100")
	assert.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.TotalRequests)
}

func TestGetGlobalStats(t *testing.T) {
	m := setupTestManager(t)

	m.CheckRequest("10.0.0.1")
	m.CheckRequest("10.0.0.2")
	m.CheckRequest("10.0.0.3")

	stats := m.GetGlobalStats()
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalIPsTracked)
	assert.True(t, stats.AvgReputation > 0)
}

func TestGetBanLog(t *testing.T) {
	cfg := testConfig()
	cfg.LoginFailureThreshold = 2
	cfg.EnableAutoBan = true
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	m.ProcessLoginAttempt(&LoginAttempt{IP: "10.0.0.200", Success: false, Timestamp: time.Now()})
	m.ProcessLoginAttempt(&LoginAttempt{IP: "10.0.0.200", Success: false, Timestamp: time.Now()})

	log := m.GetBanLog(10)
	assert.True(t, len(log) > 0)
	assert.Equal(t, "10.0.0.200", log[0].IP)
	assert.True(t, log[0].IsActive)
}

func TestReputationScorePenalty(t *testing.T) {
	m := setupTestManager(t)

	stats := m.GetIPStats("10.0.0.150")
	assert.Equal(t, 100, stats.ReputationScore)

	m.ProcessLoginAttempt(&LoginAttempt{
		IP: "10.0.0.150", Success: false, Timestamp: time.Now(),
	})

	stats = m.GetIPStats("10.0.0.150")
	assert.True(t, stats.ReputationScore < 100)
}

func TestReputationScoreBounds(t *testing.T) {
	m := setupTestManager(t)

	for i := 0; i < 100; i++ {
		m.ProcessLoginAttempt(&LoginAttempt{
			IP: "10.0.0.151", Success: false, Timestamp: time.Now(),
		})
	}

	stats := m.GetIPStats("10.0.0.151")
	assert.True(t, stats.ReputationScore >= 0)
	assert.True(t, stats.ReputationScore <= 100)
}

func TestThreatLevelCalculation(t *testing.T) {
	m := setupTestManager(t)

	assert.Equal(t, ThreatLevelLow, m.calcThreatLevel(80))
	assert.Equal(t, ThreatLevelMedium, m.calcThreatLevel(60))
	assert.Equal(t, ThreatLevelHigh, m.calcThreatLevel(40))
	assert.Equal(t, ThreatLevelCritical, m.calcThreatLevel(20))
}

func TestIPv6Whitelist(t *testing.T) {
	m := setupTestManager(t)
	m.AddToAllowList("2001:db8::100", "IPv6 测试", 0)
	assert.True(t, m.isWhitelisted("2001:db8::100"))
}

func TestIPv6Blacklist(t *testing.T) {
	m := setupTestManager(t)
	m.AddToDenyList("2001:db8::200", BanReasonManual, "IPv6 测试", 0)
	assert.True(t, m.isBlacklisted("2001:db8::200"))
}

func TestIPv6CheckRequest(t *testing.T) {
	m := setupTestManager(t)
	allowed, _ := m.CheckRequest("2001:db8::300")
	assert.True(t, allowed)
}

func TestIPv6LoginAttempt(t *testing.T) {
	cfg := testConfig()
	cfg.LoginFailureThreshold = 2
	cfg.EnableAutoBan = true
	m := setupTestManagerWithConfig(t, cfg)
	m.Start()
	defer m.Stop()

	m.ProcessLoginAttempt(&LoginAttempt{IP: "2001:db8::400", Success: false, Timestamp: time.Now()})
	m.ProcessLoginAttempt(&LoginAttempt{IP: "2001:db8::400", Success: false, Timestamp: time.Now()})

	stats := m.GetIPStats("2001:db8::400")
	assert.True(t, stats.IsBanned)
}

func TestIPv6Detection(t *testing.T) {
	d := NewDetector(nil, nil)

	for i := 0; i < 6; i++ {
		d.RecordLoginAttempt(&LoginAttempt{
			IP: "2001:db8::500", Success: false, Timestamp: time.Now(),
		})
	}

	triggered, count, _ := d.DetectLoginFailure("2001:db8::500")
	assert.True(t, triggered)
	assert.Equal(t, 6, count)
}

func TestIPv6RateLimiting(t *testing.T) {
	cfg := DefaultIPProtectionConfig()
	cfg.RateLimitRequestsPerSecond = 10
	cfg.RateLimitBurst = 3

	rlm := NewRateLimiterManager(cfg)
	defer rlm.Stop()

	ip := "2001:db8::600"

	for i := 0; i < 3; i++ {
		assert.True(t, rlm.Allow(ip))
	}
	assert.False(t, rlm.Allow(ip))
}

func TestIsIPv6(t *testing.T) {
	assert.True(t, isIPv6("2001:db8::1"))
	assert.True(t, isIPv6("::1"))
	assert.True(t, isIPv6("fe80::1"))
	assert.False(t, isIPv6("192.168.1.1"))
	assert.False(t, isIPv6("127.0.0.1"))
	assert.False(t, isIPv6("invalid"))
}

func TestValidateIP(t *testing.T) {
	assert.NoError(t, validateIP("192.168.1.1"))
	assert.NoError(t, validateIP("2001:db8::1"))
	assert.Error(t, validateIP("invalid"))
	assert.Error(t, validateIP(""))
}

func TestConcurrentCheckRequest(t *testing.T) {
	m := setupTestManager(t)

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			m.CheckRequest("10.0.0.1")
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestConcurrentLoginAttempt(t *testing.T) {
	m := setupTestManager(t)

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			m.ProcessLoginAttempt(&LoginAttempt{
				IP: "10.0.0.1", Success: false, Timestamp: time.Now(),
			})
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}
