// Package assetmgr 提供IT资产管理功能
package assetmgr

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrAssetNotFound 资产不存在.
	ErrAssetNotFound = errors.New("资产不存在")
	// ErrGroupNotFound 分组不存在.
	ErrGroupNotFound = errors.New("分组不存在")
	// ErrScheduleNotFound 维护计划不存在.
	ErrScheduleNotFound = errors.New("维护计划不存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrScanInProgress 扫描正在进行.
	ErrScanInProgress = errors.New("扫描正在进行")
)

// ========== 资产状态 ==========

// AssetStatus 资产状态.
type AssetStatus string

const (
	// StatusOnline 在线.
	StatusOnline AssetStatus = "online"
	// StatusOffline 离线.
	StatusOffline AssetStatus = "offline"
	// StatusMaintenance 维护中.
	StatusMaintenance AssetStatus = "maintenance"
	// StatusDecommissioned 已退役.
	StatusDecommissioned AssetStatus = "decommissioned"
)

// ========== 资产类型 ==========

// AssetType 资产类型.
type AssetType string

const (
	// TypeServer 服务器.
	TypeServer AssetType = "server"
	// TypeSwitch 交换机.
	TypeSwitch AssetType = "switch"
	// TypeRouter 路由器.
	TypeRouter AssetType = "router"
	// TypeFirewall 防火墙.
	TypeFirewall AssetType = "firewall"
	// TypeStorage 存储设备.
	TypeStorage AssetType = "storage"
	// TypeWorkstation 工作站.
	TypeWorkstation AssetType = "workstation"
	// TypePrinter 打印机.
	TypePrinter AssetType = "printer"
	// TypeCamera 摄像头.
	TypeCamera AssetType = "camera"
	// TypeOther 其他.
	TypeOther AssetType = "other"
)

// ========== 生命周期阶段 ==========

// LifecycleStage 生命周期阶段.
type LifecycleStage string

const (
	// StageProcurement 采购中.
	StageProcurement LifecycleStage = "procurement"
	// StageDeployed 已部署.
	StageDeployed LifecycleStage = "deployed"
	// StageInUse 使用中.
	StageInUse LifecycleStage = "in_use"
	// StageAging 老化.
	StageAging LifecycleStage = "aging"
	// StageEndOfLife 生命周期结束.
	StageEndOfLife LifecycleStage = "end_of_life"
	// StageDecommissioned 已退役.
	StageDecommissioned LifecycleStage = "decommissioned"
	// StageMaintenance 维护中.
	StageMaintenance LifecycleStage = "maintenance"
)

// ========== 核心数据结构 ==========

// Asset 资产信息.
type Asset struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         AssetType         `json:"type"`
	Status       AssetStatus       `json:"status"`
	IPAddress    string            `json:"ip_address"`
	MACAddress   string            `json:"mac_address"`
	Location     string            `json:"location"`
	Department   string            `json:"department"`
	Owner        string            `json:"owner"`
	SerialNumber string            `json:"serial_number"`
	Manufacturer string            `json:"manufacturer"`
	Model        string            `json:"model"`
	PurchaseDate time.Time         `json:"purchase_date"`
	WarrantyEnd  time.Time         `json:"warranty_end"`
	Hardware     *HardwareInfo     `json:"hardware,omitempty"`
	Software     *SoftwareInfo     `json:"software,omitempty"`
	Tags         map[string]string `json:"tags"`
	Notes        string            `json:"notes"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// HardwareInfo 硬件信息.
type HardwareInfo struct {
	CPUModel     string     `json:"cpu_model"`
	CPUCores     int        `json:"cpu_cores"`
	CPUGHz       float64    `json:"cpu_ghz"`
	RAMGB        int        `json:"ram_gb"`
	RAMType      string     `json:"ram_type"`
	StorageDisks []DiskInfo `json:"storage_disks"`
	GPUModel     string     `json:"gpu_model"`
	NetworkPorts int        `json:"network_ports"`
	PowerSupply  string     `json:"power_supply"`
	ChassisType  string     `json:"chassis_type"`
}

// DiskInfo 磁盘信息.
type DiskInfo struct {
	Model     string `json:"model"`
	Type      string `json:"type"` // ssd/hdd/nvme
	SizeGB    int    `json:"size_gb"`
	Interface string `json:"interface"` // sata/sas/nvme
}

// SoftwareInfo 软件信息.
type SoftwareInfo struct {
	OSName       string        `json:"os_name"`
	OSVersion    string        `json:"os_version"`
	Hostname     string        `json:"hostname"`
	Domain       string        `json:"domain"`
	Applications []Application `json:"applications"`
	LastBootTime time.Time     `json:"last_boot_time"`
}

// Application 已安装应用.
type Application struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Vendor  string `json:"vendor"`
}

// AssetGroup 资产分组.
type AssetGroup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	AssetIDs    []string          `json:"asset_ids"`
	Tags        map[string]string `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
}

// MaintenanceSchedule 维护计划.
type MaintenanceSchedule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AssetIDs    []string `json:"asset_ids"`
	Description string   `json:"description"`
	// IntervalDays 维护间隔天数.
	IntervalDays int `json:"interval_days"`
	// LastMaintenance 上次维护时间.
	LastMaintenance time.Time `json:"last_maintenance"`
	// NextMaintenance 下次维护时间.
	NextMaintenance time.Time `json:"next_maintenance"`
	AssignedTo      string    `json:"assigned_to"`
	CreatedAt       time.Time `json:"created_at"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	ID         string    `json:"id"`
	ScanRange  string    `json:"scan_range"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	TotalFound int       `json:"total_found"`
	NewDevices int       `json:"new_devices"`
	Assets     []*Asset  `json:"assets"`
	Status     string    `json:"status"` // running/completed/failed
}
