package container

import (
	"testing"
	"time"
)

func TestVolume_Fields(t *testing.T) {
	now := time.Now()
	volume := &Volume{
		Name:       "my-volume",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/my-volume/_data",
		Created:    now,
		Size:       1024 * 1024 * 100, // 100 MB
		SizeHuman:  "100MB",
		Labels:     map[string]string{"backup": "true"},
		Scope:      "local",
		Options:    map[string]string{"type": "none"},
		Containers: []string{"container1", "container2"},
	}

	if volume.Name != "my-volume" {
		t.Errorf("Name = %s, want my-volume", volume.Name)
	}

	if volume.Driver != "local" {
		t.Errorf("Driver = %s, want local", volume.Driver)
	}

	if volume.Size != 1024*1024*100 {
		t.Errorf("Size = %d, want %d", volume.Size, 1024*1024*100)
	}

	if len(volume.Containers) != 2 {
		t.Errorf("Containers count = %d, want 2", len(volume.Containers))
	}
}

func TestVolumeConfig_Fields(t *testing.T) {
	config := &VolumeConfig{
		Name:     "data-volume",
		Driver:   "local",
		Labels:   map[string]string{"app": "myapp"},
		Options:  map[string]string{"device": "/dev/sda1"},
		HostPath: "/mnt/data",
	}

	if config.Name != "data-volume" {
		t.Errorf("Name = %s, want data-volume", config.Name)
	}

	if config.Driver != "local" {
		t.Errorf("Driver = %s, want local", config.Driver)
	}

	if config.HostPath != "/mnt/data" {
		t.Errorf("HostPath = %s, want /mnt/data", config.HostPath)
	}
}

func TestVolumeBackup_Fields(t *testing.T) {
	now := time.Now()
	backup := &VolumeBackup{
		Name:       "my-volume_20240101_120000",
		VolumeName: "my-volume",
		BackupPath: "/backups/my-volume_20240101_120000.tar.gz",
		Size:       50 * 1024 * 1024, // 50 MB
		SizeHuman:  "50MB",
		Created:    now,
		Checksum:   "abc123def456",
		Compressed: true,
	}

	if backup.Name != "my-volume_20240101_120000" {
		t.Errorf("Name = %s, want my-volume_20240101_120000", backup.Name)
	}

	if !backup.Compressed {
		t.Error("Compressed should be true")
	}

	if backup.Checksum != "abc123def456" {
		t.Errorf("Checksum = %s, want abc123def456", backup.Checksum)
	}
}

func TestVolumeConfig_DefaultDriver(t *testing.T) {
	config := &VolumeConfig{
		Name: "test",
	}

	// Driver should be empty if not set
	if config.Driver != "" {
		t.Error("Driver should default to empty string")
	}
}

func TestVolumeBackup_Compressed(t *testing.T) {
	tests := []struct {
		name       string
		compressed bool
	}{
		{"backup.tar.gz", true},
		{"backup.tar", false},
	}

	for _, tt := range tests {
		backup := &VolumeBackup{
			Name:       tt.name,
			Compressed: tt.compressed,
		}

		if backup.Compressed != tt.compressed {
			t.Errorf("Compressed = %v, want %v", backup.Compressed, tt.compressed)
		}
	}
}

func TestVolume_Options(t *testing.T) {
	volume := &Volume{
		Name:    "nfs-volume",
		Driver:  "local",
		Options: map[string]string{"type": "nfs", "o": "addr=192.168.1.1"},
	}

	if volume.Options["type"] != "nfs" {
		t.Errorf("Options[type] = %s, want nfs", volume.Options["type"])
	}

	if volume.Options["o"] != "addr=192.168.1.1" {
		t.Errorf("Options[o] = %s, want addr=192.168.1.1", volume.Options["o"])
	}
}

func TestVolume_Labels(t *testing.T) {
	volume := &Volume{
		Name: "labeled-volume",
		Labels: map[string]string{
			"com.docker.compose.project": "myproject",
			"com.docker.compose.service": "db",
		},
	}

	if volume.Labels["com.docker.compose.project"] != "myproject" {
		t.Error("Project label not set correctly")
	}

	if volume.Labels["com.docker.compose.service"] != "db" {
		t.Error("Service label not set correctly")
	}
}

func TestVolumeConfig_Labels(t *testing.T) {
	config := &VolumeConfig{
		Name: "test",
		Labels: map[string]string{
			"backup": "daily",
			"env":    "production",
		},
	}

	if len(config.Labels) != 2 {
		t.Errorf("Labels count = %d, want 2", len(config.Labels))
	}

	if config.Labels["backup"] != "daily" {
		t.Error("Backup label not set correctly")
	}
}

func TestVolume_Scope(t *testing.T) {
	volume := &Volume{
		Name:  "local-volume",
		Scope: "local",
	}

	if volume.Scope != "local" {
		t.Errorf("Scope = %s, want local", volume.Scope)
	}
}

func TestVolume_Containers(t *testing.T) {
	volume := &Volume{
		Name:       "shared-volume",
		Containers: []string{"web", "db", "cache"},
	}

	if len(volume.Containers) != 3 {
		t.Errorf("Containers count = %d, want 3", len(volume.Containers))
	}

	// Check if specific containers are in the list
	found := false
	for _, c := range volume.Containers {
		if c == "web" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Container 'web' not found in containers list")
	}
}

func TestVolumeBackup_SizeHuman(t *testing.T) {
	backup := &VolumeBackup{
		Size:      1024 * 1024 * 256, // 256 MB
		SizeHuman: "256MB",
	}

	if backup.SizeHuman != "256MB" {
		t.Errorf("SizeHuman = %s, want 256MB", backup.SizeHuman)
	}
}

func TestVolumeConfig_HostPath(t *testing.T) {
	config := &VolumeConfig{
		Name:     "bind-mount",
		Driver:   "local",
		HostPath: "/home/user/data",
	}

	if config.HostPath != "/home/user/data" {
		t.Errorf("HostPath = %s, want /home/user/data", config.HostPath)
	}
}

func TestVolume_EmptyContainers(t *testing.T) {
	volume := &Volume{
		Name:       "unused-volume",
		Containers: []string{},
	}

	if len(volume.Containers) != 0 {
		t.Errorf("Empty containers slice should have length 0, got %d", len(volume.Containers))
	}
}

func TestVolumeBackup_Timestamp(t *testing.T) {
	now := time.Now()
	backup := &VolumeBackup{
		Name:    "test",
		Created: now,
	}

	if !backup.Created.Equal(now) {
		t.Errorf("Created = %v, want %v", backup.Created, now)
	}
}
