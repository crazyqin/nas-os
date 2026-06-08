// Package massdeploy 提供 NAS 设备批量部署与资产管理功能
// 支持设备发现、模板化配置推送、固件管理、资产追踪和费用统计
package massdeploy

import (
	"time"
)

// ==================== 设备发现 ====================

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceStatusOnline     DeviceStatus = "online"     // 在线
	DeviceStatusOffline    DeviceStatus = "offline"     // 离线
	DeviceStatusProvision  DeviceStatus = "provisioning" // 部署中
	DeviceStatusReady      DeviceStatus = "ready"       // 就绪
	DeviceStatusError      DeviceStatus = "error"       // 错误
	DeviceStatusMaintenance DeviceStatus = "maintenance" // 维护中
)

// DiscoveredDevice 网络扫描发现的设备
type DiscoveredDevice struct {
	ID         string       `json:"id"`
	IP         string       `json:"ip"`
	Hostname   string       `json:"hostname"`
	MACAddress string       `json:"mac_address"`
	Model      string       `json:"model"`
	Serial     string       `json:"serial"`
	Firmware   string       `json:"firmware"`
	Status     DeviceStatus `json:"status"`
	DiscoveredAt time.Time  `json:"discovered_at"`
}

// ScanRequest 扫描请求
type ScanRequest struct {
	Subnet    string        `json:"subnet"`    // e.g. "192.168.1.0/24"
	PortRange string        `json:"port_range"` // e.g. "5000-5001"
	Timeout   time.Duration `json:"timeout"`
}

// ScanResult 扫描结果
type ScanResult struct {
	Subnet     string             `json:"subnet"`
	Devices    []*DiscoveredDevice `json:"devices"`
	Total      int                `json:"total"`
	ScanTime   time.Duration      `json:"scan_time"`
	ScannedAt  time.Time          `json:"scanned_at"`
}

// ==================== 资产管理 ====================

// AssetType 资产类型
type AssetType string

const (
	AssetTypeNAS    AssetType = "nas"
	AssetTypeDisk   AssetType = "disk"
	AssetTypeSwitch AssetType = "switch"
	AssetTypeUPS    AssetType = "ups"
)

// AssetStatus 资产状态
type AssetStatus string

const (
	AssetStatusActive      AssetStatus = "active"
	AssetStatusInactive    AssetStatus = "inactive"
	AssetStatusRetired     AssetStatus = "retired"
	AssetStatusMaintenance AssetStatus = "maintenance"
)

// Asset 资产设备
type Asset struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Type            AssetType   `json:"type"`
	Status          AssetStatus `json:"status"`
	IPAddress       string      `json:"ip_address"`
	MACAddress      string      `json:"mac_address"`
	Hostname        string      `json:"hostname"`
	Model           string      `json:"model"`
	SerialNumber    string      `json:"serial_number"`
	FirmwareVersion string      `json:"firmware_version"`
	Location        string      `json:"location"`
	Rack            string      `json:"rack"`
	RU              int         `json:"ru"`         // 机架位置
	CPUCores        int         `json:"cpu_cores"`
	MemoryGB        int         `json:"memory_gb"`
	DiskSlots       int         `json:"disk_slots"`
	InstalledDisks  int         `json:"installed_disks"`
	TotalStorageGB  int64       `json:"total_storage_gb"`
	PurchaseDate    time.Time   `json:"purchase_date"`
	WarrantyEnd     time.Time   `json:"warranty_end"`
	PurchaseCost    float64     `json:"purchase_cost"`
	Notes           string      `json:"notes"`
	Tags            []string    `json:"tags"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// HardwareInfo 硬件详情
type HardwareInfo struct {
	AssetID        string    `json:"asset_id"`
	CPUModel       string    `json:"cpu_model"`
	CPUCores       int       `json:"cpu_cores"`
	CPUFrequency   float64   `json:"cpu_frequency_ghz"`
	MemoryTotalGB  float64   `json:"memory_total_gb"`
	MemoryType     string    `json:"memory_type"`
	DiskSlots      int       `json:"disk_slots"`
	InstalledDisks []DiskInfo `json:"installed_disks"`
	NetworkPorts   int       `json:"network_ports"`
	USBB           int       `json:"usb_ports"`
	PowerSupply    string    `json:"power_supply"`
	FanCount       int       `json:"fan_count"`
	Temperature    float64   `json:"temperature"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Slot        int    `json:"slot"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
	CapacityGB  int64  `json:"capacity_gb"`
	Type        string `json:"type"` // HDD, SSD, NVMe
	RPM         int    `json:"rpm"`
	Health      string `json:"health"`
	Temperature int    `json:"temperature"`
}

// ==================== 部署模板 ====================

// ConfigTemplate 配置模板
type ConfigTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Config      map[string]string `json:"config"`       // 键值对配置
	Scripts     []string          `json:"scripts"`      // 部署脚本
	TargetOS    string            `json:"target_os"`     // 目标系统
	MinVersion  string            `json:"min_version"`   // 最低固件版本
	Variables   map[string]string `json:"variables"`     // 模板变量
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ==================== 固件管理 ====================

// FirmwareStatus 固件状态
type FirmwareStatus string

const (
	FirmwareStatusStable   FirmwareStatus = "stable"
	FirmwareStatusBeta     FirmwareStatus = "beta"
	FirmwareStatusCritical FirmwareStatus = "critical" // 安全更新
)

// FirmwareInfo 固件信息
type FirmwareInfo struct {
	ID           string         `json:"id"`
	Version      string         `json:"version"`
	Model        string         `json:"model"`
	Status       FirmwareStatus `json:"status"`
	ReleaseDate  time.Time      `json:"release_date"`
	DownloadURL  string         `json:"download_url"`
	Checksum     string         `json:"checksum"`
	SizeBytes    int64          `json:"size_bytes"`
	ReleaseNotes string         `json:"release_notes"`
	CreatedAt    time.Time      `json:"created_at"`
}

// FirmwareUpgradeJob 固件升级任务
type FirmwareUpgradeJob struct {
	ID            string     `json:"id"`
	Version       string     `json:"version"`
	TargetDevices []string   `json:"target_devices"`
	Status        JobStatus  `json:"status"`
	Progress      int        `json:"progress"` // 0-100
	Results       map[string]string `json:"results"` // deviceID -> "success"/error
	RollbackPlan  string     `json:"rollback_plan"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   time.Time  `json:"completed_at"`
}

// ==================== 部署任务 ====================

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusRetrying  JobStatus = "retrying"
)

// DeployJob 部署任务
type DeployJob struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	TemplateID    string            `json:"template_id"`
	TargetDevices []string          `json:"target_devices"`
	Status        JobStatus         `json:"status"`
	Progress      int               `json:"progress"` // 0-100
	TotalDevices  int               `json:"total_devices"`
	SuccessCount  int               `json:"success_count"`
	FailCount     int               `json:"fail_count"`
	RetryCount    int               `json:"retry_count"`
	MaxRetries    int               `json:"max_retries"`
	Results       map[string]*DeployResult `json:"results"`
	Config        map[string]string `json:"config"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     time.Time         `json:"started_at"`
	CompletedAt   time.Time         `json:"completed_at"`
}

// DeployResult 部署结果
type DeployResult struct {
	DeviceID  string    `json:"device_id"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Retries   int       `json:"retries"`
}

// ==================== 费用统计 ====================

// CostType 费用类型
type CostType string

const (
	CostTypePurchase   CostType = "purchase"   // 采购
	CostTypeMaintenance CostType = "maintenance" // 维护
	CostTypePower      CostType = "power"       // 电费
	CostTypeNetwork    CostType = "network"     // 网络
	CostTypeLicense    CostType = "license"     // 许可
)

// CostRecord 费用记录
type CostRecord struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	Type        CostType  `json:"type"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

// DepreciationInfo 折旧信息
type DepreciationInfo struct {
	AssetID          string    `json:"asset_id"`
	PurchaseCost     float64   `json:"purchase_cost"`
	CurrentValue     float64   `json:"current_value"`
	DepreciationRate float64   `json:"depreciation_rate"` // 年折旧率
	DepreciationDate time.Time `json:"depreciation_date"` // 折旧计算截止日
	UsefulLifeYears  int       `json:"useful_life_years"`
	ElapsedYears     float64   `json:"elapsed_years"`
}

// CostSummary 费用汇总
type CostSummary struct {
	TotalPurchase    float64            `json:"total_purchase"`
	TotalMaintenance float64            `json:"total_maintenance"`
	TotalPower       float64            `json:"total_power"`
	TotalNetwork     float64            `json:"total_network"`
	TotalLicense     float64            `json:"total_license"`
	GrandTotal       float64            `json:"grand_total"`
	ByAsset          map[string]float64 `json:"by_asset"`
	ByType           map[string]float64 `json:"by_type"`
	Period           string             `json:"period"`
	CalculatedAt     time.Time          `json:"calculated_at"`
}

// ==================== 报告 ====================

// ReportType 报告类型
type ReportType string

const (
	ReportTypeDeploy  ReportType = "deploy"
	ReportTypeAsset   ReportType = "asset"
	ReportTypeCost    ReportType = "cost"
	ReportTypeFirmware ReportType = "firmware"
)

// Report 报告
type Report struct {
	ID        string     `json:"id"`
	Type      ReportType `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	GeneratedAt time.Time `json:"generated_at"`
	Period    string     `json:"period"`
	Summary   string     `json:"summary"`
}

// ==================== 统计信息 ====================

// Stats 批量部署统计
type Stats struct {
	TotalAssets      int     `json:"total_assets"`
	ActiveAssets     int     `json:"active_assets"`
	InactiveAssets   int     `json:"inactive_assets"`
	TotalDeployJobs  int     `json:"total_deploy_jobs"`
	RunningJobs      int     `json:"running_jobs"`
	CompletedJobs    int     `json:"completed_jobs"`
	FailedJobs       int     `json:"failed_jobs"`
	TotalFirmwareOps int     `json:"total_firmware_ops"`
	TotalCost        float64 `json:"total_cost"`
}
