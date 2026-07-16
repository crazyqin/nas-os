package raidzexpand

import (
	"context"
	"fmt"
	"time"
)

const defaultResilverThroughputBytesPerSec uint64 = 150 * 1024 * 1024 // 150 MiB/s, conservative HDD-friendly default.

// ExpansionRiskLevel describes the preflight risk of a RAIDZ expansion plan.
type ExpansionRiskLevel string

const (
	RiskLow      ExpansionRiskLevel = "low"
	RiskModerate ExpansionRiskLevel = "moderate"
	RiskHigh     ExpansionRiskLevel = "high"
)

// ExpansionPlan is a dry, operator-friendly summary for one RAIDZ expansion.
// It is intentionally command-free: callers can show it in UI/API before any
// zpool mutation is attempted.
type ExpansionPlan struct {
	PoolName                 string             `json:"poolName"`
	VDevPath                 string             `json:"vdevPath"`
	RAIDZLevel               RAIDZLevel         `json:"raidzLevel"`
	CurrentDiskCount         int                `json:"currentDiskCount"`
	NewDiskCount             int                `json:"newDiskCount"`
	FinalDiskCount           int                `json:"finalDiskCount"`
	ParityDiskCount          int                `json:"parityDiskCount"`
	EstimatedCapacityGain    uint64             `json:"estimatedCapacityGainBytes"`
	CurrentUsableCapacity    uint64             `json:"currentUsableCapacityBytes"`
	ProjectedUsableCapacity  uint64             `json:"projectedUsableCapacityBytes"`
	EstimatedResilverBytes   uint64             `json:"estimatedResilverBytes"`
	EstimatedResilverTime    time.Duration      `json:"estimatedResilverTime"`
	EstimatedResilverSeconds int64              `json:"estimatedResilverSeconds"`
	ResilverThroughputBps    uint64             `json:"resilverThroughputBytesPerSec"`
	RiskLevel                ExpansionRiskLevel `json:"riskLevel"`
	Warnings                 []string           `json:"warnings,omitempty"`
	Health                   *HealthCheckResult `json:"health,omitempty"`
}

// BuildExpansionPlan performs TrueNAS-style expansion preflight planning:
// topology validation, capacity projection, resilver estimate, and risk hints.
// throughputBytesPerSec may be 0 to use a conservative default estimate.
func BuildExpansionPlan(ctx context.Context, cfg *ExpansionConfig, hc *HealthChecker, diskSizeBytes uint64, throughputBytesPerSec uint64) (*ExpansionPlan, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	if diskSizeBytes == 0 {
		return nil, fmt.Errorf("磁盘容量不能为空")
	}
	if throughputBytesPerSec == 0 {
		throughputBytesPerSec = defaultResilverThroughputBytesPerSec
	}

	var health *HealthCheckResult
	currentDisks := 0
	var usedBytes uint64
	if hc != nil && hc.provider != nil {
		result, err := hc.Check(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("健康检查失败: %w", err)
		}
		health = result
		currentDisks = result.CurrentDiskCount

		_, used, _, err := hc.provider.GetPoolCapacity(ctx, cfg.PoolName)
		if err != nil {
			return nil, fmt.Errorf("查询池容量失败: %w", err)
		}
		usedBytes = used
	}

	if currentDisks == 0 {
		return nil, fmt.Errorf("无法确定当前 RAIDZ vdev 磁盘数量")
	}

	parity := parityDiskCount(cfg.RAIDZLevel)
	if currentDisks <= parity {
		return nil, fmt.Errorf("当前磁盘数量 %d 不满足 %s 最小拓扑", currentDisks, cfg.RAIDZLevel)
	}

	newDiskCount := len(cfg.NewDisks)
	capacityGain := EstimateCapacityGain(currentDisks, newDiskCount, cfg.RAIDZLevel, diskSizeBytes)
	currentUsable := uint64(currentDisks-parity) * diskSizeBytes
	projectedUsable := uint64(currentDisks+newDiskCount-parity) * diskSizeBytes
	resilverBytes := estimateResilverBytes(usedBytes, currentDisks, newDiskCount)
	resilverTime := estimateDuration(resilverBytes, throughputBytesPerSec)

	plan := &ExpansionPlan{
		PoolName:                 cfg.PoolName,
		VDevPath:                 cfg.VDevPath,
		RAIDZLevel:               cfg.RAIDZLevel,
		CurrentDiskCount:         currentDisks,
		NewDiskCount:             newDiskCount,
		FinalDiskCount:           currentDisks + newDiskCount,
		ParityDiskCount:          parity,
		EstimatedCapacityGain:    capacityGain,
		CurrentUsableCapacity:    currentUsable,
		ProjectedUsableCapacity:  projectedUsable,
		EstimatedResilverBytes:   resilverBytes,
		EstimatedResilverTime:    resilverTime,
		EstimatedResilverSeconds: int64(resilverTime / time.Second),
		ResilverThroughputBps:    throughputBytesPerSec,
		RiskLevel:                RiskLow,
		Health:                   health,
	}
	plan.Warnings = expansionWarnings(plan, health, cfg)
	plan.RiskLevel = classifyRisk(plan.Warnings, health)
	return plan, nil
}

func parityDiskCount(level RAIDZLevel) int {
	switch level {
	case RAIDZ1:
		return 1
	case RAIDZ2:
		return 2
	case RAIDZ3:
		return 3
	default:
		return 0
	}
}

func estimateResilverBytes(usedBytes uint64, currentDisks int, newDisks int) uint64 {
	if currentDisks <= 0 || newDisks <= 0 || usedBytes == 0 {
		return 0
	}
	// RAIDZ expansion rewrites/rebalances a share of existing logical data for
	// every added disk. This estimates the data movement share instead of full
	// pool size, which keeps the ETA useful for UI preflight.
	return usedBytes * uint64(newDisks) / uint64(currentDisks+newDisks)
}

func estimateDuration(bytes uint64, throughputBytesPerSec uint64) time.Duration {
	if bytes == 0 || throughputBytesPerSec == 0 {
		return 0
	}
	seconds := bytes / throughputBytesPerSec
	if bytes%throughputBytesPerSec != 0 {
		seconds++
	}
	return time.Duration(seconds) * time.Second
}

func expansionWarnings(plan *ExpansionPlan, health *HealthCheckResult, cfg *ExpansionConfig) []string {
	warnings := []string{}
	if health != nil {
		warnings = append(warnings, health.Issues...)
		if plan.EstimatedResilverBytes > 0 && health.CurrentFreeBytes < plan.EstimatedResilverBytes/10 {
			warnings = append(warnings, "空闲空间低于预计数据重排量的 10%，建议先释放空间或开启 Force 前人工确认")
		}
	}
	if cfg.RAIDZLevel == RAIDZ1 && plan.FinalDiskCount >= 6 {
		warnings = append(warnings, "RAIDZ1 扩展到 6 盘及以上容错较弱，建议新建 RAIDZ2/RAIDZ3 池或确认备份")
	}
	if plan.NewDiskCount > 1 {
		warnings = append(warnings, "一次添加多块盘会拉长重排窗口，建议逐盘执行并在每次完成后确认池健康")
	}
	if plan.EstimatedResilverTime >= 24*time.Hour {
		warnings = append(warnings, "预计重排时间超过 24 小时，建议避开业务高峰并确保 UPS/备份正常")
	}
	return warnings
}

func classifyRisk(warnings []string, health *HealthCheckResult) ExpansionRiskLevel {
	if health != nil && (!health.PoolHealthy || !health.DisksAvailable || !health.VDevExists || !health.CapacityOK) {
		return RiskHigh
	}
	switch {
	case len(warnings) >= 3:
		return RiskHigh
	case len(warnings) > 0:
		return RiskModerate
	default:
		return RiskLow
	}
}
