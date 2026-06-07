package passkey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== DeviceTrustManager Tests ==========

func TestNewDeviceTrustManager(t *testing.T) {
	cfg := DefaultDeviceTrustConfig()
	mgr := NewDeviceTrustManager(cfg, nil)
	assert.NotNil(t, mgr)
	assert.Equal(t, MaxTrustedDevicesPerUser, mgr.config.MaxDevices)
}

func TestTrustDevice(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{
			DeviceName:  "My Laptop",
			DeviceType:  "desktop",
			BrowserName: "Chrome",
			BrowserVer:  "120.0",
			OSName:      "macOS",
			OSVersion:   "14.2",
			Fingerprint: "abc123fingerprint",
			IPAddress:   "192.168.1.100",
		},
		TrustDays: 30,
		TOTPCode:  "123456",
	}

	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)
	assert.NotNil(t, device)
	assert.Equal(t, "user-1", device.UserID)
	assert.Equal(t, "My Laptop", device.DeviceName)
	assert.Equal(t, "desktop", device.DeviceType)
	assert.Equal(t, "192.168.1.100", device.IPAddress)
	assert.False(t, device.Revoked)
	assert.True(t, device.ExpiresAt.After(time.Now()))
}

func TestTrustDeviceMissingFingerprint(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{
			DeviceName:  "Test",
			Fingerprint: "",
		},
	}

	_, err := mgr.TrustDevice("user-1", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fingerprint required")
}

func TestTrustDeviceAutoName(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{
			BrowserName: "Firefox",
			OSName:      "Linux",
			Fingerprint: "fp-auto-name",
		},
	}

	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)
	assert.Equal(t, "Firefox on Linux", device.DeviceName)
}

func TestTrustDeviceRefreshExisting(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	fp := "fp-refresh-test"
	req := TrustRequest{
		DeviceInfo: DeviceInfo{
			DeviceName:  "First",
			Fingerprint: fp,
		},
		TrustDays: 7,
	}

	device1, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	// Trust again with same fingerprint - should refresh
	req.DeviceInfo.DeviceName = "Second"
	req.TrustDays = 30
	device2, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	assert.Equal(t, device1.ID, device2.ID) // Same device
	assert.Equal(t, "Second", device2.DeviceName)
}

func TestTrustDeviceMaxLimit(t *testing.T) {
	cfg := DeviceTrustConfig{
		TrustDuration: 24 * time.Hour,
		MaxDevices:    3,
	}
	mgr := NewDeviceTrustManager(cfg, nil)

	for i := 0; i < 3; i++ {
		req := TrustRequest{
			DeviceInfo: DeviceInfo{
				Fingerprint: "fp-max-" + string(rune('a'+i)),
			},
		}
		_, err := mgr.TrustDevice("user-1", req)
		require.NoError(t, err)
	}

	// 4th device should fail
	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-max-d"},
	}
	_, err := mgr.TrustDevice("user-1", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum trusted devices")
}

func TestVerifyDeviceTrust(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	fp := "fp-verify-test"
	req := TrustRequest{
		DeviceInfo: DeviceInfo{
			DeviceName:  "Verify Test Device",
			Fingerprint: fp,
		},
		TrustDays: 7,
	}
	_, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	// Verify trusted
	result := mgr.VerifyDeviceTrust("user-1", fp)
	assert.True(t, result.Trusted)
	assert.Equal(t, "Verify Test Device", result.DeviceName)
	assert.NotEmpty(t, result.DeviceID)
	assert.NotEmpty(t, result.ExpiresAt)
}

func TestVerifyDeviceTrustNotTrusted(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	result := mgr.VerifyDeviceTrust("user-1", "unknown-fp")
	assert.False(t, result.Trusted)
	assert.NotEmpty(t, result.Reason)
}

func TestVerifyDeviceTrustRevoked(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	fp := "fp-revoked-test"
	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: fp},
	}
	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	// Revoke
	err = mgr.RevokeDevice("user-1", device.ID)
	require.NoError(t, err)

	result := mgr.VerifyDeviceTrust("user-1", fp)
	assert.False(t, result.Trusted)
	assert.Contains(t, result.Reason, "revoked")
}

func TestVerifyDeviceTrustEmptyInputs(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	result := mgr.VerifyDeviceTrust("", "fp")
	assert.False(t, result.Trusted)

	result = mgr.VerifyDeviceTrust("user-1", "")
	assert.False(t, result.Trusted)
}

func TestRevokeDevice(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-revoke"},
	}
	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	err = mgr.RevokeDevice("user-1", device.ID)
	assert.NoError(t, err)

	// Should be in revoked list
	all := mgr.GetAllDevices("user-1")
	require.Len(t, all, 1)
	assert.True(t, all[0].Revoked)
	assert.NotNil(t, all[0].RevokedAt)
	assert.Equal(t, "user_revoked", all[0].RevokedReason)
}

func TestRevokeDeviceNotFound(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	err := mgr.RevokeDevice("user-1", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRevokeAllDevices(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	for i := 0; i < 3; i++ {
		req := TrustRequest{
			DeviceInfo: DeviceInfo{Fingerprint: "fp-revokeall-" + string(rune('a'+i))},
		}
		_, err := mgr.TrustDevice("user-1", req)
		require.NoError(t, err)
	}

	count := mgr.RevokeAllDevices("user-1", "security_breach")
	assert.Equal(t, 3, count)

	devices := mgr.GetTrustedDevices("user-1")
	assert.Len(t, devices, 0)
}

func TestGetTrustedDevices(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	// Add two devices
	for i := 0; i < 2; i++ {
		req := TrustRequest{
			DeviceInfo: DeviceInfo{
				DeviceName:  "Device " + string(rune('A'+i)),
				Fingerprint: "fp-list-" + string(rune('a'+i)),
			},
		}
		_, err := mgr.TrustDevice("user-1", req)
		require.NoError(t, err)
	}

	devices := mgr.GetTrustedDevices("user-1")
	assert.Len(t, devices, 2)
	// Should not expose sensitive fields
	for _, d := range devices {
		assert.Empty(t, d.Fingerprint)
		assert.Empty(t, d.TrustToken)
	}
}

func TestGetTrustedDevicesHidesRevoked(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-hide"},
	}
	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	_, err = mgr.TrustDevice("user-1", TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-active"},
	})
	require.NoError(t, err)

	// Revoke one
	err = mgr.RevokeDevice("user-1", device.ID)
	require.NoError(t, err)

	trusted := mgr.GetTrustedDevices("user-1")
	assert.Len(t, trusted, 1) // Only the active one
}

func TestGetAllDevices(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-all"},
	}
	device, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	_ = mgr.RevokeDevice("user-1", device.ID)

	all := mgr.GetAllDevices("user-1")
	assert.Len(t, all, 1)
	assert.True(t, all[0].Revoked)
}

func TestCleanupExpired(t *testing.T) {
	cfg := DeviceTrustConfig{
		TrustDuration: 1 * time.Hour,
		MaxDevices:    20,
	}
	mgr := NewDeviceTrustManager(cfg, nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-cleanup"},
	}
	_, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	// Manually expire the device
	mgr.mu.Lock()
	mgr.devices["user-1"][0].ExpiresAt = time.Now().Add(-1 * time.Hour)
	mgr.mu.Unlock()

	removed := mgr.CleanupExpired()
	assert.Equal(t, 1, removed)

	devices := mgr.GetTrustedDevices("user-1")
	assert.Len(t, devices, 0)
}

func TestUpdateLastUsed(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	req := TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-used"},
	}
	_, err := mgr.TrustDevice("user-1", req)
	require.NoError(t, err)

	// Update last used
	mgr.UpdateLastUsed("user-1", "fp-used")

	devices := mgr.GetAllDevices("user-1")
	require.Len(t, devices, 1)
	assert.WithinDuration(t, time.Now(), devices[0].LastUsedAt, 2*time.Second)
}

func TestStats(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	// Add some devices
	for i := 0; i < 2; i++ {
		_, _ = mgr.TrustDevice("user-1", TrustRequest{
			DeviceInfo: DeviceInfo{Fingerprint: "fp-stats-" + string(rune('a'+i))},
		})
	}

	stats := mgr.Stats("user-1")
	assert.Equal(t, 2, stats["total"])
	assert.Equal(t, 2, stats["active"])
	assert.Equal(t, 0, stats["expired"])
	assert.Equal(t, 0, stats["revoked"])
}

func TestGetConfig(t *testing.T) {
	cfg := DeviceTrustConfig{
		TrustDuration:    7 * 24 * time.Hour,
		MaxDevices:       10,
		RequireName:      true,
		RevokeOnPassword: true,
	}
	mgr := NewDeviceTrustManager(cfg, nil)

	got := mgr.GetConfig()
	assert.Equal(t, 7*24*time.Hour, got.TrustDuration)
	assert.Equal(t, 10, got.MaxDevices)
	assert.True(t, got.RequireName)
}

func TestUpdateConfig(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	newCfg := DeviceTrustConfig{
		TrustDuration: 14 * 24 * time.Hour,
		MaxDevices:    5,
	}
	mgr.UpdateConfig(newCfg)

	got := mgr.GetConfig()
	assert.Equal(t, 14*24*time.Hour, got.TrustDuration)
	assert.Equal(t, 5, got.MaxDevices)
}

func TestUpdateConfigClampDuration(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	// Too short
	mgr.UpdateConfig(DeviceTrustConfig{TrustDuration: 1 * time.Second})
	assert.Equal(t, MinTrustDuration, mgr.GetConfig().TrustDuration)

	// Too long
	mgr.UpdateConfig(DeviceTrustConfig{TrustDuration: 365 * 24 * time.Hour})
	assert.Equal(t, MaxTrustDuration, mgr.GetConfig().TrustDuration)
}

func TestMultipleUsersIsolation(t *testing.T) {
	mgr := NewDeviceTrustManager(DefaultDeviceTrustConfig(), nil)

	_, _ = mgr.TrustDevice("user-1", TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-user1"},
	})
	_, _ = mgr.TrustDevice("user-2", TrustRequest{
		DeviceInfo: DeviceInfo{Fingerprint: "fp-user2"},
	})

	// user-1 should not see user-2's devices
	devices := mgr.GetTrustedDevices("user-1")
	assert.Len(t, devices, 1)

	devices = mgr.GetTrustedDevices("user-2")
	assert.Len(t, devices, 1)

	// Revoking user-1's devices should not affect user-2
	mgr.RevokeAllDevices("user-1", "test")
	assert.Len(t, mgr.GetTrustedDevices("user-1"), 0)
	assert.Len(t, mgr.GetTrustedDevices("user-2"), 1)
}
