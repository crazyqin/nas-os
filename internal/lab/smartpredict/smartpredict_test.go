package smartpredict

import (
	"testing"
	"time"
)

func TestHealthScoreCalculation(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name     string
		health   *DiskHealth
		minScore float64
		maxScore float64
	}{
		{
			name: "健康磁盘",
			health: &DiskHealth{
				Device:       "/dev/sda",
				Temperature:  35,
				PowerOnHours: 1000,
				Attributes:   []SMARTAttribute{},
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "高温磁盘",
			health: &DiskHealth{
				Device:       "/dev/sdb",
				Temperature:  60,
				PowerOnHours: 1000,
				Attributes:   []SMARTAttribute{},
			},
			minScore: 70,
			maxScore: 90,
		},
		{
			name: "有坏道磁盘",
			health: &DiskHealth{
				Device:       "/dev/sdc",
				Temperature:  35,
				PowerOnHours: 1000,
				Attributes: []SMARTAttribute{
					{ID: 5, Name: "Reallocated_Sector_Ct", RawValue: 10},
				},
			},
			minScore: 50,
			maxScore: 80,
		},
		{
			name: "严重损坏磁盘",
			health: &DiskHealth{
				Device:       "/dev/sdd",
				Temperature:  70,
				PowerOnHours: 60000,
				Attributes: []SMARTAttribute{
					{ID: 5, Name: "Reallocated_Sector_Ct", RawValue: 50, Failed: true},
				},
			},
			minScore: 0,
			maxScore: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr.UpdateDisk(tt.health)
			disk, _ := mgr.GetDisk(tt.health.Device)
			if disk.HealthScore < tt.minScore || disk.HealthScore > tt.maxScore {
				t.Errorf("health score = %.1f, want between %.1f and %.1f",
					disk.HealthScore, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestFailProbCalculation(t *testing.T) {
	mgr := NewManager(nil)

	health := &DiskHealth{
		Device:      "/dev/sda",
		Temperature: 40,
		Attributes: []SMARTAttribute{
			{ID: 5, RawValue: 20},
		},
	}
	mgr.UpdateDisk(health)
	disk, _ := mgr.GetDisk("/dev/sda")

	if disk.FailProb <= 0 {
		t.Error("fail probability should be > 0 for disk with reallocated sectors")
	}
	if disk.FailProb > 1 {
		t.Error("fail probability should be <= 1")
	}
}

func TestPredictionRiskLevels(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name      string
		health    *DiskHealth
		wantLevel string
	}{
		{
			name: "低风险",
			health: &DiskHealth{
				Device:       "/dev/sda",
				Temperature:  35,
				PowerOnHours: 5000,
			},
			wantLevel: "low",
		},
		{
			name: "高风险-高温+坏道",
			health: &DiskHealth{
				Device:       "/dev/sdb",
				Temperature:  68,
				PowerOnHours: 40000,
				TBW:          900,
				TBWLimited:   1000,
				Attributes: []SMARTAttribute{
					{ID: 5, RawValue: 30},
					{ID: 197, RawValue: 10},
				},
			},
			wantLevel: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr.UpdateDisk(tt.health)
			disk, _ := mgr.GetDisk(tt.health.Device)
			if disk.Prediction == nil {
				t.Fatal("prediction should not be nil")
			}
			if disk.Prediction.RiskLevel != tt.wantLevel {
				t.Errorf("risk level = %s, want %s", disk.Prediction.RiskLevel, tt.wantLevel)
			}
		})
	}
}

func TestAlertCallback(t *testing.T) {
	mgr := NewManager(nil)

	alerted := false
	mgr.SetAlertFunc(func(device string, pred *Prediction) {
		alerted = true
		if pred.RiskLevel != "critical" {
			t.Errorf("expected critical alert, got %s", pred.RiskLevel)
		}
	})

	health := &DiskHealth{
		Device:       "/dev/sda",
		Temperature:  70,
		PowerOnHours: 60000,
		TBW:          950,
		TBWLimited:   1000,
		Attributes: []SMARTAttribute{
			{ID: 5, RawValue: 100, Failed: true},
		},
	}
	mgr.UpdateDisk(health)

	if !alerted {
		t.Error("expected alert callback to be triggered")
	}
}

func TestGetAllDisks(t *testing.T) {
	mgr := NewManager(nil)

	for i := 0; i < 3; i++ {
		mgr.UpdateDisk(&DiskHealth{
			Device:      "/dev/sd" + string(rune('a'+i)),
			Temperature: 35,
		})
	}

	disks := mgr.GetAllDisks()
	if len(disks) != 3 {
		t.Errorf("expected 3 disks, got %d", len(disks))
	}
}

func TestRiskSummary(t *testing.T) {
	mgr := NewManager(nil)

	// 添加一个健康磁盘
	mgr.UpdateDisk(&DiskHealth{
		Device:       "/dev/sda",
		Temperature:  35,
		PowerOnHours: 1000,
	})

	summary := mgr.GetRiskSummary()
	if summary["low"] != 1 {
		t.Errorf("expected 1 low risk disk, got %d", summary["low"])
	}
}

func TestEstimateRemainingDays(t *testing.T) {
	mgr := NewManager(nil)

	health := &DiskHealth{
		Device:       "/dev/sda",
		Temperature:  35,
		PowerOnHours: 25000,
	}
	mgr.UpdateDisk(health)
	disk, _ := mgr.GetDisk("/dev/sda")

	if disk.Prediction.RemainingLifeDays <= 0 {
		t.Error("remaining life days should be positive")
	}
}

func TestMonitorLifecycle(t *testing.T) {
	mgr := NewManager(&PredictConfig{
		CheckInterval: 100 * time.Millisecond,
	})

	mgr.Start()
	time.Sleep(50 * time.Millisecond)
	mgr.Stop()
	// 如果没有 panic，测试通过
}
