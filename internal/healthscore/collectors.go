package healthscore

import (
	"math"
	"runtime"
	"time"
)

// DefaultCollectors provides default health data collectors.
type DefaultCollectors struct {
	hs *HealthScore
}

// NewDefaultCollectors creates default collectors.
func NewDefaultCollectors(hs *HealthScore) *DefaultCollectors {
	return &DefaultCollectors{hs: hs}
}

// RegisterDefaultCollectors registers all default collectors.
func (dc *DefaultCollectors) RegisterDefaultCollectors() {
	dc.hs.RegisterCollector(ComponentCPU, dc.collectCPU)
	dc.hs.RegisterCollector(ComponentMemory, dc.collectMemory)
	dc.hs.RegisterCollector(ComponentDisk, dc.collectDisk)
	dc.hs.RegisterCollector(ComponentNetwork, dc.collectNetwork)
	dc.hs.RegisterCollector(ComponentTemperature, dc.collectTemperature)
	dc.hs.RegisterCollector(ComponentService, dc.collectServices)
	dc.hs.RegisterCollector(ComponentRAID, dc.collectRAID)
}

// collectCPU collects CPU health data.
func (dc *DefaultCollectors) collectCPU() (*ComponentScore, error) {
	// Get CPU info
	numCPU := runtime.NumCPU()

	// Simulate CPU usage (in production, use system APIs)
	// For now, return a reasonable score
	cpuUsage := 30.0 // Placeholder

	score := 100.0 - cpuUsage
	if score < 0 {
		score = 0
	}

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "CPU 状态正常"
	if cpuUsage > 80 {
		message = "CPU 使用率较高"
	} else if cpuUsage > 60 {
		message = "CPU 使用率中等"
	}

	return &ComponentScore{
		Type:    ComponentCPU,
		Score:   score,
		Weight:  DefaultWeights[ComponentCPU],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"num_cpu":   numCPU,
			"usage_pct": cpuUsage,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectMemory collects memory health data.
func (dc *DefaultCollectors) collectMemory() (*ComponentScore, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalMem := float64(m.Sys)
	usedMem := float64(m.Alloc)

	usagePct := (usedMem / totalMem) * 100
	score := 100.0 - usagePct
	if score < 0 {
		score = 0
	}

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "内存状态正常"
	if usagePct > 90 {
		message = "内存使用率过高"
	} else if usagePct > 70 {
		message = "内存使用率较高"
	}

	return &ComponentScore{
		Type:    ComponentMemory,
		Score:   score,
		Weight:  DefaultWeights[ComponentMemory],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"total_bytes": totalMem,
			"used_bytes":  usedMem,
			"usage_pct":   usagePct,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectDisk collects disk health data.
func (dc *DefaultCollectors) collectDisk() (*ComponentScore, error) {
	// Simulate disk metrics (in production, use disk APIs)
	diskUsagePct := 65.0 // Placeholder

	score := 100.0 - diskUsagePct
	if score < 0 {
		score = 0
	}

	// Adjust score based on SMART status (simulated)
	smartScore := 95.0 // Placeholder
	score = (score + smartScore) / 2

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "磁盘状态正常"
	if diskUsagePct > 90 {
		message = "磁盘空间严重不足"
	} else if diskUsagePct > 80 {
		message = "磁盘空间不足"
	}

	return &ComponentScore{
		Type:    ComponentDisk,
		Score:   score,
		Weight:  DefaultWeights[ComponentDisk],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"usage_pct":   diskUsagePct,
			"smart_score": smartScore,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectNetwork collects network health data.
func (dc *DefaultCollectors) collectNetwork() (*ComponentScore, error) {
	// Simulate network metrics
	latencyMs := 5.0  // Placeholder
	packetLoss := 0.1 // Placeholder

	// Score based on latency and packet loss
	latencyScore := math.Max(0, 100-latencyMs*2)
	lossScore := math.Max(0, 100-packetLoss*20)
	score := (latencyScore + lossScore) / 2

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "网络状态正常"
	if latencyMs > 100 {
		message = "网络延迟过高"
	} else if packetLoss > 1 {
		message = "存在丢包现象"
	}

	return &ComponentScore{
		Type:    ComponentNetwork,
		Score:   score,
		Weight:  DefaultWeights[ComponentNetwork],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"latency_ms":  latencyMs,
			"packet_loss": packetLoss,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectTemperature collects temperature health data.
func (dc *DefaultCollectors) collectTemperature() (*ComponentScore, error) {
	// Simulate temperature
	cpuTemp := 45.0  // Placeholder
	diskTemp := 38.0 // Placeholder

	// Score based on temperature (ideal: 30-60°C)
	maxTemp := math.Max(cpuTemp, diskTemp)
	score := 100.0

	if maxTemp > 80 {
		score = 20
	} else if maxTemp > 70 {
		score = 50
	} else if maxTemp > 60 {
		score = 70
	} else if maxTemp < 20 {
		score = 80 // Too cold is also not ideal
	}

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "温度正常"
	if maxTemp > 70 {
		message = "温度过高，需要关注散热"
	} else if maxTemp < 10 {
		message = "温度过低"
	}

	return &ComponentScore{
		Type:    ComponentTemperature,
		Score:   score,
		Weight:  DefaultWeights[ComponentTemperature],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"cpu_temp":  cpuTemp,
			"disk_temp": diskTemp,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectServices collects service health data.
func (dc *DefaultCollectors) collectServices() (*ComponentScore, error) {
	// Simulate service status
	totalServices := 10
	runningServices := 9

	score := float64(runningServices) / float64(totalServices) * 100

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "所有服务运行正常"
	if runningServices < totalServices {
		message = "部分服务异常"
	}

	return &ComponentScore{
		Type:    ComponentService,
		Score:   score,
		Weight:  DefaultWeights[ComponentService],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"total":   totalServices,
			"running": runningServices,
		},
		CollectedAt: time.Now(),
	}, nil
}

// collectRAID collects RAID health data.
func (dc *DefaultCollectors) collectRAID() (*ComponentScore, error) {
	// Simulate RAID status
	raidStatus := "healthy" // Placeholder
	degradedDisks := 0

	score := 100.0
	if degradedDisks > 0 {
		score -= float64(degradedDisks) * 30
	}
	if score < 0 {
		score = 0
	}

	status := dc.hs.GetCalculator().DetermineStatus(score)
	message := "RAID 状态正常"
	switch raidStatus {
	case "degraded":
		message = "RAID 降级运行"
	case "failed":
		message = "RAID 故障"
	}

	return &ComponentScore{
		Type:    ComponentRAID,
		Score:   score,
		Weight:  DefaultWeights[ComponentRAID],
		Status:  status,
		Message: message,
		Details: map[string]interface{}{
			"status":         raidStatus,
			"degraded_disks": degradedDisks,
		},
		CollectedAt: time.Now(),
	}, nil
}
