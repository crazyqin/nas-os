// Package raidzexpansion 单元测试
package raidzexpansion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRaidzLevel_Constants(t *testing.T) {
	assert.Equal(t, RaidzLevel(1), Raidz1)
	assert.Equal(t, RaidzLevel(2), Raidz2)
	assert.Equal(t, RaidzLevel(3), Raidz3)
}

func TestExpansionRequest_Fields(t *testing.T) {
	req := ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sdc", "/dev/sdd"},
	}

	assert.Equal(t, "tank", req.PoolName)
	assert.Len(t, req.NewDisks, 2)
}

func TestPoolInfo_Fields(t *testing.T) {
	pool := PoolInfo{
		Name:       "tank",
		RaidzLevel: Raidz1,
		DiskCount:  4,
		Disks: []*DiskInfo{
			{Device: "/dev/sda", SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, Healthy: true},
			{Device: "/dev/sdb", SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, Healthy: true},
		},
		Health:      "ONLINE",
		IsExpanding: false,
	}

	assert.Equal(t, "tank", pool.Name)
	assert.Equal(t, Raidz1, pool.RaidzLevel)
	assert.Equal(t, 4, pool.DiskCount)
	assert.Equal(t, "ONLINE", pool.Health)
	assert.False(t, pool.IsExpanding)
}

func TestDiskInfo_Fields(t *testing.T) {
	disk := DiskInfo{
		Device:      "/dev/sda",
		SizeBytes:   4 * 1024 * 1024 * 1024 * 1024,
		Healthy:     true,
		Temperature: 42,
	}

	assert.Equal(t, "/dev/sda", disk.Device)
	assert.True(t, disk.Healthy)
	assert.Equal(t, 42, disk.Temperature)
}

func TestValidationResult_Fields(t *testing.T) {
	result := ValidationResult{
		Valid:             true,
		Errors:            []ValidationError{},
		Warnings:          []ValidationWarning{},
		DiskCompatibility: []*DiskCompatibilityResult{},
	}

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
}

func TestValidationError_Fields(t *testing.T) {
	err := ValidationError{
		Code:    "EMPTY_POOL_NAME",
		Message: "存储池名称不能为空",
		Disk:    "",
	}

	assert.Equal(t, "EMPTY_POOL_NAME", err.Code)
	assert.Equal(t, "存储池名称不能为空", err.Message)
}

func TestValidationWarning_Fields(t *testing.T) {
	warn := ValidationWarning{
		Code:    "POOL_NOT_HEALTHY",
		Message: "存储池状态不健康",
	}

	assert.Equal(t, "POOL_NOT_HEALTHY", warn.Code)
}

func TestDiskCompatibilityResult_Fields(t *testing.T) {
	compat := DiskCompatibilityResult{
		Device:          "/dev/sdc",
		Compatible:      true,
		MinRequiredSize: 4 * 1024 * 1024 * 1024 * 1024,
	}

	assert.Equal(t, "/dev/sdc", compat.Device)
	assert.True(t, compat.Compatible)
}

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	assert.NotNil(t, v)
	assert.NotEmpty(t, v.knownDevicePrefixes)
}

func TestValidator_ValidateExpansionRequest_EmptyPoolName(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "",
		NewDisks: []string{"/dev/sdc"},
	}

	result := v.ValidateExpansionRequest(req, nil)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "EMPTY_POOL_NAME", result.Errors[0].Code)
}

func TestValidator_ValidateExpansionRequest_NoDisks(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{},
	}

	result := v.ValidateExpansionRequest(req, nil)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "NO_DISKS", result.Errors[0].Code)
}

func TestValidator_ValidateExpansionRequest_InvalidDevicePath(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"invalid_path"},
	}

	result := v.ValidateExpansionRequest(req, nil)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID_DEVICE_PATH", result.Errors[0].Code)
}

func TestValidator_ValidateExpansionRequest_DuplicateDisks(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sdc", "/dev/sdc"},
	}

	result := v.ValidateExpansionRequest(req, nil)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "DUPLICATE_DISKS", result.Errors[0].Code)
}

func TestValidator_ValidateExpansionRequest_Valid(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sdc", "/dev/sdd"},
	}

	result := v.ValidateExpansionRequest(req, nil)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidator_ValidateExpansionRequest_PoolExpanding(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sdc"},
	}
	pool := &PoolInfo{
		Name:        "tank",
		IsExpanding: true,
		Health:      "ONLINE",
		RaidzLevel:  Raidz1,
	}

	result := v.ValidateExpansionRequest(req, pool)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "POOL_EXPANDING", result.Errors[0].Code)
}

func TestValidator_ValidateExpansionRequest_DiskAlreadyInPool(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sda"},
	}
	pool := &PoolInfo{
		Name:       "tank",
		Health:     "ONLINE",
		RaidzLevel: Raidz1,
		DiskCount:  2,
		Disks: []*DiskInfo{
			{Device: "/dev/sda", SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, Healthy: true},
			{Device: "/dev/sdb", SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, Healthy: true},
		},
	}

	result := v.ValidateExpansionRequest(req, pool)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "DISK_ALREADY_IN_POOL", result.Errors[0].Code)
}

func TestValidator_ValidateDiskHealth_Unhealthy(t *testing.T) {
	v := NewValidator()
	disk := &DiskInfo{
		Device:      "/dev/sda",
		SizeBytes:   1024,
		Healthy:     false,
		Temperature: 40,
	}

	result := v.ValidateDiskHealth(disk)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "DISK_UNHEALTHY", result.Errors[0].Code)
}

func TestValidator_ValidateDiskHealth_CriticalTemperature(t *testing.T) {
	v := NewValidator()
	disk := &DiskInfo{
		Device:      "/dev/sda",
		SizeBytes:   1024,
		Healthy:     true,
		Temperature: 75,
	}

	result := v.ValidateDiskHealth(disk)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "CRITICAL_TEMPERATURE", result.Errors[0].Code)
}

func TestValidator_ValidateDiskHealth_HighTemperature(t *testing.T) {
	v := NewValidator()
	disk := &DiskInfo{
		Device:      "/dev/sda",
		SizeBytes:   1024,
		Healthy:     true,
		Temperature: 65,
	}

	result := v.ValidateDiskHealth(disk)
	assert.True(t, result.Valid) // 高温但未到危险级别
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "HIGH_TEMPERATURE", result.Warnings[0].Code)
}

func TestValidator_ValidateDiskHealth_Healthy(t *testing.T) {
	v := NewValidator()
	disk := &DiskInfo{
		Device:      "/dev/sda",
		SizeBytes:   4 * 1024 * 1024 * 1024 * 1024,
		Healthy:     true,
		Temperature: 40,
	}

	result := v.ValidateDiskHealth(disk)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidator_GetMinDiskSize(t *testing.T) {
	v := NewValidator()

	assert.Equal(t, uint64(0), v.GetMinDiskSize(nil))

	pool := &PoolInfo{
		Disks: []*DiskInfo{
			{SizeBytes: 4 * 1024 * 1024 * 1024 * 1024},
			{SizeBytes: 8 * 1024 * 1024 * 1024 * 1024},
		},
	}
	assert.Equal(t, uint64(4*1024*1024*1024*1024), v.GetMinDiskSize(pool))
}

func TestValidator_ValidateExpansionRequest_PoolNotHealthy(t *testing.T) {
	v := NewValidator()
	req := &ExpansionRequest{
		PoolName: "tank",
		NewDisks: []string{"/dev/sdc"},
	}
	pool := &PoolInfo{
		Name:       "tank",
		Health:     "DEGRADED",
		RaidzLevel: Raidz1,
		DiskCount:  2,
		Disks: []*DiskInfo{
			{Device: "/dev/sda", SizeBytes: 4 * 1024 * 1024 * 1024 * 1024, Healthy: true},
		},
	}

	result := v.ValidateExpansionRequest(req, pool)
	assert.True(t, result.Valid) // 仍然有效，但有警告
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "POOL_NOT_HEALTHY", result.Warnings[0].Code)
}
