// Package powerbudget 提供功耗追踪功能
package powerbudget

import (
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Tracker 功耗追踪器.
type Tracker struct {
	engine     *Engine
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewTracker 创建功耗追踪器.
func NewTracker(engine *Engine, logger *zap.Logger) *Tracker {
	return &Tracker{
		engine: engine,
		logger: logger,
	}
}

// ========== 设备画像更新 ==========

// updateDeviceProfile 更新设备功耗画像.
func (t *Tracker) updateDeviceProfile(record *PowerRecord) {
	dp, ok := t.engine.devices[record.DeviceID]
	if !ok {
		dp = &DevicePower{
			DeviceID:    record.DeviceID,
			DeviceName:  record.DeviceName,
			FirstSeen:   record.Timestamp,
			LastSeen:    record.Timestamp,
			HourlyProfile: make([]HourlyPower, 24),
		}
		t.engine.devices[record.DeviceID] = dp
	}

	dp.LastSeen = record.Timestamp
	dp.TotalEnergy += record.EnergyKWh
	dp.TotalCost += record.CostCents
	dp.RecordCount++

	if record.PowerWatts > dp.PeakPower {
		dp.PeakPower = record.PowerWatts
	}

	// 更新平均功率
	var totalPower float64
	var count int
	for _, r := range t.engine.records {
		if r.DeviceID == record.DeviceID {
			totalPower += r.PowerWatts
			count++
		}
	}
	if count > 0 {
		dp.AvgPower = totalPower / float64(count)
	}

	// 更新小时画像
	hour := record.Timestamp.Hour()
	hp := &dp.HourlyProfile[hour]
	hp.Hour = hour
	hp.RecordNum++
	hp.Energy += record.EnergyKWh
	hp.AvgPower = (hp.AvgPower*float64(hp.RecordNum-1) + record.PowerWatts) / float64(hp.RecordNum)
}

// ========== 实时监控 ==========

// GetRealtimePower 获取实时功耗（最近记录）.
func (t *Tracker) GetRealtimePower() map[string]float64 {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	result := make(map[string]float64)
	now := time.Now()
	threshold := now.Add(-5 * time.Minute)

	for _, r := range t.engine.records {
		if r.Timestamp.After(threshold) {
			result[r.DeviceID] = r.PowerWatts
		}
	}

	return result
}

// GetCurrentPower 获取当前总功率.
func (t *Tracker) GetCurrentPower() float64 {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	var total float64
	seen := make(map[string]bool)
	now := time.Now()
	threshold := now.Add(-5 * time.Minute)

	// 取每个设备最新记录
	for i := len(t.engine.records) - 1; i >= 0; i-- {
		r := t.engine.records[i]
		if r.Timestamp.After(threshold) && !seen[r.DeviceID] {
			total += r.PowerWatts
			seen[r.DeviceID] = true
		}
	}

	return total
}

// ========== 历史数据聚合 ==========

// AggregateDaily 聚合每日用电数据.
func (t *Tracker) AggregateDaily(start, end time.Time) []TrendPoint {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	dailyMap := make(map[string]*TrendPoint)

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		dateKey := r.Timestamp.Format("2006-01-02")
		if _, ok := dailyMap[dateKey]; !ok {
			t, _ := time.Parse("2006-01-02", dateKey)
			dailyMap[dateKey] = &TrendPoint{Date: t}
		}
		dailyMap[dateKey].Energy += r.EnergyKWh
		dailyMap[dateKey].Cost += r.CostCents
	}

	result := make([]TrendPoint, 0, len(dailyMap))
	for _, point := range dailyMap {
		result = append(result, *point)
	}

	sortTrendPoints(result)
	return result
}

// AggregateHourly 聚合每小时用电数据.
func (t *Tracker) AggregateHourly(date time.Time) []HourlyPower {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	hourlyMap := make(map[int]*HourlyPower)
	for i := 0; i < 24; i++ {
		hourlyMap[i] = &HourlyPower{Hour: i}
	}

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		hour := r.Timestamp.Hour()
		hp := hourlyMap[hour]
		hp.RecordNum++
		hp.Energy += r.EnergyKWh
		hp.AvgPower = (hp.AvgPower*float64(hp.RecordNum-1) + r.PowerWatts) / float64(hp.RecordNum)
	}

	result := make([]HourlyPower, 24)
	for i := 0; i < 24; i++ {
		result[i] = *hourlyMap[i]
	}

	return result
}

// AggregateByDevice 按设备聚合用电数据.
func (t *Tracker) AggregateByDevice(start, end time.Time) []DevicePower {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	deviceMap := make(map[string]*DevicePower)
	var totalEnergy float64

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		dp, ok := deviceMap[r.DeviceID]
		if !ok {
			dp = &DevicePower{
				DeviceID:   r.DeviceID,
				DeviceName: r.DeviceName,
				FirstSeen:  r.Timestamp,
				LastSeen:   r.Timestamp,
			}
			deviceMap[r.DeviceID] = dp
		}

		dp.TotalEnergy += r.EnergyKWh
		dp.TotalCost += r.CostCents
		dp.RecordCount++
		if r.Timestamp.Before(dp.FirstSeen) {
			dp.FirstSeen = r.Timestamp
		}
		if r.Timestamp.After(dp.LastSeen) {
			dp.LastSeen = r.Timestamp
		}
		totalEnergy += r.EnergyKWh
	}

	result := make([]DevicePower, 0, len(deviceMap))
	for _, dp := range deviceMap {
		if totalEnergy > 0 {
			dp.UsagePercent = dp.TotalEnergy / totalEnergy * 100.0
		}
		result = append(result, *dp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalEnergy > result[j].TotalEnergy
	})

	return result
}

// GetPeakPower 获取指定时间范围内的峰值功率.
func (t *Tracker) GetPeakPower(start, end time.Time) (float64, string) {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	var peakPower float64
	var peakDevice string

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		if r.PowerWatts > peakPower {
			peakPower = r.PowerWatts
			peakDevice = r.DeviceName
		}
	}

	return peakPower, peakDevice
}

// GetAveragePower 获取指定时间范围内的平均功率.
func (t *Tracker) GetAveragePower(start, end time.Time) float64 {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	var totalPower float64
	var count int

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		totalPower += r.PowerWatts
		count++
	}

	if count == 0 {
		return 0
	}

	return totalPower / float64(count)
}

// GetMinPower 获取指定时间范围内的最低功率.
func (t *Tracker) GetMinPower(start, end time.Time) float64 {
	t.engine.mu.RLock()
	defer t.engine.mu.RUnlock()

	minPower := math.MaxFloat64
	found := false

	for _, r := range t.engine.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		if r.PowerWatts < minPower {
			minPower = r.PowerWatts
			found = true
		}
	}

	if !found {
		return 0
	}

	return minPower
}
