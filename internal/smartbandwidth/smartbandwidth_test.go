package smartbandwidth

import (
	"testing"
)

func TestNewSmartBandwidthManager(t *testing.T) {
	config := &SmartBandwidthConfig{
		TotalBandwidthMbps: 1000,
		Enabled:            true,
		Interface:          "eth0",
		AdjustInterval:     30,
	}

	manager := NewSmartBandwidthManager(config)

	if manager == nil {
		t.Fatal("NewSmartBandwidthManager returned nil")
	}

	if manager.config.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected TotalBandwidthMbps 1000, got %d", manager.config.TotalBandwidthMbps)
	}

	if !manager.config.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

func TestNewSmartBandwidthManagerWithNilConfig(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	if manager == nil {
		t.Fatal("NewSmartBandwidthManager returned nil")
	}

	if manager.config.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected default TotalBandwidthMbps 1000, got %d", manager.config.TotalBandwidthMbps)
	}
}

func TestSetBandwidthLimit(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     8,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected ID to be set")
	}

	if created.Name != "Test Rule" {
		t.Errorf("Expected name 'Test Rule', got '%s'", created.Name)
	}

	if created.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", created.Priority)
	}

	if !created.Enabled {
		t.Error("Expected rule to be enabled")
	}
}

func TestSetBandwidthLimitValidation(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	tests := []struct {
		name    string
		rule    *BandwidthRule
		wantErr bool
	}{
		{
			name: "empty name",
			rule: &BandwidthRule{
				Priority: 5,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid priority low",
			rule: &BandwidthRule{
				Name:     "Test",
				Priority: 0,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid priority high",
			rule: &BandwidthRule{
				Name:     "Test",
				Priority: 11,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "zero max bandwidth",
			rule: &BandwidthRule{
				Name:     "Test",
				Priority: 5,
				MaxMbps:  0,
			},
			wantErr: true,
		},
		{
			name: "negative min bandwidth",
			rule: &BandwidthRule{
				Name:     "Test",
				Priority: 5,
				MinMbps:  -1,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "min greater than max",
			rule: &BandwidthRule{
				Name:     "Test",
				Priority: 5,
				MinMbps:  200,
				MaxMbps:  100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.SetBandwidthLimit(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetBandwidthLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClassifyTraffic(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	tests := []struct {
		name     string
		srcIP    string
		dstIP    string
		srcPort  int
		dstPort  int
		protocol string
		want     TrafficClass
	}{
		{
			name:     "HTTPS traffic",
			dstPort:  443,
			protocol: "tcp",
			want:     TrafficClassWeb,
		},
		{
			name:     "HTTP traffic",
			dstPort:  80,
			protocol: "tcp",
			want:     TrafficClassWeb,
		},
		{
			name:     "SSH traffic",
			dstPort:  22,
			protocol: "tcp",
			want:     TrafficClassFileTransfer,
		},
		{
			name:     "AI inference",
			dstPort:  8080,
			protocol: "tcp",
			want:     TrafficClassAIInference,
		},
		{
			name:     "VoIP SIP",
			dstPort:  5060,
			protocol: "udp",
			want:     TrafficClassVoIP,
		},
		{
			name:     "RDP traffic",
			dstPort:  3389,
			protocol: "tcp",
			want:     TrafficClassGaming,
		},
		{
			name:     "Plex streaming",
			dstPort:  32400,
			protocol: "tcp",
			want:     TrafficClassStreaming,
		},
		{
			name:     "FTP traffic",
			dstPort:  21,
			protocol: "tcp",
			want:     TrafficClassDownload,
		},
		{
			name:     "Rsync backup",
			dstPort:  873,
			protocol: "tcp",
			want:     TrafficClassBackup,
		},
		{
			name:     "unknown traffic",
			dstPort:  12345,
			protocol: "tcp",
			want:     TrafficClassOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.ClassifyTraffic(tt.srcIP, tt.dstIP, tt.srcPort, tt.dstPort, tt.protocol)
			if got != tt.want {
				t.Errorf("ClassifyTraffic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBandwidthStats(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	// 创建规则
	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 获取统计
	stats, err := manager.GetBandwidthStats(created.ID)
	if err != nil {
		t.Fatalf("GetBandwidthStats failed: %v", err)
	}

	if stats.RuleID != created.ID {
		t.Errorf("Expected RuleID %s, got %s", created.ID, stats.RuleID)
	}

	if stats.TrafficClass != TrafficClassVideo {
		t.Errorf("Expected TrafficClass 'video', got '%s'", stats.TrafficClass)
	}
}

func TestGetBandwidthStatsNotFound(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	_, err := manager.GetBandwidthStats("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent stats")
	}
}

func TestGetAllBandwidthStats(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	// 创建多个规则
	rules := []*BandwidthRule{
		{Name: "Rule 1", TrafficClass: TrafficClassVideo, Priority: 5, MinMbps: 10, MaxMbps: 100},
		{Name: "Rule 2", TrafficClass: TrafficClassWeb, Priority: 3, MinMbps: 5, MaxMbps: 50},
	}

	for _, rule := range rules {
		_, err := manager.SetBandwidthLimit(rule)
		if err != nil {
			t.Fatalf("SetBandwidthLimit failed: %v", err)
		}
	}

	stats := manager.GetAllBandwidthStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats, got %d", len(stats))
	}
}

func TestGetBandwidthStatsByClass(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	// 创建多个规则
	rules := []*BandwidthRule{
		{Name: "Video 1", TrafficClass: TrafficClassVideo, Priority: 5, MinMbps: 10, MaxMbps: 100},
		{Name: "Video 2", TrafficClass: TrafficClassVideo, Priority: 7, MinMbps: 20, MaxMbps: 200},
		{Name: "Web 1", TrafficClass: TrafficClassWeb, Priority: 3, MinMbps: 5, MaxMbps: 50},
	}

	for _, rule := range rules {
		_, err := manager.SetBandwidthLimit(rule)
		if err != nil {
			t.Fatalf("SetBandwidthLimit failed: %v", err)
		}
	}

	videoStats := manager.GetBandwidthStatsByClass(TrafficClassVideo)
	if len(videoStats) != 2 {
		t.Errorf("Expected 2 video stats, got %d", len(videoStats))
	}

	webStats := manager.GetBandwidthStatsByClass(TrafficClassWeb)
	if len(webStats) != 1 {
		t.Errorf("Expected 1 web stat, got %d", len(webStats))
	}
}

func TestApplyQoSPolicy(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	policy := &QoSPolicy{
		Name:     "Critical Service",
		Priority: 10,
		MinMbps:  50,
		MaxMbps:  500,
	}

	created, err := manager.ApplyQoSPolicy(policy)
	if err != nil {
		t.Fatalf("ApplyQoSPolicy failed: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected ID to be set")
	}

	if created.Name != "Critical Service" {
		t.Errorf("Expected name 'Critical Service', got '%s'", created.Name)
	}

	if created.Priority != 10 {
		t.Errorf("Expected priority 10, got %d", created.Priority)
	}
}

func TestApplyQoSPolicyValidation(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	tests := []struct {
		name    string
		policy  *QoSPolicy
		wantErr bool
	}{
		{
			name: "empty name",
			policy: &QoSPolicy{
				Priority: 5,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			policy: &QoSPolicy{
				Name:     "Test",
				Priority: 11,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "zero max bandwidth",
			policy: &QoSPolicy{
				Name:     "Test",
				Priority: 5,
				MaxMbps:  0,
			},
			wantErr: true,
		},
		{
			name: "min greater than max",
			policy: &QoSPolicy{
				Name:     "Test",
				Priority: 5,
				MinMbps:  200,
				MaxMbps:  100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ApplyQoSPolicy(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyQoSPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdjustDynamic(t *testing.T) {
	manager := NewSmartBandwidthManager(&SmartBandwidthConfig{
		TotalBandwidthMbps: 1000,
		Enabled:            true,
		AdjustInterval:     30,
	})

	// 创建规则
	rules := []*BandwidthRule{
		{Name: "High Priority", TrafficClass: TrafficClassVideo, Priority: 10, MinMbps: 100, MaxMbps: 500},
		{Name: "Low Priority", TrafficClass: TrafficClassWeb, Priority: 3, MinMbps: 10, MaxMbps: 200},
	}

	for _, rule := range rules {
		_, err := manager.SetBandwidthLimit(rule)
		if err != nil {
			t.Fatalf("SetBandwidthLimit failed: %v", err)
		}
	}

	// 执行动态调整
	err := manager.AdjustDynamic()
	if err != nil {
		t.Fatalf("AdjustDynamic failed: %v", err)
	}

	// 验证统计已更新
	stats := manager.GetAllBandwidthStats()
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats, got %d", len(stats))
	}
}

func TestAdjustDynamicDisabled(t *testing.T) {
	manager := NewSmartBandwidthManager(&SmartBandwidthConfig{
		TotalBandwidthMbps: 1000,
		Enabled:            false,
		AdjustInterval:     30,
	})

	err := manager.AdjustDynamic()
	if err == nil {
		t.Error("Expected error when disabled")
	}
}

func TestDeleteBandwidthRule(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 删除规则
	err = manager.DeleteBandwidthRule(created.ID)
	if err != nil {
		t.Fatalf("DeleteBandwidthRule failed: %v", err)
	}

	// 验证已删除
	_, err = manager.GetBandwidthRule(created.ID)
	if err == nil {
		t.Error("Expected error for deleted rule")
	}
}

func TestDeleteBandwidthRuleNotFound(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	err := manager.DeleteBandwidthRule("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestUpdateBandwidthRule(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 更新规则
	update := &BandwidthRule{
		Name:     "Updated Rule",
		Priority: 8,
		MaxMbps:  200,
	}

	updated, err := manager.UpdateBandwidthRule(created.ID, update)
	if err != nil {
		t.Fatalf("UpdateBandwidthRule failed: %v", err)
	}

	if updated.Name != "Updated Rule" {
		t.Errorf("Expected name 'Updated Rule', got '%s'", updated.Name)
	}

	if updated.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", updated.Priority)
	}

	if updated.MaxMbps != 200 {
		t.Errorf("Expected MaxMbps 200, got %d", updated.MaxMbps)
	}
}

func TestUpdateBandwidthRuleNotFound(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	_, err := manager.UpdateBandwidthRule("nonexistent", &BandwidthRule{Name: "Test"})
	if err == nil {
		t.Error("Expected error for nonexistent rule")
	}
}

func TestEnableDisableBandwidthRule(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 禁用规则
	err = manager.DisableBandwidthRule(created.ID)
	if err != nil {
		t.Fatalf("DisableBandwidthRule failed: %v", err)
	}

	fetched, _ := manager.GetBandwidthRule(created.ID)
	if fetched.Enabled {
		t.Error("Expected rule to be disabled")
	}

	// 启用规则
	err = manager.EnableBandwidthRule(created.ID)
	if err != nil {
		t.Fatalf("EnableBandwidthRule failed: %v", err)
	}

	fetched, _ = manager.GetBandwidthRule(created.ID)
	if !fetched.Enabled {
		t.Error("Expected rule to be enabled")
	}
}

func TestCreateTrafficProfile(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	profile := &TrafficProfile{
		Name:          "Video Streaming",
		TrafficClass:  TrafficClassStreaming,
		Priority:      7,
		MinMbps:       20,
		MaxMbps:       200,
		Description:   "Video streaming profile",
	}

	created, err := manager.CreateTrafficProfile(profile)
	if err != nil {
		t.Fatalf("CreateTrafficProfile failed: %v", err)
	}

	if created.ID == "" {
		t.Error("Expected ID to be set")
	}

	if created.Name != "Video Streaming" {
		t.Errorf("Expected name 'Video Streaming', got '%s'", created.Name)
	}
}

func TestCreateTrafficProfileValidation(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	tests := []struct {
		name    string
		profile *TrafficProfile
		wantErr bool
	}{
		{
			name: "empty name",
			profile: &TrafficProfile{
				Priority: 5,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid priority",
			profile: &TrafficProfile{
				Name:     "Test",
				Priority: 11,
				MaxMbps:  100,
			},
			wantErr: true,
		},
		{
			name: "zero max bandwidth",
			profile: &TrafficProfile{
				Name:     "Test",
				Priority: 5,
				MaxMbps:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.CreateTrafficProfile(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTrafficProfile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTrafficProfiles(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	profiles := []*TrafficProfile{
		{Name: "Profile 1", TrafficClass: TrafficClassVideo, Priority: 5, MinMbps: 10, MaxMbps: 100},
		{Name: "Profile 2", TrafficClass: TrafficClassWeb, Priority: 3, MinMbps: 5, MaxMbps: 50},
	}

	for _, profile := range profiles {
		_, err := manager.CreateTrafficProfile(profile)
		if err != nil {
			t.Fatalf("CreateTrafficProfile failed: %v", err)
		}
	}

	result := manager.GetTrafficProfiles()
	if len(result) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(result))
	}
}

func TestGetTrafficProfile(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	profile := &TrafficProfile{
		Name:         "Video Streaming",
		TrafficClass: TrafficClassStreaming,
		Priority:     7,
		MinMbps:      20,
		MaxMbps:      200,
	}

	created, err := manager.CreateTrafficProfile(profile)
	if err != nil {
		t.Fatalf("CreateTrafficProfile failed: %v", err)
	}

	fetched, err := manager.GetTrafficProfile(created.ID)
	if err != nil {
		t.Fatalf("GetTrafficProfile failed: %v", err)
	}

	if fetched.Name != "Video Streaming" {
		t.Errorf("Expected name 'Video Streaming', got '%s'", fetched.Name)
	}
}

func TestGetTrafficProfileNotFound(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	_, err := manager.GetTrafficProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestDeleteQoSPolicy(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	policy := &QoSPolicy{
		Name:     "Test Policy",
		Priority: 5,
		MinMbps:  10,
		MaxMbps:  100,
	}

	created, err := manager.ApplyQoSPolicy(policy)
	if err != nil {
		t.Fatalf("ApplyQoSPolicy failed: %v", err)
	}

	err = manager.DeleteQoSPolicy(created.ID)
	if err != nil {
		t.Fatalf("DeleteQoSPolicy failed: %v", err)
	}

	_, err = manager.GetQoSPolicy(created.ID)
	if err == nil {
		t.Error("Expected error for deleted policy")
	}
}

func TestDeleteQoSPolicyNotFound(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	err := manager.DeleteQoSPolicy("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent policy")
	}
}

func TestUpdateQoSPolicy(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	policy := &QoSPolicy{
		Name:     "Test Policy",
		Priority: 5,
		MinMbps:  10,
		MaxMbps:  100,
	}

	created, err := manager.ApplyQoSPolicy(policy)
	if err != nil {
		t.Fatalf("ApplyQoSPolicy failed: %v", err)
	}

	update := &QoSPolicy{
		Name:     "Updated Policy",
		Priority: 8,
		MaxMbps:  200,
	}

	updated, err := manager.UpdateQoSPolicy(created.ID, update)
	if err != nil {
		t.Fatalf("UpdateQoSPolicy failed: %v", err)
	}

	if updated.Name != "Updated Policy" {
		t.Errorf("Expected name 'Updated Policy', got '%s'", updated.Name)
	}

	if updated.Priority != 8 {
		t.Errorf("Expected priority 8, got %d", updated.Priority)
	}
}

func TestListQoSPolicies(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	policies := []*QoSPolicy{
		{Name: "Policy 1", Priority: 5, MinMbps: 10, MaxMbps: 100},
		{Name: "Policy 2", Priority: 8, MinMbps: 20, MaxMbps: 200},
	}

	for _, policy := range policies {
		_, err := manager.ApplyQoSPolicy(policy)
		if err != nil {
			t.Fatalf("ApplyQoSPolicy failed: %v", err)
		}
	}

	result := manager.ListQoSPolicies()
	if len(result) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(result))
	}
}

func TestListBandwidthRules(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	rules := []*BandwidthRule{
		{Name: "Rule 1", TrafficClass: TrafficClassVideo, Priority: 5, MinMbps: 10, MaxMbps: 100},
		{Name: "Rule 2", TrafficClass: TrafficClassWeb, Priority: 3, MinMbps: 5, MaxMbps: 50},
	}

	for _, rule := range rules {
		_, err := manager.SetBandwidthLimit(rule)
		if err != nil {
			t.Fatalf("SetBandwidthLimit failed: %v", err)
		}
	}

	result := manager.ListBandwidthRules()
	if len(result) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(result))
	}
}

func TestManager(t *testing.T) {
	config := &SmartBandwidthConfig{
		TotalBandwidthMbps: 1000,
		Enabled:            true,
		Interface:          "eth0",
		AdjustInterval:     30,
	}

	manager := NewManager(config)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// 测试创建规则
	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 测试获取带宽使用情况
	usage := manager.GetBandwidthUsage()
	if usage == nil {
		t.Fatal("GetBandwidthUsage returned nil")
	}

	if usage.TotalMbps != 1000 {
		t.Errorf("Expected TotalMbps 1000, got %f", usage.TotalMbps)
	}

	if usage.RuleCount != 1 {
		t.Errorf("Expected RuleCount 1, got %d", usage.RuleCount)
	}

	// 测试获取类汇总
	summary := manager.GetClassSummary()
	if summary == nil {
		t.Fatal("GetClassSummary returned nil")
	}

	videoSummary, exists := summary[TrafficClassVideo]
	if !exists {
		t.Fatal("Expected video class in summary")
	}

	if videoSummary.RuleCount != 1 {
		t.Errorf("Expected RuleCount 1, got %d", videoSummary.RuleCount)
	}

	// 测试重置统计
	err = manager.ResetRuleStats(created.ID)
	if err != nil {
		t.Fatalf("ResetRuleStats failed: %v", err)
	}

	// 测试更新统计
	err = manager.UpdateStats(created.ID, 50.0, 1024, 10)
	if err != nil {
		t.Fatalf("UpdateStats failed: %v", err)
	}

	stats, _ := manager.GetBandwidthStats(created.ID)
	if stats.CurrentMbps != 50.0 {
		t.Errorf("Expected CurrentMbps 50.0, got %f", stats.CurrentMbps)
	}

	if stats.TotalBytes != 1024 {
		t.Errorf("Expected TotalBytes 1024, got %d", stats.TotalBytes)
	}

	// 测试获取配置
	cfg := manager.GetConfig()
	if cfg.TotalBandwidthMbps != 1000 {
		t.Errorf("Expected TotalBandwidthMbps 1000, got %d", cfg.TotalBandwidthMbps)
	}

	// 测试更新配置
	manager.UpdateConfig(&SmartBandwidthConfig{
		TotalBandwidthMbps: 2000,
		Interface:          "eth1",
	})

	cfg = manager.GetConfig()
	if cfg.TotalBandwidthMbps != 2000 {
		t.Errorf("Expected TotalBandwidthMbps 2000, got %d", cfg.TotalBandwidthMbps)
	}

	if cfg.Interface != "eth1" {
		t.Errorf("Expected Interface 'eth1', got '%s'", cfg.Interface)
	}

	// 测试重置所有统计
	manager.ResetStats()
	stats, _ = manager.GetBandwidthStats(created.ID)
	if stats.CurrentMbps != 0 {
		t.Errorf("Expected CurrentMbps 0 after reset, got %f", stats.CurrentMbps)
	}
}

func TestClassifyTrafficWithRules(t *testing.T) {
	manager := NewSmartBandwidthManager(nil)

	// 创建自定义规则
	rule := &BandwidthRule{
		Name:         "Custom AI Service",
		TrafficClass: TrafficClassAIInference,
		DestPort:     9999,
		Priority:     8,
		MinMbps:      50,
		MaxMbps:      500,
	}

	_, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 测试匹配自定义规则
	class := manager.ClassifyTraffic("192.168.1.1", "192.168.1.100", 12345, 9999, "tcp")
	if class != TrafficClassAIInference {
		t.Errorf("Expected TrafficClassAIInference, got %s", class)
	}

	// 测试不匹配自定义规则，使用默认分类
	class = manager.ClassifyTraffic("192.168.1.1", "192.168.1.100", 12345, 80, "tcp")
	if class != TrafficClassWeb {
		t.Errorf("Expected TrafficClassWeb, got %s", class)
	}
}

func TestUpdateStatsWithUtilization(t *testing.T) {
	manager := NewManager(nil)

	rule := &BandwidthRule{
		Name:         "Test Rule",
		TrafficClass: TrafficClassVideo,
		Priority:     5,
		MinMbps:      10,
		MaxMbps:      100,
	}

	created, err := manager.SetBandwidthLimit(rule)
	if err != nil {
		t.Fatalf("SetBandwidthLimit failed: %v", err)
	}

	// 更新统计，触发利用率计算
	err = manager.UpdateStats(created.ID, 75.0, 1024, 10)
	if err != nil {
		t.Fatalf("UpdateStats failed: %v", err)
	}

	stats, _ := manager.GetBandwidthStats(created.ID)
	expectedUtilization := 75.0 / 100.0 * 100 // 75%
	if stats.Utilization != expectedUtilization {
		t.Errorf("Expected Utilization %f, got %f", expectedUtilization, stats.Utilization)
	}
}
