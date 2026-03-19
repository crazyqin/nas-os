package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSizeNasctl(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		assert.Equal(t, tt.expected, result, "formatSize(%d)", tt.bytes)
	}
}

func TestStatusBool(t *testing.T) {
	assert.Equal(t, "✓ 运行中", statusBool(true))
	assert.Equal(t, "✗ 未运行", statusBool(false))
}

func TestVolumeStruct(t *testing.T) {
	volume := Volume{
		Name:        "test-volume",
		UUID:        "12345678-1234-1234-1234-123456789012",
		Devices:     []string{"/dev/sda1", "/dev/sda2"},
		Size:        1073741824,
		Used:        536870912,
		Free:        536870912,
		DataProfile: "single",
		MetaProfile: "single",
		MountPoint:  "/mnt/test",
		Subvolumes:  []SubVolume{},
		Status:      VolumeStatus{Healthy: true},
	}

	assert.Equal(t, "test-volume", volume.Name)
	assert.Equal(t, uint64(1073741824), volume.Size)
	assert.True(t, volume.Status.Healthy)
}

func TestSubVolumeStruct(t *testing.T) {
	subvolume := SubVolume{
		ID:       256,
		Name:     "home",
		Path:     "/mnt/volume/home",
		ParentID: 5,
		ReadOnly: false,
		UUID:     "abcdef12-3456-7890-abcd-ef1234567890",
		Size:     536870912,
	}

	assert.Equal(t, uint64(256), subvolume.ID)
	assert.Equal(t, "home", subvolume.Name)
	assert.False(t, subvolume.ReadOnly)
}

func TestSnapshotStruct(t *testing.T) {
	snapshot := Snapshot{
		Name:       "snap-20240101",
		Path:       "/mnt/volume/.snapshots/snap-20240101",
		Source:     "home",
		SourceUUID: "abcdef12-3456-7890-abcd-ef1234567890",
		ReadOnly:   true,
		Size:       536870912,
	}

	assert.Equal(t, "snap-20240101", snapshot.Name)
	assert.True(t, snapshot.ReadOnly)
}

func TestShareStruct(t *testing.T) {
	share := Share{
		Type: "smb",
		Name: "documents",
		Path: "/mnt/volume/documents",
	}

	assert.Equal(t, "smb", share.Type)
	assert.Equal(t, "documents", share.Name)
}

func TestStatusStruct(t *testing.T) {
	status := Status{
		Version:    "2.0.0",
		Uptime:     "10 days",
		Hostname:   "nas-server",
		SMBRunning: true,
		NFSRunning: false,
		Volumes:    []Volume{},
	}

	assert.Equal(t, "2.0.0", status.Version)
	assert.True(t, status.SMBRunning)
	assert.False(t, status.NFSRunning)
}

func TestAPIResponseStruct(t *testing.T) {
	resp := APIResponse{
		Code:    200,
		Message: "success",
		Data:    []byte(`{"name":"test"}`),
	}

	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "success", resp.Message)
}

func TestOutputConstants(t *testing.T) {
	assert.Equal(t, "text", OutputText)
	assert.Equal(t, "json", OutputJSON)
}
