// Package enclosure 提供机箱管理的数据结构定义
package enclosure

import (
	"time"
)

// EnclosureStatus 机箱状态.
type EnclosureStatus string

const (
	// StatusOnline 机箱在线.
	StatusOnline EnclosureStatus = "online"
	// StatusDegraded 机箱降级.
	StatusDegraded EnclosureStatus = "degraded"
	// StatusOffline 机箱离线.
	StatusOffline EnclosureStatus = "offline"
)

// LEDState 指示灯状态.
type LEDState string

const (
	// LEDOff 关闭.
	LEDOff LEDState = "off"
	// LEDOn 常亮.
	LEDOn LEDState = "on"
	// LEDBlink 闪烁.
	LEDBlink LEDState = "blink"
)

// LEDType 指示灯类型.
type LEDType string

const (
	// LEDLocate 定位灯.
	LEDLocate LEDType = "locate"
	// LEDFault 故障灯.
	LEDFault LEDType = "fault"
	// LEDActivity 活动灯.
	LEDActivity LEDType = "activity"
)

// Enclosure 机箱信息.
type Enclosure struct {
	// ID 机箱唯一标识
	ID string `json:"id"`
	// Vendor 厂商
	Vendor string `json:"vendor"`
	// Model 型号
	Model string `json:"model"`
	// Serial 序列号
	Serial string `json:"serial"`
	// Firmware 固件版本
	Firmware string `json:"firmware"`
	// Status 机箱状态
	Status EnclosureStatus `json:"status"`
	// Slots 硬盘槽位列表
	Slots []*Slot `json:"slots"`
	// Sensors 传感器列表
	Sensors []*Sensor `json:"sensors"`
	// PowerSupplies 电源列表
	PowerSupplies []*PowerSupply `json:"powerSupplies"`
	// Fans 风扇列表
	Fans []*Fan `json:"fans"`
	// LastSeen 最后发现时间
	LastSeen time.Time `json:"lastSeen"`
}

// Slot 硬盘槽位.
type Slot struct {
	// ID 槽位编号
	ID int `json:"id"`
	// Device 设备路径（如 /dev/sda）
	Device string `json:"device"`
	// SASAddress SAS 地址
	SASAddress string `json:"sasAddress,omitempty"`
	// Status 槽位状态
	Status SlotStatus `json:"status"`
	// LEDStates 指示灯状态
	LEDStates map[LEDType]LEDState `json:"ledStates"`
	// DiskPresent 是否有磁盘
	DiskPresent bool `json:"diskPresent"`
	// DiskInfo 磁盘信息（如果有）
	DiskInfo *SlotDiskInfo `json:"diskInfo,omitempty"`
}

// SlotStatus 槽位状态.
type SlotStatus string

const (
	// SlotEmpty 空槽位.
	SlotEmpty SlotStatus = "empty"
	// SlotActive 正常工作.
	SlotActive SlotStatus = "active"
	// SlotFault 故障.
	SlotFault SlotStatus = "fault"
	// SlotDisabled 禁用.
	SlotDisabled SlotStatus = "disabled"
)

// SlotDiskInfo 槽位中的磁盘信息.
type SlotDiskInfo struct {
	// Model 型号
	Model string `json:"model"`
	// Serial 序列号
	Serial string `json:"serial"`
	// Size 大小（字节）
	Size uint64 `json:"size"`
	// Temperature 温度（摄氏度）
	Temperature float64 `json:"temperature"`
}

// Sensor 传感器.
type Sensor struct {
	// ID 传感器编号
	ID int `json:"id"`
	// Name 传感器名称
	Name string `json:"name"`
	// Type 传感器类型
	Type SensorType `json:"type"`
	// Value 当前值
	Value float64 `json:"value"`
	// Unit 单位
	Unit string `json:"unit"`
	// ThresholdHigh 高阈值
	ThresholdHigh float64 `json:"thresholdHigh,omitempty"`
	// ThresholdLow 低阈值
	ThresholdLow float64 `json:"thresholdLow,omitempty"`
	// Status 传感器状态
	Status SensorStatus `json:"status"`
}

// SensorType 传感器类型.
type SensorType string

const (
	// SensorTemperature 温度传感器.
	SensorTemperature SensorType = "temperature"
	// SensorVoltage 电压传感器.
	SensorVoltage SensorType = "voltage"
	// SensorCurrent 电流传感器.
	SensorCurrent SensorType = "current"
)

// SensorStatus 传感器状态.
type SensorStatus string

const (
	// SensorNormal 正常.
	SensorNormal SensorStatus = "normal"
	// SensorWarning 警告.
	SensorWarning SensorStatus = "warning"
	// SensorCritical 严重.
	SensorCritical SensorStatus = "critical"
)

// PowerSupply 电源.
type PowerSupply struct {
	// ID 电源编号
	ID int `json:"id"`
	// Name 电源名称
	Name string `json:"name"`
	// Status 电源状态
	Status PowerStatus `json:"status"`
	// Watts 当前功率（瓦）
	Watts float64 `json:"watts"`
	// MaxWatts 最大功率（瓦）
	MaxWatts float64 `json:"maxWatts"`
	// Voltage 电压
	Voltage float64 `json:"voltage"`
	// Current 电流
	Current float64 `json:"current"`
}

// PowerStatus 电源状态.
type PowerStatus string

const (
	// PowerOn 开启.
	PowerOn PowerStatus = "on"
	// PowerOff 关闭.
	PowerOff PowerStatus = "off"
	// PowerFault 故障.
	PowerFault PowerStatus = "fault"
	// PowerStandby 待机.
	PowerStandby PowerStatus = "standby"
)

// Fan 风扇.
type Fan struct {
	// ID 风扇编号
	ID int `json:"id"`
	// Name 风扇名称
	Name string `json:"name"`
	// RPM 当前转速
	RPM int `json:"rpm"`
	// MaxRPM 最大转速
	MaxRPM int `json:"maxRPM"`
	// Status 风扇状态
	Status FanStatus `json:"status"`
}

// FanStatus 风扇状态.
type FanStatus string

const (
	// FanNormal 正常.
	FanNormal FanStatus = "normal"
	// FanSlow 转速过低.
	FanSlow FanStatus = "slow"
	// FanFailed 故障.
	FanFailed FanStatus = "failed"
)

// EnclosureTopology 机箱拓扑（用于可视化）.
type EnclosureTopology struct {
	// Enclosures 机箱列表
	Enclosures []*Enclosure `json:"enclosures"`
	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generatedAt"`
}

// LEDControlRequest LED 控制请求.
type LEDControlRequest struct {
	// SlotID 槽位编号
	SlotID int `json:"slotId"`
	// LEDType 灯类型
	LEDType LEDType `json:"ledType"`
	// State 目标状态
	State LEDState `json:"state"`
}

// PowerControlRequest 电源控制请求.
type PowerControlRequest struct {
	// PowerSupplyID 电源编号
	PowerSupplyID int `json:"powerSupplyId"`
	// Action 操作（on/off/standby）
	Action PowerStatus `json:"action"`
}
