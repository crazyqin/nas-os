package tcodashboard

import (
	"fmt"
	"testing"
)

func TestCompute_BasicTCO(t *testing.T) {
	breakdowns := Compute(Signal{
		Hardware: HardwareProfile{
			DriveBays:     4,
			DriveCount:    4,
			DriveCostEach: 200,
			NVMeCount:     2,
			NVMeCostEach:  150,
			SystemCost:    1000,
			WattageIdle:   30,
			WattageLoad:   60,
			ElectricityRate: 0.12,
			UptimeHoursPct: 100,
		},
		Cloud: CloudProfile{
			CloudBackupGB:   500,
			CloudBackupPerGB: 0.01,
			CloudSyncGB:     200,
			CloudSyncPerGB:   0.02,
		},
		SoftwareLicense: 0,
		MaintenancePct:  10,
		LaborHoursWeek:  2,
		LaborRateHourly: 50,
		YearsProjection: 5,
	})
	if len(breakdowns) != 5 {
		t.Fatalf("expected 5 yearly breakdowns, got %d", len(breakdowns))
	}
	if breakdowns[0].Hardware != 1000 {
		t.Errorf("expected year 1 hardware = 1000, got %.0f", breakdowns[0].Hardware)
	}
	if breakdowns[0].Storage != 1100 {
		t.Errorf("expected year 1 storage = 1100 (4*200 + 2*150), got %.0f", breakdowns[0].Storage)
	}
	if breakdowns[4].Total <= 0 {
		t.Error("year 5 total should be positive")
	}
	if breakdowns[4].CumulativeTotal <= breakdowns[0].Total {
		t.Error("cumulative should increase over years")
	}
}

func TestAnalyze_PowerHigh(t *testing.T) {
	recs := Analyze(Signal{
		Hardware: HardwareProfile{
			DriveCount:    16,
			DriveCostEach: 100,
			NVMeCount:     1,
			NVMeCostEach:  100,
			SystemCost:    300,
			WattageIdle:   300,
			WattageLoad:   600,
			ElectricityRate: 0.50,
			UptimeHoursPct: 100,
		},
		Cloud: CloudProfile{},
		MaintenancePct: 10,
		LaborHoursWeek: 1,
		LaborRateHourly: 50,
		YearsProjection: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tco-reduce-power" {
			found = true
		}
	}
	if !found {
		t.Error("expected tco-reduce-power recommendation for high power cost")
	}
}

func TestAnalyze_CloudOptimization(t *testing.T) {
	recs := Analyze(Signal{
		Hardware: HardwareProfile{
			SystemCost: 1000,
			WattageIdle: 30,
			WattageLoad: 60,
			ElectricityRate: 0.12,
		},
		Cloud: CloudProfile{
			CloudBackupGB:    5000,
			CloudBackupPerGB:  0.05,
			CloudSyncGB:       2000,
			CloudSyncPerGB:     0.04,
		},
		MaintenancePct: 10,
		LaborHoursWeek: 1,
		LaborRateHourly: 50,
		YearsProjection: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tco-cloud-optimization" {
			found = true
		}
	}
	if !found {
		t.Error("expected tco-cloud-optimization recommendation")
	}
}

func TestAnalyze_VsSynology(t *testing.T) {
	recs := Analyze(Signal{
		Hardware: HardwareProfile{
			SystemCost: 500,
			WattageIdle: 20,
			WattageLoad: 40,
			ElectricityRate: 0.12,
			UptimeHoursPct: 100,
		},
		MaintenancePct: 10,
		LaborHoursWeek: 1,
		LaborRateHourly: 50,
		YearsProjection: 5,
		CompareSynology: 100000,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tco-vs-synology" {
			found = true
		}
	}
	if !found {
		t.Error("expected tco-vs-synology recommendation")
	}
}

func TestAnalyze_NVMeOverspend(t *testing.T) {
	recs := Analyze(Signal{
		Hardware: HardwareProfile{
			SystemCost: 500,
			NVMeCount: 6,
			NVMeCostEach: 400,
			WattageIdle: 30,
			WattageLoad: 60,
			ElectricityRate: 0.12,
		},
		MaintenancePct: 10,
		LaborHoursWeek: 1,
		LaborRateHourly: 50,
		YearsProjection: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tco-nvme-overspend" {
			found = true
		}
	}
	if !found {
		t.Error(fmt.Errorf("expected tco-nvme-overspend recommendation"))
	}
}

func TestAnalyze_LowMaintenance(t *testing.T) {
	recs := Analyze(Signal{
		Hardware: HardwareProfile{
			SystemCost: 1000,
			WattageIdle: 30,
			WattageLoad: 60,
			ElectricityRate: 0.12,
		},
		MaintenancePct: 3,
		LaborHoursWeek: 1,
		LaborRateHourly: 50,
		YearsProjection: 5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "tco-maint-budget" {
			found = true
		}
	}
	if !found {
		t.Error("expected tco-maint-budget recommendation")
	}
}