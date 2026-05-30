package hwhealthpredict

import (
	"testing"
	"time"
)

// ========== 设备注册测试 ==========

func TestRegisterDevice(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	device := &Device{
		ID:   "disk-001",
		Name: "系统盘",
		Type: DeviceTypeSSD,
	}

	err := p.RegisterDevice(device)
	if err != nil {
		t.Fatalf("注册设备失败: %v", err)
	}

	d, err := p.GetDevice("disk-001")
	if err != nil {
		t.Fatalf("获取设备失败: %v", err)
	}
	if d.Name != "系统盘" {
		t.Errorf("期望设备名 '系统盘', 得到 '%s'", d.Name)
	}
	if d.CreatedAt.IsZero() {
		t.Error("CreatedAt 不应为零值")
	}
}

func TestRegisterDeviceValidation(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	err := p.RegisterDevice(nil)
	if err == nil {
		t.Error("注册nil设备应返回错误")
	}

	err = p.RegisterDevice(&Device{Type: DeviceTypeSSD})
	if err == nil {
		t.Error("注册空ID设备应返回错误")
	}

	err = p.RegisterDevice(&Device{ID: "test", Type: "InvalidType"})
	if err != ErrInvalidDeviceType {
		t.Errorf("期望 ErrInvalidDeviceType, 得到 %v", err)
	}
}

func TestRegisterDeviceDuplicate(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	device := &Device{ID: "disk-001", Name: "盘1", Type: DeviceTypeHDD}
	_ = p.RegisterDevice(device)

	err := p.RegisterDevice(device)
	if err != ErrDeviceExists {
		t.Errorf("期望 ErrDeviceExists, 得到 %v", err)
	}
}

func TestUnregisterDevice(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	device := &Device{ID: "disk-001", Name: "盘1", Type: DeviceTypeHDD}
	_ = p.RegisterDevice(device)

	err := p.UnregisterDevice("disk-001")
	if err != nil {
		t.Fatalf("注销设备失败: %v", err)
	}

	_, err = p.GetDevice("disk-001")
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound, 得到 %v", err)
	}
}

func TestListDevices(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	_ = p.RegisterDevice(&Device{ID: "d1", Type: DeviceTypeHDD})
	_ = p.RegisterDevice(&Device{ID: "d2", Type: DeviceTypeSSD})
	_ = p.RegisterDevice(&Device{ID: "d3", Type: DeviceTypeCPU})

	all := p.ListDevices()
	if len(all) != 3 {
		t.Errorf("期望3个设备, 得到 %d", len(all))
	}

	disks := p.ListDevicesByType(DeviceTypeHDD)
	if len(disks) != 1 {
		t.Errorf("期望1个HDD, 得到 %d", len(disks))
	}
}

// ========== SMART数据测试 ==========

func TestRecordSMARTData(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "disk-001", Type: DeviceTypeSSD})

	data := &SMARTData{
		DeviceID:    "disk-001",
		Temperature: 45,
		PowerOnHours: 10000,
		SSDLifeLeft: 85,
	}

	err := p.RecordSMARTData(data)
	if err != nil {
		t.Fatalf("记录SMART数据失败: %v", err)
	}

	latest, err := p.GetLatestSMARTData("disk-001")
	if err != nil {
		t.Fatalf("获取最新数据失败: %v", err)
	}
	if latest.Temperature != 45 {
		t.Errorf("期望温度45, 得到 %d", latest.Temperature)
	}
}

func TestRecordSMARTDataValidation(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	err := p.RecordSMARTData(nil)
	if err != ErrInvalidSMARTData {
		t.Errorf("期望 ErrInvalidSMARTData, 得到 %v", err)
	}

	err = p.RecordSMARTData(&SMARTData{})
	if err == nil {
		t.Error("空设备ID应返回错误")
	}

	err = p.RecordSMARTData(&SMARTData{DeviceID: "nonexistent"})
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound, 得到 %v", err)
	}
}

func TestGetSMARTHistory(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "disk-001", Type: DeviceTypeSSD})

	now := time.Now()
	for i := 0; i < 5; i++ {
		_ = p.RecordSMARTData(&SMARTData{
			DeviceID:    "disk-001",
			Timestamp:   now.Add(-time.Duration(5-i) * time.Hour),
			Temperature: 40 + i*2,
		})
	}

	history, err := p.GetSMARTHistory("disk-001", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("期望5条记录, 得到 %d", len(history))
	}
}

func TestGetLatestSMARTDataNoData(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "disk-001", Type: DeviceTypeSSD})

	_, err := p.GetLatestSMARTData("disk-001")
	if err != ErrNoHistoryData {
		t.Errorf("期望 ErrNoHistoryData, 得到 %v", err)
	}
}

// ========== 健康评分测试 ==========

func TestCalculateHealthScoreSSD(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:    "ssd-001",
		Temperature: 35,
		PowerOnHours: 5000,
		SSDLifeLeft: 90,
	})

	score, err := p.CalculateHealthScore("ssd-001")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score < 80 {
		t.Errorf("健康SSD分数应>=80, 得到 %d", score.Score)
	}
	if score.Status != HealthStatusExcellent && score.Status != HealthStatusGood {
		t.Errorf("健康SSD状态应为excellent或good, 得到 %s", score.Status)
	}
}

func TestCalculateHealthScoreBadSSD(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "ssd-002", Type: DeviceTypeSSD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:         "ssd-002",
		Temperature:      65,
		PowerOnHours:     55000,
		BadSectors:       50,
		ReallocatedSects: 30,
		SSDLifeLeft:      10,
		ReadErrorRate:    0.05,
	})

	score, err := p.CalculateHealthScore("ssd-002")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score > 50 {
		t.Errorf("问题SSD分数应<=50, 得到 %d", score.Score)
	}
	if len(score.RiskFactors) == 0 {
		t.Error("问题SSD应有风险因素")
	}
}

func TestCalculateHealthScoreHDD(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "hdd-001", Type: DeviceTypeHDD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:       "hdd-001",
		Temperature:    42,
		PowerOnHours:   20000,
		SeekErrorRate:  0.01,
		SpinRetryCount: 0,
	})

	score, err := p.CalculateHealthScore("hdd-001")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score < 70 {
		t.Errorf("正常HDD分数应>=70, 得到 %d", score.Score)
	}
}

func TestCalculateHealthScoreCPU(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "cpu-001", Type: DeviceTypeCPU})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:     "cpu-001",
		CPUTemp:      65,
		CPUUsage:     50.0,
		PowerOnHours: 10000,
	})

	score, err := p.CalculateHealthScore("cpu-001")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score < 80 {
		t.Errorf("正常CPU分数应>=80, 得到 %d", score.Score)
	}
}

func TestCalculateHealthScoreMemory(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "mem-001", Type: DeviceTypeMemory})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:     "mem-001",
		Temperature:  45,
		MemoryUsage:  60.0,
		MemoryErrors: 0,
	})

	score, err := p.CalculateHealthScore("mem-001")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score < 80 {
		t.Errorf("正常内存分数应>=80, 得到 %d", score.Score)
	}
}

func TestCalculateHealthScorePSU(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "psu-001", Type: DeviceTypePSU})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:     "psu-001",
		Temperature:  45,
		PSUVoltage:   12.1,
		PowerOnHours: 20000,
	})

	score, err := p.CalculateHealthScore("psu-001")
	if err != nil {
		t.Fatalf("计算健康分失败: %v", err)
	}
	if score.Score < 70 {
		t.Errorf("正常电源分数应>=70, 得到 %d", score.Score)
	}
}

func TestCalculateHealthScoreNoData(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "disk-001", Type: DeviceTypeSSD})

	_, err := p.CalculateHealthScore("disk-001")
	if err != ErrNoHistoryData {
		t.Errorf("期望 ErrNoHistoryData, 得到 %v", err)
	}
}

func TestCalculateHealthScoreNotFound(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	_, err := p.CalculateHealthScore("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound, 得到 %v", err)
	}
}

// ========== 健康历史测试 ==========

func TestGetHealthHistory(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	// 录入多条数据
	for i := 0; i < 5; i++ {
		_ = p.RecordSMARTData(&SMARTData{
			DeviceID:    "ssd-001",
			Temperature: 35 + i*3,
			SSDLifeLeft: 90 - i*5,
		})
	}

	history, err := p.GetHealthHistory("ssd-001", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}
	if history.Total != 5 {
		t.Errorf("期望5条记录, 得到 %d", history.Total)
	}
}

// ========== 寿命预测测试 ==========

func TestPredictLifespan(t *testing.T) {
	config := &Config{
		WarningThreshold:  50,
		CriticalThreshold: 30,
		FatalThreshold:    10,
		EnablePrediction:  true,
	}
	p := NewHardwareHealthPredictor(config)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	// 录入多条递减数据
	for i := 0; i < 10; i++ {
		_ = p.RecordSMARTData(&SMARTData{
			DeviceID:    "ssd-001",
			Temperature: 35,
			SSDLifeLeft: 95 - i*5,
			PowerOnHours: int64(1000 + i*1000),
		})
	}

	prediction, err := p.PredictLifespan("ssd-001")
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}
	if prediction.PredictedLifeDays <= 0 {
		t.Error("预测寿命应大于0")
	}
	if prediction.Trend == "" {
		t.Error("趋势不应为空")
	}
}

func TestPredictLifespanDisabled(t *testing.T) {
	config := &Config{
		EnablePrediction: false,
	}
	p := NewHardwareHealthPredictor(config)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	_ = p.RecordSMARTData(&SMARTData{DeviceID: "ssd-001", SSDLifeLeft: 90})
	_ = p.RecordSMARTData(&SMARTData{DeviceID: "ssd-001", SSDLifeLeft: 85})

	_, err := p.PredictLifespan("ssd-001")
	if err == nil {
		t.Error("预测功能禁用时应返回错误")
	}
}

// ========== 告警测试 ==========

func TestAlertGeneration(t *testing.T) {
	config := &Config{
		WarningThreshold:  50,
		CriticalThreshold: 30,
		FatalThreshold:    10,
	}
	p := NewHardwareHealthPredictor(config)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	// 录入导致低分的数据
	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:         "ssd-001",
		Temperature:      70,
		PowerOnHours:     60000,
		BadSectors:       100,
		ReallocatedSects: 50,
		SSDLifeLeft:      5,
		ReadErrorRate:    0.2,
	})

	alerts := p.GetAlerts("", false)
	if len(alerts) == 0 {
		t.Error("低分设备应生成告警")
	}
}

func TestAlertHandler(t *testing.T) {
	config := &Config{
		WarningThreshold:  50,
		CriticalThreshold: 30,
		FatalThreshold:    10,
	}
	p := NewHardwareHealthPredictor(config)

	alertReceived := false
	p.SetAlertHandler(func(alert *Alert) {
		alertReceived = true
	})

	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:    "ssd-001",
		Temperature: 70,
		BadSectors:  200,
		SSDLifeLeft: 5,
	})

	if !alertReceived {
		t.Error("告警回调应被触发")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	config := &Config{
		WarningThreshold:  50,
		CriticalThreshold: 30,
		FatalThreshold:    10,
	}
	p := NewHardwareHealthPredictor(config)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:         "ssd-001",
		Temperature:      70,
		PowerOnHours:     60000,
		BadSectors:       200,
		ReallocatedSects: 100,
		SSDLifeLeft:      3,
		ReadErrorRate:    0.3,
		UDMAErrors:       5,
	})

	alerts := p.GetAlerts("", false)
	if len(alerts) == 0 {
		t.Fatal("应有告警")
	}

	err := p.AcknowledgeAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("确认告警失败: %v", err)
	}

	unacked := p.GetAlerts("", true)
	for _, a := range unacked {
		if a.ID == alerts[0].ID {
			t.Error("已确认告警不应出现在未确认列表中")
		}
	}
}

// ========== 维护计划测试 ==========

func TestGenerateMaintenancePlan(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "hdd-001", Type: DeviceTypeHDD})

	_ = p.RecordSMARTData(&SMARTData{
		DeviceID:     "hdd-001",
		Temperature:  55,
		PowerOnHours: 45000,
		BadSectors:   10,
	})

	plan, err := p.GenerateMaintenancePlan("hdd-001")
	if err != nil {
		t.Fatalf("生成维护计划失败: %v", err)
	}
	if plan.DeviceID != "hdd-001" {
		t.Errorf("期望设备ID hdd-001, 得到 %s", plan.DeviceID)
	}
	if plan.Priority == "" {
		t.Error("优先级不应为空")
	}
	if plan.Replacement == nil {
		t.Error("更换计划不应为nil")
	}
}

func TestGenerateMaintenancePlanNotFound(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	_, err := p.GenerateMaintenancePlan("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("期望 ErrDeviceNotFound, 得到 %v", err)
	}
}

// ========== 导出和摘要测试 ==========

func TestExportData(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "ssd-001", Type: DeviceTypeSSD})
	_ = p.RecordSMARTData(&SMARTData{DeviceID: "ssd-001", Temperature: 40})

	data, err := p.ExportData("ssd-001")
	if err != nil {
		t.Fatalf("导出数据失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("导出数据不应为空")
	}
}

func TestGetSummary(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)
	_ = p.RegisterDevice(&Device{ID: "d1", Type: DeviceTypeHDD})
	_ = p.RegisterDevice(&Device{ID: "d2", Type: DeviceTypeSSD})
	_ = p.RecordSMARTData(&SMARTData{DeviceID: "d1", Temperature: 40})
	_ = p.RecordSMARTData(&SMARTData{DeviceID: "d2", Temperature: 40})

	summary := p.GetSummary()
	total := summary["total_devices"].(int)
	if total != 2 {
		t.Errorf("期望2个设备, 得到 %d", total)
	}
}

// ========== 健康状态测试 ==========

func TestGetHealthStatus(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{95, HealthStatusExcellent},
		{80, HealthStatusGood},
		{60, HealthStatusFair},
		{40, HealthStatusPoor},
		{20, HealthStatusCritical},
	}

	for _, tt := range tests {
		status := getHealthStatus(tt.score)
		if status != tt.expected {
			t.Errorf("分数 %d: 期望 %s, 得到 %s", tt.score, tt.expected, status)
		}
	}
}

// ========== 设备类型验证测试 ==========

func TestIsValidDeviceType(t *testing.T) {
	validTypes := []string{
		DeviceTypeHDD, DeviceTypeSSD, DeviceTypeCPU,
		DeviceTypeMemory, DeviceTypePSU, DeviceTypeGPU,
		DeviceTypeNIC, DeviceTypeRAID,
	}

	for _, vt := range validTypes {
		if !isValidDeviceType(vt) {
			t.Errorf("类型 %s 应为有效类型", vt)
		}
	}

	if isValidDeviceType("InvalidType") {
		t.Error("InvalidType 应为无效类型")
	}
}

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	p := NewHardwareHealthPredictor(nil)

	if p.config.WarningThreshold != DefaultWarningThreshold {
		t.Errorf("默认警告阈值应为 %d, 得到 %d", DefaultWarningThreshold, p.config.WarningThreshold)
	}
	if p.config.CriticalThreshold != DefaultCriticalThreshold {
		t.Errorf("默认严重阈值应为 %d, 得到 %d", DefaultCriticalThreshold, p.config.CriticalThreshold)
	}
	if !p.config.EnablePrediction {
		t.Error("默认应启用预测")
	}
}

func TestCustomConfig(t *testing.T) {
	config := &Config{
		WarningThreshold:  60,
		CriticalThreshold: 40,
		FatalThreshold:    20,
		MaxHistoryDays:    180,
		EnablePrediction:  false,
	}
	p := NewHardwareHealthPredictor(config)

	if p.config.WarningThreshold != 60 {
		t.Errorf("自定义警告阈值应为60, 得到 %d", p.config.WarningThreshold)
	}
}
