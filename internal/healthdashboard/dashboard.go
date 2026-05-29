// Package healthdashboard provides a unified storage health monitoring dashboard.
// It aggregates disk health, pool status, SMART data, and capacity trends
// into a single view. Inspired by Synology Storage Manager and TrueNAS Dashboard.
package healthdashboard

import (
	"fmt"
	"sync"
	"time"
)

// PoolStatus represents the health state of a storage pool.
type PoolStatus string

const (
	PoolHealthy  PoolStatus = "healthy"
	PoolDegraded PoolStatus = "degraded"
	PoolFaulted  PoolStatus = "faulted"
	PoolUnknown  PoolStatus = "unknown"
)

// DiskRole indicates the disk's role in the pool.
type DiskRole string

const (
	DiskData    DiskRole = "data"
	DiskSpare   DiskRole = "spare"
	DiskCache   DiskRole = "cache"
	DiskLog     DiskRole = "log"
	DiskUnknown DiskRole = "unknown"
)

// DiskHealth represents SMART and operational health for a single disk.
type DiskHealth struct {
	Device       string    `json:"device"`        // e.g., "/dev/sda"
	Model        string    `json:"model"`
	Serial       string    `json:"serial"`
	CapacityGB   int64     `json:"capacity_gb"`
	Temperature  int       `json:"temperature_c"` // Celsius
	PowerOnHours int64     `json:"power_on_hours"`
	HealthScore  int       `json:"health_score"`  // 0-100
	Reallocated  int64     `json:"reallocated_sectors"`
	Pending      int64     `json:"pending_sectors"`
	Uncorrectable int64    `json:"uncorrectable_errors"`
	Role         DiskRole  `json:"role"`
	PoolID       string    `json:"pool_id"`
	LastCheck    time.Time `json:"last_check"`
	Warnings     []string  `json:"warnings,omitempty"`
}

// PoolHealth represents the health of a storage pool.
type PoolHealth struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Status       PoolStatus  `json:"status"`
	RAIDLevel    string      `json:"raid_level"`
	TotalGB      int64       `json:"total_gb"`
	UsedGB       int64       `json:"used_gb"`
	FreeGB       int64       `json:"free_gb"`
	UsagePercent float64     `json:"usage_percent"`
	DiskCount    int         `json:"disk_count"`
	HealthyDisks int         `json:"healthy_disks"`
	DegradedDisks int        `json:"degraded_disks"`
	FailedDisks  int         `json:"failed_disks"`
	LastScrub    time.Time   `json:"last_scrub"`
	ScrubStatus  string      `json:"scrub_status"`
}

// CapacityTrend tracks storage usage over time for trend analysis.
type CapacityTrend struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalGB      int64     `json:"total_gb"`
	UsedGB       int64     `json:"used_gb"`
	GrowthRateGB float64   `json:"growth_rate_gb_per_day"`
	DaysUntilFull int      `json:"days_until_full"` // -1 if not applicable
}

// Dashboard aggregates all health information.
type Dashboard struct {
	Pools      []*PoolHealth   `json:"pools"`
	Disks      []*DiskHealth   `json:"disks"`
	Trends     []*CapacityTrend `json:"trends"`
	OverallScore int           `json:"overall_score"` // 0-100
	AlertCount   int           `json:"alert_count"`
	LastUpdate   time.Time     `json:"last_update"`
}

// Collector gathers health data from various subsystems.
type Collector struct {
	mu     sync.RWMutex
	pools  map[string]*PoolHealth
	disks  map[string]*DiskHealth
	trends []*CapacityTrend
}

// NewCollector creates a new health dashboard collector.
func NewCollector() *Collector {
	return &Collector{
		pools:  make(map[string]*PoolHealth),
		disks:  make(map[string]*DiskHealth),
		trends: make([]*CapacityTrend, 0),
	}
}

// UpdatePool updates or inserts pool health data.
func (c *Collector) UpdatePool(pool PoolHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pools[pool.ID] = &pool
}

// UpdateDisk updates or inserts disk health data.
func (c *Collector) UpdateDisk(disk DiskHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Auto-generate warnings
	if disk.Temperature > 55 {
		disk.Warnings = append(disk.Warnings, fmt.Sprintf("High temperature: %d°C", disk.Temperature))
	}
	if disk.Reallocated > 0 {
		disk.Warnings = append(disk.Warnings, fmt.Sprintf("Reallocated sectors: %d", disk.Reallocated))
	}
	if disk.Pending > 0 {
		disk.Warnings = append(disk.Warnings, fmt.Sprintf("Pending sectors: %d", disk.Pending))
	}
	if disk.HealthScore < 50 {
		disk.Warnings = append(disk.Warnings, "Critical health score - replace recommended")
	}
	disk.LastCheck = time.Now()

	c.disks[disk.Device] = &disk
}

// AddTrend records a capacity data point.
func (c *Collector) AddTrend(totalGB, usedGB int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	trend := &CapacityTrend{
		Timestamp: time.Now(),
		TotalGB:   totalGB,
		UsedGB:    usedGB,
	}

	// Calculate growth rate from last 2 data points
	if len(c.trends) > 0 {
		last := c.trends[len(c.trends)-1]
		days := trend.Timestamp.Sub(last.Timestamp).Hours() / 24
		if days > 0 {
			trend.GrowthRateGB = float64(trend.UsedGB-last.UsedGB) / days
			if trend.GrowthRateGB > 0 {
				freeGB := float64(trend.TotalGB - trend.UsedGB)
				trend.DaysUntilFull = int(freeGB / trend.GrowthRateGB)
			}
		}
	}

	c.trends = append(c.trends, trend)
	// Keep last 90 days of data
	if len(c.trends) > 90 {
		c.trends = c.trends[len(c.trends)-90:]
	}
}

// GetDashboard builds the aggregated dashboard view.
func (c *Collector) GetDashboard() *Dashboard {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dash := &Dashboard{
		LastUpdate: time.Now(),
	}

	var totalScore, diskCount int

	for _, p := range c.pools {
		dash.Pools = append(dash.Pools, p)
		if p.Status == PoolDegraded {
			dash.AlertCount++
		} else if p.Status == PoolFaulted {
			dash.AlertCount += 3
		}
	}

	for _, d := range c.disks {
		dash.Disks = append(dash.Disks, d)
		totalScore += d.HealthScore
		diskCount++
		dash.AlertCount += len(d.Warnings)
	}

	dash.Trends = c.trends

	if diskCount > 0 {
		dash.OverallScore = totalScore / diskCount
	}

	return dash
}

// GetWarnings returns all active warnings across disks and pools.
func (c *Collector) GetWarnings() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var warnings []string
	for _, d := range c.disks {
		for _, w := range d.Warnings {
			warnings = append(warnings, fmt.Sprintf("[%s] %s", d.Device, w))
		}
	}
	for _, p := range c.pools {
		if p.Status != PoolHealthy {
			warnings = append(warnings, fmt.Sprintf("[Pool:%s] Status: %s", p.Name, p.Status))
		}
		if p.UsagePercent > 90 {
			warnings = append(warnings, fmt.Sprintf("[Pool:%s] Usage critical: %.1f%%", p.Name, p.UsagePercent))
		}
	}
	return warnings
}

// GetHealthScore calculates the overall health score.
func (c *Collector) GetHealthScore() *HealthScore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var storageScore, cpuScore, memoryScore, networkScore int
	var diskCount int

	// Calculate storage score from disk health
	for _, d := range c.disks {
		storageScore += d.HealthScore
		diskCount++
	}
	if diskCount > 0 {
		storageScore = storageScore / diskCount
	}

	// Calculate pool health impact
	for _, p := range c.pools {
		switch p.Status {
		case PoolDegraded:
			storageScore = storageScore * 80 / 100
		case PoolFaulted:
			storageScore = storageScore * 50 / 100
		}
	}

	// Get CPU, memory, network scores from system metrics
	cpuScore = 85 // Placeholder
	memoryScore = 90 // Placeholder
	networkScore = 95 // Placeholder

	overall := (storageScore + cpuScore + memoryScore + networkScore) / 4

	return &HealthScore{
		Overall:   overall,
		Storage:   storageScore,
		CPU:       cpuScore,
		Memory:    memoryScore,
		Network:   networkScore,
		UpdatedAt: time.Now(),
		Details: map[string]interface{}{
			"disk_count":  diskCount,
			"pool_count":  len(c.pools),
			"alert_count": c.getAlertCount(),
		},
	}
}

// GetRealTimeMetrics returns current system metrics.
func (c *Collector) GetRealTimeMetrics() []*HealthMetric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]*HealthMetric, 0)

	// Add storage metrics
	for _, d := range c.disks {
		status := "normal"
		if d.HealthScore < 50 {
			status = "critical"
		} else if d.HealthScore < 70 {
			status = "warning"
		}

		metrics = append(metrics, &HealthMetric{
			Name:      "disk_health_" + d.Device,
			Value:     float64(d.HealthScore),
			Unit:      "score",
			Status:    status,
			Threshold: 70,
			Timestamp: d.LastCheck,
		})

		metrics = append(metrics, &HealthMetric{
			Name:      "disk_temp_" + d.Device,
			Value:     float64(d.Temperature),
			Unit:      "celsius",
			Status:    getTemperatureStatus(d.Temperature),
			Threshold: 55,
			Timestamp: d.LastCheck,
		})
	}

	// Add pool metrics
	for _, p := range c.pools {
		metrics = append(metrics, &HealthMetric{
			Name:      "pool_usage_" + p.ID,
			Value:     p.UsagePercent,
			Unit:      "percent",
			Status:    getUsageStatus(p.UsagePercent),
			Threshold: 90,
			Timestamp: time.Now(),
		})
	}

	return metrics
}

// GetTrends returns trend data for the specified period.
func (c *Collector) GetTrends(period string) []*TrendSeries {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	default:
		days = 7
	}

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -days)

	// Build storage usage trend
	storageTrend := &TrendSeries{
		Metric: "storage_usage",
		Unit:   "GB",
		Period: period,
		Points: make([]*TrendDataPoint, 0),
	}

	for _, t := range c.trends {
		if t.Timestamp.After(cutoff) {
			storageTrend.Points = append(storageTrend.Points, &TrendDataPoint{
				Timestamp: t.Timestamp,
				Value:     float64(t.UsedGB),
			})
		}
	}

	return []*TrendSeries{storageTrend}
}

func (c *Collector) getAlertCount() int {
	count := 0
	for _, d := range c.disks {
		count += len(d.Warnings)
	}
	for _, p := range c.pools {
		if p.Status != PoolHealthy {
			count++
		}
	}
	return count
}

func getTemperatureStatus(temp int) string {
	if temp > 60 {
		return "critical"
	}
	if temp > 55 {
		return "warning"
	}
	return "normal"
}

func getUsageStatus(percent float64) string {
	if percent > 95 {
		return "critical"
	}
	if percent > 90 {
		return "warning"
	}
	return "normal"
}

// PredictCapacity forecasts when a pool will be full based on current trends.
func (c *Collector) PredictCapacity(poolID string, days int) (projectedGB int64, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pool, ok := c.pools[poolID]
	if !ok {
		return 0, fmt.Errorf("pool %s not found", poolID)
	}

	if len(c.trends) < 2 {
		return pool.UsedGB, nil // Not enough data for prediction
	}

	// Use last 7 trends for prediction
	sampleSize := 7
	if len(c.trends) < sampleSize {
		sampleSize = len(c.trends)
	}

	recent := c.trends[len(c.trends)-sampleSize:]
	var totalGrowth float64
	for i := 1; i < len(recent); i++ {
		totalGrowth += float64(recent[i].UsedGB-recent[i-1].UsedGB)
	}
	avgDailyGrowth := totalGrowth / float64(len(recent)-1)

	projectedGB = pool.UsedGB + int64(avgDailyGrowth*float64(days))
	if projectedGB > pool.TotalGB {
		projectedGB = pool.TotalGB
	}
	if projectedGB < 0 {
		projectedGB = 0
	}

	return projectedGB, nil
}
