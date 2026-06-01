package storageml

import (
	"sync"
	"time"
)

// Collector collects storage metrics from the system
type Collector struct {
	ml        *StorageML
	mu        sync.RWMutex
	stopCh    chan struct{}
	interval  time.Duration
	running   bool
}

// NewCollector creates a new metric collector
func NewCollector(ml *StorageML, interval time.Duration) *Collector {
	return &Collector{
		ml:       ml,
		stopCh:   make(chan struct{}),
		interval: interval,
	}
}

// Start begins collecting metrics
func (c *Collector) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go c.collectLoop()
}

// Stop stops the collector
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	close(c.stopCh)
	c.running = false
}

// collectLoop runs the collection loop
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectAllPools()
		}
	}
}

// collectAllPools collects metrics for all configured pools
func (c *Collector) collectAllPools() {
	c.ml.mu.RLock()
	pools := make([]string, 0, len(c.ml.poolConfigs))
	for poolID := range c.ml.poolConfigs {
		pools = append(pools, poolID)
	}
	c.ml.mu.RUnlock()

	for _, poolID := range pools {
		c.collectPool(poolID)
	}
}

// collectPool collects metrics for a specific pool
func (c *Collector) collectPool(poolID string) {
	now := time.Now()

	// Collect capacity metric
	capacity := c.getPoolCapacity(poolID)
	c.ml.AddDataPoint(DataPoint{
		Timestamp: now,
		Value:     capacity,
		Type:      MetricCapacity,
		PoolID:    poolID,
	})

	// Collect IOPS metric
	iops := c.getPoolIOPS(poolID)
	c.ml.AddDataPoint(DataPoint{
		Timestamp: now,
		Value:     iops,
		Type:      MetricIOPS,
		PoolID:    poolID,
	})

	// Collect throughput metric
	throughput := c.getPoolThroughput(poolID)
	c.ml.AddDataPoint(DataPoint{
		Timestamp: now,
		Value:     throughput,
		Type:      MetricThroughput,
		PoolID:    poolID,
	})
}

// getPoolCapacity returns current pool capacity usage in GB
func (c *Collector) getPoolCapacity(poolID string) float64 {
	// In production, this would query the actual storage system
	c.ml.mu.RLock()
	config, exists := c.ml.poolConfigs[poolID]
	c.ml.mu.RUnlock()
	if !exists {
		return 0
	}
	// Simulate gradual capacity increase
	return config.TotalCapacity * 0.6 // 60% usage
}

// getPoolIOPS returns current pool IOPS
func (c *Collector) getPoolIOPS(poolID string) float64 {
	// In production, this would query the actual storage system
	return 1000.0 // Placeholder
}

// getPoolThroughput returns current pool throughput in MB/s
func (c *Collector) getPoolThroughput(poolID string) float64 {
	// In production, this would query the actual storage system
	return 500.0 // Placeholder
}

// CollectManual manually collects a data point
func (c *Collector) CollectManual(poolID string, metricType MetricType, value float64) {
	c.ml.AddDataPoint(DataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Type:      metricType,
		PoolID:    poolID,
	})
}
