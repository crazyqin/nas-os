// Package warrantytracker 保修追踪器
// 对标飞牛fnOS设备保修管理功能
package warrantytracker

import (
	"time"
)

// DeviceCategory 设备分类
type DeviceCategory string

const (
	CategoryNAS         DeviceCategory = "nas"          // NAS存储
	CategoryServer      DeviceCategory = "server"       // 服务器
	CategoryNetwork     DeviceCategory = "network"      // 网络设备
	CategoryStorage     DeviceCategory = "storage"      // 存储设备
	CategoryAccessory   DeviceCategory = "accessory"    // 配件
	CategoryOther       DeviceCategory = "other"        // 其他
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	StatusActive      DeviceStatus = "active"       // 在用
	StatusStorage     DeviceStatus = "storage"       // 库存
	StatusRepairing   DeviceStatus = "repairing"     // 维修中
	StatusRetired     DeviceStatus = "retired"       // 已退役
	StatusDisposed    DeviceStatus = "disposed"      // 已处置
)

// WarrantyStatus 保修状态
type WarrantyStatus string

const (
	WarrantyActive     WarrantyStatus = "active"      // 保修中
	WarrantyExpiring   WarrantyStatus = "expiring"    // 即将到期
	WarrantyExpired    WarrantyStatus = "expired"     // 已过期
	WarrantyExtended   WarrantyStatus = "extended"    // 延保中
)

// DocumentType 文档类型
type DocumentType string

const (
	DocInvoice     DocumentType = "invoice"      // 发票
	DocWarranty    DocumentType = "warranty"      // 保修卡
	DocPhoto       DocumentType = "photo"         // 照片
	DocReceipt     DocumentType = "receipt"       // 收据
	DocManual      DocumentType = "manual"        // 说明书
	DocOther       DocumentType = "other"         // 其他
)

// Device 设备信息
type Device struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Category        DeviceCategory `json:"category"`
	Brand           string         `json:"brand"`
	Model           string         `json:"model"`
	SerialNumber    string         `json:"serial_number"`
	Status          DeviceStatus   `json:"status"`
	PurchaseDate    time.Time      `json:"purchase_date"`
	PurchasePrice   float64        `json:"purchase_price"`
	PurchaseChannel string         `json:"purchase_channel"`
	Location        string         `json:"location,omitempty"`
	Notes           string         `json:"notes,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// Warranty 保修信息
type Warranty struct {
	ID              string         `json:"id"`
	DeviceID        string         `json:"device_id"`
	Type            string         `json:"type"`           // standard, extended, premium
	Status          WarrantyStatus `json:"status"`
	StartDate       time.Time      `json:"start_date"`
	EndDate         time.Time      `json:"end_date"`
	Provider        string         `json:"provider"`       // 保修提供方
	WarrantyNumber  string         `json:"warranty_number,omitempty"`
	CoverageDetails string         `json:"coverage_details,omitempty"`
	ReminderDays    int            `json:"reminder_days"`  // 提前提醒天数
	Notified        bool           `json:"notified"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ExtendedWarranty 延保信息
type ExtendedWarranty struct {
	ID              string    `json:"id"`
	WarrantyID      string    `json:"warranty_id"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	Cost            float64   `json:"cost"`
	Provider        string    `json:"provider"`
	Description     string    `json:"description,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// RepairRecord 维修记录
type RepairRecord struct {
	ID              string    `json:"id"`
	DeviceID        string    `json:"device_id"`
	RepairDate      time.Time `json:"repair_date"`
	FaultDesc       string    `json:"fault_desc"`
	RepairDesc      string    `json:"repair_desc,omitempty"`
	Cost            float64   `json:"cost"`
	ServiceProvider string    `json:"service_provider"`
	Technician      string    `json:"technician,omitempty"`
	Status          string    `json:"status"`           // pending, in_progress, completed, cancelled
	WarrantyClaim   bool      `json:"warranty_claim"`   // 是否保修索赔
	PartsReplaced   []string  `json:"parts_replaced,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Attachment 文档附件
type Attachment struct {
	ID          string       `json:"id"`
	DeviceID    string       `json:"device_id"`
	Type        DocumentType `json:"type"`
	Name        string       `json:"name"`
	FilePath    string       `json:"file_path"`
	FileSize    int64        `json:"file_size"`
	MimeType    string       `json:"mime_type,omitempty"`
	Description string       `json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// DepreciationConfig 折旧配置
type DepreciationConfig struct {
	Method          string  `json:"method"`            // straight_line, declining_balance, sum_of_years
	UsefulLifeYears int     `json:"useful_life_years"` // 使用年限
	ResidualRate    float64 `json:"residual_rate"`     // 残值率(0-1)
}

// AssetValuation 资产估值
type AssetValuation struct {
	DeviceID          string    `json:"device_id"`
	PurchasePrice     float64   `json:"purchase_price"`
	CurrentValue      float64   `json:"current_value"`
	DepreciationTotal float64   `json:"depreciation_total"`
	DepreciationRate  float64   `json:"depreciation_rate"` // 折旧百分比
	RepairCostTotal   float64   `json:"repair_cost_total"`
	EvaluatedAt       time.Time `json:"evaluated_at"`
}

// WarrantyStats 保修统计
type WarrantyStats struct {
	TotalDevices      int     `json:"total_devices"`
	ActiveDevices     int     `json:"active_devices"`
	TotalValue        float64 `json:"total_value"`
	CurrentValue      float64 `json:"current_value"`
	RepairCostTotal   float64 `json:"repair_cost_total"`
	WarrantyActive    int     `json:"warranty_active"`
	WarrantyExpiring  int     `json:"warranty_expiring"`
	WarrantyExpired   int     `json:"warranty_expired"`
	RepairsTotal      int     `json:"repairs_total"`
	RepairsWarranty   int     `json:"repairs_warranty"`
}

// Reminder 到期提醒
type Reminder struct {
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	WarrantyID  string    `json:"warranty_id"`
	EndDate     time.Time `json:"end_date"`
	DaysLeft    int       `json:"days_left"`
	Type        string    `json:"type"` // expiring, expired
}
