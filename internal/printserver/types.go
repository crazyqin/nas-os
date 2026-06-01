// Package printserver 提供打印服务器管理功能
// 支持 CUPS 集成、打印机管理、打印队列、打印任务调度、耗材监控等
package printserver

import "time"

// PrinterStatus 打印机状态
type PrinterStatus string

const (
	StatusIdle       PrinterStatus = "idle"
	StatusPrinting   PrinterStatus = "printing"
	StatusPaused     PrinterStatus = "paused"
	StatusError      PrinterStatus = "error"
	StatusOffline    PrinterStatus = "offline"
	StatusWarming    PrinterStatus = "warming"
	StatusProcessing PrinterStatus = "processing"
)

// PrinterType 打印机类型
type PrinterType string

const (
	TypeLaser     PrinterType = "laser"
	TypeInkjet    PrinterType = "inkjet"
	TypeThermal   PrinterType = "thermal"
	TypeDotMatrix PrinterType = "dot_matrix"
	TypeLabel     PrinterType = "label"
	Type3D        PrinterType = "3d"
)

// ConnectionType 连接类型
type ConnectionType string

const (
	ConnUSB     ConnectionType = "usb"
	ConnNetwork ConnectionType = "network"
	ConnSerial  ConnectionType = "serial"
	ConnBluetooth ConnectionType = "bluetooth"
)

// PrintQuality 打印质量
type PrintQuality string

const (
	QualityDraft   PrintQuality = "draft"
	QualityNormal  PrintQuality = "normal"
	QualityHigh    PrintQuality = "high"
	QualityBest    PrintQuality = "best"
)

// ColorMode 色彩模式
type ColorMode string

const (
	ColorBW    ColorMode = "bw"
	ColorGray  ColorMode = "grayscale"
	ColorColor ColorMode = "color"
)

// PaperSize 纸张大小
type PaperSize string

const (
	PaperA4     PaperSize = "A4"
	PaperA3     PaperSize = "A3"
	PaperA5     PaperSize = "A5"
	PaperLetter PaperSize = "Letter"
	PaperLegal  PaperSize = "Legal"
	PaperB5     PaperSize = "B5"
	PaperCustom PaperSize = "custom"
)

// DuplexMode 双面模式
type DuplexMode string

const (
	DuplexNone       DuplexMode = "none"
	DuplexLongEdge   DuplexMode = "long_edge"
	DuplexShortEdge  DuplexMode = "short_edge"
)

// PrintJobStatus 打印任务状态
type PrintJobStatus string

const (
	JobPending    PrintJobStatus = "pending"
	JobProcessing PrintJobStatus = "processing"
	JobPrinting   PrintJobStatus = "printing"
	JobCompleted  PrintJobStatus = "completed"
	JobCancelled  PrintJobStatus = "cancelled"
	JobFailed     PrintJobStatus = "failed"
	JobHeld       PrintJobStatus = "held"
)

// Printer 打印机配置
type Printer struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Type           PrinterType    `json:"type"`
	Connection     ConnectionType `json:"connection"`
	Address        string         `json:"address"`
	Driver         string         `json:"driver"`
	Status         PrinterStatus  `json:"status"`
	Location       string         `json:"location"`
	IsDefault      bool           `json:"is_default"`
	Shared         bool           `json:"shared"`
	MaxCopies      int            `json:"max_copies"`
	SupportedSizes []PaperSize    `json:"supported_sizes"`
	SupportsDuplex bool           `json:"supports_duplex"`
	SupportsColor  bool           `json:"supports_color"`
	SupportsScan   bool           `json:"supports_scan"`
	TotalPages     int64          `json:"total_pages"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// TonerLevel 耗材水平
type TonerLevel struct {
	Color    string `json:"color"`
	Current  int    `json:"current"`
	Max      int    `json:"max"`
	Percent  float64 `json:"percent"`
	Low      bool   `json:"low"`
}

// PrinterHealth 打印机健康状态
type PrinterHealth struct {
	PrinterID      string       `json:"printer_id"`
	Status         PrinterStatus `json:"status"`
	TonerLevels    []TonerLevel `json:"toner_levels"`
	DrumLife       float64      `json:"drum_life"`
	PaperLevel     float64      `json:"paper_level"`
	ErrorCount     int          `json:"error_count"`
	TotalPrints    int64        `json:"total_prints"`
	LastMaintenance time.Time   `json:"last_maintenance"`
	NextMaintenance time.Time   `json:"next_maintenance"`
	Alerts         []string     `json:"alerts,omitempty"`
}

// PrintJob 打印任务
type PrintJob struct {
	ID          string        `json:"id"`
	PrinterID   string        `json:"printer_id"`
	PrinterName string        `json:"printer_name"`
	UserID      string        `json:"user_id"`
	UserName    string        `json:"user_name"`
	DocumentName string       `json:"document_name"`
	FilePath    string        `json:"file_path"`
	FileType    string        `json:"file_type"`
	FileSize    int64         `json:"file_size"`
	Copies      int           `json:"copies"`
	PaperSize   PaperSize     `json:"paper_size"`
	Orientation string        `json:"orientation"`
	Quality     PrintQuality  `json:"quality"`
	ColorMode   ColorMode     `json:"color_mode"`
	Duplex      DuplexMode    `json:"duplex"`
	Status      PrintJobStatus `json:"status"`
	PagesPrinted int          `json:"pages_printed"`
	TotalPages  int           `json:"total_pages"`
	ErrorMessage string       `json:"error_message,omitempty"`
	SubmittedAt time.Time     `json:"submitted_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Priority    int           `json:"priority"`
}

// PrintQueue 打印队列
type PrintQueue struct {
	PrinterID   string      `json:"printer_id"`
	PrinterName string      `json:"printer_name"`
	Jobs        []*PrintJob `json:"jobs"`
	TotalJobs   int         `json:"total_jobs"`
	ActiveJobs  int         `json:"active_jobs"`
	PendingJobs int         `json:"pending_jobs"`
}

// PrintPolicy 打印策略
type PrintPolicy struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	MaxCopies       int       `json:"max_copies"`
	AllowedSizes    []PaperSize `json:"allowed_sizes"`
	AllowColor      bool      `json:"allow_color"`
	AllowDuplex     bool      `json:"allow_duplex"`
	MaxFileSize     int64     `json:"max_file_size"`
	AllowedFileTypes []string `json:"allowed_file_types"`
	QuotaDaily      int       `json:"quota_daily"`
	QuotaMonthly    int       `json:"quota_monthly"`
	Watermark       string    `json:"watermark,omitempty"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

// PrintStats 打印统计
type PrintStats struct {
	TotalPrinters  int     `json:"total_printers"`
	OnlinePrinters int     `json:"online_printers"`
	TotalJobs      int64   `json:"total_jobs"`
	JobsToday      int     `json:"jobs_today"`
	JobsThisWeek   int     `json:"jobs_this_week"`
	JobsThisMonth  int     `json:"jobs_this_month"`
	TotalPages     int64   `json:"total_pages"`
	PagesToday     int     `json:"pages_today"`
	AvgJobTime     float64 `json:"avg_job_time"`
	ErrorRate      float64 `json:"error_rate"`
	TopPrinters    []PrinterUsage `json:"top_printers"`
	TopUsers       []UserUsage    `json:"top_users"`
}

// PrinterUsage 打印机使用统计
type PrinterUsage struct {
	PrinterID   string `json:"printer_id"`
	PrinterName string `json:"printer_name"`
	JobCount    int64  `json:"job_count"`
	PageCount   int64  `json:"page_count"`
}

// UserUsage 用户使用统计
type UserUsage struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	JobCount int64  `json:"job_count"`
	PageCount int64 `json:"page_count"`
}

// PrintTemplate 打印模板
type PrintTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Content     string    `json:"content"`
	Variables   []string  `json:"variables"`
	Preview     string    `json:"preview,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ScheduledPrint 定时打印
type ScheduledPrint struct {
	ID          string    `json:"id"`
	PrinterID   string    `json:"printer_id"`
	TemplateID  string    `json:"template_id,omitempty"`
	FilePath    string    `json:"file_path"`
	CronExpr    string    `json:"cron_expr"`
	Options     PrintOptions `json:"options"`
	Enabled     bool      `json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PrintOptions 打印选项
type PrintOptions struct {
	Copies      int          `json:"copies"`
	PaperSize   PaperSize    `json:"paper_size"`
	Orientation string       `json:"orientation"`
	Quality     PrintQuality `json:"quality"`
	ColorMode   ColorMode    `json:"color_mode"`
	Duplex      DuplexMode   `json:"duplex"`
	Scale       float64      `json:"scale"`
	Margins     Margins      `json:"margins"`
}

// Margins 页边距
type Margins struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}
