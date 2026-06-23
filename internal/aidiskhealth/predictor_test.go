package aidiskhealth

import (
	"testing"
)

func TestRegisterDisk(t *testing.T) {
	predictor := NewAIHealthPredictor()

	disk := &DiskInfo{
		Device:  "/dev/sda",
		Model:   "WD Red 4TB",
		Type:    DiskTypeHDD,
		Capacity: 4000000000000,
		Temperature: 45,
		PowerOnHours: 10000,
	}

	predictor.RegisterDisk(disk)

	score, err := predictor.GetHealthScore("/dev/sda")
	if err != nil {
		t.Fatalf("获取健康评分失败: %v", err)
	}

	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("评分应在 0-100 之间, got %.1f", score.Overall)
	}
}

func TestHealthScoreWithSMART(t *testing.T) {
	predictor := NewAIHealthPredictor()

	disk := &DiskInfo{
		Device:      "/dev/sdb",
		Model:       "Seagate 8TB",
		Type:        DiskTypeHDD,
		Temperature: 42,
		SMART: []SMARTAttribute{
			{ID: 5, Name: "Reallocated_Sector_Ct", RawValue: 10},
		},
	}

	predictor.RegisterDisk(disk)

	score, err := predictor.GetHealthScore("/dev/sdb")
	if err != nil {
		t.Fatalf("获取健康评分失败: %v", err)
	}

	// 有重分配扇区，评分应该降低
	if score.Overall >= 100 {
		t.Error("有坏扇区时评分不应为满分")
	}
}

func TestGetHealthReport(t *testing.T) {
	predictor := NewAIHealthPredictor()

	disk := &DiskInfo{
		Device:      "/dev/sdc",
		Model:       "Samsung 970 EVO",
		Type:        DiskTypeNVMe,
		Temperature: 38,
		PowerOnHours: 5000,
	}

	predictor.RegisterDisk(disk)

	report, err := predictor.GetHealthReport("/dev/sdc")
	if err != nil {
		t.Fatalf("获取健康报告失败: %v", err)
	}

	if report.DiskID != "/dev/sdc" {
		t.Errorf("磁盘 ID 应为 /dev/sdc, got %s", report.DiskID)
	}

	if report.Score.Prediction == nil {
		t.Error("预测信息不应为空")
	}
}

func TestGetAllDisksHealth(t *testing.T) {
	predictor := NewAIHealthPredictor()

	predictor.RegisterDisk(&DiskInfo{Device: "/dev/sda", Temperature: 40})
	predictor.RegisterDisk(&DiskInfo{Device: "/dev/sdb", Temperature: 55})

	all := predictor.GetAllDisksHealth()
	if len(all) != 2 {
		t.Errorf("应有 2 个磁盘, got %d", len(all))
	}
}
