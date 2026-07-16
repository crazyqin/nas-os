package cxlmempool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCXLVersions(t *testing.T) {
	tests := []struct {
		name     string
		version  CXLVersion
		expected string
	}{
		{"CXL 1.1", CXL11, "1.1"},
		{"CXL 2.0", CXL20, "2.0"},
		{"CXL 3.0", CXL30, "3.0"},
		{"CXL 3.1", CXL31, "3.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.version))
		})
	}
}

func TestDeviceTypes(t *testing.T) {
	assert.Equal(t, "memory", string(DeviceTypeMemory))
	assert.Equal(t, "switch", string(DeviceTypeSwitch))
	assert.Equal(t, "type1", string(DeviceTypeType1))
	assert.Equal(t, "type2", string(DeviceTypeType2))
	assert.Equal(t, "type3", string(DeviceTypeType3))
}

func TestMemoryTiers(t *testing.T) {
	assert.Equal(t, "hot", string(TierHot))
	assert.Equal(t, "warm", string(TierWarm))
	assert.Equal(t, "cold", string(TierCold))
	assert.Equal(t, "archive", string(TierArchive))
}

func TestManagerRegisterDevice(t *testing.T) {
	mgr := NewManager(PolicyBestFit)
	defer mgr.Close()

	dev := &CXLDevice{
		ID:           "dev-001",
		Name:         "CXL Memory 0",
		Version:      CXL20,
		Type:         DeviceTypeMemory,
		TotalMemory:  64 * 1024 * 1024 * 1024, // 64GB
		AvailableMem: 64 * 1024 * 1024 * 1024,
		Bandwidth:    64.0,
		Latency:      150.0,
	}

	err := mgr.RegisterDevice(dev)
	require.NoError(t, err)
	assert.Equal(t, StateOnline, dev.State)

	// Duplicate registration
	err = mgr.RegisterDevice(dev)
	assert.ErrorIs(t, err, ErrDeviceAlreadyExists)
}

func TestManagerCreatePoolAndAllocate(t *testing.T) {
	mgr := NewManager(PolicyBestFit)
	defer mgr.Close()

	dev := &CXLDevice{
		ID:           "dev-001",
		Name:         "CXL Memory 0",
		Version:      CXL20,
		Type:         DeviceTypeMemory,
		TotalMemory:  64 * 1024 * 1024 * 1024,
		AvailableMem: 64 * 1024 * 1024 * 1024,
	}
	require.NoError(t, mgr.RegisterDevice(dev))

	pool, err := mgr.CreatePool("warm-pool", TierWarm, []string{"dev-001"})
	require.NoError(t, err)
	assert.Equal(t, uint64(64*1024*1024*1024), pool.TotalMemory)

	alloc, err := mgr.Allocate(pool.ID, 4*1024*1024*1024) // 4GB
	require.NoError(t, err)
	assert.Equal(t, uint64(4*1024*1024*1024), alloc.Size)

	// Over-allocate
	_, err = mgr.Allocate(pool.ID, 128*1024*1024*1024)
	assert.ErrorIs(t, err, ErrInsufficientMemory)
}

func TestManagerFree(t *testing.T) {
	mgr := NewManager(PolicyBestFit)
	defer mgr.Close()

	dev := &CXLDevice{
		ID:           "dev-001",
		Name:         "CXL Memory 0",
		Version:      CXL20,
		Type:         DeviceTypeMemory,
		TotalMemory:  64 * 1024 * 1024 * 1024,
		AvailableMem: 64 * 1024 * 1024 * 1024,
	}
	require.NoError(t, mgr.RegisterDevice(dev))

	pool, err := mgr.CreatePool("test-pool", TierWarm, []string{"dev-001"})
	require.NoError(t, err)

	alloc, err := mgr.Allocate(pool.ID, 8*1024*1024*1024)
	require.NoError(t, err)

	err = mgr.Free(alloc.ID)
	assert.NoError(t, err)

	// Double free
	err = mgr.Free(alloc.ID)
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestManagerClosed(t *testing.T) {
	mgr := NewManager(PolicyBestFit)
	mgr.Close()

	dev := &CXLDevice{ID: "dev-001"}
	err := mgr.RegisterDevice(dev)
	assert.ErrorIs(t, err, ErrManagerClosed)
}

func TestManagerConcurrency(t *testing.T) {
	mgr := NewManager(PolicyBestFit)
	defer mgr.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			dev := &CXLDevice{
				ID:           fmt.Sprintf("dev-%03d", id),
				Name:         fmt.Sprintf("CXL Device %d", id),
				Version:      CXL20,
				Type:         DeviceTypeMemory,
				TotalMemory:  1024 * 1024 * 1024,
				AvailableMem: 1024 * 1024 * 1024,
			}
			_ = mgr.RegisterDevice(dev)
		}(i)
	}
	wg.Wait()

	devices := mgr.ListDevices()
	assert.Len(t, devices, 10)
}

func BenchmarkAllocate(b *testing.B) {
	mgr := NewManager(PolicyBestFit)
	defer mgr.Close()

	dev := &CXLDevice{
		ID:           "dev-001",
		Name:         "CXL Memory 0",
		Version:      CXL20,
		Type:         DeviceTypeMemory,
		TotalMemory:  1024 * 1024 * 1024 * 1024, // 1TB
		AvailableMem: 1024 * 1024 * 1024 * 1024,
	}
	_ = mgr.RegisterDevice(dev)
	pool, _ := mgr.CreatePool("bench-pool", TierWarm, []string{"dev-001"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		alloc, _ := mgr.Allocate(pool.ID, 4096)
		_ = mgr.Free(alloc.ID)
	}
}
