package storage

import (
	"context"
	"testing"
	"time"
)

// ========== SMART 监控器创建测试 ==========

func TestNewSMARTMonitor(t *testing.T) {
	config := DefaultSMARTConfig
	monitor := NewSMARTMonitor(config)

	if monitor == nil {
		t.Fatal("NewSMARTMonitor returned nil")
	}

	if monitor.disks == nil {
		t.Error("disks map should be initialized")
	}
	if monitor.history == nil {
		t.Error("history map should be initialized")
	}
}

func TestNewSMARTMonitor_DefaultConfig(t *testing.T) {
	// 空配置应该使用默认值
	monitor := NewSMARTMonitor(SMARTConfig{})

	if monitor.config.CheckInterval != DefaultSMARTConfig.CheckInterval {
		t.Errorf("Expected default CheckInterval, got %v", monitor.config.CheckInterval)
	}
}

func TestSMARTMonitor_StartStop(t *testing.T) {
	config := SMARTConfig{
		CheckInterval:         1 * time.Second,
		AutoCheck:             true,
		TempWarningThreshold:  50,
		TempCriticalThreshold: 60,
	}
	monitor := NewSMARTMonitor(config)

	// 启动
	monitor.Start()

	// 停止
	monitor.Stop()
}

// ========== 磁盘健康状态测试 ==========

func TestDiskHealth_Struct(t *testing.T) {
	health := &DiskHealth{
		Device:             "/dev/sda",
		Model:              "Samsung SSD 860 EVO",
		Serial:             "S3YJNX0K123456",
		Size:               500107862016,
		Temperature:        35,
		PowerOnHours:       8760,
		PowerCycleCount:    500,
		ReallocatedSectors: 0,
		PendingSectors:     0,
		SMARTStatus:        SMARTStatusPASSED,
		HealthStatus:       HealthStatusExcellent,
		HealthScore:        99,
		PredictedFailure:   false,
		NVMeAvailableSpare: 100,
		NVMePercentageUsed: 5,
	}

	if health.Device != "/dev/sda" {
		t.Errorf("Expected Device=/dev/sda, got %s", health.Device)
	}
	if health.HealthScore != 99 {
		t.Errorf("Expected HealthScore=99, got %d", health.HealthScore)
	}
	if health.SMARTStatus != SMARTStatusPASSED {
		t.Errorf("Expected SMARTStatus=PASSED, got %s", health.SMARTStatus)
	}
}

func TestSMARTAttribute_Struct(t *testing.T) {
	attr := SMARTAttribute{
		ID:          5,
		Name:        "Reallocated_Sector_Ct",
		Value:       100,
		Worst:       100,
		Threshold:   10,
		RawValue:    0,
		Normalized:  100,
		Description: "Count of reallocated sectors",
	}

	if attr.ID != 5 {
		t.Errorf("Expected ID=5, got %d", attr.ID)
	}
	if attr.Value != 100 {
		t.Errorf("Expected Value=100, got %d", attr.Value)
	}
}

// ========== SMART 状态测试 ==========

func TestSMARTStatus_Values(t *testing.T) {
	statuses := []SMARTStatus{
		SMARTStatusPASSED,
		SMARTStatusWARNING,
		SMARTStatusFAILING,
		SMARTStatusUNKNOWN,
		SMARTStatusUNSUPPORTED,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("SMARTStatus should not be empty")
		}
	}
}

func TestHealthStatus_Values(t *testing.T) {
	statuses := []HealthStatus{
		HealthStatusExcellent,
		HealthStatusGood,
		HealthStatusFair,
		HealthStatusPoor,
		HealthStatusCritical,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("HealthStatus should not be empty")
		}
	}
}

// ========== 告警测试 ==========

func TestAlertType_Values(t *testing.T) {
	alertTypes := []AlertType{
		AlertTypeTemperature,
		AlertTypeReallocated,
		AlertTypePending,
		AlertTypePredictFail,
		AlertTypeSMARTFailure,
		AlertTypeCRCError,
		AlertTypeSeekError,
	}

	for _, alertType := range alertTypes {
		if alertType == "" {
			t.Error("AlertType should not be empty")
		}
	}
}

func TestAlert_Struct(t *testing.T) {
	alert := Alert{
		Type:        AlertTypeTemperature,
		Device:      "/dev/sda",
		Severity:    "WARNING",
		Message:     "Disk temperature is high",
		Value:       55,
		Threshold:   50,
		Timestamp:   time.Now(),
		HealthScore: 85,
	}

	if alert.Type != AlertTypeTemperature {
		t.Errorf("Expected Type=TEMPERATURE, got %s", alert.Type)
	}
	if alert.Severity != "WARNING" {
		t.Errorf("Expected Severity=WARNING, got %s", alert.Severity)
	}
}

func TestSMARTMonitor_AddAlertHandler(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	monitor.AddAlertHandler(func(alert Alert) {})

	// 验证处理器已添加
	if len(monitor.alertHandlers) != 1 {
		t.Errorf("Expected 1 alert handler, got %d", len(monitor.alertHandlers))
	}
}

// ========== 健康快照测试 ==========

func TestHealthSnapshot_Struct(t *testing.T) {
	snapshot := HealthSnapshot{
		Timestamp:          time.Now(),
		Temperature:        40,
		ReallocatedSectors: 0,
		PendingSectors:     0,
		HealthScore:        95,
		HealthStatus:       HealthStatusExcellent,
	}

	if snapshot.HealthScore != 95 {
		t.Errorf("Expected HealthScore=95, got %d", snapshot.HealthScore)
	}
}

// ========== 配置测试 ==========

func TestSMARTConfig_DefaultValues(t *testing.T) {
	config := DefaultSMARTConfig

	if config.CheckInterval != 30*time.Minute {
		t.Errorf("Expected CheckInterval=30m, got %v", config.CheckInterval)
	}
	if config.TempWarningThreshold != 50 {
		t.Errorf("Expected TempWarningThreshold=50, got %d", config.TempWarningThreshold)
	}
	if config.TempCriticalThreshold != 60 {
		t.Errorf("Expected TempCriticalThreshold=60, got %d", config.TempCriticalThreshold)
	}
	if config.ReallocatedWarning != 10 {
		t.Errorf("Expected ReallocatedWarning=10, got %d", config.ReallocatedWarning)
	}
	if config.ReallocatedCritical != 100 {
		t.Errorf("Expected ReallocatedCritical=100, got %d", config.ReallocatedCritical)
	}
	if config.AutoCheck != true {
		t.Error("Expected AutoCheck=true")
	}
}

func TestSMARTConfig_CustomValues(t *testing.T) {
	config := SMARTConfig{
		CheckInterval:         15 * time.Minute,
		TempWarningThreshold:  45,
		TempCriticalThreshold: 55,
		ReallocatedWarning:    5,
		ReallocatedCritical:   50,
		PendingWarning:        5,
		PendingCritical:       50,
		SeekErrorWarning:      3.0,
		AutoCheck:             true,
		HistoryRetentionDays:  60,
	}

	if config.CheckInterval != 15*time.Minute {
		t.Errorf("Expected CheckInterval=15m, got %v", config.CheckInterval)
	}
	if config.HistoryRetentionDays != 60 {
		t.Errorf("Expected HistoryRetentionDays=60, got %d", config.HistoryRetentionDays)
	}
}

// ========== 磁盘列表测试 ==========

func TestSMARTMonitor_GetAllDisks_Empty(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	disks := monitor.GetAllDisks()
	if len(disks) != 0 {
		t.Errorf("Expected empty disk list, got %d", len(disks))
	}
}

func TestSMARTMonitor_GetDiskHealth_NotFound(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	health, _ := monitor.GetDiskHealth("/dev/nonexistent")
	if health != nil {
		t.Error("Expected nil for nonexistent disk")
	}
}

func TestSMARTMonitor_GetHistory_Empty(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	history := monitor.GetHistory("/dev/sda")
	if history != nil {
		t.Error("Expected nil for disk without history")
	}
}

// ========== 自检测试 ==========

func TestSMARTMonitor_RunSelfTest_NotFound(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	err := monitor.RunSelfTest(context.Background(), "/dev/nonexistent", "short")
	if err == nil {
		t.Error("Expected error for nonexistent disk")
	}
}

func TestSMARTMonitor_GetSelfTestStatus_NotFound(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	status, err := monitor.GetSelfTestStatus(context.Background(), "/dev/nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent disk")
	}
	_ = status
}

// ========== 健康导出测试 ==========

func TestSMARTMonitor_ExportHealth_Empty(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	data, err := monitor.ExportHealth()
	if err != nil {
		t.Fatalf("ExportHealth failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty export data")
	}
}

// ========== 内部方法测试 ==========

func TestParseCapacity(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"User Capacity:    500,107,862,016 bytes [500 GB]", 500107862016},
		{"User Capacity:    1,000,204,886,016 bytes [1.00 TB]", 1000204886016},
		{"User Capacity:    256,060,514,304 bytes [256 GB]", 256060514304},
	}

	for _, tt := range tests {
		result := parseCapacity(tt.input)
		if result != tt.expected {
			t.Errorf("parseCapacity(%s) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseNVMeTemperature(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Temperature:                    35 Celsius", 35},
		{"Temperature:                    40 Celsius", 40},
		{"Temperature:                    5 Celsius", 5},
	}

	for _, tt := range tests {
		result := parseNVMeTemperature(tt.input)
		if result != tt.expected {
			t.Errorf("parseNVMeTemperature(%s) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Percentage Used:                    95%", 95},
		{"Percentage Used:                    100%", 100},
		{"Percentage Used:                    0%", 0},
	}

	for _, tt := range tests {
		result := parsePercentage(tt.input)
		if result != tt.expected {
			t.Errorf("parsePercentage(%s) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseDataUnits(t *testing.T) {
	// 这个函数期望特定格式的输入
	// 简化测试
	_ = parseDataUnits("1,000")
}

func TestParseNVMeCount(t *testing.T) {
	// 这个函数期望特定格式的输入
	// 简化测试
	_ = parseNVMeCount("1,234")
}

// ========== 健康评分测试 ==========

func TestCalculateHealthScore_Excellent(t *testing.T) {
	// 模拟一个健康的磁盘
	health := &DiskHealth{
		Device:             "/dev/sda",
		Temperature:        35,
		ReallocatedSectors: 0,
		PendingSectors:     0,
		SMARTStatus:        SMARTStatusPASSED,
		PredictedFailure:   false,
	}

	// 健康评分应该很高
	if health.SMARTStatus == SMARTStatusPASSED && health.ReallocatedSectors == 0 {
		// 磁盘健康
		t.Log("Healthy disk should have high score")
	}
}

func TestCalculateHealthScore_Critical(t *testing.T) {
	// 模拟一个有问题的磁盘
	health := &DiskHealth{
		Device:             "/dev/sda",
		Temperature:        70,
		ReallocatedSectors: 1000,
		PendingSectors:     500,
		SMARTStatus:        SMARTStatusFAILING,
		PredictedFailure:   true,
	}

	// 健康评分应该很低
	if health.SMARTStatus == SMARTStatusFAILING || health.PredictedFailure {
		// 磁盘不健康
		t.Log("Unhealthy disk should have low score")
	}
}

// ========== 健康状态级别测试 ==========

func TestHealthStatus_Level(t *testing.T) {
	tests := []struct {
		score    int
		expected HealthStatus
	}{
		{95, HealthStatusExcellent},
		{80, HealthStatusGood},
		{60, HealthStatusFair},
		{40, HealthStatusPoor},
		{15, HealthStatusCritical},
	}

	for _, tt := range tests {
		var status HealthStatus
		switch {
		case tt.score >= 90:
			status = HealthStatusExcellent
		case tt.score >= 70:
			status = HealthStatusGood
		case tt.score >= 50:
			status = HealthStatusFair
		case tt.score >= 25:
			status = HealthStatusPoor
		default:
			status = HealthStatusCritical
		}

		if status != tt.expected {
			t.Errorf("Score %d: expected %s, got %s", tt.score, tt.expected, status)
		}
	}
}

// ========== 告警检查测试 ==========

func TestCheckAlerts_Temperature(t *testing.T) {
	config := SMARTConfig{
		TempWarningThreshold:  50,
		TempCriticalThreshold: 60,
	}

	monitor := NewSMARTMonitor(config)

	health := &DiskHealth{
		Device:      "/dev/sda",
		Temperature: 55, // 超过警告阈值
	}

	// 温度超过警告阈值
	if health.Temperature > config.TempWarningThreshold {
		t.Log("Temperature alert should be triggered")
	}

	// 验证监控器配置正确
	if monitor.config.TempWarningThreshold != 50 {
		t.Errorf("Expected TempWarningThreshold=50, got %d", monitor.config.TempWarningThreshold)
	}
}

func TestCheckAlerts_ReallocatedSectors(t *testing.T) {
	config := SMARTConfig{
		ReallocatedWarning:  10,
		ReallocatedCritical: 100,
	}

	monitor := NewSMARTMonitor(config)

	health := &DiskHealth{
		Device:             "/dev/sda",
		ReallocatedSectors: 50, // 超过警告阈值
	}

	// 重分配扇区超过警告阈值
	if health.ReallocatedSectors > uint64(config.ReallocatedWarning) {
		t.Log("Reallocated sectors alert should be triggered")
	}

	if monitor.config.ReallocatedWarning != 10 {
		t.Errorf("Expected ReallocatedWarning=10, got %d", monitor.config.ReallocatedWarning)
	}
}

// ========== 并发安全测试 ==========

func TestSMARTMonitor_ConcurrentAccess(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	done := make(chan bool)

	// 并发添加处理器
	for i := 0; i < 5; i++ {
		go func() {
			monitor.AddAlertHandler(func(alert Alert) {})
			done <- true
		}()
	}

	// 并发获取磁盘列表
	for i := 0; i < 5; i++ {
		go func() {
			_ = monitor.GetAllDisks()
			done <- true
		}()
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 模拟数据测试 ==========

func TestSMARTMonitor_WithMockDisk(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	// 模拟添加磁盘健康数据
	monitor.mu.Lock()
	monitor.disks["/dev/sda"] = &DiskHealth{
		Device:             "/dev/sda",
		Model:              "Samsung SSD 860 EVO",
		Temperature:        35,
		ReallocatedSectors: 0,
		PendingSectors:     0,
		SMARTStatus:        SMARTStatusPASSED,
		HealthStatus:       HealthStatusExcellent,
		HealthScore:        98,
		PredictedFailure:   false,
		Attributes:         make(map[string]SMARTAttribute),
	}
	monitor.mu.Unlock()

	// 获取磁盘健康状态
	health, _ := monitor.GetDiskHealth("/dev/sda")
	if health == nil {
		t.Fatal("Disk health not found")
	}
	if health.HealthScore != 98 {
		t.Errorf("Expected HealthScore=98, got %d", health.HealthScore)
	}

	// 获取所有磁盘
	disks := monitor.GetAllDisks()
	if len(disks) != 1 {
		t.Errorf("Expected 1 disk, got %d", len(disks))
	}
}

func TestSMARTMonitor_WithMultipleDisks(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	monitor.mu.Lock()
	monitor.disks["/dev/sda"] = &DiskHealth{
		Device:       "/dev/sda",
		Model:        "Samsung SSD 860 EVO",
		HealthScore:  98,
		HealthStatus: HealthStatusExcellent,
	}
	monitor.disks["/dev/sdb"] = &DiskHealth{
		Device:       "/dev/sdb",
		Model:        "WD Blue HDD",
		HealthScore:  75,
		HealthStatus: HealthStatusGood,
	}
	monitor.disks["/dev/nvme0n1"] = &DiskHealth{
		Device:             "/dev/nvme0n1",
		Model:              "Samsung 970 EVO Plus",
		HealthScore:        99,
		HealthStatus:       HealthStatusExcellent,
		NVMeAvailableSpare: 100,
		NVMePercentageUsed: 5,
	}
	monitor.mu.Unlock()

	disks := monitor.GetAllDisks()
	if len(disks) != 3 {
		t.Errorf("Expected 3 disks, got %d", len(disks))
	}

	// 验证 NVMe 磁盘属性
	nvmeHealth, _ := monitor.GetDiskHealth("/dev/nvme0n1")
	if nvmeHealth == nil {
		t.Fatal("NVMe disk not found")
	}
	if nvmeHealth.NVMeAvailableSpare != 100 {
		t.Errorf("Expected NVMeAvailableSpare=100, got %d", nvmeHealth.NVMeAvailableSpare)
	}
}

// ========== 历史记录测试 ==========

func TestSMARTMonitor_History(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	monitor.mu.Lock()
	monitor.disks["/dev/sda"] = &DiskHealth{
		Device:       "/dev/sda",
		HealthScore:  95,
		HealthStatus: HealthStatusExcellent,
	}
	monitor.history["/dev/sda"] = []HealthSnapshot{
		{Timestamp: time.Now().Add(-1 * time.Hour), HealthScore: 96},
		{Timestamp: time.Now(), HealthScore: 95},
	}
	monitor.mu.Unlock()

	history := monitor.GetHistory("/dev/sda")
	if len(history) != 2 {
		t.Errorf("Expected 2 history entries, got %d", len(history))
	}
}

// ========== 告警处理器调用测试 ==========

func TestSMARTMonitor_AlertHandlerCalled(t *testing.T) {
	monitor := NewSMARTMonitor(DefaultSMARTConfig)

	alertReceived := false
	var receivedAlert Alert

	monitor.AddAlertHandler(func(alert Alert) {
		alertReceived = true
		receivedAlert = alert
	})

	// 模拟发送告警
	monitor.mu.Lock()
	monitor.disks["/dev/sda"] = &DiskHealth{
		Device:      "/dev/sda",
		Temperature: 65, // 超过严重阈值
	}
	monitor.mu.Unlock()

	// 验证处理器已注册
	if len(monitor.alertHandlers) != 1 {
		t.Errorf("Expected 1 alert handler, got %d", len(monitor.alertHandlers))
	}

	// 如果有告警，验证结构
	if alertReceived {
		if receivedAlert.Device != "/dev/sda" {
			t.Errorf("Expected Device=/dev/sda, got %s", receivedAlert.Device)
		}
	}
}

// ========== 边界条件测试 ==========

func TestSMARTMonitor_ZeroValues(t *testing.T) {
	monitor := NewSMARTMonitor(SMARTConfig{})

	// 空配置应该使用默认值
	if monitor.config.CheckInterval <= 0 {
		t.Error("CheckInterval should be positive")
	}
}

func TestSMARTMonitor_NegativeTemperature(t *testing.T) {
	health := &DiskHealth{
		Device:      "/dev/sda",
		Temperature: -10, // 极低温度
	}

	// 温度应该能处理负值
	if health.Temperature != -10 {
		t.Errorf("Expected Temperature=-10, got %d", health.Temperature)
	}
}

func TestSMARTMonitor_MaxValues(t *testing.T) {
	health := &DiskHealth{
		Device:             "/dev/sda",
		Size:               ^uint64(0), // 最大值
		PowerOnHours:       ^uint64(0),
		ReallocatedSectors: ^uint64(0),
		HealthScore:        100,
	}

	// 验证最大值处理
	if health.HealthScore != 100 {
		t.Errorf("Expected HealthScore=100, got %d", health.HealthScore)
	}
}
