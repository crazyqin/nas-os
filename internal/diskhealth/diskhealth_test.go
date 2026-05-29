package diskhealth

import (
	"context"
	"testing"
	"time"
)

func TestNewDiskHealthPredict(t *testing.T) {
	dhp := NewDiskHealthPredict(0)
	if dhp == nil {
		t.Fatal("expected non-nil DiskHealthPredict")
	}
	if dhp.checkInterval != 1*time.Hour {
		t.Errorf("expected default interval 1h, got %v", dhp.checkInterval)
	}
}

func TestAddDisk(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	info := DiskInfo{
		Device: "/dev/sda",
		Model:  "Test Disk",
		Size:   1024 * 1024 * 1024 * 1024, // 1TB
	}

	dhp.AddDisk(info)

	disks := dhp.ListDisks()
	if len(disks) != 1 {
		t.Errorf("expected 1 disk, got %d", len(disks))
	}
}

func TestGetDiskHealth(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.AddDisk(DiskInfo{
		Device:      "/dev/sda",
		HealthScore: 85.5,
	})

	disk, err := dhp.GetDiskHealth("/dev/sda")
	if err != nil {
		t.Fatal(err)
	}
	if disk.HealthScore != 85.5 {
		t.Errorf("expected health score 85.5, got %.1f", disk.HealthScore)
	}
}

func TestGetDiskHealthNotFound(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	_, err := dhp.GetDiskHealth("/dev/sda")
	if err == nil {
		t.Error("expected error for non-existent disk")
	}
}

func TestRemoveDisk(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.AddDisk(DiskInfo{Device: "/dev/sda"})
	dhp.RemoveDisk("/dev/sda")

	disks := dhp.ListDisks()
	if len(disks) != 0 {
		t.Errorf("expected 0 disks, got %d", len(disks))
	}
}

func TestPredictFailure(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.AddDisk(DiskInfo{
		Device:      "/dev/sda",
		HealthScore: 40.0,
	})

	failure, err := dhp.PredictFailure("/dev/sda")
	if err != nil {
		t.Fatal(err)
	}
	if failure == nil {
		t.Error("expected failure prediction for low health score")
	}
}

func TestPredictFailureHealthy(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.AddDisk(DiskInfo{
		Device:      "/dev/sda",
		HealthScore: 90.0,
	})

	failure, err := dhp.PredictFailure("/dev/sda")
	if err != nil {
		t.Fatal(err)
	}
	if failure != nil {
		t.Error("expected no failure prediction for healthy disk")
	}
}

func TestGetAlerts(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.alerts = []*HealthAlert{
		{Disk: "/dev/sda", Level: "warning", Message: "test"},
		{Disk: "/dev/sdb", Level: "critical", Message: "test", Acked: true},
	}

	all := dhp.GetAlerts(false)
	if len(all) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(all))
	}

	unacked := dhp.GetAlerts(true)
	if len(unacked) != 1 {
		t.Errorf("expected 1 unacked alert, got %d", len(unacked))
	}
}

func TestAckAlert(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.alerts = []*HealthAlert{
		{Disk: "/dev/sda", Level: "warning", Message: "test"},
	}

	err := dhp.AckAlert(0)
	if err != nil {
		t.Fatal(err)
	}
	if !dhp.alerts[0].Acked {
		t.Error("expected alert to be acknowledged")
	}
}

func TestStartStop(t *testing.T) {
	dhp := NewDiskHealthPredict(0)
	ctx := context.Background()

	err := dhp.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = dhp.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}

	dhp.Stop()
}

func TestGetStats(t *testing.T) {
	dhp := NewDiskHealthPredict(0)

	dhp.AddDisk(DiskInfo{Device: "/dev/sda", HealthScore: 90})
	dhp.AddDisk(DiskInfo{Device: "/dev/sdb", HealthScore: 50})
	dhp.AddDisk(DiskInfo{Device: "/dev/sdc", HealthScore: 20})

	stats := dhp.GetStats()
	if stats["total_disks"] != 3 {
		t.Errorf("expected 3 disks, got %v", stats["total_disks"])
	}
}
