// Package scrubsched 提供智能Scrub调度功能
package scrubsched

import (
	"log"
	"sync"
	"time"
)

// ========== IO负载分析器 ==========

// IOAnalyzer IO负载分析器，负责IO负载监控和高峰检测.
type IOAnalyzer struct {
	manager       *Manager
	peakPatterns  map[string]*PeakPattern // poolID -> 高峰模式
	mu            sync.RWMutex
}

// PeakPattern 高峰模式数据.
type PeakPattern struct {
	PoolID       string           `json:"pool_id"`       // 存储池ID
	HourlyAvg    [24]float64      `json:"hourly_avg"`    // 每小时平均IOPS
	DailyPattern [7][24]float64   `json:"daily_pattern"` // 按星期几的每小时模式
	LastUpdated  time.Time        `json:"last_updated"`  // 最后更新时间
	SampleCount  int              `json:"sample_count"`  // 样本数
	IsLearned    bool             `json:"is_learned"`    // 是否已完成学习
}

// NewIOAnalyzer 创建IO分析器.
func NewIOAnalyzer(mgr *Manager) *IOAnalyzer {
	return &IOAnalyzer{
		manager:      mgr,
		peakPatterns: make(map[string]*PeakPattern),
	}
}

// Start 启动IO分析器.
func (a *IOAnalyzer) Start() {
	// IO采集间隔
	collectTicker := time.NewTicker(10 * time.Second)
	defer collectTicker.Stop()

	// 模式分析间隔
	analyzeTicker := time.NewTicker(5 * time.Minute)
	defer analyzeTicker.Stop()

	// IO阈值检查间隔
	checkTicker := time.NewTicker(5 * time.Second)
	defer checkTicker.Stop()

	log.Println("[scrubsched] IO分析器已启动")

	for {
		select {
		case <-a.manager.stopCh:
			log.Println("[scrubsched] IO分析器已停止")
			return
		case <-collectTicker.C:
			a.collectIOData()
		case <-analyzeTicker.C:
			a.analyzePatterns()
		case <-checkTicker.C:
			a.checkIOThresholds()
		}
	}
}

// collectIOData 采集IO数据.
func (a *IOAnalyzer) collectIOData() {
	if a.manager.ioCollector == nil {
		return
	}

	loads, err := a.manager.ioCollector.CollectAllIOLoad()
	if err != nil {
		log.Printf("[scrubsched] IO采集失败: %v", err)
		return
	}

	for poolID, load := range loads {
		load.Timestamp = time.Now()
		a.manager.addIORecord(poolID, load)
	}
}

// analyzePatterns 分析IO模式.
func (a *IOAnalyzer) analyzePatterns() {
	a.manager.mu.RLock()
	ioHistory := make(map[string][]*IOLoad)
	for k, v := range a.manager.ioHistory {
		cpy := make([]*IOLoad, len(v))
		copy(cpy, v)
		ioHistory[k] = cpy
	}
	a.manager.mu.RUnlock()

	for poolID, records := range ioHistory {
		if len(records) < 100 {
			// 样本太少，不进行分析
			continue
		}

		a.mu.Lock()
		pattern, ok := a.peakPatterns[poolID]
		if !ok {
			pattern = &PeakPattern{
				PoolID: poolID,
			}
			a.peakPatterns[poolID] = pattern
		}

		// 计算每小时平均IOPS
		var hourlySum [24]float64
		var hourlyCount [24]int
		// 按星期和小时
		var dailySum [7][24]float64
		var dailyCount [7][24]int

		for _, r := range records {
			hour := r.Timestamp.Hour()
			weekday := int(r.Timestamp.Weekday())
			iops := float64(r.IOPS)

			hourlySum[hour] += iops
			hourlyCount[hour]++
			dailySum[weekday][hour] += iops
			dailyCount[weekday][hour]++
		}

		for h := 0; h < 24; h++ {
			if hourlyCount[h] > 0 {
				pattern.HourlyAvg[h] = hourlySum[h] / float64(hourlyCount[h])
			}
			for d := 0; d < 7; d++ {
				if dailyCount[d][h] > 0 {
					pattern.DailyPattern[d][h] = dailySum[d][h] / float64(dailyCount[d][h])
				}
			}
		}

		pattern.LastUpdated = time.Now()
		pattern.SampleCount = len(records)
		if pattern.SampleCount >= 168 { // 至少一周的数据
			pattern.IsLearned = true
		}

		a.mu.Unlock()
	}
}

// checkIOThresholds 检查IO阈值.
func (a *IOAnalyzer) checkIOThresholds() {
	a.manager.mu.RLock()
	// 复制运行中的状态
	type poolState struct {
		status *ScrubStatus
		policy *Policy
	}
	running := make(map[string]*poolState)
	for poolID, status := range a.manager.statuses {
		if status.State == StateRunning || status.State == StatePaused {
			ps := &poolState{status: status}
			if status.PolicyID != "" {
				if p, ok := a.manager.policies[status.PolicyID]; ok {
					ps.policy = p
				}
			}
			running[poolID] = ps
		}
	}
	a.manager.mu.RUnlock()

	// 获取当前IO负载
	if a.manager.ioCollector == nil {
		return
	}

	loads, err := a.manager.ioCollector.CollectAllIOLoad()
	if err != nil {
		return
	}

	for poolID, ps := range running {
		load, ok := loads[poolID]
		if !ok {
			continue
		}

		if ps.policy == nil || !ps.policy.AvoidPeak {
			continue
		}

		threshold := ps.policy.IOThreshold

		// 检查IO是否超过阈值
		overload := false
		var reason string

		if threshold.IOPSMax > 0 && load.IOPS > threshold.IOPSMax {
			overload = true
			reason = "IOPS超过阈值"
		}
		if threshold.BandwidthMax > 0 && load.Bandwidth > threshold.BandwidthMax {
			overload = true
			reason = "带宽超过阈值"
		}
		if threshold.LatencyMax > 0 && load.Latency > threshold.LatencyMax {
			overload = true
			reason = "延迟超过阈值"
		}

		if overload && ps.status.State == StateRunning {
			log.Printf("[scrubsched] 池 %s IO超载（%s），暂停Scrub", poolID, reason)
			_ = a.manager.PauseScrub(poolID, "IO负载过高: "+reason)
		} else if !overload && ps.status.State == StatePaused {
			// 检查是否满足恢复条件
			resumeOK := true
			if threshold.IOPSMax > 0 {
				resumeThreshold := float64(threshold.IOPSMax) * threshold.ResumeRatio
				if float64(load.IOPS) > resumeThreshold {
					resumeOK = false
				}
			}
			if threshold.BandwidthMax > 0 {
				resumeThreshold := threshold.BandwidthMax * threshold.ResumeRatio
				if load.Bandwidth > resumeThreshold {
					resumeOK = false
				}
			}

			if resumeOK && ps.status.PauseReason != "避峰调度：进入业务高峰时段" {
				log.Printf("[scrubsched] 池 %s IO恢复正常，恢复Scrub", poolID)
				_ = a.manager.ResumeScrub(poolID)
			}
		}
	}
}

// GetPeakPattern 获取存储池的IO高峰模式.
func (a *IOAnalyzer) GetPeakPattern(poolID string) *PeakPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()

	p, ok := a.peakPatterns[poolID]
	if !ok {
		return nil
	}
	return p
}

// GetIsPeakHour 判断指定小时是否为高峰时段.
func (a *IOAnalyzer) GetIsPeakHour(poolID string, hour int) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	p, ok := a.peakPatterns[poolID]
	if !ok || !p.IsLearned {
		return false
	}

	// 计算全天平均
	var totalAvg float64
	for h := 0; h < 24; h++ {
		totalAvg += p.HourlyAvg[h]
	}
	totalAvg /= 24

	// 如果该小时IOPS高于平均值1.5倍，认为是高峰
	return p.HourlyAvg[hour] > totalAvg*1.5
}
