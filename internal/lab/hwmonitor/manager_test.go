package hwmonitor

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if m.maxHistory != 360 {
		t.Errorf("expected maxHistory 360, got %d", m.maxHistory)
	}
}

func TestGetReportBeforeCollect(t *testing.T) {
	m := NewManager()

	_, err := m.GetReport()
	if err == nil {
		t.Error("expected error before collect")
	}
}

func TestGetCPU(t *testing.T) {
	m := NewManager()

	_, err := m.GetCPU()
	if err == nil {
		t.Error("expected error before collect")
	}

	// 启动并等待采集
	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	cpu, err := m.GetCPU()
	if err != nil {
		t.Fatalf("get cpu failed: %v", err)
	}
	if cpu.Cores == 0 {
		t.Error("expected non-zero cores")
	}
	if cpu.Temp == 0 {
		t.Error("expected non-zero temp")
	}
}

func TestGetMemory(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	mem, err := m.GetMemory()
	if err != nil {
		t.Fatalf("get memory failed: %v", err)
	}
	if mem.Total == 0 {
		t.Error("expected non-zero total")
	}
	if mem.Used == 0 {
		t.Error("expected non-zero used")
	}
	if mem.Available == 0 {
		t.Error("expected non-zero available")
	}
}

func TestGetDiskTemps(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	disks, err := m.GetDiskTemps()
	if err != nil {
		t.Fatalf("get disk temps failed: %v", err)
	}
	if len(disks) == 0 {
		t.Error("expected at least 1 disk")
	}
	for _, d := range disks {
		if d.Device == "" {
			t.Error("expected non-empty device")
		}
		if d.Temp == 0 {
			t.Error("expected non-zero temp")
		}
	}
}

func TestGetNetIO(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	net, err := m.GetNetIO()
	if err != nil {
		t.Fatalf("get net IO failed: %v", err)
	}
	if len(net) == 0 {
		t.Error("expected at least 1 interface")
	}
}

func TestGetVoltages(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	voltages, err := m.GetVoltages()
	if err != nil {
		t.Fatalf("get voltages failed: %v", err)
	}
	if len(voltages) == 0 {
		t.Error("expected at least 1 voltage")
	}
}

func TestSetThreshold(t *testing.T) {
	m := NewManager()

	err := m.SetThreshold("cpu_temp", 90.0)
	if err != nil {
		t.Fatalf("set threshold failed: %v", err)
	}

	// 无效指标
	err = m.SetThreshold("invalid", 50.0)
	if err == nil {
		t.Error("expected error for invalid metric")
	}

	// 无效值
	err = m.SetThreshold("cpu_temp", -1.0)
	if err == nil {
		t.Error("expected error for negative value")
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(200 * time.Millisecond)

	report, err := m.GetReport()
	if err != nil {
		t.Fatalf("get report failed: %v", err)
	}
	if report.CPU == nil {
		t.Error("expected non-nil CPU")
	}
	if report.Memory == nil {
		t.Error("expected non-nil Memory")
	}
	if report.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestGetHistory(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	defer m.Stop()
	time.Sleep(500 * time.Millisecond) // 等几轮采集

	history := m.GetHistory(1 * time.Hour)
	if len(history) == 0 {
		t.Error("expected history records")
	}

	// 查看短时间范围应该有记录
	history = m.GetHistory(1 * time.Second)
	if len(history) == 0 {
		t.Error("expected recent history records")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager()

	// 启动
	m.Start(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	report, err := m.GetReport()
	if err != nil {
		t.Fatalf("get report failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected report")
	}

	// 停止
	m.Stop()

	// 重复停止不应 panic
	m.Stop()
}

func TestGetReportAfterStop(t *testing.T) {
	m := NewManager()

	m.Start(100 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	m.Stop()

	// 停止后仍可获取数据
	report, err := m.GetReport()
	if err != nil {
		t.Fatalf("get report after stop failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected report")
	}
}
