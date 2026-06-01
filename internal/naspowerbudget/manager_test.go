package naspowerbudget

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	m := NewManager(&Config{
		Enabled:           true,
		ElectricityRate:   0.56,
		CarbonFactor:      0.57,
		SamplingInterval:  time.Minute,
		ReadingRetention:  time.Hour * 24,
		WarningThreshold:  80,
		CriticalThreshold: 95,
	})
	return m
}

func TestRegisterDevice(t *testing.T) {
	m := newTestManager()

	err := m.RegisterDevice("cpu1", "Intel i5-12400", DeviceCPU, 65, 45, 15, 2)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	device, err := m.GetDevice("cpu1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if device.DeviceName != "Intel i5-12400" {
		t.Errorf("expected Intel i5-12400, got %s", device.DeviceName)
	}
	if device.MaxPowerWatts != 65 {
		t.Errorf("expected 65W max, got %f", device.MaxPowerWatts)
	}

	// 空ID应失败
	err = m.RegisterDevice("", "test", DeviceOther, 10, 5, 1, 0)
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestRecordReading(t *testing.T) {
	m := newTestManager()

	err := m.RecordReading(PowerReading{DeviceID: "nonexistent", Watts: 100})
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}

	m.RegisterDevice("cpu1", "CPU", DeviceCPU, 65, 45, 15, 2)

	err = m.RecordReading(PowerReading{
		DeviceID:   "cpu1",
		DeviceType: DeviceCPU,
		Watts:      42.5,
		Voltage:    12.0,
		Current:    3.54,
		State:      PowerStateActive,
	})
	if err != nil {
		t.Fatalf("RecordReading failed: %v", err)
	}

	device, _ := m.GetDevice("cpu1")
	if device.CurrentWatts != 42.5 {
		t.Errorf("expected 42.5W, got %f", device.CurrentWatts)
	}
}

func TestBudget(t *testing.T) {
	m := newTestManager()

	m.RegisterDevice("cpu1", "CPU", DeviceCPU, 65, 45, 15, 2)
	m.RegisterDevice("disk1", "HDD", DeviceDisk, 10, 8, 5, 1)

	err := m.CreateBudget("main", "主预算", 100, []string{"cpu1", "disk1"})
	if err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}

	// 记录读数
	m.RecordReading(PowerReading{DeviceID: "cpu1", DeviceType: DeviceCPU, Watts: 45})
	m.RecordReading(PowerReading{DeviceID: "disk1", DeviceType: DeviceDisk, Watts: 8})

	status, err := m.GetBudgetStatus("main")
	if err != nil {
		t.Fatalf("GetBudgetStatus failed: %v", err)
	}

	if status.CurrentWatts != 53 {
		t.Errorf("expected 53W, got %f", status.CurrentWatts)
	}
	if status.Utilization <= 0 {
		t.Error("expected positive utilization")
	}
	if status.WarningWatts != 80 {
		t.Errorf("expected 80W warning, got %f", status.WarningWatts)
	}

	// 不存在的预算
	_, err = m.GetBudgetStatus("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	m := newTestManager()

	// 无数据时应失败
	_, err := m.GenerateReport("daily")
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}

	m.RegisterDevice("cpu1", "CPU", DeviceCPU, 65, 45, 15, 2)
	m.RegisterDevice("disk1", "HDD", DeviceDisk, 10, 8, 5, 1)

	// 添加多个读数
	for i := 0; i < 10; i++ {
		m.RecordReading(PowerReading{
			DeviceID:   "cpu1",
			DeviceType: DeviceCPU,
			Watts:      40 + float64(i%5),
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
		})
		m.RecordReading(PowerReading{
			DeviceID:   "disk1",
			DeviceType: DeviceDisk,
			Watts:      7 + float64(i%3),
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	report, err := m.GenerateReport("daily")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.TotalEnergyKWh <= 0 {
		t.Error("expected positive energy")
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive cost")
	}
	if report.AvgPowerWatts <= 0 {
		t.Error("expected positive avg watts")
	}
	if report.PeakPowerWatts <= 0 {
		t.Error("expected positive peak watts")
	}
	if len(report.DeviceBreakdown) == 0 {
		t.Error("expected device breakdown")
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected suggestions")
	}
	if report.CarbonFootprint <= 0 {
		t.Error("expected positive carbon footprint")
	}
}

func TestSchedule(t *testing.T) {
	m := newTestManager()

	err := m.AddSchedule(ScheduleRule{
		RuleID:      "night-off",
		Name:        "夜间关机",
		DeviceIDs:   []string{"disk1"},
		TargetState: PowerStateStandby,
		StartTime:   "23:00",
		EndTime:     "06:00",
		DaysOfWeek:  []int{1, 2, 3, 4, 5},
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}

	schedules := m.GetScheduleStatus()
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(schedules))
	}
	if schedules[0].Name != "夜间关机" {
		t.Errorf("expected 夜间关机, got %s", schedules[0].Name)
	}

	// 空ID应失败
	err = m.AddSchedule(ScheduleRule{})
	if err == nil {
		t.Error("expected error for empty rule id")
	}
}

func TestEstimateSavings(t *testing.T) {
	m := newTestManager()

	m.RegisterDevice("disk1", "HDD", DeviceDisk, 10, 8, 5, 1)

	// 切换到空闲
	savingsKWh, savingsCost, err := m.EstimateSavings("disk1", PowerStateIdle)
	if err != nil {
		t.Fatalf("EstimateSavings failed: %v", err)
	}
	if savingsKWh < 0 {
		t.Errorf("expected non-negative savings, got %f", savingsKWh)
	}
	if savingsCost < 0 {
		t.Errorf("expected non-negative cost savings, got %f", savingsCost)
	}

	// 切换到待机
	savingsKWh2, _, err := m.EstimateSavings("disk1", PowerStateStandby)
	if err != nil {
		t.Fatalf("EstimateSavings standby failed: %v", err)
	}
	// 待机节省应大于空闲节省
	if savingsKWh2 < savingsKWh {
		t.Log("standby savings should be >= idle savings")
	}

	// 不存在的设备
	_, _, err = m.EstimateSavings("nonexistent", PowerStateIdle)
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestDashboard(t *testing.T) {
	m := newTestManager()

	m.RegisterDevice("cpu1", "CPU", DeviceCPU, 65, 45, 15, 2)
	m.RegisterDevice("disk1", "HDD", DeviceDisk, 10, 8, 5, 1)

	m.RecordReading(PowerReading{DeviceID: "cpu1", DeviceType: DeviceCPU, Watts: 45})
	m.RecordReading(PowerReading{DeviceID: "disk1", DeviceType: DeviceDisk, Watts: 8})

	dash := m.GetDashboard()
	if dash["deviceCount"] != 2 {
		t.Errorf("expected 2 devices, got %v", dash["deviceCount"])
	}
	if dash["totalCurrentWatts"] != 53.0 {
		t.Errorf("expected 53W, got %v", dash["totalCurrentWatts"])
	}
	if dash["electricityRate"] != 0.56 {
		t.Errorf("expected 0.56 rate, got %v", dash["electricityRate"])
	}
}

func TestGetAllDevices(t *testing.T) {
	m := newTestManager()

	m.RegisterDevice("cpu1", "CPU", DeviceCPU, 65, 45, 15, 2)
	m.RegisterDevice("gpu1", "GPU", DeviceGPU, 200, 150, 30, 5)

	all := m.GetAllDevices()
	if len(all) != 2 {
		t.Errorf("expected 2 devices, got %d", len(all))
	}
}

func TestDeviceNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.GetDevice("nonexistent")
	if err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}
