package storagesetupwizard

import (
	"testing"
)

func TestCreateSession(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Model: "WD10EZEX", Size: 1000000000, Type: "hdd", Health: "healthy"},
		{ID: "disk2", Device: "/dev/sdb", Model: "WD10EZEX", Size: 1000000000, Type: "hdd", Health: "healthy"},
	}

	session, err := manager.CreateSession(disks)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.CurrentStep != StepDiskSelection {
		t.Errorf("expected step %s, got %s", StepDiskSelection, session.CurrentStep)
	}
	if session.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", session.Status)
	}
	if len(session.Disks) != 2 {
		t.Errorf("expected 2 disks, got %d", len(session.Disks))
	}
}

func TestCreateSessionNoDisks(t *testing.T) {
	manager := NewManager()

	_, err := manager.CreateSession([]DiskInfo{})
	if err == nil {
		t.Error("expected error for empty disks")
	}
}

func TestGetSession(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	got, err := manager.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("expected session ID %s, got %s", session.ID, got.ID)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestUpdateStep(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	err := manager.UpdateStep(session.ID, StepRAIDConfig)
	if err != nil {
		t.Fatalf("UpdateStep failed: %v", err)
	}

	got, _ := manager.GetSession(session.ID)
	if got.CurrentStep != StepRAIDConfig {
		t.Errorf("expected step %s, got %s", StepRAIDConfig, got.CurrentStep)
	}
}

func TestSetPoolConfig(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
		{ID: "disk2", Device: "/dev/sdb", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	config := PoolConfig{
		Name: "test-pool",
		RAID: RAIDConfig{
			Type:  RAID1,
			Disks: []string{"disk1", "disk2"},
		},
		Compression: "lz4",
	}

	err := manager.SetPoolConfig(session.ID, config)
	if err != nil {
		t.Fatalf("SetPoolConfig failed: %v", err)
	}

	got, _ := manager.GetSession(session.ID)
	if got.Pool.Name != "test-pool" {
		t.Errorf("expected pool name test-pool, got %s", got.Pool.Name)
	}
	if got.CurrentStep != StepPoolCreation {
		t.Errorf("expected step %s, got %s", StepPoolCreation, got.CurrentStep)
	}
}

func TestSetPoolConfigInvalidRAID(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	config := PoolConfig{
		Name: "test-pool",
		RAID: RAIDConfig{
			Type:  RAID5,
			Disks: []string{"disk1"},
		},
	}

	err := manager.SetPoolConfig(session.ID, config)
	if err == nil {
		t.Error("expected error for RAID5 with 1 disk")
	}
}

func TestValidateRAIDConfig(t *testing.T) {
	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
		{ID: "disk2", Device: "/dev/sdb", Size: 1000000000},
		{ID: "disk3", Device: "/dev/sdc", Size: 1000000000},
	}

	tests := []struct {
		name    string
		config  RAIDConfig
		wantErr bool
	}{
		{
			name: "valid RAID1",
			config: RAIDConfig{
				Type:  RAID1,
				Disks: []string{"disk1", "disk2"},
			},
			wantErr: false,
		},
		{
			name: "RAID1 with 1 disk",
			config: RAIDConfig{
				Type:  RAID1,
				Disks: []string{"disk1"},
			},
			wantErr: true,
		},
		{
			name: "valid RAID5",
			config: RAIDConfig{
				Type:  RAID5,
				Disks: []string{"disk1", "disk2", "disk3"},
			},
			wantErr: false,
		},
		{
			name: "RAID5 with 2 disks",
			config: RAIDConfig{
				Type:  RAID5,
				Disks: []string{"disk1", "disk2"},
			},
			wantErr: true,
		},
		{
			name: "nonexistent disk",
			config: RAIDConfig{
				Type:  RAID1,
				Disks: []string{"disk1", "nonexistent"},
			},
			wantErr: true,
		},
		{
			name: "empty disks",
			config: RAIDConfig{
				Type:  RAID1,
				Disks: []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRAIDConfig(tt.config, disks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRAIDConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecommendRAID(t *testing.T) {
	tests := []struct {
		name      string
		diskCount int
		priority  string
		wantType  RAIDType
	}{
		{
			name:      "1 disk",
			diskCount: 1,
			priority:  "balanced",
			wantType:  RAIDBasic,
		},
		{
			name:      "2 disks balanced",
			diskCount: 2,
			priority:  "balanced",
			wantType:  RAID1,
		},
		{
			name:      "2 disks performance",
			diskCount: 2,
			priority:  "performance",
			wantType:  RAID0,
		},
		{
			name:      "4 disks balanced",
			diskCount: 4,
			priority:  "balanced",
			wantType:  RAID5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := RecommendRAID(tt.diskCount, tt.priority)
			if len(recs) == 0 {
				t.Fatal("expected recommendations")
			}

			found := false
			for _, r := range recs {
				if r.Type == tt.wantType && r.Recommended {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected recommended RAID type %s", tt.wantType)
			}
		})
	}
}

func TestEstimateCapacity(t *testing.T) {
	tests := []struct {
		name      string
		diskCount int
		diskSize  int64
		raidType  RAIDType
		wantUsable int64
	}{
		{
			name:      "RAID1 2 disks",
			diskCount: 2,
			diskSize:  1000000000,
			raidType:  RAID1,
			wantUsable: 1000000000,
		},
		{
			name:      "RAID5 3 disks",
			diskCount: 3,
			diskSize:  1000000000,
			raidType:  RAID5,
			wantUsable: 2000000000,
		},
		{
			name:      "RAID0 2 disks",
			diskCount: 2,
			diskSize:  1000000000,
			raidType:  RAID0,
			wantUsable: 2000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateCapacity(tt.diskCount, tt.diskSize, tt.raidType)
			if result.Usable != tt.wantUsable {
				t.Errorf("expected usable %d, got %d", tt.wantUsable, result.Usable)
			}
		})
	}
}

func TestCompleteSession(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	err := manager.CompleteSession(session.ID)
	if err != nil {
		t.Fatalf("CompleteSession failed: %v", err)
	}

	got, _ := manager.GetSession(session.ID)
	if got.Status != "completed" {
		t.Errorf("expected status completed, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestListSessions(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	manager.CreateSession(disks)
	manager.CreateSession(disks)

	sessions := manager.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestDeleteSession(t *testing.T) {
	manager := NewManager()

	disks := []DiskInfo{
		{ID: "disk1", Device: "/dev/sda", Size: 1000000000},
	}

	session, _ := manager.CreateSession(disks)

	err := manager.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, err = manager.GetSession(session.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}
