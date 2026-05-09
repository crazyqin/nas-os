package storagedash

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Dashboard 存储仪表盘引擎
type Dashboard struct {
	logger        *zap.Logger
	mu            sync.RWMutex
	poolProviders []PoolProvider
	tierProviders []TierProvider

	// 缓存数据
	cachedOverview *StorageOverview
	cachedTrends   []CapacityTrend
	cachedAlerts   *AlertSummary
	cacheTime      time.Time
	cacheTTL       time.Duration
}

// NewDashboard 创建仪表盘引擎实例
func NewDashboard(logger *zap.Logger) *Dashboard {
	return &Dashboard{
		logger:        logger,
		poolProviders: make([]PoolProvider, 0),
		tierProviders: make([]TierProvider, 0),
		cacheTTL:      30 * time.Second,
	}
}

// RegisterPoolProvider 注册存储池数据源
func (d *Dashboard) RegisterPoolProvider(fn PoolProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.poolProviders = append(d.poolProviders, fn)
	d.logger.Info("注册存储池数据源", zap.Int("total", len(d.poolProviders)))
}

// RegisterTierProvider 注册分层数据源
func (d *Dashboard) RegisterTierProvider(fn TierProvider) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tierProviders = append(d.tierProviders, fn)
	d.logger.Info("注册分层数据源", zap.Int("total", len(d.tierProviders)))
}

// GetOverview 获取存储总体概览
func (d *Dashboard) GetOverview() (*StorageOverview, error) {
	d.mu.RLock()
	// 检查缓存是否有效
	if d.cachedOverview != nil && time.Since(d.cacheTime) < d.cacheTTL {
		overview := d.cachedOverview
		d.mu.RUnlock()
		return overview, nil
	}
	d.mu.RUnlock()

	// 缓存失效，重新获取
	return d.fetchOverview()
}

// fetchOverview 从数据源获取存储概览
func (d *Dashboard) fetchOverview() (*StorageOverview, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 二次检查（避免并发重复拉取）
	if d.cachedOverview != nil && time.Since(d.cacheTime) < d.cacheTTL {
		return d.cachedOverview, nil
	}

	var allPools []PoolSummary
	var allTiers []TierSummary

	// 从所有注册的数据源收集存储池数据
	for _, provider := range d.poolProviders {
		pools, err := provider()
		if err != nil {
			d.logger.Warn("获取存储池数据失败", zap.Error(err))
			continue
		}
		allPools = append(allPools, pools...)
	}

	// 从所有注册的数据源收集分层数据
	for _, provider := range d.tierProviders {
		tiers, err := provider()
		if err != nil {
			d.logger.Warn("获取分层数据失败", zap.Error(err))
			continue
		}
		allTiers = append(allTiers, tiers...)
	}

	// 汇总计算
	overview := &StorageOverview{
		Pools: allPools,
		Tiers: allTiers,
	}

	for _, p := range allPools {
		overview.TotalCapacity += p.TotalBytes
		overview.UsedCapacity += p.UsedBytes
	}

	overview.FreeCapacity = overview.TotalCapacity - overview.UsedCapacity
	if overview.TotalCapacity > 0 {
		overview.Utilization = float64(overview.UsedCapacity) / float64(overview.TotalCapacity)
	}

	// 判断整体健康状态
	overview.Health = d.evaluateHealth(allPools, overview.Utilization)

	// 更新缓存
	d.cachedOverview = overview
	d.cacheTime = time.Now()

	return overview, nil
}

// evaluateHealth 根据存储池状态和使用率评估健康等级
func (d *Dashboard) evaluateHealth(pools []PoolSummary, utilization float64) string {
	// 任何池故障则整体为 critical
	for _, p := range pools {
		if p.Status == "faulted" || p.Status == "offline" {
			return "critical"
		}
	}

	// 池降级或使用率超过 90% 为 warning
	for _, p := range pools {
		if p.Status == "degraded" {
			return "warning"
		}
	}
	if utilization > 0.90 {
		return "warning"
	}

	return "healthy"
}

// GetCapacityTrends 获取容量趋势数据
func (d *Dashboard) GetCapacityTrends(days int) ([]CapacityTrend, error) {
	if days <= 0 {
		days = 7
	}

	d.mu.RLock()
	// 检查缓存（趋势数据与概览共享缓存时间）
	if d.cachedTrends != nil && time.Since(d.cacheTime) < d.cacheTTL {
		trends := d.cachedTrends
		d.mu.RUnlock()
		return d.trimTrends(trends, days), nil
	}
	d.mu.RUnlock()

	// 生成趋势数据（基于当前概览模拟历史数据）
	overview, err := d.GetOverview()
	if err != nil {
		return nil, fmt.Errorf("获取概览失败: %w", err)
	}

	trends := d.generateTrends(overview, days)

	d.mu.Lock()
	d.cachedTrends = trends
	d.mu.Unlock()

	return trends, nil
}

// generateTrends 基于当前数据生成趋势序列
func (d *Dashboard) generateTrends(overview *StorageOverview, days int) []CapacityTrend {
	trends := make([]CapacityTrend, 0, days)
	now := time.Now()

	// 使用日均增长率反推历史数据
	// 默认增长率按总容量的 0.1% 计算
	dailyGrowth := float64(overview.TotalCapacity) * 0.001
	if overview.FreeCapacity > 0 && dailyGrowth > 0 {
		// 计算剩余天数
		_ = float64(overview.FreeCapacity) / dailyGrowth
	}

	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		// 估算该日已用容量：从当前容量往前回推
		usedBytes := overview.UsedCapacity - int64(dailyGrowth*float64(i))
		if usedBytes < 0 {
			usedBytes = 0
		}

		daysUntilFull := -1
		growthRate := dailyGrowth
		if growthRate > 0 && overview.FreeCapacity > 0 {
			daysUntilFull = int(float64(overview.FreeCapacity) / growthRate)
		}

		trends = append(trends, CapacityTrend{
			Date:          date,
			UsedBytes:     usedBytes,
			GrowthRate:    growthRate,
			DaysUntilFull: daysUntilFull,
		})
	}

	return trends
}

// trimTrends 截取指定天数的趋势数据
func (d *Dashboard) trimTrends(trends []CapacityTrend, days int) []CapacityTrend {
	if len(trends) <= days {
		return trends
	}
	return trends[len(trends)-days:]
}

// GetAlerts 获取告警汇总
func (d *Dashboard) GetAlerts() (*AlertSummary, error) {
	d.mu.RLock()
	if d.cachedAlerts != nil && time.Since(d.cacheTime) < d.cacheTTL {
		alerts := d.cachedAlerts
		d.mu.RUnlock()
		return alerts, nil
	}
	d.mu.RUnlock()

	overview, err := d.GetOverview()
	if err != nil {
		return nil, fmt.Errorf("获取概览失败: %w", err)
	}

	alerts := d.generateAlerts(overview)

	d.mu.Lock()
	d.cachedAlerts = alerts
	d.mu.Unlock()

	return alerts, nil
}

// generateAlerts 基于存储状态生成告警
func (d *Dashboard) generateAlerts(overview *StorageOverview) *AlertSummary {
	summary := &AlertSummary{
		RecentAlerts: make([]Alert, 0),
	}
	now := time.Now()

	// 检查存储池状态
	for _, p := range overview.Pools {
		switch p.Status {
		case "faulted":
			summary.Critical++
			summary.RecentAlerts = append(summary.RecentAlerts, Alert{
				Level:     "critical",
				Message:   fmt.Sprintf("存储池 %s 已故障", p.Name),
				Source:    p.Name,
				Time:      now,
			})
		case "degraded":
			summary.Warning++
			summary.RecentAlerts = append(summary.RecentAlerts, Alert{
				Level:     "warning",
				Message:   fmt.Sprintf("存储池 %s 状态降级", p.Name),
				Source:    p.Name,
				Time:      now,
			})
		case "offline":
			summary.Critical++
			summary.RecentAlerts = append(summary.RecentAlerts, Alert{
				Level:     "critical",
				Message:   fmt.Sprintf("存储池 %s 离线", p.Name),
				Source:    p.Name,
				Time:      now,
			})
		}

		// 检查池容量
		if p.TotalBytes > 0 {
			poolUtil := float64(p.UsedBytes) / float64(p.TotalBytes)
			if poolUtil > 0.95 {
				summary.Critical++
				summary.RecentAlerts = append(summary.RecentAlerts, Alert{
					Level:     "critical",
					Message:   fmt.Sprintf("存储池 %s 容量使用率 %.1f%%，即将满", p.Name, poolUtil*100),
					Source:    p.Name,
					Time:      now,
				})
			} else if poolUtil > 0.85 {
				summary.Warning++
				summary.RecentAlerts = append(summary.RecentAlerts, Alert{
					Level:     "warning",
					Message:   fmt.Sprintf("存储池 %s 容量使用率 %.1f%%", p.Name, poolUtil*100),
					Source:    p.Name,
					Time:      now,
				})
			}
		}
	}

	// 检查分层迁移
	for _, t := range overview.Tiers {
		if t.MigrationPending > 100 {
			summary.Warning++
			summary.RecentAlerts = append(summary.RecentAlerts, Alert{
				Level:     "warning",
				Message:   fmt.Sprintf("分层 %s 有 %d 个待迁移任务", t.Tier, t.MigrationPending),
				Source:    t.Tier,
				Time:      now,
			})
		}
	}

	// 限制最近告警数量
	if len(summary.RecentAlerts) > 50 {
		summary.RecentAlerts = summary.RecentAlerts[:50]
	}

	return summary
}

// RefreshCache 强制刷新缓存
func (d *Dashboard) RefreshCache() error {
	d.mu.Lock()
	d.cachedOverview = nil
	d.cachedTrends = nil
	d.cachedAlerts = nil
	d.cacheTime = time.Time{}
	d.mu.Unlock()

	// 重新拉取数据填充缓存
	_, err := d.fetchOverview()
	if err != nil {
		return fmt.Errorf("刷新缓存失败: %w", err)
	}

	d.logger.Info("缓存已刷新")
	return nil
}
