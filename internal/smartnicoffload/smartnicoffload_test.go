package smartnicoffload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceTypes(t *testing.T) {
	assert.Equal(t, "smartnic", string(DeviceTypeSmartNIC))
	assert.Equal(t, "dpu", string(DeviceTypeDPU))
	assert.Equal(t, "ipu", string(DeviceTypeIPU))
}

func TestOffloadTypes(t *testing.T) {
	assert.Equal(t, "ovs", string(OffloadOVS))
	assert.Equal(t, "ipsec", string(OffloadIPsec))
	assert.Equal(t, "tls", string(OffloadTLS))
	assert.Equal(t, "compress", string(OffloadCompress))
	assert.Equal(t, "firewall", string(OffloadFirewall))
	assert.Equal(t, "rdma", string(OffloadRDMA))
}

func TestRegisterDevice(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	dev := &SmartNICDevice{
		ID:      "nic-001",
		Name:    "BlueField-3",
		Type:    DeviceTypeDPU,
		PCIeAddr: "0000:03:00.0",
		NumCores: 16,
		Memory:  16 * 1024 * 1024 * 1024,
		NumPorts: 2,
		MaxOffloads: 8,
		Offloads: []OffloadType{OffloadOVS, OffloadIPsec, OffloadTLS, OffloadRDMA},
		Speed:   400,
	}

	err := mgr.RegisterDevice(dev)
	require.NoError(t, err)
	assert.Equal(t, StateReady, dev.State)

	// Duplicate
	err = mgr.RegisterDevice(dev)
	assert.ErrorIs(t, err, ErrDeviceExists)
}

func TestEnableOffload(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	dev := &SmartNICDevice{
		ID:       "nic-001",
		Name:     "BlueField-3",
		Type:     DeviceTypeDPU,
		Offloads: []OffloadType{OffloadOVS, OffloadIPsec},
	}
	_ = mgr.RegisterDevice(dev)

	offload, err := mgr.EnableOffload("nic-001", OffloadOVS, nil)
	require.NoError(t, err)
	assert.Equal(t, OffloadStateEnabled, offload.State)
	assert.Equal(t, OffloadType("ovs"), offload.Type)

	// Unsupported offload
	_, err = mgr.EnableOffload("nic-001", OffloadMLInference, nil)
	assert.ErrorIs(t, err, ErrOffloadNotSupported)
}

func TestDisableOffload(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	dev := &SmartNICDevice{
		ID:       "nic-001",
		Name:     "SmartNIC-0",
		Type:     DeviceTypeSmartNIC,
		Offloads: []OffloadType{OffloadFirewall},
	}
	_ = mgr.RegisterDevice(dev)

	offload, _ := mgr.EnableOffload("nic-001", OffloadFirewall, nil)

	err := mgr.DisableOffload(offload.ID)
	assert.NoError(t, err)
}

func TestListDevices(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	_ = mgr.RegisterDevice(&SmartNICDevice{ID: "nic-001", Name: "NIC1", Offloads: []OffloadType{}})
	_ = mgr.RegisterDevice(&SmartNICDevice{ID: "nic-002", Name: "NIC2", Offloads: []OffloadType{}})

	devices := mgr.ListDevices()
	assert.Len(t, devices, 2)
}

func TestListOffloads(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	dev := &SmartNICDevice{
		ID:       "nic-001",
		Name:     "NIC-0",
		Type:     DeviceTypeSmartNIC,
		Offloads: []OffloadType{OffloadOVS, OffloadIPsec, OffloadTLS},
	}
	_ = mgr.RegisterDevice(dev)

	_, _ = mgr.EnableOffload("nic-001", OffloadOVS, nil)
	_, _ = mgr.EnableOffload("nic-001", OffloadIPsec, nil)

	offloads := mgr.ListOffloads("nic-001")
	assert.Len(t, offloads, 2)
}

func TestUnregisterDevice(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	_ = mgr.RegisterDevice(&SmartNICDevice{ID: "nic-001", Name: "NIC", Offloads: []OffloadType{}})

	err := mgr.UnregisterDevice("nic-001")
	assert.NoError(t, err)

	_, err = mgr.GetDevice("nic-001")
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}

func TestManagerClosed(t *testing.T) {
	mgr := NewManager()
	mgr.Close()

	err := mgr.RegisterDevice(&SmartNICDevice{ID: "nic-001"})
	assert.ErrorIs(t, err, ErrManagerClosed)
}

func TestEnableOnNonexistentDevice(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	_, err := mgr.EnableOffload("nonexistent", OffloadOVS, nil)
	assert.ErrorIs(t, err, ErrDeviceNotFound)
}
