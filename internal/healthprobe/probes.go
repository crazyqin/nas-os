// Package healthprobe - 内置探针实现
package healthprobe

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// CPUProbe CPU 使用率探针
type CPUProbe struct {
	name    string
	percent float64 // 上次采样值
}

// NewCPUProbe 创建 CPU 探针
func NewCPUProbe(name string) *CPUProbe {
	return &CPUProbe{name: name}
}

func (p *CPUProbe) Name() string     { return p.name }
func (p *CPUProbe) Type() MetricType { return MetricCPU }

func (p *CPUProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	// 使用 runtime 采样获取 CPU 使用率估算
	// 真实场景可用 gopsutil
	numCPU := runtime.NumCPU()
	goroutines := runtime.NumGoroutine()

	// 估算 CPU 负载 (简化方式)
	loadEstimate := float64(goroutines) / float64(numCPU*100) * 100
	if loadEstimate > 100 {
		loadEstimate = 100
	}
	p.percent = loadEstimate

	level := LevelHealthy
	message := "CPU 使用率正常"
	if loadEstimate > 90 {
		level = LevelCritical
		message = "CPU 使用率过高"
	} else if loadEstimate > 70 {
		level = LevelDegraded
		message = "CPU 使用率偏高"
	}

	return &ProbeResult{
		Name:      p.name,
		Type:      MetricCPU,
		Level:     level,
		Value:     loadEstimate,
		Unit:      "%",
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"numCPU":     numCPU,
			"goroutines": goroutines,
		},
	}, nil
}

// MemoryProbe 内存使用探针
type MemoryProbe struct {
	name string
}

// NewMemoryProbe 创建内存探针
func NewMemoryProbe(name string) *MemoryProbe {
	return &MemoryProbe{name: name}
}

func (p *MemoryProbe) Name() string     { return p.name }
func (p *MemoryProbe) Type() MetricType { return MetricMemory }

func (p *MemoryProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var usedPercent float64
	if m.Sys > 0 {
		usedPercent = float64(m.Alloc) / float64(m.Sys) * 100
	}

	level := LevelHealthy
	message := "内存使用正常"
	if usedPercent > 90 {
		level = LevelCritical
		message = "内存使用率过高"
	} else if usedPercent > 80 {
		level = LevelDegraded
		message = "内存使用率偏高"
	}

	return &ProbeResult{
		Name:      p.name,
		Type:      MetricMemory,
		Level:     level,
		Value:     usedPercent,
		Unit:      "%",
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"alloc_mb":      m.Alloc / 1024 / 1024,
			"totalAlloc_mb": m.TotalAlloc / 1024 / 1024,
			"sys_mb":        m.Sys / 1024 / 1024,
			"heapAlloc_mb":  m.HeapAlloc / 1024 / 1024,
			"heapSys_mb":    m.HeapSys / 1024 / 1024,
			"numGC":         m.NumGC,
		},
	}, nil
}

// DiskProbe 磁盘使用探针
type DiskProbe struct {
	name      string
	path      string
	threshold float64
}

// NewDiskProbe 创建磁盘探针
func NewDiskProbe(name, path string, threshold float64) *DiskProbe {
	if path == "" {
		path = "/"
	}
	if threshold <= 0 {
		threshold = 90
	}
	return &DiskProbe{
		name:      name,
		path:      path,
		threshold: threshold,
	}
}

func (p *DiskProbe) Name() string     { return p.name }
func (p *DiskProbe) Type() MetricType { return MetricDisk }

func (p *DiskProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	var stat syscall.Statfs_t
	if err := syscall.Statfs(p.path, &stat); err != nil {
		return &ProbeResult{
			Name:      p.name,
			Type:      MetricDisk,
			Level:     LevelCritical,
			Message:   fmt.Sprintf("获取磁盘信息失败: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
			Details:   map[string]interface{}{"path": p.path, "error": err.Error()},
		}, nil
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	var usedPercent float64
	if totalBytes > 0 {
		usedPercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	level := LevelHealthy
	message := "磁盘空间充足"
	if usedPercent > p.threshold {
		level = LevelCritical
		message = fmt.Sprintf("磁盘使用率 %.1f%% 超过阈值 %.0f%%", usedPercent, p.threshold)
	} else if usedPercent > p.threshold*0.9 {
		level = LevelDegraded
		message = fmt.Sprintf("磁盘使用率 %.1f%% 接近阈值", usedPercent)
	}

	return &ProbeResult{
		Name:      p.name,
		Type:      MetricDisk,
		Level:     level,
		Value:     usedPercent,
		Unit:      "%",
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"path":       p.path,
			"total_gb":   float64(totalBytes) / 1024 / 1024 / 1024,
			"used_gb":    float64(usedBytes) / 1024 / 1024 / 1024,
			"free_gb":    float64(freeBytes) / 1024 / 1024 / 1024,
			"threshold":  p.threshold,
		},
	}, nil
}

// NetworkProbe 网络连通性探针
type NetworkProbe struct {
	name    string
	targets []string
	timeout time.Duration
}

// NewNetworkProbe 创建网络探针
func NewNetworkProbe(name string, targets []string, timeout time.Duration) *NetworkProbe {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if len(targets) == 0 {
		targets = []string{"8.8.8.8:53", "1.1.1.1:53"}
	}
	return &NetworkProbe{
		name:    name,
		targets: targets,
		timeout: timeout,
	}
}

func (p *NetworkProbe) Name() string     { return p.name }
func (p *NetworkProbe) Type() MetricType { return MetricNetwork }

func (p *NetworkProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	successCount := 0
	totalRTT := time.Duration(0)
	details := make(map[string]interface{})
	var errors []string

	for _, target := range p.targets {
		dialStart := time.Now()
		dialer := &net.Dialer{Timeout: p.timeout}
		conn, err := dialer.DialContext(ctx, "tcp", target)
		rtt := time.Since(dialStart)

		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", target, err))
			details[target] = map[string]interface{}{
				"status": "unreachable",
				"error":  err.Error(),
			}
			continue
		}
		_ = conn.Close()

		successCount++
		totalRTT += rtt
		details[target] = map[string]interface{}{
			"status":   "reachable",
			"rtt_ms":   rtt.Milliseconds(),
		}
	}

	// 可达率
	reachability := float64(successCount) / float64(len(p.targets)) * 100

	// 平均 RTT
	var avgRTT float64
	if successCount > 0 {
		avgRTT = float64(totalRTT.Milliseconds()) / float64(successCount)
	}

	level := LevelHealthy
	message := fmt.Sprintf("网络连通性正常 (%d/%d)", successCount, len(p.targets))
	if successCount == 0 {
		level = LevelCritical
		message = "所有网络目标不可达"
	} else if successCount < len(p.targets) {
		level = LevelDegraded
		message = fmt.Sprintf("部分网络目标不可达 (%d/%d)", successCount, len(p.targets))
	}

	details["successCount"] = successCount
	details["totalTargets"] = len(p.targets)
	details["reachability"] = reachability
	details["avgRTT_ms"] = avgRTT
	if len(errors) > 0 {
		details["errors"] = errors
	}

	return &ProbeResult{
		Name:      p.name,
		Type:      MetricNetwork,
		Level:     level,
		Value:     reachability,
		Unit:      "%",
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details:   details,
	}, nil
}

// TemperatureProbe 温度探针
type TemperatureProbe struct {
	name        string
	warmThresh  float64
	hotThresh   float64
	critThresh  float64
}

// NewTemperatureProbe 创建温度探针
func NewTemperatureProbe(name string, warmThresh, hotThresh, critThresh float64) *TemperatureProbe {
	if warmThresh <= 0 {
		warmThresh = 60
	}
	if hotThresh <= 0 {
		hotThresh = 75
	}
	if critThresh <= 0 {
		critThresh = 90
	}
	return &TemperatureProbe{
		name:       name,
		warmThresh: warmThresh,
		hotThresh:  hotThresh,
		critThresh: critThresh,
	}
}

func (p *TemperatureProbe) Name() string     { return p.name }
func (p *TemperatureProbe) Type() MetricType { return MetricTemp }

func (p *TemperatureProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	// 尝试从 sysfs 读取温度
	temp, err := p.readCPUTemp()
	if err != nil {
		// 无法读取温度时返回 unknown
		return &ProbeResult{
			Name:      p.name,
			Type:      MetricTemp,
			Level:     LevelUnknown,
			Value:     0,
			Unit:      "°C",
			Message:   fmt.Sprintf("无法读取温度: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
			Details:   map[string]interface{}{"error": err.Error()},
		}, nil
	}

	level := LevelHealthy
	message := "温度正常"
	if temp >= p.critThresh {
		level = LevelCritical
		message = fmt.Sprintf("温度 %.1f°C 达到临界值", temp)
	} else if temp >= p.hotThresh {
		level = LevelDegraded
		message = fmt.Sprintf("温度 %.1f°C 过高", temp)
	} else if temp >= p.warmThresh {
		level = LevelDegraded
		message = fmt.Sprintf("温度 %.1f°C 偏高", temp)
	}

	return &ProbeResult{
		Name:      p.name,
		Type:      MetricTemp,
		Level:     level,
		Value:     temp,
		Unit:      "°C",
		Message:   message,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"warmThresh": p.warmThresh,
			"hotThresh":  p.hotThresh,
			"critThresh": p.critThresh,
		},
	}, nil
}

// readCPUTemp 从 sysfs 读取 CPU 温度
func (p *TemperatureProbe) readCPUTemp() (float64, error) {
	thermalPath := "/sys/class/thermal"
	entries, err := os.ReadDir(thermalPath)
	if err != nil {
		return 0, fmt.Errorf("无法读取 thermal 目录: %w", err)
	}

	var maxTemp float64
	found := false

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}

		typePath := fmt.Sprintf("%s/%s/type", thermalPath, entry.Name())
		typeData, err := os.ReadFile(typePath)
		if err != nil {
			continue
		}
		typeName := strings.TrimSpace(string(typeData))
		// 只关注 CPU 相关的温度区域
		if !strings.Contains(strings.ToLower(typeName), "cpu") &&
			!strings.Contains(strings.ToLower(typeName), "x86_pkg") &&
			!strings.Contains(strings.ToLower(typeName), "soc") {
			continue
		}

		tempPath := fmt.Sprintf("%s/%s/temp", thermalPath, entry.Name())
		tempData, err := os.ReadFile(tempPath)
		if err != nil {
			continue
		}

		var tempVal float64
		_, err = fmt.Sscanf(strings.TrimSpace(string(tempData)), "%f", &tempVal)
		if err != nil {
			continue
		}

		temp := tempVal / 1000.0
		if temp > 200 {
			temp = tempVal // 已经是摄氏度
		}

		if temp > maxTemp {
			maxTemp = temp
			found = true
		}
	}

	if !found {
		return 0, fmt.Errorf("未找到 CPU 温度传感器")
	}

	return maxTemp, nil
}

// ServiceProbe 服务健康探针
type ServiceProbe struct {
	name    string
	checkFn func(ctx context.Context) error
}

// NewServiceProbe 创建服务探针
func NewServiceProbe(name string, checkFn func(ctx context.Context) error) *ServiceProbe {
	return &ServiceProbe{
		name:    name,
		checkFn: checkFn,
	}
}

func (p *ServiceProbe) Name() string     { return p.name }
func (p *ServiceProbe) Type() MetricType { return MetricCustom }

func (p *ServiceProbe) Collect(ctx context.Context) (*ProbeResult, error) {
	start := time.Now()

	if p.checkFn == nil {
		return &ProbeResult{
			Name:      p.name,
			Type:      MetricCustom,
			Level:     LevelUnknown,
			Message:   "检查函数未定义",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
			Details:   make(map[string]interface{}),
		}, nil
	}

	err := p.checkFn(ctx)
	duration := time.Since(start)

	result := &ProbeResult{
		Name:      p.name,
		Type:      MetricCustom,
		Timestamp: time.Now(),
		Duration:  duration,
		Unit:      "ms",
		Value:     float64(duration.Milliseconds()),
		Details:   make(map[string]interface{}),
	}

	if err != nil {
		result.Level = LevelCritical
		result.Message = fmt.Sprintf("服务检查失败: %v", err)
		result.Details["error"] = err.Error()
	} else {
		result.Level = LevelHealthy
		result.Message = "服务正常"
	}

	return result, nil
}
