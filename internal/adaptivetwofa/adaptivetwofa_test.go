package adaptivetwofa

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.LowRiskThreshold != 25 {
		t.Errorf("expected LowRiskThreshold 25, got %d", config.LowRiskThreshold)
	}

	if config.MediumRiskThreshold != 50 {
		t.Errorf("expected MediumRiskThreshold 50, got %d", config.MediumRiskThreshold)
	}

	if config.HighRiskThreshold != 75 {
		t.Errorf("expected HighRiskThreshold 75, got %d", config.HighRiskThreshold)
	}

	if config.TrustedDeviceTTL != 30*24*time.Hour {
		t.Errorf("expected TrustedDeviceTTL 30 days, got %v", config.TrustedDeviceTTL)
	}

	if config.MaxTrustedDevices != 5 {
		t.Errorf("expected MaxTrustedDevices 5, got %d", config.MaxTrustedDevices)
	}
}

func TestRiskLevelString(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskCritical, "critical"},
		{RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("RiskLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestTrustedDeviceIsExpired(t *testing.T) {
	device := &TrustedDevice{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	if !device.IsExpired() {
		t.Error("expected device to be expired")
	}

	device.ExpiresAt = time.Now().Add(1 * time.Hour)
	if device.IsExpired() {
		t.Error("expected device to not be expired")
	}
}

func TestAuthChallengeIsExpired(t *testing.T) {
	challenge := &AuthChallenge{
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}

	if !challenge.IsExpired() {
		t.Error("expected challenge to be expired")
	}

	challenge.ExpiresAt = time.Now().Add(5 * time.Minute)
	if challenge.IsExpired() {
		t.Error("expected challenge to not be expired")
	}
}

func TestNewRiskEngine(t *testing.T) {
	config := DefaultConfig()
	engine := NewRiskEngine(config)

	if engine == nil {
		t.Fatal("NewRiskEngine returned nil")
	}

	if engine.config != config {
		t.Error("config not set correctly")
	}
}

func TestEvaluateRiskLowRisk(t *testing.T) {
	config := DefaultConfig()
	engine := NewRiskEngine(config)

	// 先记录一次登录建立历史，使IP成为已知IP
	recordCtx := &LoginContext{
		UserID:    "user1",
		Username:  "testuser",
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Timestamp: time.Now().Add(-1 * time.Hour),
	}
	engine.RecordLogin(recordCtx, true, 10)

	ctx := &LoginContext{
		UserID:    "user1",
		Username:  "testuser",
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Timestamp: time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 0, 0, 0, time.Local), // 确保在工作时间
	}

	// 创建信任设备
	trustedDevice := &TrustedDevice{
		Fingerprint: "trusted-fp",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
	}

	score := engine.EvaluateRisk(ctx, trustedDevice)

	if score == nil {
		t.Fatal("EvaluateRisk returned nil")
	}

	// 信任设备应该是低风险
	if score.Level != RiskLow {
		t.Errorf("expected RiskLow for trusted device, got %v (score=%d, factors=%v)", score.Level, score.Score, score.Factors)
	}
}

func TestEvaluateRiskNewUser(t *testing.T) {
	config := DefaultConfig()
	engine := NewRiskEngine(config)

	ctx := &LoginContext{
		UserID:    "newuser",
		Username:  "newuser",
		IP:        "1.2.3.4",
		UserAgent: "Mozilla/5.0",
		Timestamp: time.Now(),
	}

	score := engine.EvaluateRisk(ctx, nil)

	if score == nil {
		t.Fatal("EvaluateRisk returned nil")
	}

	// 新用户应该是中等或高风险
	if score.Score == 0 {
		t.Error("expected non-zero risk score for new user")
	}
}

func TestRecordLogin(t *testing.T) {
	config := DefaultConfig()
	engine := NewRiskEngine(config)

	ctx := &LoginContext{
		UserID:    "user1",
		Username:  "testuser",
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
		Timestamp: time.Now(),
	}

	// 记录登录
	engine.RecordLogin(ctx, true, 30)

	stats := engine.GetUserStats("user1")
	if stats == nil {
		t.Fatal("expected user stats to exist")
	}

	if stats.TotalLogins != 1 {
		t.Errorf("expected 1 login, got %d", stats.TotalLogins)
	}

	if stats.LastIPs[0] != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", stats.LastIPs[0])
	}
}

func TestGetLoginHistory(t *testing.T) {
	config := DefaultConfig()
	engine := NewRiskEngine(config)

	// 记录多次登录
	for i := 0; i < 5; i++ {
		ctx := &LoginContext{
			UserID:    "user1",
			Username:  "testuser",
			IP:        "192.168.1.1",
			UserAgent: "Mozilla/5.0",
			Timestamp: time.Now().Add(time.Duration(i) * time.Hour),
		}
		engine.RecordLogin(ctx, true, 30)
	}

	history := engine.GetLoginHistory("user1", 3)
	if len(history) != 3 {
		t.Errorf("expected 3 history records, got %d", len(history))
	}
}

func TestNewFingerprintGenerator(t *testing.T) {
	gen := NewFingerprintGenerator("test-salt")

	if gen == nil {
		t.Fatal("NewFingerprintGenerator returned nil")
	}

	if gen.salt != "test-salt" {
		t.Errorf("expected salt 'test-salt', got '%s'", gen.salt)
	}
}

func TestGenerateFingerprint(t *testing.T) {
	gen := NewFingerprintGenerator("test-salt")

	components := &FingerprintComponents{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		IP:        "192.168.1.1",
	}

	fp := gen.Generate(components)

	if fp == nil {
		t.Fatal("Generate returned nil")
	}

	if fp.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}

	if fp.Confidence < 20 {
		t.Errorf("expected confidence >= 20, got %d", fp.Confidence)
	}
}

func TestGenerateFingerprintConsistency(t *testing.T) {
	gen := NewFingerprintGenerator("test-salt")

	components := &FingerprintComponents{
		UserAgent: "Mozilla/5.0",
		IP:        "192.168.1.1",
	}

	fp1 := gen.Generate(components)
	fp2 := gen.Generate(components)

	if fp1.Fingerprint != fp2.Fingerprint {
		t.Error("expected same fingerprint for same components")
	}
}

func TestIsKnownDevice(t *testing.T) {
	gen := NewFingerprintGenerator("test-salt")

	components := &FingerprintComponents{
		UserAgent: "Mozilla/5.0",
		IP:        "192.168.1.1",
	}

	fp := gen.Generate(components)

	if !gen.IsKnownDevice(fp.Fingerprint) {
		t.Error("expected device to be known")
	}

	if gen.IsKnownDevice("unknown-fingerprint") {
		t.Error("expected unknown device to not be known")
	}
}

func TestGenerateSimpleFingerprint(t *testing.T) {
	fp1 := GenerateSimpleFingerprint("192.168.1.1", "Mozilla/5.0")
	fp2 := GenerateSimpleFingerprint("192.168.1.1", "Mozilla/5.0")
	fp3 := GenerateSimpleFingerprint("10.0.0.1", "Chrome/90")

	if fp1 == "" {
		t.Error("expected non-empty fingerprint")
	}

	if fp1 != fp2 {
		t.Error("expected same fingerprint for same inputs")
	}

	if fp1 == fp3 {
		t.Error("expected different fingerprint for different inputs")
	}
}

func TestCompareFingerprints(t *testing.T) {
	gen := NewFingerprintGenerator("test-salt")

	fp1 := gen.Generate(&FingerprintComponents{
		UserAgent:        "Mozilla/5.0",
		IP:               "192.168.1.1",
		ScreenResolution: "1920x1080",
		Timezone:         "Asia/Shanghai",
	})

	fp2 := gen.Generate(&FingerprintComponents{
		UserAgent:        "Mozilla/5.0",
		IP:               "192.168.1.2",
		ScreenResolution: "1920x1080",
		Timezone:         "Asia/Shanghai",
	})

	similarity := CompareFingerprints(fp1, fp2)

	if similarity < 0.5 {
		t.Errorf("expected similarity > 0.5, got %f", similarity)
	}
}

func TestNewAdaptiveManager(t *testing.T) {
	config := DefaultConfig()
	mgr, err := NewAdaptiveManager("", config)

	if err != nil {
		t.Fatalf("NewAdaptiveManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("NewAdaptiveManager returned nil")
	}
}

func TestEvaluateLoginLowRisk(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	// 使用简单的设备指纹
	fingerprint := GenerateSimpleFingerprint("192.168.1.1", "Mozilla/5.0")

	// 先信任设备
	mgr.TrustDevice("user1", fingerprint, "192.168.1.1", "Mozilla/5.0", nil)

	// 先记录一次登录建立历史，使IP成为已知IP
	recordCtx := &LoginContext{
		UserID:            "user1",
		Username:          "testuser",
		IP:                "192.168.1.1",
		UserAgent:         "Mozilla/5.0",
		DeviceFingerprint: fingerprint,
		Timestamp:         time.Now().Add(-1 * time.Hour),
	}
	mgr.GetRiskEngine().RecordLogin(recordCtx, true, 10)

	ctx := &LoginContext{
		UserID:            "user1",
		Username:          "testuser",
		IP:                "192.168.1.1",
		UserAgent:         "Mozilla/5.0",
		DeviceFingerprint: fingerprint,
		Timestamp:         time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 0, 0, 0, time.Local), // 确保在工作时间
	}

	result := mgr.EvaluateLogin(ctx)

	if result == nil {
		t.Fatal("EvaluateLogin returned nil")
	}

	if !result.Allowed {
		t.Error("expected login to be allowed for trusted device")
	}

	if result.RiskScore.Level != RiskLow {
		t.Errorf("expected low risk, got %v (score=%d, factors=%v)", result.RiskScore.Level, result.RiskScore.Score, result.RiskScore.Factors)
	}
}

func TestEvaluateLoginHighRisk(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	// 模拟多次登录建立历史
	for i := 0; i < 5; i++ {
		ctx := &LoginContext{
			UserID:            "user1",
			Username:          "testuser",
			IP:                "192.168.1.1",
			UserAgent:         "Mozilla/5.0",
			DeviceFingerprint: "known-device",
			Timestamp:         time.Now().Add(time.Duration(-i) * time.Hour),
		}
		mgr.GetRiskEngine().RecordLogin(ctx, true, 20)
	}

	// 使用完全不同的IP和设备登录
	ctx := &LoginContext{
		UserID:            "user1",
		Username:          "testuser",
		IP:                "10.0.0.1",
		UserAgent:         "Chrome/90",
		DeviceFingerprint: "unknown-device",
		Timestamp:         time.Now(),
	}

	result := mgr.EvaluateLogin(ctx)

	if result == nil {
		t.Fatal("EvaluateLogin returned nil")
	}

	// 应该是中等或高风险
	if result.RiskScore.Score < 30 {
		t.Errorf("expected higher risk score, got %d", result.RiskScore.Score)
	}
}

func TestTrustDevice(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	device, err := mgr.TrustDevice("user1", "fp1", "192.168.1.1", "Mozilla/5.0", nil)
	if err != nil {
		t.Fatalf("TrustDevice failed: %v", err)
	}

	if device.UserID != "user1" {
		t.Errorf("expected user1, got %s", device.UserID)
	}

	if device.Fingerprint != "fp1" {
		t.Errorf("expected fp1, got %s", device.Fingerprint)
	}

	// 验证设备已添加
	devices := mgr.GetTrustedDevices("user1")
	if len(devices) != 1 {
		t.Errorf("expected 1 trusted device, got %d", len(devices))
	}
}

func TestRevokeTrust(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	device, _ := mgr.TrustDevice("user1", "fp1", "192.168.1.1", "Mozilla/5.0", nil)

	err := mgr.RevokeTrust("user1", device.DeviceID)
	if err != nil {
		t.Fatalf("RevokeTrust failed: %v", err)
	}

	devices := mgr.GetTrustedDevices("user1")
	if len(devices) != 0 {
		t.Errorf("expected 0 trusted devices after revoke, got %d", len(devices))
	}
}

func TestRevokeAllTrust(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	mgr.TrustDevice("user1", "fp1", "192.168.1.1", "Mozilla/5.0", nil)
	mgr.TrustDevice("user1", "fp2", "192.168.1.2", "Chrome/90", nil)

	err := mgr.RevokeAllTrust("user1")
	if err != nil {
		t.Fatalf("RevokeAllTrust failed: %v", err)
	}

	devices := mgr.GetTrustedDevices("user1")
	if len(devices) != 0 {
		t.Errorf("expected 0 trusted devices after revoke all, got %d", len(devices))
	}
}

func TestMaxTrustedDevices(t *testing.T) {
	config := DefaultConfig()
	config.MaxTrustedDevices = 2
	mgr, _ := NewAdaptiveManager("", config)

	mgr.TrustDevice("user1", "fp1", "192.168.1.1", "Mozilla/5.0", nil)
	mgr.TrustDevice("user1", "fp2", "192.168.1.2", "Chrome/90", nil)
	mgr.TrustDevice("user1", "fp3", "192.168.1.3", "Firefox/89", nil)

	devices := mgr.GetTrustedDevices("user1")
	if len(devices) > 2 {
		t.Errorf("expected max 2 trusted devices, got %d", len(devices))
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultConfig()
	mgr, _ := NewAdaptiveManager("", config)

	mgr.TrustDevice("user1", "fp1", "192.168.1.1", "Mozilla/5.0", nil)
	mgr.TrustDevice("user2", "fp2", "192.168.1.2", "Chrome/90", nil)

	stats := mgr.GetStats()

	if stats["total_users_with_trust"] != 2 {
		t.Errorf("expected 2 users with trust, got %v", stats["total_users_with_trust"])
	}

	if stats["total_trusted_devices"] != 2 {
		t.Errorf("expected 2 trusted devices, got %v", stats["total_trusted_devices"])
	}
}
