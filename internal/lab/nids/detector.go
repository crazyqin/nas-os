package nids

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Detector 入侵检测引擎.
type Detector struct {
	mu          sync.RWMutex
	mgr         *Manager
	running     bool
	packetCount int64
	attackCount int64
	// thresholdTracker 阈值跟踪器: ruleID -> trackKey -> []time.Time
	thresholdTracker map[string]map[string][]time.Time
	// portScanTracker 端口扫描跟踪: srcIP -> map[dstPort]time.Time
	portScanTracker map[string]map[int]time.Time
}

// NewDetector 创建检测引擎.
func NewDetector(mgr *Manager) *Detector {
	return &Detector{
		mgr:              mgr,
		thresholdTracker: make(map[string]map[string][]time.Time),
		portScanTracker:  make(map[string]map[int]time.Time),
	}
}

// Start 启动检测引擎.
func (d *Detector) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		return
	}
	d.running = true
	log.Println("[NIDS] 检测引擎启动")
}

// Stop 停止检测引擎.
func (d *Detector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	log.Println("[NIDS] 检测引擎停止")
}

// IsRunning 返回引擎运行状态.
func (d *Detector) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// AnalyzePacket 分析数据包，返回触发的告警列表.
func (d *Detector) AnalyzePacket(pkt *PacketInfo) []*Alert {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.packetCount++
	var alerts []*Alert

	// 检查白名单
	if d.mgr.isIPWhitelisted(pkt.SrcIP.String()) {
		return nil
	}

	// 检查黑名单
	if d.mgr.isIPBlacklisted(pkt.SrcIP.String()) {
		alert := d.createBlacklistAlert(pkt)
		alerts = append(alerts, alert)
		return alerts
	}

	// 签名检测
	for _, rule := range d.mgr.rules {
		if rule.Type != DetectionSignature && rule.Type != DetectionBoth {
			continue
		}
		if MatchRule(rule, pkt) {
			alert := d.createAlert(rule, pkt)
			alerts = append(alerts, alert)
			rule.HitCount++
		}
	}

	// 异常检测
	anomalyAlerts := d.detectAnomalies(pkt)
	alerts = append(alerts, anomalyAlerts...)

	// 处理告警
	for _, alert := range alerts {
		d.attackCount++
		d.mgr.processAlert(alert)
	}

	return alerts
}

// detectAnomalies 异常检测.
func (d *Detector) detectAnomalies(pkt *PacketInfo) []*Alert {
	var alerts []*Alert

	// 检查异常流量基线
	if baseline, ok := d.mgr.baselines[pkt.Protocol]; ok {
		if baseline.AvgPPS > 0 {
			currentPPS := d.getCurrentPPS(pkt.Protocol)
			if currentPPS > baseline.AvgPPS*d.mgr.config.AnomalyFactor {
				alert := &Alert{
					ID:       fmt.Sprintf("alert_%d", d.mgr.nextAlertID()),
					RuleID:   "anomaly-pps",
					RuleName: "Traffic Anomaly - PPS Spike",
					Severity: SeverityHigh,
					Status:   AlertOpen,
					Action:   ActionAlert,
					SrcIP:    pkt.SrcIP,
					DstIP:    pkt.DstIP,
					Protocol: pkt.Protocol,
					Description: fmt.Sprintf("PPS %.0f exceeds baseline %.0f by %.1fx",
						currentPPS, baseline.AvgPPS, d.mgr.config.AnomalyFactor),
					FirstSeen: time.Now(),
					LastSeen:  time.Now(),
				}
				alerts = append(alerts, alert)
			}
		}
	}

	// 阈值规则检测
	for _, rule := range d.mgr.rules {
		if rule.Threshold == nil {
			continue
		}
		if rule.Type != DetectionAnomaly && rule.Type != DetectionBoth {
			continue
		}
		if !MatchRule(rule, pkt) {
			continue
		}

		trackKey := d.getTrackKey(rule, pkt)
		if d.checkThreshold(rule, trackKey) {
			alert := d.createAlert(rule, pkt)
			alerts = append(alerts, alert)
			rule.HitCount++
		}
	}

	return alerts
}

// getTrackKey 获取阈值跟踪的 key.
func (d *Detector) getTrackKey(rule *Rule, pkt *PacketInfo) string {
	switch rule.Threshold.TrackBy {
	case "src":
		return pkt.SrcIP.String()
	case "dst":
		return pkt.DstIP.String()
	case "both":
		return pkt.SrcIP.String() + "->" + pkt.DstIP.String()
	default:
		return pkt.SrcIP.String()
	}
}

// checkThreshold 检查阈值是否触发.
func (d *Detector) checkThreshold(rule *Rule, trackKey string) bool {
	if d.thresholdTracker[rule.ID] == nil {
		d.thresholdTracker[rule.ID] = make(map[string][]time.Time)
	}

	now := time.Now()
	cutoff := now.Add(-time.Duration(rule.Threshold.Seconds) * time.Second)

	// 清理过期记录
	times := d.thresholdTracker[rule.ID][trackKey]
	validTimes := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}
	validTimes = append(validTimes, now)
	d.thresholdTracker[rule.ID][trackKey] = validTimes

	return len(validTimes) >= rule.Threshold.Count
}

// getCurrentPPS 获取当前 PPS（简化实现）.
func (d *Detector) getCurrentPPS(proto Protocol) float64 {
	// 简化：返回基于 packetCount 的估算
	if d.packetCount == 0 {
		return 0
	}
	return float64(d.packetCount)
}

// createAlert 创建告警.
func (d *Detector) createAlert(rule *Rule, pkt *PacketInfo) *Alert {
	return &Alert{
		ID:          fmt.Sprintf("alert_%d", d.mgr.nextAlertID()),
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Severity:    rule.Severity,
		Status:      AlertOpen,
		Action:      rule.Action,
		SrcIP:       pkt.SrcIP,
		DstIP:       pkt.DstIP,
		SrcPort:     pkt.SrcPort,
		DstPort:     pkt.DstPort,
		Protocol:    pkt.Protocol,
		Description: rule.Description,
		PacketInfo:  pkt,
		Count:       1,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
}

// createBlacklistAlert 创建黑名单告警.
func (d *Detector) createBlacklistAlert(pkt *PacketInfo) *Alert {
	return &Alert{
		ID:          fmt.Sprintf("alert_%d", d.mgr.nextAlertID()),
		RuleID:      "blacklist",
		RuleName:    "Blacklisted IP",
		Severity:    SeverityCritical,
		Status:      AlertOpen,
		Action:      ActionBlock,
		SrcIP:       pkt.SrcIP,
		DstIP:       pkt.DstIP,
		SrcPort:     pkt.SrcPort,
		DstPort:     pkt.DstPort,
		Protocol:    pkt.Protocol,
		Description: fmt.Sprintf("Traffic from blacklisted IP: %s", pkt.SrcIP),
		PacketInfo:  pkt,
		Count:       1,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
	}
}

// GetStats 获取检测器统计.
func (d *Detector) GetStats() (packets int64, attacks int64) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.packetCount, d.attackCount
}

// UpdateBaseline 更新流量基线.
func (d *Detector) UpdateBaseline(proto Protocol, pps, bps float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	baseline, ok := d.mgr.baselines[proto]
	if !ok {
		baseline = &TrafficBaseline{
			Protocol: proto,
		}
		d.mgr.baselines[proto] = baseline
	}

	// 移动平均
	if baseline.SampleCount == 0 {
		baseline.AvgPPS = pps
		baseline.AvgBPS = bps
	} else {
		alpha := 0.1 // EMA 系数
		baseline.AvgPPS = baseline.AvgPPS*(1-alpha) + pps*alpha
		baseline.AvgBPS = baseline.AvgBPS*(1-alpha) + bps*alpha
	}

	if pps > baseline.MaxPPS {
		baseline.MaxPPS = pps
	}
	if bps > baseline.MaxBPS {
		baseline.MaxBPS = bps
	}

	baseline.SampleCount++
	baseline.LastUpdate = time.Now()
}

// CleanupTrackers 清理过期的阈值跟踪数据.
func (d *Detector) CleanupTrackers() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for ruleID, trackers := range d.thresholdTracker {
		for key, times := range trackers {
			valid := make([]time.Time, 0)
			cutoff := time.Now().Add(-10 * time.Minute)
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(trackers, key)
			} else {
				trackers[key] = valid
			}
		}
		if len(trackers) == 0 {
			delete(d.thresholdTracker, ruleID)
		}
	}
}
