// Package disklifecycle provides disk lifecycle management for NAS-OS.
// Features:
//   - S.M.A.R.T. health monitoring and trend analysis
//   - Predictive failure detection with ML-based scoring
//   - Automated disk retirement recommendations
//   - Warranty tracking and replacement scheduling
//   - Performance degradation monitoring
//   - Disk fleet management and reporting
package disklifecycle

import (
	"sync"
	"time"
)

// Manager is the central disk lifecycle manager.
type Manager struct {
	mu       sync.RWMutex
	config   Config
	disks    map[string]*Disk
	alerts   []*Alert
	events   []*Event
	predictor *Predictor
	logger   Logger
	stopCh   chan struct{}
}

// Logger interface for lifecycle logging.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// Config holds disk lifecycle configuration.
type Config struct {
	Enabled            bool    `json:"enabled"`
	ScanIntervalHours  int     `json:"scanIntervalHours"`
	HealthThreshold    float64 `json:"healthThreshold"`    // 0-100, alert below this
	RetireThreshold    float64 `json:"retireThreshold"`    // 0-100, recommend retire below this
	TrendWindowDays    int     `json:"trendWindowDays"`
	MaxAlerts          int     `json:"maxAlerts"`
	EnablePrediction   bool    `json:"enablePrediction"`
	PredictionModel    string  `json:"predictionModel"`    // simple, linear, ml
	StoragePath        string  `json:"storagePath"`
}

// Disk represents a managed disk.
type Disk struct {
	ID             string         `json:"id"`
	Device         string         `json:"device"`         // e.g., /dev/sda
	Serial         string         `json:"serial"`
	Model          string         `json:"model"`
	Vendor         string         `json:"vendor"`
	CapacityBytes  int64          `json:"capacityBytes"`
	Interface      string         `json:"interface"`      // SATA, SAS, NVMe
	FormFactor     string         `json:"formFactor"`     // 2.5", 3.5"
	SmartStatus    SmartStatus    `json:"smartStatus"`
	HealthScore    float64        `json:"healthScore"`    // 0-100
	TrendScore     float64        `json:"trendScore"`     // -100 to +100 (negative = declining)
	PredictedLife  int            `json:"predictedLifeDays"`
	Status         DiskStatus     `json:"status"`
	InstallDate    time.Time      `json:"installDate"`
	WarrantyExpiry time.Time      `json:"warrantyExpiry"`
	LastScan       time.Time      `json:"lastScan"`
	TotalBytesRead int64          `json:"totalBytesRead"`
	TotalBytesWrite int64         `json:"totalBytesWrite"`
	PowerOnHours   int64          `json:"powerOnHours"`
	Temperature    float64        `json:"temperature"`
	ReallocatedSectors int64      `json:"reallocatedSectors"`
	PendingSectors int64          `json:"pendingSectors"`
	HealthHistory  []HealthSample `json:"healthHistory,omitempty"`
	Tags           []string       `json:"tags"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// DiskStatus represents the lifecycle status of a disk.
type DiskStatus string

const (
	StatusHealthy     DiskStatus = "healthy"
	StatusWarning     DiskStatus = "warning"
	StatusCritical    DiskStatus = "critical"
	StatusFailed      DiskStatus = "failed"
	StatusRetired     DiskStatus = "retired"
	StatusReplaced    DiskStatus = "replaced"
	StatusUnknown     DiskStatus = "unknown"
)

// SmartStatus represents S.M.A.R.T. health status.
type SmartStatus string

const (
	SmartPassed  SmartStatus = "passed"
	SmartFailed  SmartStatus = "failed"
	SmartUnknown SmartStatus = "unknown"
)

// HealthSample represents a point-in-time health measurement.
type HealthSample struct {
	Timestamp       time.Time `json:"timestamp"`
	HealthScore     float64   `json:"healthScore"`
	Temperature     float64   `json:"temperature"`
	ReallocatedSectors int64  `json:"reallocatedSectors"`
	PendingSectors  int64     `json:"pendingSectors"`
	SeekErrorRate   float64   `json:"seekErrorRate"`
	SpinRetryCount  int       `json:"spinRetryCount"`
}

// Alert represents a disk health alert.
type Alert struct {
	ID          string      `json:"id"`
	DiskID      string      `json:"diskId"`
	Device      string      `json:"device"`
	Type        AlertType   `json:"type"`
	Severity    Severity    `json:"severity"`
	Title       string      `json:"title"`
	Message     string      `json:"message"`
	Value       float64     `json:"value,omitempty"`
	Threshold   float64     `json:"threshold,omitempty"`
	Dismissed   bool        `json:"dismissed"`
	CreatedAt   time.Time   `json:"createdAt"`
	DismissedAt *time.Time  `json:"dismissedAt,omitempty"`
}

// AlertType represents the type of disk alert.
type AlertType string

const (
	AlertHealthDecline     AlertType = "health_decline"
	AlertTemperatureHigh   AlertType = "temperature_high"
	AlertReallocatedSectors AlertType = "reallocated_sectors"
	AlertPendingSectors    AlertType = "pending_sectors"
	AlertSmartFailure      AlertType = "smart_failure"
	AlertWarrantyExpiring  AlertType = "warranty_expiring"
	AlertPredictedFailure  AlertType = "predicted_failure"
	AlertPerformanceDegradation AlertType = "performance_degradation"
)

// Severity represents alert severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
	SeverityEmergency Severity = "emergency"
)

// Event represents a disk lifecycle event.
type Event struct {
	ID        string    `json:"id"`
	DiskID    string    `json:"diskId"`
	Type      EventType `json:"type"`
	Message   string    `json:"message"`
	OldValue  string    `json:"oldValue,omitempty"`
	NewValue  string    `json:"newValue,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// EventType represents the type of lifecycle event.
type EventType string

const (
	EventDiskAdded      EventType = "disk_added"
	EventDiskRemoved    EventType = "disk_removed"
	EventStatusChange   EventType = "status_change"
	EventHealthAlert    EventType = "health_alert"
	EventScanCompleted  EventType = "scan_completed"
	EventRetired        EventType = "disk_retired"
	EventReplaced       EventType = "disk_replaced"
	EventWarrantyExpiry EventType = "warranty_expiry"
)

// Predictor provides disk failure prediction.
type Predictor struct {
	model    string
	window   int
}

// PredictionResult represents a failure prediction.
type PredictionResult struct {
	DiskID           string    `json:"diskId"`
	FailureProb      float64   `json:"failureProbability"` // 0-1
	PredictedDays    int       `json:"predictedDaysLeft"`
	Confidence       float64   `json:"confidence"`         // 0-1
	Factors          []string  `json:"factors"`
	Recommendation   string    `json:"recommendation"`
	GeneratedAt      time.Time `json:"generatedAt"`
}

// FleetSummary represents a summary of all managed disks.
type FleetSummary struct {
	TotalDisks      int              `json:"totalDisks"`
	HealthyDisks    int              `json:"healthyDisks"`
	WarningDisks    int              `json:"warningDisks"`
	CriticalDisks   int              `json:"criticalDisks"`
	FailedDisks     int              `json:"failedDisks"`
	RetiredDisks    int              `json:"retiredDisks"`
	AvgHealthScore  float64          `json:"avgHealthScore"`
	AvgAge          float64          `json:"avgAgeDays"`
	TotalCapacity   int64            `json:"totalCapacityBytes"`
	ByInterface     map[string]int   `json:"byInterface"`
	ByVendor        map[string]int   `json:"byVendor"`
	AlertsCount     int              `json:"alertsCount"`
	PredictedFails  int              `json:"predictedFailures"`
}

// DefaultConfig returns a default disk lifecycle configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:           true,
		ScanIntervalHours: 6,
		HealthThreshold:   70.0,
		RetireThreshold:   30.0,
		TrendWindowDays:   30,
		MaxAlerts:         1000,
		EnablePrediction:  true,
		PredictionModel:   "linear",
		StoragePath:       "/var/lib/nas-os/disklifecycle",
	}
}
