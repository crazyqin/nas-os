// Package resmetering 提供资源计量能力
// 按用户/容器/服务维度采集CPU、内存、存储、网络带宽使用量
// 支持小时/日/月维度的聚合统计与报告
package resmetering

import (
	"sync"
	"time"
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceCPU     ResourceType = "cpu"      // CPU
	ResourceMemory  ResourceType = "memory"   // 内存
	ResourceStorage ResourceType = "storage"  // 存储
	ResourceNetwork ResourceType = "network"  // 网络带宽
)

// AggregationPeriod 聚合周期
type AggregationPeriod string

const (
	PeriodHourly AggregationPeriod = "hourly" // 小时
	PeriodDaily  AggregationPeriod = "daily"  // 日
	PeriodMonthly AggregationPeriod = "monthly" // 月
)

// Sample 资源使用采样数据
type Sample struct {
	Timestamp   time.Time    `json:"timestamp"`    // 采样时间
	UserID      string       `json:"user_id"`      // 用户ID
	ContainerID string       `json:"container_id"` // 容器ID
	ServiceName string       `json:"service_name"` // 服务名称
	CPU         CPUUsage     `json:"cpu"`          // CPU使用
	Memory      MemoryUsage  `json:"memory"`       // 内存使用
	Storage     StorageUsage `json:"storage"`      // 存储使用
	Network     NetworkUsage `json:"network"`      // 网络使用
}

// CPUUsage CPU使用量
type CPUUsage struct {
	Cores      float64 `json:"cores"`        // 占用核心数
	Percent    float64 `json:"percent"`      // 使用率百分比
	UsedSeconds float64 `json:"used_seconds"` // 累计使用秒数
}

// MemoryUsage 内存使用量
type MemoryUsage struct {
	UsedBytes  uint64 `json:"used_bytes"`  // 已用字节
	LimitBytes uint64 `json:"limit_bytes"` // 限制字节
	Percent    float64 `json:"percent"`    // 使用率百分比
}

// StorageUsage 存储使用量
type StorageUsage struct {
	UsedBytes  uint64 `json:"used_bytes"`  // 已用字节
	TotalBytes uint64 `json:"total_bytes"` // 总量字节
	Percent    float64 `json:"percent"`    // 使用率百分比
}

// NetworkUsage 网络使用量
type NetworkUsage struct {
	RxBytes float64 `json:"rx_bytes"` // 接收字节
	TxBytes float64 `json:"tx_bytes"` // 发送字节
	RxRate  float64 `json:"rx_rate"`  // 接收速率（bps）
	TxRate  float64 `json:"tx_rate"`  // 发送速率（bps）
}

// AggregatedUsage 聚合后的使用量
type AggregatedUsage struct {
	Key          string  `json:"key"`           // 聚合键（用户ID或容器ID）
	CPU          CPUUsage     `json:"cpu"`      // CPU汇总
	Memory       MemoryUsage  `json:"memory"`   // 内存汇总
	Storage      StorageUsage `json:"storage"`  // 存储汇总
	Network      NetworkUsage `json:"network"`  // 网络汇总
	SampleCount  int          `json:"sample_count"`  // 采样数
	PeriodStart  time.Time    `json:"period_start"`  // 周期开始
	PeriodEnd    time.Time    `json:"period_end"`    // 周期结束
}

// Summary 资源使用汇总
type Summary struct {
	GeneratedAt     time.Time         `json:"generated_at"`      // 生成时间
	Period          AggregationPeriod `json:"period"`            // 聚合周期
	PeriodStart     time.Time         `json:"period_start"`      // 周期开始
	PeriodEnd       time.Time         `json:"period_end"`        // 周期结束
	TotalCPU        CPUUsage          `json:"total_cpu"`         // CPU总量
	TotalMemory     MemoryUsage       `json:"total_memory"`      // 内存总量
	TotalStorage    StorageUsage      `json:"total_storage"`     // 存储总量
	TotalNetwork    NetworkUsage      `json:"total_network"`     // 网络总量
	SampleCount     int               `json:"sample_count"`      // 总采样数
	UniqueUsers     int               `json:"unique_users"`      // 唯一用户数
	UniqueContainers int              `json:"unique_containers"` // 唯一容器数
}

// ByUserReport 按用户维度的报告
type ByUserReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Period      AggregationPeriod  `json:"period"`
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
	Users       []AggregatedUsage  `json:"users"`
}

// ByContainerReport 按容器维度的报告
type ByContainerReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Period      AggregationPeriod  `json:"period"`
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
	Containers  []AggregatedUsage  `json:"containers"`
}

// Service 资源计量服务
type Service struct {
	mu      sync.RWMutex
	samples []Sample
	maxSamples int
}

// NewService 创建资源计量服务
func NewService() *Service {
	return &Service{
		samples:    make([]Sample, 0, 10000),
		maxSamples: 100000,
	}
}

// Record 记录一条采样
func (s *Service) Record(sample Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.samples = append(s.samples, sample)
	if len(s.samples) > s.maxSamples {
		s.samples = s.samples[len(s.samples)-s.maxSamples:]
	}
}

// GetSummary 获取汇总报告
func (s *Service) GetSummary(period AggregationPeriod, from, to time.Time) Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := Summary{
		GeneratedAt: time.Now(),
		Period:      period,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	users := make(map[string]bool)
	containers := make(map[string]bool)

	for _, s := range s.samples {
		if s.Timestamp.Before(from) || s.Timestamp.After(to) {
			continue
		}

		summary.SampleCount++
		users[s.UserID] = true
		containers[s.ContainerID] = true

		// 累加CPU
		summary.TotalCPU.Cores += s.CPU.Cores
		summary.TotalCPU.Percent += s.CPU.Percent
		summary.TotalCPU.UsedSeconds += s.CPU.UsedSeconds

		// 累加内存（取最大值作为总量参考）
		summary.TotalMemory.UsedBytes += s.Memory.UsedBytes
		if s.Memory.LimitBytes > summary.TotalMemory.LimitBytes {
			summary.TotalMemory.LimitBytes = s.Memory.LimitBytes
		}

		// 累加存储
		summary.TotalStorage.UsedBytes += s.Storage.UsedBytes
		if s.Storage.TotalBytes > summary.TotalStorage.TotalBytes {
			summary.TotalStorage.TotalBytes = s.Storage.TotalBytes
		}

		// 累加网络
		summary.TotalNetwork.RxBytes += s.Network.RxBytes
		summary.TotalNetwork.TxBytes += s.Network.TxBytes
		summary.TotalNetwork.RxRate += s.Network.RxRate
		summary.TotalNetwork.TxRate += s.Network.TxRate
	}

	summary.UniqueUsers = len(users)
	summary.UniqueContainers = len(containers)

	// 计算平均百分比
	if summary.SampleCount > 0 {
		summary.TotalCPU.Percent /= float64(summary.SampleCount)
		summary.TotalMemory.Percent /= float64(summary.SampleCount)
		summary.TotalStorage.Percent /= float64(summary.SampleCount)
	}

	return summary
}

// GetByUser 按用户聚合
func (s *Service) GetByUser(period AggregationPeriod, from, to time.Time) ByUserReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := ByUserReport{
		GeneratedAt: time.Now(),
		Period:      period,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	aggregated := make(map[string]*AggregatedUsage)

	for _, sample := range s.samples {
		if sample.Timestamp.Before(from) || sample.Timestamp.After(to) {
			continue
		}

		key := sample.UserID
		if key == "" {
			continue
		}

		agg, exists := aggregated[key]
		if !exists {
			agg = &AggregatedUsage{
				Key:         key,
				PeriodStart: from,
				PeriodEnd:   to,
			}
			aggregated[key] = agg
		}

		agg.SampleCount++
		agg.CPU.Cores += sample.CPU.Cores
		agg.CPU.Percent += sample.CPU.Percent
		agg.CPU.UsedSeconds += sample.CPU.UsedSeconds
		agg.Memory.UsedBytes += sample.Memory.UsedBytes
		if sample.Memory.LimitBytes > agg.Memory.LimitBytes {
			agg.Memory.LimitBytes = sample.Memory.LimitBytes
		}
		agg.Storage.UsedBytes += sample.Storage.UsedBytes
		if sample.Storage.TotalBytes > agg.Storage.TotalBytes {
			agg.Storage.TotalBytes = sample.Storage.TotalBytes
		}
		agg.Network.RxBytes += sample.Network.RxBytes
		agg.Network.TxBytes += sample.Network.TxBytes
		agg.Network.RxRate += sample.Network.RxRate
		agg.Network.TxRate += sample.Network.TxRate
	}

	// 计算平均值
	for _, agg := range aggregated {
		if agg.SampleCount > 0 {
			agg.CPU.Percent /= float64(agg.SampleCount)
			agg.Memory.Percent /= float64(agg.SampleCount)
			agg.Storage.Percent /= float64(agg.SampleCount)
		}
		report.Users = append(report.Users, *agg)
	}

	return report
}

// GetByContainer 按容器聚合
func (s *Service) GetByContainer(period AggregationPeriod, from, to time.Time) ByContainerReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := ByContainerReport{
		GeneratedAt: time.Now(),
		Period:      period,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	aggregated := make(map[string]*AggregatedUsage)

	for _, sample := range s.samples {
		if sample.Timestamp.Before(from) || sample.Timestamp.After(to) {
			continue
		}

		key := sample.ContainerID
		if key == "" {
			continue
		}

		agg, exists := aggregated[key]
		if !exists {
			agg = &AggregatedUsage{
				Key:         key,
				PeriodStart: from,
				PeriodEnd:   to,
			}
			aggregated[key] = agg
		}

		agg.SampleCount++
		agg.CPU.Cores += sample.CPU.Cores
		agg.CPU.Percent += sample.CPU.Percent
		agg.CPU.UsedSeconds += sample.CPU.UsedSeconds
		agg.Memory.UsedBytes += sample.Memory.UsedBytes
		if sample.Memory.LimitBytes > agg.Memory.LimitBytes {
			agg.Memory.LimitBytes = sample.Memory.LimitBytes
		}
		agg.Storage.UsedBytes += sample.Storage.UsedBytes
		if sample.Storage.TotalBytes > agg.Storage.TotalBytes {
			agg.Storage.TotalBytes = sample.Storage.TotalBytes
		}
		agg.Network.RxBytes += sample.Network.RxBytes
		agg.Network.TxBytes += sample.Network.TxBytes
		agg.Network.RxRate += sample.Network.RxRate
		agg.Network.TxRate += sample.Network.TxRate
	}

	// 计算平均值
	for _, agg := range aggregated {
		if agg.SampleCount > 0 {
			agg.CPU.Percent /= float64(agg.SampleCount)
			agg.Memory.Percent /= float64(agg.SampleCount)
			agg.Storage.Percent /= float64(agg.SampleCount)
		}
		report.Containers = append(report.Containers, *agg)
	}

	return report
}
