// Package enclosure 单元测试
package enclosure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnclosureStatus_Constants(t *testing.T) {
	assert.Equal(t, EnclosureStatus("online"), StatusOnline)
	assert.Equal(t, EnclosureStatus("degraded"), StatusDegraded)
	assert.Equal(t, EnclosureStatus("offline"), StatusOffline)
}

func TestLEDState_Constants(t *testing.T) {
	assert.Equal(t, LEDState("off"), LEDOff)
	assert.Equal(t, LEDState("on"), LEDOn)
	assert.Equal(t, LEDState("blink"), LEDBlink)
}

func TestLEDType_Constants(t *testing.T) {
	assert.Equal(t, LEDType("locate"), LEDLocate)
	assert.Equal(t, LEDType("fault"), LEDFault)
	assert.Equal(t, LEDType("activity"), LEDActivity)
}

func TestEnclosure_Fields(t *testing.T) {
	enc := Enclosure{
		ID:       "enc0",
		Vendor:   "SuperMicro",
		Model:    "CSE-836",
		Serial:   "SN12345",
		Firmware: "1.0.0",
		Status:   StatusOnline,
		Slots:    []*Slot{},
		Sensors:  []*Sensor{},
	}

	assert.Equal(t, "enc0", enc.ID)
	assert.Equal(t, "SuperMicro", enc.Vendor)
	assert.Equal(t, StatusOnline, enc.Status)
}

func TestSlot_Fields(t *testing.T) {
	slot := Slot{
		ID:          0,
		Status:      SlotActive,
		LEDStates:   map[LEDType]LEDState{LEDLocate: LEDOff, LEDFault: LEDOff},
		DiskPresent: true,
		Device:      "/dev/sda",
	}

	assert.Equal(t, 0, slot.ID)
	assert.Equal(t, SlotActive, slot.Status)
	assert.True(t, slot.DiskPresent)
	assert.Equal(t, "/dev/sda", slot.Device)
}

func TestSlotDiskInfo_Fields(t *testing.T) {
	diskInfo := SlotDiskInfo{
		Model:  "WD Red Plus",
		Serial: "WD-WCC1234567",
	}

	assert.Equal(t, "WD Red Plus", diskInfo.Model)
	assert.Equal(t, "WD-WCC1234567", diskInfo.Serial)
}

func TestSensor_Fields(t *testing.T) {
	sensor := Sensor{
		ID:           0,
		Name:         "温度传感器 0",
		Type:         SensorTemperature,
		Value:        42.5,
		Unit:         "°C",
		ThresholdHigh: 60.0,
		Status:       SensorNormal,
	}

	assert.Equal(t, 0, sensor.ID)
	assert.Equal(t, SensorTemperature, sensor.Type)
	assert.Equal(t, 42.5, sensor.Value)
	assert.Equal(t, SensorNormal, sensor.Status)
}

func TestSlotStatus_Constants(t *testing.T) {
	assert.NotEmpty(t, SlotActive)
	assert.NotEmpty(t, SlotEmpty)
	assert.NotEmpty(t, SlotFault)
}

func TestSensorType_Constants(t *testing.T) {
	assert.NotEmpty(t, SensorTemperature)
	assert.NotEmpty(t, SensorVoltage)
}

func TestSensorStatus_Constants(t *testing.T) {
	assert.NotEmpty(t, SensorNormal)
	assert.NotEmpty(t, SensorWarning)
	assert.NotEmpty(t, SensorCritical)
}

func TestPowerSupply_Fields(t *testing.T) {
	ps := PowerSupply{
		ID: 0,
	}

	assert.Equal(t, 0, ps.ID)
}

func TestFan_Fields(t *testing.T) {
	fan := Fan{
		ID: 0,
	}

	assert.Equal(t, 0, fan.ID)
}

func TestBuildControlByte(t *testing.T) {
	tests := []struct {
		ledType LEDType
		state   LEDState
		want    string
	}{
		{LEDLocate, LEDOn, "locate=1"},
		{LEDLocate, LEDBlink, "locate=1"},
		{LEDLocate, LEDOff, "locate=0"},
		{LEDFault, LEDOn, "fault=1"},
		{LEDFault, LEDOff, "fault=0"},
		{LEDActivity, LEDOn, ""},
	}

	for _, tt := range tests {
		got := buildControlByte(tt.ledType, tt.state)
		assert.Equal(t, tt.want, got)
	}
}

func TestNewSESClient(t *testing.T) {
	client := NewSESClient("/dev/sg0")
	assert.NotNil(t, client)
	assert.Equal(t, "/dev/sg0", client.device)
}

func TestSESClient_GetEnclosureInfo_EmptyDevice(t *testing.T) {
	client := NewSESClient("")
	_, err := client.GetEnclosureInfo()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "设备路径为空")
}

func TestSESClient_SetSlotLED_EmptyDevice(t *testing.T) {
	client := NewSESClient("")
	err := client.SetSlotLED(0, LEDLocate, LEDOn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "设备路径为空")
}

func TestSESClient_GetSensors_EmptyDevice(t *testing.T) {
	client := NewSESClient("")
	_, err := client.GetSensors()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "设备路径为空")
}
