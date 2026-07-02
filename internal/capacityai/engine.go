package capacityai

import (
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// StoragePool 存储池信息.
type StoragePool struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	TotalBytes int64   `json:"totalBytes"`
	UsedBytes  int64   `json:"usedBytes"`
	FreeBytes  int64   `json:"freeBytes"`
	UsageRatio float64 `json:"usageRatio"`
	PoolType   string  `json:"poolType"` // single, mirror, raidz1, raidz2, raidz3
	Tier       string  `json:"tier"`     // nvme, ssd, hdd
}

// CapacityForecast 容量预测.
type CapacityForecast struct {
	PoolID         string    `json:"poolId"`
	PoolName       string    `json:"poolName"`
	CurrentUsage   float64   `json:"currentUsage"`
	PredictedUsage float64   `json:"predictedUsage"` // 30天后
	DaysUntilFull  int       `json:"daysUntilFull"`
	GrowthRateGB   float64   `json:"growthRateGB"` // 每日增长GB
	Recommendation string    `json:"recommendation"`
	Urgency        string    `json:"urgency"` // low, medium, high, critical
	ForecastTime   time.Time `json:"forecastTime"`
}

// CostOptimization 成本优化建议.
type CostOptimization struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"` // tier_migration, dedup, compress, expand
	Description   string  `json:"description"`
	EstimatedSave float64 `json:"estimatedSave"` // 预估节省金额
	Priority      string  `json:"priority"`
	Action        string  `json:"action"`
}

// CapacityAI AI容量规划与成本优化
// 对标 TrueNAS 成本效益分析 + 群晖存储分析.
type CapacityAI struct {
	mu            sync.RWMutex
	pools         map[string]*StoragePool
	forecasts     []CapacityForecast
	optimizations []CostOptimization
	growthHist    []GrowthSample
	stopCh        chan struct{}
	running       bool
}

type GrowthSample struct {
	Timestamp time.Time `json:"timestamp"`
	PoolID    string    `json:"poolId"`
	UsedBytes int64     `json:"usedBytes"`
}

// NewCapacityAI 创建容量AI.
func NewCapacityAI() *CapacityAI {
	return &CapacityAI{
		pools:         make(map[string]*StoragePool),
		forecasts:     make([]CapacityForecast, 0),
		optimizations: make([]CostOptimization, 0),
		growthHist:    make([]GrowthSample, 0),
		stopCh:        make(chan struct{}),
	}
}

// Start 启动.
func (c *CapacityAI) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	go c.analysisLoop()
	log.Println("[CapacityAI] AI容量规划已启动")
}

// Stop 停止.
func (c *CapacityAI) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	close(c.stopCh)
	c.running = false
	log.Println("[CapacityAI] AI容量规划已停止")
}

// RegisterPool 注册存储池.
func (c *CapacityAI) RegisterPool(pool StoragePool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pool.FreeBytes = pool.TotalBytes - pool.UsedBytes
	if pool.TotalBytes > 0 {
		pool.UsageRatio = float64(pool.UsedBytes) / float64(pool.TotalBytes)
	}
	c.pools[pool.ID] = &pool
}

// RecordUsage 记录使用量.
func (c *CapacityAI) RecordUsage(poolID string, usedBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.pools[poolID]; ok {
		p.UsedBytes = usedBytes
		p.FreeBytes = p.TotalBytes - usedBytes
		if p.TotalBytes > 0 {
			p.UsageRatio = float64(usedBytes) / float64(p.TotalBytes)
		}
		c.growthHist = append(c.growthHist, GrowthSample{
			Timestamp: time.Now(),
			PoolID:    poolID,
			UsedBytes: usedBytes,
		})
		// 保留最近 90 天数据
		cutoff := time.Now().Add(-90 * 24 * time.Hour)
		filtered := make([]GrowthSample, 0)
		for _, s := range c.growthHist {
			if s.Timestamp.After(cutoff) {
				filtered = append(filtered, s)
			}
		}
		c.growthHist = filtered
	}
}

// analyze 执行分析.
func (c *CapacityAI) analyze() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.forecasts = make([]CapacityForecast, 0)
	c.optimizations = make([]CostOptimization, 0)

	for _, pool := range c.pools {
		forecast := c.forecastPool(pool)
		c.forecasts = append(c.forecasts, forecast)
		c.generateOptimizations(pool)
	}

	sort.Slice(c.forecasts, func(i, j int) bool {
		return urgencyOrder(c.forecasts[i].Urgency) < urgencyOrder(c.forecasts[j].Urgency)
	})
}

func (c *CapacityAI) forecastPool(pool *StoragePool) CapacityForecast {
	// 计算增长率
	growthRate := c.calculateGrowthRate(pool.ID)
	daysUntilFull := -1
	if growthRate > 0 {
		freeGB := float64(pool.FreeBytes) / (1024 * 1024 * 1024)
		daysUntilFull = int(freeGB / growthRate)
	}

	predicted := float64(pool.UsedBytes) + growthRate*30*(1024*1024*1024)
	if predicted > float64(pool.TotalBytes) {
		predicted = float64(pool.TotalBytes)
	}
	predictedRatio := predicted / float64(pool.TotalBytes)

	urgency := "low"
	rec := "存储空间充足"
	switch {
	case daysUntilFull >= 0 && daysUntilFull <= 7:
		urgency = "critical"
		rec = "紧急：建议立即扩容或迁移冷数据"
	case daysUntilFull >= 0 && daysUntilFull <= 30:
		urgency = "high"
		rec = "建议尽快扩容或启用数据分层"
	case daysUntilFull >= 0 && daysUntilFull <= 90:
		urgency = "medium"
		rec = "建议规划扩容方案"
	case pool.UsageRatio > 0.8:
		urgency = "medium"
		rec = "使用率超过80%，建议关注"
	}

	return CapacityForecast{
		PoolID:         pool.ID,
		PoolName:       pool.Name,
		CurrentUsage:   pool.UsageRatio,
		PredictedUsage: predictedRatio,
		DaysUntilFull:  daysUntilFull,
		GrowthRateGB:   growthRate,
		Recommendation: rec,
		Urgency:        urgency,
		ForecastTime:   time.Now(),
	}
}

func (c *CapacityAI) calculateGrowthRate(poolID string) float64 {
	var samples []GrowthSample
	for _, s := range c.growthHist {
		if s.PoolID == poolID {
			samples = append(samples, s)
		}
	}
	if len(samples) < 2 {
		return 0
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Timestamp.Before(samples[j].Timestamp) })
	first := samples[0]
	last := samples[len(samples)-1]
	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days <= 0 {
		return 0
	}
	growthBytes := float64(last.UsedBytes - first.UsedBytes)
	return growthBytes / (1024 * 1024 * 1024) / days
}

func (c *CapacityAI) generateOptimizations(pool *StoragePool) {
	if pool.UsageRatio > 0.7 {
		c.optimizations = append(c.optimizations, CostOptimization{
			ID:            "opt-" + pool.ID + "-tier",
			Type:          "tier_migration",
			Description:   "将冷数据迁移到低层级存储可释放 " + pool.Name + " 空间",
			EstimatedSave: float64(pool.UsedBytes) * 0.3 / (1024 * 1024 * 1024) * 0.1,
			Priority:      "medium",
			Action:        "启用智能分层引擎",
		})
	}
	if pool.PoolType == "raidz1" && pool.UsageRatio > 0.6 {
		c.optimizations = append(c.optimizations, CostOptimization{
			ID:          "opt-" + pool.ID + "-upgrade",
			Type:        "expand",
			Description: pool.Name + " 使用 RAIDZ1，建议升级到 RAIDZ2 提高安全性",
			Priority:    "low",
			Action:      "规划存储池升级",
		})
	}
}

func urgencyOrder(u string) int {
	switch u {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func (c *CapacityAI) analysisLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.analyze()
		case <-c.stopCh:
			return
		}
	}
}

// GetForecasts 获取预测.
func (c *CapacityAI) GetForecasts() []CapacityForecast {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.forecasts
}

// GetOptimizations 获取优化建议.
func (c *CapacityAI) GetOptimizations() []CostOptimization {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.optimizations
}

// GetPools 获取存储池.
func (c *CapacityAI) GetPools() []StoragePool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]StoragePool, 0, len(c.pools))
	for _, p := range c.pools {
		result = append(result, *p)
	}
	return result
}

// suppress unused import.
var _ = math.Log
