// Package smartpredict 提供基于 AI 的磁盘故障预测与智能维护建议
// 对标 TrueNAS SMART 监控 + 群晖存储健康分析，增加 AI 预测能力
package smartpredict

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// SMARTAttribute SMART 属性.
type SMARTAttribute struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Value     int    `json:"value"`
	Worst     int    `json:"worst"`
	Threshold int    `json:"threshold"`
	RawValue  int64  `json:"raw_value"`
	Failed    bool   `json:"failed"`
	Critical  bool   `json:"critical"`
}

// DiskHealth 磁盘健康状态.
type DiskHealth struct {
	Device       string           `json:"device"`
	Model        string           `json:"model"`
	Serial       string           `json:"serial"`
	TBW          int64            `json:"tbw"`         // 已写入 TB
	TBWLimited   int64            `json:"tbw_limited"` // TBW 上限
	PowerOnHours int64            `json:"power_on_hours"`
	Temperature  int              `json:"temperature"`
	Attributes   []SMARTAttribute `json:"attributes"`
	HealthScore  float64          `json:"health_score"` // 0-100
	FailProb     float64          `json:"fail_prob"`    // 故障概率 0-1
	Prediction   *Prediction      `json:"prediction,omitempty"`
}

// Prediction 预测结果.
type Prediction struct {
	EstimatedFailDate *time.Time `json:"estimated_fail_date,omitempty"`
	Confidence        float64    `json:"confidence"` // 0-1
	RiskLevel         string     `json:"risk_level"` // low/medium/high/critical
	Factors           []string   `json:"factors"`
	Recommendations   []string   `json:"recommendations"`
	RemainingLifeDays int        `json:"remaining_life_days"`
}

// PredictConfig 预测配置.
type PredictConfig struct {
	CheckInterval     time.Duration `json:"check_interval"`
	TemperatureWarn   int           `json:"temperature_warn"`
	TemperatureCrit   int           `json:"temperature_crit"`
	ReallocatedThresh int           `json:"reallocated_thresh"`
	PendingThresh     int           `json:"pending_thresh"`
	UNCThresh         int           `json:"unc_thresh"`
	TBWWarningPct     float64       `json:"tbw_warning_pct"` // TBW 使用百分比警告
}

// DefaultConfig 默认配置.
func DefaultConfig() *PredictConfig {
	return &PredictConfig{
		CheckInterval:     1 * time.Hour,
		TemperatureWarn:   55,
		TemperatureCrit:   65,
		ReallocatedThresh: 10,
		PendingThresh:     5,
		UNCThresh:         1,
		TBWWarningPct:     80.0,
	}
}

// Manager 管理器.
type Manager struct {
	config    *PredictConfig
	disks     map[string]*DiskHealth
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	alertFunc func(device string, prediction *Prediction)
}

// NewManager 创建管理器.
func NewManager(config *PredictConfig) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config: config,
		disks:  make(map[string]*DiskHealth),
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetAlertFunc 设置告警回调.
func (m *Manager) SetAlertFunc(fn func(device string, prediction *Prediction)) {
	m.alertFunc = fn
}

// Start 启动监控.
func (m *Manager) Start() {
	go m.monitorLoop()
}

// Stop 停止监控.
func (m *Manager) Stop() {
	m.cancel()
}

// UpdateDisk 更新磁盘健康数据.
func (m *Manager) UpdateDisk(health *DiskHealth) {
	m.mu.Lock()
	defer m.mu.Unlock()

	health.HealthScore = m.calculateHealthScore(health)
	health.FailProb = m.calculateFailProb(health)
	health.Prediction = m.predict(health)
	m.disks[health.Device] = health

	if m.alertFunc != nil && health.Prediction != nil && health.Prediction.RiskLevel == "critical" {
		m.alertFunc(health.Device, health.Prediction)
	}
}

// GetDisk 获取磁盘健康信息.
func (m *Manager) GetDisk(device string) (*DiskHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.disks[device]
	return d, ok
}

// GetAllDisks 获取所有磁盘.
func (m *Manager) GetAllDisks() map[string]*DiskHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*DiskHealth, len(m.disks))
	for k, v := range m.disks {
		result[k] = v
	}
	return result
}

// calculateHealthScore 计算健康评分 (0-100).
func (m *Manager) calculateHealthScore(h *DiskHealth) float64 {
	score := 100.0

	// 温度影响
	if h.Temperature >= m.config.TemperatureCrit {
		score -= 30
	} else if h.Temperature >= m.config.TemperatureWarn {
		score -= 15
	}

	// SMART 属性影响
	for _, attr := range h.Attributes {
		if attr.Failed {
			score -= 40
			break
		}
		switch attr.ID {
		case 5: // Reallocated Sectors
			if attr.RawValue > 0 {
				score -= math.Min(float64(attr.RawValue)*2, 30)
			}
		case 187: // Reported Uncorrectable
			if attr.RawValue > 0 {
				score -= math.Min(float64(attr.RawValue)*3, 25)
			}
		case 188: // Command Timeout
			if attr.RawValue > 0 {
				score -= math.Min(float64(attr.RawValue)*1.5, 20)
			}
		case 197: // Current Pending Sector
			if attr.RawValue > 0 {
				score -= math.Min(float64(attr.RawValue)*2, 25)
			}
		case 198: // Offline Uncorrectable
			if attr.RawValue > 0 {
				score -= math.Min(float64(attr.RawValue)*2, 25)
			}
		}
	}

	// TBW 影响
	if h.TBWLimited > 0 {
		tbwPct := float64(h.TBW) / float64(h.TBWLimited) * 100
		if tbwPct > 90 {
			score -= 25
		} else if tbwPct > m.config.TBWWarningPct {
			score -= 15
		}
	}

	// 通电时间影响
	if h.PowerOnHours > 50000 { // ~5.7 年
		score -= 15
	} else if h.PowerOnHours > 30000 { // ~3.4 年
		score -= 8
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

// calculateFailProb 计算故障概率.
func (m *Manager) calculateFailProb(h *DiskHealth) float64 {
	prob := 0.0

	// 基于 SMART 属性
	for _, attr := range h.Attributes {
		if attr.Failed {
			prob = math.Max(prob, 0.95)
		}
		switch attr.ID {
		case 5: // Reallocated
			if attr.RawValue > 0 {
				prob = math.Max(prob, math.Min(float64(attr.RawValue)*0.05, 0.8))
			}
		case 197: // Pending
			if attr.RawValue > 0 {
				prob = math.Max(prob, math.Min(float64(attr.RawValue)*0.04, 0.7))
			}
		}
	}

	// 温度因素
	if h.Temperature >= m.config.TemperatureCrit {
		prob = math.Max(prob, 0.4)
	}

	// TBW 因素
	if h.TBWLimited > 0 {
		tbwPct := float64(h.TBW) / float64(h.TBWLimited)
		if tbwPct > 0.95 {
			prob = math.Max(prob, 0.6)
		}
	}

	return math.Round(prob*1000) / 1000
}

// predict 生成预测.
func (m *Manager) predict(h *DiskHealth) *Prediction {
	if h.HealthScore > 80 && h.FailProb < 0.1 {
		return &Prediction{
			RiskLevel:         "low",
			Confidence:        0.9,
			RemainingLifeDays: 365 * 3,
			Factors:           []string{"磁盘状态良好"},
			Recommendations:   []string{"继续定期监控"},
		}
	}

	pred := &Prediction{
		Confidence: 0.75,
	}

	// 评估风险等级
	switch {
	case h.HealthScore < 30 || h.FailProb > 0.7:
		pred.RiskLevel = "critical"
		pred.Confidence = 0.85
	case h.HealthScore < 50 || h.FailProb > 0.4:
		pred.RiskLevel = "high"
		pred.Confidence = 0.8
	case h.HealthScore < 70 || h.FailProb > 0.2:
		pred.RiskLevel = "medium"
	default:
		pred.RiskLevel = "low"
	}

	// 收集风险因素
	pred.Factors = m.collectFactors(h)
	pred.Recommendations = m.generateRecommendations(h, pred.RiskLevel)

	// 估算剩余寿命
	pred.RemainingLifeDays = m.estimateRemainingDays(h)
	if pred.RemainingLifeDays > 0 {
		t := time.Now().AddDate(0, 0, pred.RemainingLifeDays)
		pred.EstimatedFailDate = &t
	}

	return pred
}

// collectFactors 收集风险因素.
func (m *Manager) collectFactors(h *DiskHealth) []string {
	var factors []string

	for _, attr := range h.Attributes {
		switch attr.ID {
		case 5:
			if attr.RawValue > 0 {
				factors = append(factors, fmt.Sprintf("重分配扇区: %d", attr.RawValue))
			}
		case 197:
			if attr.RawValue > 0 {
				factors = append(factors, fmt.Sprintf("待处理扇区: %d", attr.RawValue))
			}
		case 198:
			if attr.RawValue > 0 {
				factors = append(factors, fmt.Sprintf("离线不可修复扇区: %d", attr.RawValue))
			}
		}
	}

	if h.Temperature >= m.config.TemperatureWarn {
		factors = append(factors, fmt.Sprintf("温度过高: %d°C", h.Temperature))
	}

	if h.TBWLimited > 0 {
		pct := float64(h.TBW) / float64(h.TBWLimited) * 100
		if pct > m.config.TBWWarningPct {
			factors = append(factors, fmt.Sprintf("TBW 使用率: %.1f%%", pct))
		}
	}

	if len(factors) == 0 {
		factors = append(factors, "未发现明显风险因素")
	}
	return factors
}

// generateRecommendations 生成建议.
func (m *Manager) generateRecommendations(h *DiskHealth, riskLevel string) []string {
	var recs []string

	switch riskLevel {
	case "critical":
		recs = append(recs, "⚠️ 立即备份重要数据")
		recs = append(recs, "🔄 尽快更换磁盘")
		recs = append(recs, "📋 联系供应商进行质保")
	case "high":
		recs = append(recs, "📦 尽快安排数据备份")
		recs = append(recs, "🔍 增加监控频率")
		recs = append(recs, "📋 准备替换磁盘")
	case "medium":
		recs = append(recs, "📊 持续监控 SMART 数据")
		recs = append(recs, "🌡️ 检查散热条件")
		recs = append(recs, "📅 计划预防性维护")
	default:
		recs = append(recs, "✅ 继续定期检查")
	}

	if h.Temperature >= m.config.TemperatureWarn {
		recs = append(recs, "🌡️ 改善机箱散热")
	}

	return recs
}

// estimateRemainingDays 估算剩余寿命（天）.
func (m *Manager) estimateRemainingDays(h *DiskHealth) int {
	days := 365 * 5 // 默认 5 年

	// 基于通电时间
	if h.PowerOnHours > 0 {
		// 假设总寿命 50000 小时
		remainingHours := 50000 - h.PowerOnHours
		if remainingHours < 0 {
			return 30
		}
		days = int(remainingHours / 24)
	}

	// 基于健康评分调整
	factor := h.HealthScore / 100.0
	if factor < 0.1 {
		factor = 0.1
	}
	days = int(float64(days) * factor)

	// 基于故障概率调整
	if h.FailProb > 0.5 {
		days = days / 3
	} else if h.FailProb > 0.3 {
		days = days / 2
	}

	if days < 1 {
		days = 1
	}
	return days
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll 检查所有磁盘.
func (m *Manager) checkAll() {
	m.mu.RLock()
	disks := make(map[string]*DiskHealth, len(m.disks))
	for k, v := range m.disks {
		disks[k] = v
	}
	m.mu.RUnlock()

	for _, disk := range disks {
		if disk.Prediction != nil && disk.Prediction.RiskLevel == "critical" && m.alertFunc != nil {
			m.alertFunc(disk.Device, disk.Prediction)
		}
	}
}

// GetRiskSummary 获取风险摘要.
func (m *Manager) GetRiskSummary() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[string]int{
		"low": 0, "medium": 0, "high": 0, "critical": 0,
	}
	for _, d := range m.disks {
		if d.Prediction != nil {
			summary[d.Prediction.RiskLevel]++
		}
	}
	return summary
}
