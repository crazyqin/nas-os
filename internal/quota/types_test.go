package quota

import (
	"testing"
	"time"
)

// ========== 错误定义测试 ==========

func TestErrors(t *testing.T) {
	errors := []error{
		ErrQuotaNotFound,
		ErrQuotaExists,
		ErrQuotaExceeded,
		ErrUserNotFound,
		ErrGroupNotFound,
		ErrVolumeNotFound,
		ErrInvalidLimit,
		ErrCleanupPolicyNotFound,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// ========== 配额类型测试 ==========

func TestQuotaType_Values(t *testing.T) {
	types := []QuotaType{
		QuotaTypeUser,
		QuotaTypeGroup,
		QuotaTypeDirectory,
	}

	for _, qt := range types {
		if qt == "" {
			t.Error("QuotaType should not be empty")
		}
	}
}

// ========== 配额结构测试 ==========

func TestQuota_Struct(t *testing.T) {
	quota := &Quota{
		ID:         "quota-1",
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		TargetName: "Test User",
		VolumeName: "data",
		Path:       "/data/user1",
		HardLimit:  10737418240, // 10 GB
		SoftLimit:  8589934592,  // 8 GB
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if quota.ID != "quota-1" {
		t.Errorf("Expected ID=quota-1, got %s", quota.ID)
	}
	if quota.Type != QuotaTypeUser {
		t.Errorf("Expected Type=user, got %s", quota.Type)
	}
	if quota.HardLimit != 10737418240 {
		t.Errorf("Expected HardLimit=10737418240, got %d", quota.HardLimit)
	}
}

func TestQuota_GroupQuota(t *testing.T) {
	quota := &Quota{
		ID:         "group-quota-1",
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		TargetName: "Developers Group",
		VolumeName: "shared",
		Path:       "/shared/developers",
		HardLimit:  107374182400, // 100 GB
		SoftLimit:  85899345920,  // 80 GB
	}

	if quota.Type != QuotaTypeGroup {
		t.Errorf("Expected Type=group, got %s", quota.Type)
	}
	if quota.TargetID != "developers" {
		t.Errorf("Expected TargetID=developers, got %s", quota.TargetID)
	}
}

func TestQuota_DirectoryQuota(t *testing.T) {
	quota := &Quota{
		ID:        "dir-quota-1",
		Type:      QuotaTypeDirectory,
		TargetID:  "/data/projects",
		Path:      "/data/projects",
		HardLimit: 536870912000, // 500 GB
		SoftLimit: 429496729600, // 400 GB
	}

	if quota.Type != QuotaTypeDirectory {
		t.Errorf("Expected Type=directory, got %s", quota.Type)
	}
}

// ========== 配额使用情况测试 ==========

func TestQuotaUsage_Struct(t *testing.T) {
	usage := &QuotaUsage{
		QuotaID:      "quota-1",
		Type:         QuotaTypeUser,
		TargetID:     "user1",
		TargetName:   "Test User",
		VolumeName:   "data",
		Path:         "/data/user1",
		HardLimit:    10737418240,
		SoftLimit:    8589934592,
		UsedBytes:    5368709120, // 5 GB
		Available:    5368709120,
		UsagePercent: 50.0,
		IsOverSoft:   false,
		IsOverHard:   false,
		LastChecked:  time.Now(),
	}

	if usage.UsagePercent != 50.0 {
		t.Errorf("Expected UsagePercent=50.0, got %f", usage.UsagePercent)
	}
	if usage.IsOverSoft {
		t.Error("Expected IsOverSoft=false")
	}
}

func TestQuotaUsage_OverSoftLimit(t *testing.T) {
	usage := &QuotaUsage{
		HardLimit:    10737418240, // 10 GB
		SoftLimit:    8589934592,  // 8 GB
		UsedBytes:    9663676416,  // 9 GB (超过软限制)
		UsagePercent: 90.0,
		IsOverSoft:   true,
		IsOverHard:   false,
	}

	if !usage.IsOverSoft {
		t.Error("Expected IsOverSoft=true")
	}
	if usage.IsOverHard {
		t.Error("Expected IsOverHard=false")
	}
}

func TestQuotaUsage_OverHardLimit(t *testing.T) {
	usage := &QuotaUsage{
		HardLimit:    10737418240, // 10 GB
		SoftLimit:    8589934592,  // 8 GB
		UsedBytes:    11811160064, // 11 GB (超过硬限制)
		UsagePercent: 110.0,
		IsOverSoft:   true,
		IsOverHard:   true,
	}

	if !usage.IsOverSoft {
		t.Error("Expected IsOverSoft=true")
	}
	if !usage.IsOverHard {
		t.Error("Expected IsOverHard=true")
	}
}

// ========== 告警类型测试 ==========

func TestAlertType_Values(t *testing.T) {
	types := []AlertType{
		AlertTypeSoftLimit,
		AlertTypeHardLimit,
		AlertTypeCleanup,
	}

	for _, at := range types {
		if at == "" {
			t.Error("AlertType should not be empty")
		}
	}
}

func TestAlertSeverity_Values(t *testing.T) {
	severities := []AlertSeverity{
		AlertSeverityInfo,
		AlertSeverityWarning,
		AlertSeverityCritical,
		AlertSeverityEmergency,
	}

	for _, s := range severities {
		if s == "" {
			t.Error("AlertSeverity should not be empty")
		}
	}
}

func TestAlertStatus_Values(t *testing.T) {
	statuses := []AlertStatus{
		AlertStatusActive,
		AlertStatusResolved,
		AlertStatusSilenced,
		AlertStatusEscalated,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("AlertStatus should not be empty")
		}
	}
}

// ========== 告警结构测试 ==========

func TestAlert_Struct(t *testing.T) {
	alert := &Alert{
		ID:           "alert-1",
		QuotaID:      "quota-1",
		Type:         AlertTypeSoftLimit,
		Severity:     AlertSeverityWarning,
		Status:       AlertStatusActive,
		TargetID:     "user1",
		TargetName:   "Test User",
		VolumeName:   "data",
		Path:         "/data/user1",
		UsedBytes:    9663676416,
		LimitBytes:   8589934592,
		UsagePercent: 90.0,
		Threshold:    80.0,
		Message:      "用户 user1 已超过软限制",
		CreatedAt:    time.Now(),
	}

	if alert.Type != AlertTypeSoftLimit {
		t.Errorf("Expected Type=soft_limit, got %s", alert.Type)
	}
	if alert.Severity != AlertSeverityWarning {
		t.Errorf("Expected Severity=warning, got %s", alert.Severity)
	}
	if alert.Status != AlertStatusActive {
		t.Errorf("Expected Status=active, got %s", alert.Status)
	}
}

func TestAlert_Resolved(t *testing.T) {
	now := time.Now()
	alert := &Alert{
		ID:         "alert-2",
		QuotaID:    "quota-1",
		Type:       AlertTypeHardLimit,
		Severity:   AlertSeverityCritical,
		Status:     AlertStatusResolved,
		ResolvedAt: &now,
	}

	if alert.Status != AlertStatusResolved {
		t.Errorf("Expected Status=resolved, got %s", alert.Status)
	}
	if alert.ResolvedAt == nil {
		t.Error("ResolvedAt should not be nil")
	}
}

// ========== 清理策略类型测试 ==========

func TestCleanupPolicyType_Values(t *testing.T) {
	types := []CleanupPolicyType{
		CleanupPolicyAge,
		CleanupPolicySize,
		CleanupPolicyPattern,
		CleanupPolicyQuota,
		CleanupPolicyAccess,
	}

	for _, pt := range types {
		if pt == "" {
			t.Error("CleanupPolicyType should not be empty")
		}
	}
}

func TestCleanupAction_Values(t *testing.T) {
	actions := []CleanupAction{
		CleanupActionDelete,
		CleanupActionArchive,
		CleanupActionMove,
	}

	for _, a := range actions {
		if a == "" {
			t.Error("CleanupAction should not be empty")
		}
	}
}

// ========== 清理策略结构测试 ==========

func TestCleanupPolicy_Struct(t *testing.T) {
	policy := &CleanupPolicy{
		ID:         "cleanup-1",
		Name:       "清理旧文件",
		VolumeName: "data",
		Path:       "/data/temp",
		Type:       CleanupPolicyAge,
		Action:     CleanupActionDelete,
		Enabled:    true,
	}

	if policy.Type != CleanupPolicyAge {
		t.Errorf("Expected Type=age, got %s", policy.Type)
	}
	if policy.Action != CleanupActionDelete {
		t.Errorf("Expected Action=delete, got %s", policy.Action)
	}
	if !policy.Enabled {
		t.Error("Expected Enabled=true")
	}
}

// ========== 配额限制验证测试 ==========

func TestQuota_Validation(t *testing.T) {
	tests := []struct {
		name      string
		hardLimit uint64
		softLimit uint64
		valid     bool
	}{
		{"valid limits", 10737418240, 8589934592, true},
		{"equal limits", 10737418240, 10737418240, true},
		{"zero hard limit", 0, 8589934592, false},
		{"soft exceeds hard", 8589934592, 10737418240, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.hardLimit > 0 && tt.softLimit <= tt.hardLimit
			if valid != tt.valid {
				t.Errorf("Expected valid=%v, got %v", tt.valid, valid)
			}
		})
	}
}

// ========== 使用率计算测试 ==========

func TestQuotaUsage_Percentage(t *testing.T) {
	tests := []struct {
		hardLimit   uint64
		usedBytes   uint64
		minExpected float64
		maxExpected float64
	}{
		{10737418240, 5368709120, 49.9, 50.1},    // 10GB / 5GB = 50%
		{10737418240, 10737418240, 99.9, 100.1},  // 10GB / 10GB = 100%
		{10737418240, 11811160064, 109.9, 110.1}, // 10GB / 11GB = 110%
		{10737418240, 0, -0.1, 0.1},              // 0%
	}

	for _, tt := range tests {
		percent := float64(tt.usedBytes) / float64(tt.hardLimit) * 100
		if percent < tt.minExpected || percent > tt.maxExpected {
			t.Errorf("HardLimit=%d, UsedBytes=%d: expected %.1f%%, got %.1f%%",
				tt.hardLimit, tt.usedBytes, (tt.minExpected+tt.maxExpected)/2, percent)
		}
	}
}

// ========== 并发安全测试 ==========

func TestQuota_ConcurrentAccess(t *testing.T) {
	quota := &Quota{
		ID:        "concurrent-quota",
		HardLimit: 10737418240,
	}

	done := make(chan bool)

	// 并发修改
	for i := 0; i < 10; i++ {
		go func(i int) {
			quota.HardLimit += uint64(i)
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 告警升级测试 ==========

func TestAlert_Escalation(t *testing.T) {
	now := time.Now()
	alert := &Alert{
		ID:              "escalated-alert",
		Type:            AlertTypeHardLimit,
		Severity:        AlertSeverityEmergency,
		Status:          AlertStatusEscalated,
		EscalatedAt:     &now,
		EscalationLevel: 2,
	}

	if alert.Status != AlertStatusEscalated {
		t.Errorf("Expected Status=escalated, got %s", alert.Status)
	}
	if alert.EscalationLevel != 2 {
		t.Errorf("Expected EscalationLevel=2, got %d", alert.EscalationLevel)
	}
}

// ========== 可用空间计算测试 ==========

func TestQuotaUsage_Available(t *testing.T) {
	usage := &QuotaUsage{
		HardLimit: 10737418240, // 10 GB
		UsedBytes: 3221225472,  // 3 GB
	}

	available := usage.HardLimit - usage.UsedBytes
	if available != 7516192768 { // 7 GB
		t.Errorf("Expected available=7516192768, got %d", available)
	}
}
