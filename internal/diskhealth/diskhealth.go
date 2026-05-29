package diskhealth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DiskInfo represents disk information
type DiskInfo struct {
	Device      string    `json:"device"`
	Model       string    `json:"model"`
	Serial      string    `json:"serial"`
	Size        int64     `json:"size"`
	Temperature int       `json:"temperature"`
	PowerOnHours int64    `json:"power_on_hours"`
	HealthScore float64   `json:"health_score"` // 0-100
	PredictedFailure time.Time `json:"predicted_failure,omitempty"`
	LastCheck   time.Time `json:"last_check"`
	Status      string    `json:"status"` // healthy, warning, critical, failed
}

// SMARTAttribute represents a S.M.A.R.T attribute
type SMARTAttribute struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Threshold  int    `json:"threshold"`
	RawValue   int64  `json:"raw_value"`
	Failed     bool   `json:"failed"`
}

// HealthAlert represents a health alert
type HealthAlert struct {
	Disk      string    `json:"disk"`
	Level     string    `json:"level"` // info, warning, critical
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"`
}

// DiskHealthPredict provides predictive disk health monitoring
// Inspired by TrueNAS S.M.A.R.T monitoring
type DiskHealthPredict struct {
	mu          sync.RWMutex
	disks       map[string]*DiskInfo
	alerts      []*HealthAlert
	running     bool
	stopCh      chan struct{}
	checkInterval time.Duration
}

// NewDiskHealthPredict creates a new DiskHealthPredict instance
func NewDiskHealthPredict(checkInterval time.Duration) *DiskHealthPredict {
	if checkInterval == 0 {
		checkInterval = 1 * time.Hour
	}
	return &DiskHealthPredict{
		disks:         make(map[string]*DiskInfo),
		stopCh:        make(chan struct{}),
		checkInterval: checkInterval,
	}
}

// AddDisk adds a disk to monitor
func (dhp *DiskHealthPredict) AddDisk(info DiskInfo) {
	dhp.mu.Lock()
	defer dhp.mu.Unlock()
	info.LastCheck = time.Now()
	info.Status = "healthy"
	dhp.disks[info.Device] = &info
}

// RemoveDisk removes a disk from monitoring
func (dhp *DiskHealthPredict) RemoveDisk(device string) {
	dhp.mu.Lock()
	defer dhp.mu.Unlock()
	delete(dhp.disks, device)
}

// GetDiskHealth returns health info for a disk
func (dhp *DiskHealthPredict) GetDiskHealth(device string) (*DiskInfo, error) {
	dhp.mu.RLock()
	defer dhp.mu.RUnlock()

	disk, exists := dhp.disks[device]
	if !exists {
		return nil, fmt.Errorf("disk not found: %s", device)
	}
	return disk, nil
}

// ListDisks returns all monitored disks
func (dhp *DiskHealthPredict) ListDisks() []*DiskInfo {
	dhp.mu.RLock()
	defer dhp.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(dhp.disks))
	for _, d := range dhp.disks {
		disks = append(disks, d)
	}
	return disks
}

// GetAlerts returns all alerts
func (dhp *DiskHealthPredict) GetAlerts(unackedOnly bool) []*HealthAlert {
	dhp.mu.RLock()
	defer dhp.mu.RUnlock()

	var alerts []*HealthAlert
	for _, a := range dhp.alerts {
		if unackedOnly && a.Acked {
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts
}

// AckAlert acknowledges an alert
func (dhp *DiskHealthPredict) AckAlert(index int) error {
	dhp.mu.Lock()
	defer dhp.mu.Unlock()

	if index < 0 || index >= len(dhp.alerts) {
		return fmt.Errorf("invalid alert index")
	}
	dhp.alerts[index].Acked = true
	return nil
}

// PredictFailure predicts potential disk failure
func (dhp *DiskHealthPredict) PredictFailure(device string) (*time.Time, error) {
	dhp.mu.RLock()
	defer dhp.mu.RUnlock()

	disk, exists := dhp.disks[device]
	if !exists {
		return nil, fmt.Errorf("disk not found: %s", device)
	}

	if disk.HealthScore < 50 {
		// Predict failure within 30 days
		failure := time.Now().Add(30 * 24 * time.Hour)
		return &failure, nil
	}
	return nil, nil
}

// Start begins health monitoring
func (dhp *DiskHealthPredict) Start(ctx context.Context) error {
	dhp.mu.Lock()
	if dhp.running {
		dhp.mu.Unlock()
		return fmt.Errorf("already running")
	}
	dhp.running = true
	dhp.mu.Unlock()

	go dhp.monitorLoop(ctx)
	return nil
}

// Stop stops health monitoring
func (dhp *DiskHealthPredict) Stop() {
	dhp.mu.Lock()
	defer dhp.mu.Unlock()
	if dhp.running {
		close(dhp.stopCh)
		dhp.running = false
	}
}

// GetStats returns monitoring statistics
func (dhp *DiskHealthPredict) GetStats() map[string]interface{} {
	dhp.mu.RLock()
	defer dhp.mu.RUnlock()

	healthy := 0
	warning := 0
	critical := 0
	for _, d := range dhp.disks {
		switch d.Status {
		case "healthy":
			healthy++
		case "warning":
			warning++
		case "critical":
			critical++
		}
	}

	return map[string]interface{}{
		"total_disks": len(dhp.disks),
		"healthy":     healthy,
		"warning":     warning,
		"critical":    critical,
		"alerts":      len(dhp.alerts),
	}
}

func (dhp *DiskHealthPredict) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(dhp.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dhp.stopCh:
			return
		case <-ticker.C:
			dhp.checkAllDisks(ctx)
		}
	}
}

func (dhp *DiskHealthPredict) checkAllDisks(ctx context.Context) {
	dhp.mu.RLock()
	disks := make([]*DiskInfo, 0, len(dhp.disks))
	for _, d := range dhp.disks {
		disks = append(disks, d)
	}
	dhp.mu.RUnlock()

	for _, disk := range disks {
		select {
		case <-ctx.Done():
			return
		default:
		}

		dhp.checkDisk(disk)
	}
}

func (dhp *DiskHealthPredict) checkDisk(disk *DiskInfo) {
	dhp.mu.Lock()
	defer dhp.mu.Unlock()

	disk.LastCheck = time.Now()

	// Simulate health check
	if disk.HealthScore < 30 {
		disk.Status = "critical"
		dhp.alerts = append(dhp.alerts, &HealthAlert{
			Disk:      disk.Device,
			Level:     "critical",
			Message:   fmt.Sprintf("Disk %s health critical: %.1f%%", disk.Device, disk.HealthScore),
			Timestamp: time.Now(),
		})
	} else if disk.HealthScore < 60 {
		disk.Status = "warning"
		dhp.alerts = append(dhp.alerts, &HealthAlert{
			Disk:      disk.Device,
			Level:     "warning",
			Message:   fmt.Sprintf("Disk %s health warning: %.1f%%", disk.Device, disk.HealthScore),
			Timestamp: time.Now(),
		})
	} else {
		disk.Status = "healthy"
	}
}
