// Package fctarget 单元测试
package fctarget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTargetMode_Constants(t *testing.T) {
	assert.Equal(t, TargetMode("initiator"), TargetModeInitiator)
	assert.Equal(t, TargetMode("target"), TargetModeTarget)
	assert.Equal(t, TargetMode("dual"), TargetModeDual)
}

func TestFCTarget_Fields(t *testing.T) {
	now := time.Now()
	target := FCTarget{
		ID:          "fc-target-1",
		Name:        "Production Target",
		Alias:       "prod-fc",
		WWPN:        "50:00:00:00:00:00:00:01",
		WWNN:        "50:00:00:00:00:00:00:00",
		Mode:        TargetModeTarget,
		LUNs:        []*LUN{},
		Ports:       []*Port{},
		Zones:       []*Zone{},
		MaxSessions: 64,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assert.Equal(t, "fc-target-1", target.ID)
	assert.Equal(t, "Production Target", target.Name)
	assert.Equal(t, TargetModeTarget, target.Mode)
	assert.Equal(t, 64, target.MaxSessions)
	assert.True(t, target.Enabled)
}

func TestTargetInput_Fields(t *testing.T) {
	input := TargetInput{
		Name:        "New Target",
		Alias:       "new-fc",
		Mode:        TargetModeDual,
		MaxSessions: 32,
	}

	assert.Equal(t, "New Target", input.Name)
	assert.Equal(t, TargetModeDual, input.Mode)
}

func TestLUN_Fields(t *testing.T) {
	now := time.Now()
	lun := LUN{
		ID:        "lun-1",
		Number:    0,
		Name:      "Data LUN",
		Type:      LUNTypeBlock,
		Path:      "/dev/sdb",
		Size:      1024 * 1024 * 1024 * 1024,
		BlockSize: 4096,
		ReadOnly:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "lun-1", lun.ID)
	assert.Equal(t, 0, lun.Number)
	assert.Equal(t, LUNTypeBlock, lun.Type)
	assert.Equal(t, int64(1024*1024*1024*1024), lun.Size)
	assert.False(t, lun.ReadOnly)
}

func TestLUNType_Constants(t *testing.T) {
	assert.Equal(t, LUNType("block"), LUNTypeBlock)
	assert.Equal(t, LUNType("file"), LUNTypeFile)
}

func TestPort_Fields(t *testing.T) {
	now := time.Now()
	port := Port{
		ID:        "port-1",
		Name:      "fc0",
		WWPN:      "50:00:00:00:00:00:01:01",
		Speed:     "16G",
		State:     PortStateOnline,
		Type:      PortTypeTarget,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "port-1", port.ID)
	assert.Equal(t, "fc0", port.Name)
	assert.Equal(t, "16G", port.Speed)
	assert.True(t, port.Enabled)
}

func TestPortState_Constants(t *testing.T) {
	assert.NotEmpty(t, PortStateOnline)
	assert.NotEmpty(t, PortStateOffline)
}

func TestPortType_Constants(t *testing.T) {
	assert.NotEmpty(t, PortTypeTarget)
	assert.NotEmpty(t, PortTypeInitiator)
}

func TestZone_Fields(t *testing.T) {
	now := time.Now()
	zone := Zone{
		ID:   "zone-1",
		Name: "Production Zone",
		Members: []ZoneMember{
			{WWPN: "50:00:00:00:00:00:01:01"},
			{WWPN: "50:00:00:00:00:00:02:01"},
		},
		Enabled:   true,
		CreatedAt: now,
	}

	assert.Equal(t, "zone-1", zone.ID)
	assert.Equal(t, "Production Zone", zone.Name)
	assert.Len(t, zone.Members, 2)
	assert.True(t, zone.Enabled)
}

func TestZoneMember_Fields(t *testing.T) {
	member := ZoneMember{
		WWPN: "50:00:00:00:00:00:01:01",
	}

	assert.Equal(t, "50:00:00:00:00:00:01:01", member.WWPN)
}

func TestSession_Fields(t *testing.T) {
	now := time.Now()
	session := Session{
		ID:            "session-1",
		InitiatorWWPN: "50:00:00:00:00:00:02:01",
		TargetWWPN:    "50:00:00:00:00:00:01:01",
		PortID:        "port-1",
		State:         "logged_in",
		ConnectedAt:   now,
	}

	assert.Equal(t, "session-1", session.ID)
	assert.Equal(t, "logged_in", session.State)
	assert.Equal(t, "port-1", session.PortID)
}

func TestTargetStatus_Fields(t *testing.T) {
	status := TargetStatus{
		ID: "fc-target-1",
	}

	assert.Equal(t, "fc-target-1", status.ID)
}

func TestPerformanceStats_Fields(t *testing.T) {
	stats := PerformanceStats{}

	assert.NotNil(t, stats)
}

func TestLUNInput_Fields(t *testing.T) {
	input := LUNInput{
		Name: "Data LUN",
		Type: LUNTypeBlock,
	}

	assert.Equal(t, "Data LUN", input.Name)
	assert.Equal(t, LUNTypeBlock, input.Type)
}

func TestZoneInput_Fields(t *testing.T) {
	input := ZoneInput{
		Name: "New Zone",
	}

	assert.Equal(t, "New Zone", input.Name)
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{}

	assert.NotNil(t, cfg)
}

func TestFCError_Fields(t *testing.T) {
	fcErr := FCError{
		Code:    "TARGET_NOT_FOUND",
		Message: "Target not found",
	}

	assert.Equal(t, "TARGET_NOT_FOUND", fcErr.Code)
	assert.Equal(t, "Target not found", fcErr.Message)
}
